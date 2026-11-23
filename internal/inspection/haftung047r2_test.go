package inspection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// R2-01: once the test touched the device (here: failsafe values differ), an active
// limit is never taken as "unchanged", even when its value equals the original;
// the restore always ends with an inactive limit.
func TestActiveLimitIsReleasedOnRestore047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true})
	f.limits["LPC"] = LimitState{Watts: 4200, HasValue: true, Known: true, Active: true}
	p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": f.failsafe["LPC"]}, Limits: map[string]LimitState{"LPC": f.limits["LPC"]}}
	f.failsafe["LPC"] = FailsafeValues{Power: 500, Minimum: 2 * time.Hour}
	steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond)
	if !ok || f.limitWrites != 1 || f.limits["LPC"].Active {
		t.Fatal(ok, steps, f.limitWrites, f.limits)
	}
	s := steps[len(steps)-1]
	if s.Name != "LPC Grenzen aufheben" || !s.Passed {
		t.Fatal(steps)
	}
}

// R2-03: without a saved original limit the restore never writes a guessed 0 W.
func TestRestoreWithoutSavedLimitWritesNoLimit047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true})
	f.limits["LPC"] = LimitState{Watts: 4200, HasValue: true, Known: true, Active: true}
	p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": f.failsafe["LPC"]}}
	steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond)
	if ok || f.limitWrites != 0 {
		t.Fatal(ok, steps, f.limitWrites)
	}
	// R3-04: an inactive limit without saved original is not proof of the original
	// value either: nothing written, not confirmed, the record stays.
	f.limits["LPC"] = LimitState{Watts: 4200, HasValue: true, Known: true}
	if steps, ok = RestoreOriginals(context.Background(), f, p, 50*time.Millisecond); ok || f.limitWrites != 0 {
		t.Fatal(ok, steps, f.limitWrites)
	}
}

// R3-03: a crash after saving but before the first configuration write leaves the
// device untouched; a limit that was active before the test (another EMS) stays.
func TestUntouchedActiveLimitStays047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, LPP: true})
	f.limits["LPC"] = LimitState{Watts: 4200, HasValue: true, Known: true, Active: true}
	p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": f.failsafe["LPC"], "LPP": f.failsafe["LPP"]}, Limits: map[string]LimitState{"LPC": f.limits["LPC"], "LPP": f.limits["LPP"]}}
	steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond)
	if !ok || f.limitWrites != 0 || len(f.writes) != 0 || !f.limits["LPC"].Active || strings.Contains(strings.Join(f.calls, ","), "heartbeat") {
		t.Fatal(ok, steps, f.limitWrites, f.writes, f.limits)
	}
	// Any difference in another direction means the test wrote: release then.
	f.failsafe["LPP"] = FailsafeValues{Power: 500, Minimum: 2 * time.Hour}
	if steps, ok = RestoreOriginals(context.Background(), f, p, 50*time.Millisecond); !ok || f.limits["LPC"].Active || f.failsafe["LPP"].Power != 666 {
		t.Fatal(ok, steps, f.limits, f.failsafe)
	}
}

type failRelease047 struct{ *device047 }

func (f failRelease047) Limit(ctx context.Context, k string, v float64, on bool) (string, error) {
	if !on {
		return "", errors.New("keine Quittung")
	}
	return f.device047.Limit(ctx, k, v, on)
}

// R2-03: a failed release write is "Nein", never "nur quittiert".
func TestFailedReleaseWriteIsNo047(t *testing.T) {
	f := failRelease047{newDevice047(Capabilities{LPC: true, MPC: true})}
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if r.OriginalsRestored() != "no" || r.ReleaseVerified == nil || *r.ReleaseVerified || !r.RestorePending {
		t.Fatal(r.OriginalsRestored(), r.ReleaseVerified, r.RestorePending)
	}
	if s := step047(t, r, "Grenzen aufheben"); s.Passed || !strings.Contains(s.Detail, "keine Quittung") {
		t.Fatal(s)
	}
}

// R2-07: only the half-milliwatt rounding of the write path is tolerated.
func TestLimitToleranceIsHalfMilliwatt047(t *testing.T) {
	if sameLimitWatts(1000.001, 1000) || !sameLimitWatts(1000.0004, 1000) || !sameLimitWatts(4200.001, 4200.0005) || sameLimitWatts(4200.0011, 4200.0005) {
		t.Fatal("Toleranz")
	}
}

// R4-03: an unreadable device state is no detected change: nothing is written, the
// restore is not confirmed (the record stays), and a later readable state restores.
func TestUnreadableStateWritesNothing047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true})
	f.limits["LPC"] = LimitState{Watts: 4200, HasValue: true, Known: true, Active: true}
	p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": f.failsafe["LPC"]}, Limits: map[string]LimitState{"LPC": f.limits["LPC"]}}
	f.limitErr = errors.New("Zeitlimit")
	steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond)
	if ok || f.limitWrites != 0 || len(f.writes) != 0 || !f.limits["LPC"].Active || strings.Contains(strings.Join(f.calls, ","), "heartbeat") {
		t.Fatal(ok, steps, f.limitWrites, f.writes, f.calls)
	}
	if len(steps) != 1 || steps[0].Name != "LPC Originalwerte lesen" || steps[0].Passed {
		t.Fatal(steps)
	}
	f.limitErr = nil
	f.failsafe["LPC"] = FailsafeValues{Power: 500, Minimum: 2 * time.Hour}
	if steps, ok = RestoreOriginals(context.Background(), f, p, 50*time.Millisecond); !ok || f.failsafe["LPC"].Power != 777 {
		t.Fatal(ok, steps, f.failsafe)
	}
}
