package inspection

import (
	"context"
	"testing"
	"time"
)

type onsiteFake struct {
	fake
	caps    Capabilities
	active  map[string]bool
	receipt time.Time
}

// 046: the receipt is fixed at its first query (the heartbeat stop); a receipt
// moving with the clock would be an ever later acknowledgement.
func (f *onsiteFake) LastHeartbeatReceipt() (time.Time, string) {
	if f.receipt.IsZero() {
		f.receipt = time.Now()
	}
	return f.receipt, "Test: Empfang am Gerät"
}
func (f *onsiteFake) ReadFailsafe(context.Context, string) (FailsafeValues, error) {
	return FailsafeValues{Power: 900, Minimum: 2 * time.Hour}, nil
}
func (f *onsiteFake) WriteFailsafe(context.Context, string, string, FailsafeValues) error { return nil }
func (f *onsiteFake) Connect(context.Context) error                                       { f.calls = append(f.calls, "connect"); return nil }
func (f *onsiteFake) Capabilities(context.Context) (Capabilities, error) {
	f.calls = append(f.calls, "capabilities")
	return f.caps, nil
}
func (f *onsiteFake) DeviceInfo() Device            { return Device{Brand: "SPINE-Hersteller", Serial: "123"} }
func (f *onsiteFake) Recover(context.Context) error { return nil }
func (f *onsiteFake) Limit(ctx context.Context, k string, v float64, on bool) (string, error) {
	if f.active == nil {
		f.active = map[string]bool{}
	}
	f.active[k] = on
	return f.fake.Limit(ctx, k, v, on)
}
func onsiteConfig() Config {
	c := testConfig()
	c.Authorized = true
	c.FailsafeTimeout = 50 * time.Millisecond
	c.TestHeartbeatTimeout = time.Nanosecond // 046: synthetic device with the configured timeout
	c.FailsafeMinimum = 2 * time.Hour
	c.EffectTimeout = 50 * time.Millisecond
	c.RecoveryTimeout = 50 * time.Millisecond
	c.CleanupTimeout = 50 * time.Millisecond
	return c
}
func TestOnsiteAuthorizationFlagIsOptional(t *testing.T) {
	for _, authorized := range []bool{false, true} {
		f := &onsiteFake{}
		c := onsiteConfig()
		c.Authorized = authorized
		if _, err := RunOnSite(context.Background(), c, Device{}, f); err != nil || len(f.calls) == 0 {
			t.Fatal("compatibility flag must not gate execution", authorized, err, f.calls)
		}
	}
}
func TestOnsiteMissingLPPAndLowPowerAreUnverified(t *testing.T) {
	f := &onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{100, 100}}}
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	var skipped int
	for _, s := range r.Steps {
		if s.Status == NotChecked {
			skipped++
		}
		if !s.Passed && s.Status != NotChecked {
			t.Fatal(s)
		}
	}
	if skipped == 0 || r.Passed || r.Mode != ModeOnSite || r.Device.Serial != "123" {
		t.Fatal(r)
	}
	if f.active["LPC"] || f.active["LPP"] {
		t.Fatal("limit left active")
	}
}
func TestOnsiteRejectedWriteReleasesLimit(t *testing.T) {
	f := &onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{reject: true, powers: []float64{11000}}}
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil || r.Passed || f.active["LPC"] {
		t.Fatal(r, err)
	}
}

type unrecoverable struct{ onsiteFake }

func (f *unrecoverable) Recover(context.Context) error { return context.DeadlineExceeded }
func TestOnsitePermanentDisconnectCannotClaimRelease(t *testing.T) {
	f := &unrecoverable{onsiteFake: onsiteFake{caps: Capabilities{LPC: true}, fake: fake{}}}
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	last := r.Steps[len(r.Steps)-1]
	if last.Name != "Grenzen aufheben" || last.Passed || r.Passed {
		t.Fatal(r)
	}
}
func TestDeviceTimeoutValidation(t *testing.T) {
	c := DefaultDeviceConfig()
	c.LPP = 3000 // explicit test fixture, no device default
	c.Business = "Test"
	c.Inspector = "Test"
	c.Authorized = true
	if err := c.ValidateDevice(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []time.Duration{0, time.Hour, 25 * time.Hour} {
		c.FailsafeMinimum = d
		if c.ValidateDevice() == nil {
			t.Fatal("unsafe minimum", d)
		}
	}
}

type slowDevice struct {
	onsiteFake
	began  time.Time
	cancel context.CancelFunc
}

func (f *slowDevice) Power(context.Context) (float64, error) {
	if f.began.IsZero() {
		f.began = time.Now()
		return 11000, nil
	}
	if time.Since(f.began) < 100*time.Millisecond {
		return 11000, nil
	}
	return 4200, nil
}
func (f *slowDevice) Heartbeat(ctx context.Context, on bool) error {
	if !on {
		f.cancel()
	}
	return ctx.Err()
}
func TestOnsiteEffectUsesItsOwnLongerDeadline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &slowDevice{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, cancel: cancel}
	c := onsiteConfig()
	c.StepTimeout = 30 * time.Millisecond
	c.EffectTimeout = time.Second
	r, err := RunOnSite(ctx, c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for _, v := range r.Steps {
		if v.Name == "LPC gemessene Wirkung" {
			seen = v.Passed
		}
	}
	if !seen || r.Passed {
		t.Fatal(r)
	}
	if last := r.Steps[len(r.Steps)-1]; !last.Passed {
		t.Fatal(last)
	}
}
