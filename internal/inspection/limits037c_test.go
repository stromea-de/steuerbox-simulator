package inspection

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func limitTrace037c(r Report) []string {
	var out []string
	for _, c := range r.Limits {
		v := "-"
		if c.Watts != nil {
			v = fmt.Sprintf("%.0f", *c.Watts)
		}
		if c.Failsafe {
			v += "F"
		}
		if c.Unconfirmed {
			v += "U"
		}
		out = append(out, c.Direction+":"+v)
	}
	return out
}

func equal037c(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 037c: the limit timeline comes from acknowledged writes and heartbeat changes
// of the run itself (signed: consumption +, feed-in −; null = no limit).
func Test037cRunRecordsLimitTimeline(t *testing.T) {
	f := &fake{powers: []float64{11000, 4200, 500, 4200, -3000, -500, -3000, -8000}}
	r, err := Run(context.Background(), testConfig(), Device{SKI: "abc"}, f)
	if err != nil || !r.Passed {
		t.Fatalf("%v %v", r.Steps, err)
	}
	want := []string{
		"LPC:4200",              // LPC limit acknowledged
		"LPC:500F", "LPP:-500F", // heartbeat stopped: both configured directions in failsafe
		"LPC:4200", "LPP:-", // simulator: heartbeat back ends failsafe at once
		"LPP:-3000",
		"LPC:500F", "LPP:-500F",
		"LPC:4200", "LPP:-3000",
		"LPC:-", "LPP:-", // cleanup releases acknowledged
	}
	if got := limitTrace037c(r); !equal037c(got, want) {
		t.Fatalf("limit timeline\n got %v\nwant %v", got, want)
	}
	for i := 1; i < len(r.Limits); i++ {
		if r.Limits[i].At.Before(r.Limits[i-1].At) {
			t.Fatal("timeline not monotonic")
		}
	}
	var limitStep Step
	for _, s := range r.Steps {
		if s.Name == "LPC Grenze + Quittung" {
			limitStep = s
		}
	}
	if r.Limits[0].At.After(limitStep.At) || r.Limits[0].At.Before(r.Started) {
		t.Fatal("limit start must be the acknowledgement moment within its step")
	}
}

// A rejected write is not a limit in effect.
func Test037cRejectedWriteRecordsNoLimit(t *testing.T) {
	f := &fake{reject: true, powers: []float64{11000, -8000}}
	r, _ := Run(context.Background(), testConfig(), Device{}, f)
	if len(r.Limits) != 0 {
		t.Fatalf("no limit was acknowledged: %v", limitTrace037c(r))
	}
}

// 037d (Review 037c P2): on site an acknowledged write after the heartbeat
// returns does not prove the end of failsafe. Without a measured return the
// failsafe line continues as unconfirmed.
func Test037cOnsiteFailsafeEndsWithNextAcknowledgedWrite(t *testing.T) {
	f := &onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{100, 100}}}
	c := onsiteConfig()
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	// 046 (R046-05): without power evidence the failsafe itself is unproven and
	// drawn unconfirmed from its start (formerly confirmed until the next write).
	want := []string{"LPC:4200", "LPC:500FU"}
	if got := limitTrace037c(r); !equal037c(got, want) {
		t.Fatalf("onsite timeline\n got %v\nwant %v", got, want)
	}
	// Failsafe starts at the deadline: reference (stop or later receipt) + timeout.
	h := r.Heartbeats[0]
	if d := r.Limits[1].At.Sub(h.Reference); d < c.FailsafeTimeout-time.Millisecond || d > c.FailsafeTimeout+time.Millisecond {
		t.Fatalf("failsafe start %v after reference, want the configured timeout %v", d, c.FailsafeTimeout)
	}
}

// A measured return ends the unconfirmed failsafe.
func Test037dOnsiteMeasuredReturnEndsFailsafe(t *testing.T) {
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	f := &onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{11000, 4200, 500, 11000}}}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"LPC:4200", "LPC:500F", "LPC:500FU", "LPC:-"}
	if got := limitTrace037c(r); !equal037c(got, want) {
		t.Fatalf("onsite timeline\n got %v\nwant %v", got, want)
	}
}

type delayed037d struct {
	fake
	stopped time.Time
}

func (f *delayed037d) FailsafeDelay() time.Duration { return 40 * time.Millisecond }
func (f *delayed037d) Heartbeat(ctx context.Context, on bool) error {
	if !on && f.stopped.IsZero() {
		f.stopped = time.Now()
	}
	return f.fake.Heartbeat(ctx, on)
}

// Simulator: failsafe begins when its heartbeat timeout expires, not at the stop.
func Test037dSimulatorFailsafeStartsAfterTimeout(t *testing.T) {
	c := testConfig()
	c.Scope = ScopeConsumption
	c.StepTimeout = time.Second
	f := &delayed037d{fake: fake{powers: []float64{11000, 4200, 4200, 4200, 4200, 4200, 4200, 500, 4200, 11000}}}
	r, _ := Run(context.Background(), c, Device{}, f)
	var failsafe *LimitChange
	for i := range r.Limits {
		if r.Limits[i].Failsafe {
			failsafe = &r.Limits[i]
			break
		}
	}
	if failsafe == nil {
		t.Fatalf("no failsafe: %v", limitTrace037c(r))
	}
	if d := failsafe.At.Sub(f.stopped); d < 40*time.Millisecond || d > 45*time.Millisecond {
		t.Fatalf("failsafe start %v after stop, want the 40 ms timeout", d)
	}
}
