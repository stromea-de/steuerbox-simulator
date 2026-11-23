package inspection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type preservingDevice struct {
	onsiteFake
	writes      []string
	missing     bool
	failRestore bool
	// R5-03 (236c): the device holds what was written; the cleanup reads before it
	// writes, so the fake must report the test values after Configure.
	held map[string]FailsafeValues
}

func (f *preservingDevice) Configure(_ context.Context, c Config) error {
	f.calls = append(f.calls, "configure")
	f.held = map[string]FailsafeValues{"LPC": {Power: c.Failsafe, Minimum: c.FailsafeMinimum}, "LPP": {Power: c.Failsafe, Minimum: c.FailsafeMinimum}}
	return nil
}
func (f *preservingDevice) ReadFailsafe(_ context.Context, k string) (FailsafeValues, error) {
	f.calls = append(f.calls, "read-"+k)
	if f.missing {
		return FailsafeValues{}, errors.New("unlesbar")
	}
	if v, ok := f.held[k]; ok {
		return v, nil
	}
	return FailsafeValues{Power: 777, Minimum: 3 * time.Hour}, nil
}
func (f *preservingDevice) WriteFailsafe(ctx context.Context, k, field string, v FailsafeValues) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.writes = append(f.writes, k+"-"+field)
	if v.Power != 777 || v.Minimum != 3*time.Hour {
		return errors.New("falsche Ursprungswerte")
	}
	if f.failRestore {
		return errors.New("Rückschreiben abgewiesen")
	}
	if f.held != nil {
		cur := f.held[k]
		if field == "power" {
			cur.Power = v.Power
		} else {
			cur.Minimum = v.Minimum
		}
		f.held[k] = cur
	}
	return nil
}
func TestStandardLPPOnlyPreservesFailsafeWithoutLongWait(t *testing.T) {
	f := &preservingDevice{onsiteFake: onsiteFake{caps: Capabilities{LPP: true, MPC: true}, fake: fake{powers: []float64{-8000, -3000, -500, -8000}}}}
	start := time.Now()
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil || time.Since(start) > time.Second {
		t.Fatal(err, r)
	}
	for _, s := range r.Steps {
		if !s.Passed && s.Status != NotChecked {
			t.Fatal(s)
		}
	}
	if strings.Join(f.calls[:4], ",") != "connect,capabilities,read-LPP,heartbeat" {
		t.Fatal("snapshot must precede writes", f.calls)
	}
	if len(f.writes) != 2 || strings.Join(f.writes, ",") != "LPP-power,LPP-minimum" {
		t.Fatal(f.writes)
	}
	found := false
	for _, s := range r.Steps {
		if s.Name == "LPP Wiederkehr" {
			found = s.Status == NotChecked && strings.Contains(s.Detail, "Langtest")
		}
	}
	if !found {
		t.Fatal(r.Steps)
	}
}
func TestMissingOriginalsDoNotOverwriteUnknownFailsafe(t *testing.T) {
	f := &preservingDevice{missing: true, onsiteFake: onsiteFake{caps: Capabilities{LPC: true}}}
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil || len(f.writes) != 0 || strings.Contains(strings.Join(f.calls, ","), "configure") || strings.Contains(strings.Join(f.calls, ","), "heartbeat") {
		t.Fatal(err, f.writes)
	}
	found := false
	for _, s := range r.Steps {
		if s.Name == "Failsafe-Werte setzen" && s.Passed {
			t.Fatal(s)
		}
		if strings.Contains(s.Detail, "Notbetriebs-Werte am Gerät prüfen") {
			found = true
		}
	}
	if !found {
		t.Fatal(r)
	}
}
func TestRestoreFailuresAreSeparateAndDoNotSkipSecondField(t *testing.T) {
	f := &preservingDevice{failRestore: true, onsiteFake: onsiteFake{caps: Capabilities{LPC: true, LPP: true}}}
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil || len(f.writes) != 4 {
		t.Fatal(err, f.writes)
	}
	failures := 0
	for _, s := range r.Steps {
		if strings.Contains(s.Name, "zurückstellen") && !s.Passed {
			failures++
		}
	}
	if failures != 4 {
		t.Fatal(r.Steps)
	}
}
func TestDetectionCannotExceed120Seconds(t *testing.T) {
	c := onsiteConfig()
	c.FailsafeTimeout = 121 * time.Second
	if c.ValidateDevice() == nil {
		t.Fatal("unbounded outage")
	}
}

type longTestDevice struct {
	preservingDevice
	cancel context.CancelFunc
}

func (f *longTestDevice) StepStarted(name string) {
	if name == "LPP Wiederkehr" {
		go func() { time.Sleep(10 * time.Millisecond); f.cancel() }()
	}
}
func TestLongTestWaitIsCancellableAndRestoresValues(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &longTestDevice{preservingDevice: preservingDevice{onsiteFake: onsiteFake{caps: Capabilities{LPP: true, MPC: true}, fake: fake{powers: []float64{-8000, -3000, -500}}}}, cancel: cancel}
	c := onsiteConfig()
	c.LongTest = true
	c.RecoveryTimeout = c.FailsafeMinimum + time.Minute
	r, err := RunOnSite(ctx, c, Device{}, f)
	if err != nil || len(f.writes) != 2 {
		t.Fatal(err, r)
	}
	found := false
	for _, s := range r.Steps {
		if s.Name == "LPP Wiederkehr" {
			found = !s.Passed && s.Status == NotChecked && strings.Contains(s.Detail, "Prüfung abgebrochen")
		}
	}
	if !found {
		t.Fatal("long test must wait rather than claim success", r.Steps)
	}
}
func TestOptionalEarlyRecoveryIsNotInferredFromAcknowledgement(t *testing.T) {
	f := &preservingDevice{onsiteFake: onsiteFake{caps: Capabilities{LPP: true, MPC: true}, fake: fake{powers: []float64{-8000, -3000, -500, -500}}}}
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range r.Steps {
		if s.Name == "LPP Vorzeitige Rückkehr gemessene Wirkung" {
			found = s.Status == NotChecked && strings.Contains(s.Detail, "Notbetriebs-Werte am Gerät prüfen")
		}
	}
	if !found {
		t.Fatal(r.Steps)
	}
}
