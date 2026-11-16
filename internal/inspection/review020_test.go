package inspection

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The device reaches failsafe after its receive timer expires, then the fresh
// power reply arrives with latency. A deadline at local stop + timeout cuts it off.
type latency020 struct {
	onsiteFake
	stopped time.Time
}

func (f *latency020) Heartbeat(ctx context.Context, on bool) error {
	if !on {
		f.stopped = time.Now()
	}
	return ctx.Err()
}
func (f *latency020) LastHeartbeatReceipt() (time.Time, string) {
	return f.stopped, "Test: Empfang am Gerät"
}
func (f *latency020) Power(ctx context.Context) (float64, error) {
	if f.stopped.IsZero() {
		return f.fake.Power(ctx)
	}
	timer := time.NewTimer(80 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-timer.C:
		return 500, nil
	}
}
func TestHeartbeatReceiveDeadline020(t *testing.T) {
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	f := &latency020{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{11000, 4200}}}}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range r.Steps {
		if s.Name == "LPC Failsafe gemessene Wirkung" {
			if !s.Passed {
				t.Fatal(s)
			}
			return
		}
	}
	t.Fatal("failsafe evidence missing")
}
func TestHeartbeatReferenceAndReserve020(t *testing.T) {
	received := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	deadline := HeartbeatDeadline(received, 120*time.Second)
	if !received.Add(125*time.Second).Before(deadline) || deadline.Sub(received) != 150*time.Second {
		t.Fatal(deadline)
	}
}
func TestProfiles020(t *testing.T) {
	c := DefaultDeviceConfig()
	c.Business = "Test"
	c.Inspector = "Test"
	c.Scope = ScopeConsumption
	if c.ProfileLabel() != "Funktionsprüfung" || c.TimeBudget() != 1200*time.Second {
		t.Fatal(c.TimeBudget(), c.ProfileLabel())
	}
	c.TestProfile = "full"
	if !c.FullTest() || c.TimeBudget() != 8400*time.Second {
		t.Fatal(c.TimeBudget())
	}
	c.Scope = ScopeBoth
	if c.TimeBudget() != 16800*time.Second {
		t.Fatal(c.TimeBudget())
	}
	c.TestProfile = "quick"
	c.LongTest = true
	if c.FullTest() {
		t.Fatal("explicit profile must win over legacy flag")
	}
	c.TestProfile = "bogus"
	if c.Validate() == nil {
		t.Fatal("invalid profile")
	}
	c.TestProfile = ""
	if !c.FullTest() {
		t.Fatal("legacy long test compatibility")
	}
	c.TestProfile = "full"
	data, _ := json.Marshal(c)
	if !strings.Contains(string(data), `"testProfile":"full"`) {
		t.Fatal(string(data))
	}
}
