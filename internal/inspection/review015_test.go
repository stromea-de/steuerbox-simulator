package inspection

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

func TestAmbiguousThousandsRejected(t *testing.T) {
	for _, raw := range []string{"4.200", "12.345", "123.456"} {
		if _, err := ParsePower(raw); err == nil {
			t.Fatal("ambiguous input accepted", raw)
		}
	}
	v, err := ParsePower("4200,125")
	if err != nil || v != 4200.125 {
		t.Fatal(v, err)
	}
}

func TestConsumptionScopeHasNoLPPWritesOrMissingCoverage(t *testing.T) {
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	f := &preservingDevice{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, LPP: true, MPC: true}, fake: fake{powers: []float64{11000, 4200, 500, 11000}}}}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range r.Steps {
		if strings.HasPrefix(s.Name, "LPP") {
			t.Fatal(s)
		}
	}
	for _, s := range f.calls {
		if s == "LPP" {
			t.Fatal(f.calls)
		}
	}
	if r.Outcome() != "Bestanden" {
		t.Fatal(r.Outcome())
	}
	r.Steps = append([]Step(nil), r.Steps...)
	for i := range r.Steps {
		if strings.Contains(r.Steps[i].Detail, "Langtest") {
			r.Steps[i].Status = ""
			r.Steps[i].Passed = true
		}
	}
	r.Passed = true
	if r.Outcome() != "Bestanden" || r.OutcomeLabel() != "LPC-Funktionsprüfung bestanden" {
		t.Fatal(r.Outcome(), r.OutcomeLabel())
	}
	c.LPP = 0
	if err := c.Validate(); err != nil {
		t.Fatal("unused LPP must not be required", err)
	}
}

func TestSamplesBoundedFiniteAndOwned(t *testing.T) {
	r := &Report{}
	m := newMeasurements(r, &fake{})
	m.current = "Ausgangsmessung MPC"
	for i := 0; i < 650; i++ {
		m.last = time.Time{}
		m.record(float64(i), nil)
	}
	m.last = time.Time{}
	m.record(math.NaN(), nil)
	if len(r.Samples) != 600 || r.Samples[0].Watts == nil || *r.Samples[0].Watts != 51 || r.Samples[599].Watts != nil {
		t.Fatal(len(r.Samples), r.Samples[0])
	}
	if r.Samples[0].Step != "Ausgangsmessung MPC" || r.Samples[0].At.IsZero() {
		t.Fatal(r.Samples[0])
	}
}

type lowLoadDevice struct {
	preservingDevice
	decisions      int
	writesAtPrompt int
}

func (f *lowLoadDevice) LowLoad(_ context.Context, v LowLoad) (bool, error) {
	f.decisions++
	f.writesAtPrompt = len(f.calls)
	if v.Watts != 800 || v.Required != 4250 || v.Direction != "LPC" {
		return false, context.Canceled
	}
	return true, nil
}
func TestLowLoadRetryBeforeActiveLimit(t *testing.T) {
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	c.PauseLowLoad = true
	f := &lowLoadDevice{preservingDevice: preservingDevice{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{800, 11000, 11000, 4200, 500, 11000}}}}}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil || f.decisions != 1 {
		t.Fatal(err, f.decisions, r)
	}
	for _, call := range f.calls[:f.writesAtPrompt] {
		if call == "LPC" {
			t.Fatal("limit sent before decision")
		}
	}
	if len(r.Samples) < 5 {
		t.Fatal(r.Samples)
	}
}

func TestPureConsumerPassesOnlySelectedScope(t *testing.T) {
	for _, scope := range []string{ScopeConsumption, ScopeBoth} {
		c := onsiteConfig()
		c.Scope = scope
		f := &preservingDevice{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{11000, 4200, 500, 11000}}}}
		r, err := RunOnSite(context.Background(), c, Device{}, f)
		if err != nil {
			t.Fatal(err)
		}
		if scope == ScopeConsumption {
			if r.Outcome() != "Bestanden" {
				t.Fatal(r)
			}
		} else if r.Outcome() != "Unvollständig" {
			t.Fatal(r)
		}
	}
}
func TestLongWaitSamplesAndCancellation(t *testing.T) {
	r := &Report{}
	m := newMeasurements(r, &fake{powers: []float64{500, 501}})
	m.current = "LPC Wiederkehr"
	if err := m.wait(context.Background(), 2100*time.Millisecond, time.Second); err != nil || len(r.Samples) != 2 {
		t.Fatal(err, r.Samples)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.wait(ctx, time.Hour, time.Second); err != context.Canceled {
		t.Fatal(err)
	}
}
