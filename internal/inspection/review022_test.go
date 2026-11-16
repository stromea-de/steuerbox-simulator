package inspection

import (
	"context"
	"strings"
	"testing"
	"time"
)

type protocol022 struct {
	onsiteFake
	stopped   time.Time
	restarted bool
	renewed   bool
	recovered bool
}

func (f *protocol022) Heartbeat(ctx context.Context, on bool) error {
	if !on {
		f.stopped = time.Now()
	} else if !f.stopped.IsZero() {
		f.restarted = true
	}
	return ctx.Err()
}
func (f *protocol022) Limit(ctx context.Context, k string, v float64, on bool) (string, error) {
	if on && f.restarted {
		f.renewed = true
	}
	return f.onsiteFake.Limit(ctx, k, v, on)
}
func (f *protocol022) Recover(ctx context.Context) error {
	if f.renewed {
		f.recovered = true
	}
	return ctx.Err()
}
func TestProtocolWithoutMeasurableEffect022(t *testing.T) {
	for _, mpc := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-mpc", true: "low-load"}[mpc], func(t *testing.T) {
			f := &protocol022{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: mpc}, fake: fake{powers: []float64{100, 100}}}}
			c := onsiteConfig()
			c.Scope = ScopeConsumption
			r, e := RunOnSite(context.Background(), c, Device{}, f)
			if e != nil {
				t.Fatal(e)
			}
			if len(r.Heartbeats) != 1 || !f.restarted || !f.renewed || !f.recovered {
				t.Fatalf("protocol missing: %+v %+v", f, r.Steps)
			}
			if !mpc && r.AbortReason == "insufficient_load" {
				t.Fatal("missing measurement is not evidence of insufficient load")
			}
			h := r.Heartbeats[0]
			if time.Since(f.stopped) < c.FailsafeTimeout+HeartbeatReserve || h.Deadline.Sub(h.Reference) != c.FailsafeTimeout+HeartbeatReserve {
				t.Fatal("outage was shortened", h)
			}
			if r.Outcome() != "Unvollständig" {
				t.Fatal(r.Outcome())
			}
			for _, n := range []string{"LPC gemessene Wirkung", "LPC Failsafe gemessene Wirkung", "LPC Wiederkehr gemessene Wirkung"} {
				found := false
				for _, s := range r.Steps {
					if s.Name == n {
						found = true
						if s.Passed || s.Status != NotChecked {
							t.Fatal(s)
						}
					}
				}
				if !found {
					t.Fatal("missing", n)
				}
			}
		})
	}
}
func TestDeviceDefaultProduction022(t *testing.T) {
	if DefaultDeviceConfig().LPP != 1000 {
		t.Fatal("expected labelled simulator default")
	}
}

func TestProtocolBudget022(t *testing.T) {
	c := DefaultDeviceConfig()
	c.Scope = ScopeBoth
	c.TestProfile = "full"
	if c.ProtocolTimeBudget() != 28*time.Minute {
		t.Fatal(c.ProtocolTimeBudget())
	}
}

type cancelProtocol022 struct {
	protocol022
	cancel context.CancelFunc
}

func (f *cancelProtocol022) Heartbeat(ctx context.Context, on bool) error {
	err := f.protocol022.Heartbeat(ctx, on)
	if !on {
		f.cancel()
	}
	return err
}
func TestCanceledProtocolWaitReleasesAndRestores022(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &cancelProtocol022{protocol022: protocol022{onsiteFake: onsiteFake{caps: Capabilities{LPP: true}}}, cancel: cancel}
	c := onsiteConfig()
	c.Scope = ScopeProduction
	r, e := RunOnSite(ctx, c, Device{}, f)
	if e != nil {
		t.Fatal(e)
	}
	if r.Outcome() != "Unvollständig" || r.AbortReason != "user_cancel" {
		t.Fatal(r.Outcome())
	}
	var release, restore int
	for _, s := range r.Steps {
		if s.Name == "LPP Wiederkehr Protokoll" && s.Passed {
			t.Fatal("canceled wait resumed", s)
		}
		if s.Name == "Grenzen aufheben" && s.Passed {
			release++
		}
		if strings.Contains(s.Name, "zurückstellen") && s.Passed {
			restore++
		}
	}
	if release != 1 || restore != 2 {
		t.Fatal(r.Steps)
	}
}
