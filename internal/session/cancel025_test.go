package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func TestCancelReconnect025(t *testing.T) {
	// An untrusted endpoint fails before dialing: exercise the real retry loop
	// without a listener, including cancellation of independent cleanup contexts.
	s := &Session{EG: &eebus.Endpoint{}, Device: discovery.Device{Address: "127.0.0.1:4711", SKI: "0123456789abcdef0123456789abcdef01234567"}}
	done := make(chan error, 1)
	go func() { done <- s.Recover(context.Background()) }()
	deadline := time.After(time.Second)
	for s.Reconnection().Attempt < 1 {
		select {
		case <-deadline:
			t.Fatal("no attempt")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	p := s.Reconnection()
	if !p.Active || time.Until(p.Deadline) > 30*time.Second || time.Until(p.Deadline) <= 0 {
		t.Fatal(p)
	}
	s.CancelRecovery()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry loop stuck")
	}
	p = s.Reconnection()
	if p.Active || s.LiveConnected() {
		t.Fatal("canceled recovery still live", p)
	}
	if err := s.Recover(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatal("cleanup retried after explicit cancel", err)
	}
	if s.Reconnection().Attempt != p.Attempt {
		t.Fatal("late retry")
	}
}
func TestReconnectDeadline025(t *testing.T) {
	s := &Session{EG: &eebus.Endpoint{}, Device: discovery.Device{Address: "127.0.0.1:4711", SKI: "0123456789abcdef0123456789abcdef01234567"}}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := s.Recover(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if p := s.Reconnection(); p.Active || !p.TimedOut || p.Attempt != 1 {
		t.Fatal(p)
	}
}

// With a healthy transport, abort must still release limits and restore the
// originals; only fresh connection attempts are suppressed by CancelRecovery.
func TestCancelRunRetainsCleanup025(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	if err = s.SelectDevice(s.Device, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	c := inspection.DefaultDeviceConfig()
	c.Business = "Test"
	c.Inspector = "Test"
	c.Authorized = true
	c.Scope = inspection.ScopeConsumption
	c.PauseLowLoad = false
	// Device-mode heartbeat startup waits two intervals, both in the flow and
	// during cleanup. Leave time for those waits and the SPINE acknowledgements.
	c.StepTimeout = 2*eebus.HeartbeatInterval + time.Second
	c.CleanupTimeout = 2*eebus.HeartbeatInterval + time.Second
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	type outcome struct {
		report inspection.Report
		err    error
	}
	result := make(chan outcome, 1)
	go func() { r, err := s.Run(runCtx, c); result <- outcome{r, err} }()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
waiting:
	for {
		for _, v := range s.Progress().Steps {
			if v.Name == "Failsafe-Werte setzen" && v.Passed {
				break waiting
			}
		}
		select {
		case got := <-result:
			t.Fatalf("run ended before cancellation point: err=%v steps=%+v", got.err, got.report.Steps)
		case <-ctx.Done():
			t.Fatalf("cancellation point not reached: %v; progress=%+v", ctx.Err(), s.Progress())
		case <-ticker.C:
		}
	}
	s.CancelRecovery()
	stop()
	select {
	case got := <-result:
		if got.err != nil {
			t.Fatal(got.err)
		}
		r := got.report
		released := false
		restored := 0
		for _, v := range r.Steps {
			if v.Name == "Grenzen aufheben" {
				released = v.Passed
			}
			if strings.Contains(v.Name, "zurückstellen") {
				restored++
				if !v.Passed {
					t.Fatal("restoration lost on cancel", v)
				}
			}
		}
		if !released {
			t.Fatal("release missing after cancel", r.Steps)
		}
		if restored != 2 || r.AbortReason != "user_cancel" || r.Passed {
			t.Fatalf("incomplete canceled report: restored=%d reason=%s passed=%t", restored, r.AbortReason, r.Passed)
		}
		original, ok := r.FailsafeOriginals["LPC"]
		actual, readErr := s.ReadFailsafe(ctx, "LPC")
		if !ok || readErr != nil || actual.Power != original.Power || actual.Minimum != original.Minimum {
			t.Fatalf("restored device values differ: actual=%+v original=%+v err=%v", actual, original, readErr)
		}
		if p := s.Reconnection(); p.Active || p.Attempt != 0 {
			t.Fatal("unexpected recovery after cancel", p)
		}
	case <-ctx.Done():
		t.Fatal("cleanup stuck")
	}
}
