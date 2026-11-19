package inspection

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Virtual device clock: the measurement occurs at stop + delay without waiting
// 125/151 real seconds. The actual context deadline determines its availability.
// 046: the failsafe read now waits for the real heartbeat timeout, so the
// timeout is scaled to 500 ms (below the 1 s sampling tick) and the delays to
// timeout + 5 s / timeout + 31 s; the 30 s reserve is unchanged.
type outage020b struct {
	onsiteFake
	receipt time.Time
	stopped time.Time
	delay   time.Duration
	calls   int
}

func (f *outage020b) Heartbeat(ctx context.Context, on bool) error {
	if !on {
		f.stopped = time.Now()
	}
	return ctx.Err()
}
func (f *outage020b) LastHeartbeatReceipt() (time.Time, string) {
	return f.receipt, "SPINE-Quittung lokal empfangen"
}
func (f *outage020b) Power(ctx context.Context) (float64, error) {
	f.calls++
	switch f.calls {
	case 1:
		return 11000, nil
	case 2:
		return 4200, nil
	case 3:
		deadline, ok := ctx.Deadline()
		if !ok || f.stopped.Add(f.delay).After(deadline) {
			return 4200, context.DeadlineExceeded
		}
		return 500, nil
	default:
		return 4200, nil
	}
}

const timeout020b = 500 * time.Millisecond

func TestHeartbeatOptionalReceiptAndStopDeadline020b(t *testing.T) {
	for _, receipt := range []string{"missing", "positive-older"} {
		for _, tc := range []struct {
			delay time.Duration
			pass  bool
		}{{timeout020b + 5*time.Second, true}, {timeout020b + 31*time.Second, false}} {
			t.Run(receipt+"/"+tc.delay.String(), func(t *testing.T) {
				f := &outage020b{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, delay: tc.delay}
				if receipt == "positive-older" {
					f.receipt = time.Now().Add(-time.Minute)
				}
				// A negative result is deliberately not a positive receive timestamp, just
				// like no response. The packet-level negative path is tested in eebus.
				c := onsiteConfig()
				c.Scope = ScopeConsumption
				c.FailsafeTimeout = timeout020b
				r, err := RunOnSite(context.Background(), c, Device{}, f)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, s := range r.Steps {
					if s.Name == "LPC Failsafe gemessene Wirkung" {
						found = true
						if s.Passed != tc.pass {
							t.Fatalf("effect=%+v, want pass=%t", s, tc.pass)
						}
					}
				}
				if !found || len(r.Heartbeats) != 1 {
					t.Fatalf("missing outage: %+v", r.Steps)
				}
				h := r.Heartbeats[0]
				if h.Reference.Before(f.stopped) || h.Reference.Sub(f.stopped) > time.Second || h.Deadline.Sub(h.Reference) != timeout020b+30*time.Second || !strings.Contains(h.Basis, "lokaler Stopp") {
					t.Fatalf("wrong reference: %+v", h)
				}
				if tc.pass && !r.Passed {
					t.Fatalf("optional receipt affected outcome: %+v", r.Steps)
				}
				if !tc.pass && r.Outcome() != "Nicht bestanden" {
					t.Fatal(r.Outcome())
				}
			})
		}
	}
}

func TestObservationNeverShortensAndKeepsOptionalReceipt020b(t *testing.T) {
	stopped := time.Date(2026, 10, 30, 12, 0, 0, 0, time.FixedZone("CET", 3600))
	for _, delta := range []time.Duration{-time.Minute, 0, 2 * time.Second} {
		receipt := stopped.Add(delta)
		h := NewHeartbeatObservation("LPP", stopped, receipt, 120*time.Second)
		reference := stopped
		if receipt.After(reference) {
			reference = receipt
		}
		if !h.Stopped.Equal(stopped) || !h.Reference.Equal(reference) || h.Receipt == nil || !h.Receipt.Equal(receipt) || !h.Deadline.Equal(reference.Add(150*time.Second)) {
			t.Fatalf("%+v", h)
		}
	}
	h := NewHeartbeatObservation("LPC", stopped, time.Time{}, 120*time.Second)
	if h.Receipt != nil || !h.Reference.Equal(stopped) {
		t.Fatal(h)
	}
}

type cancelledEffect020b struct {
	outage020b
	cancel context.CancelFunc
}

func (f *cancelledEffect020b) Power(ctx context.Context) (float64, error) {
	if f.calls == 2 {
		f.calls++
		f.cancel()
		return 500, nil
	}
	return f.outage020b.Power(ctx)
}
func TestLateMeasurementCannotPassAfterCancellation020b(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &cancelledEffect020b{outage020b: outage020b{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}}, cancel: cancel}
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, err := RunOnSite(ctx, c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range r.Steps {
		if step.Name == "LPC Failsafe gemessene Wirkung" {
			if step.Passed {
				t.Fatal("late response passed", step)
			}
			return
		}
	}
	t.Fatal("missing failsafe step")
}
