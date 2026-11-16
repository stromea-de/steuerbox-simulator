package inspection

import (
	"testing"
	"time"
)

// 040 F-05: the API/CLI accept no timeouts below 1 s (the form enforces the same minimum).
func TestTimeoutMinimums040(t *testing.T) {
	c := DefaultConfig()
	c.Business, c.Inspector = "B", "P"
	if err := c.ValidateMinimums(false); err != nil {
		t.Fatal(err)
	}
	c.StepTimeout = time.Nanosecond
	if c.ValidateMinimums(false) == nil {
		t.Fatal("1 ns step timeout accepted")
	}
	d := DefaultDeviceConfig()
	d.Business, d.Inspector = "B", "P"
	if err := d.ValidateMinimums(true); err != nil {
		t.Fatal(err)
	}
	for _, set := range []func(*Config){
		func(c *Config) { c.EffectTimeout = time.Nanosecond },
		func(c *Config) { c.CleanupTimeout = time.Millisecond },
		func(c *Config) { c.FailsafeTimeout = 999 * time.Millisecond },
	} {
		v := d
		set(&v)
		if v.ValidateMinimums(true) == nil {
			t.Fatalf("sub-second device timeout accepted: %+v", v)
		}
	}
	// The recovery timeout is only used by the long-duration test.
	d.RecoveryTimeout = time.Nanosecond
	if err := d.ValidateMinimums(true); err != nil {
		t.Fatalf("function test must ignore the recovery timeout: %v", err)
	}
	d.LongTest = true
	if d.ValidateMinimums(true) == nil {
		t.Fatal("long test with 1 ns recovery timeout accepted")
	}
}

// 040 F-06: the hard run deadline covers the displayed time budget, the wait for the outage
// deadline and the load decision, independent of the recovery timeout in the function test.
func TestRunDeadlineCoversBudget040(t *testing.T) {
	for _, long := range []bool{false, true} {
		for _, scope := range []string{ScopeConsumption, ScopeBoth} {
			c := DefaultDeviceConfig()
			c.Scope, c.LongTest, c.PauseLowLoad = scope, long, true
			if !long {
				c.RecoveryTimeout = time.Second
			}
			n := time.Duration(1)
			if scope == ScopeBoth {
				n = 2
			}
			need := c.TimeBudget() + n*(c.FailsafeTimeout+HeartbeatReserve) + LowLoadDecision
			if long {
				need += n * (c.RecoveryTimeout - c.FailsafeMinimum)
			}
			if got := c.RunDeadline(); got < need {
				t.Fatalf("long=%v scope=%s: deadline %s < needed %s (budget %s)", long, scope, got, need, c.TimeBudget())
			}
		}
	}
}
