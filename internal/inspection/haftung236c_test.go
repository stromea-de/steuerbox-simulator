package inspection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// R5-03: the regular end of a test reads before it writes; a failsafe minimum that
// LPC and LPP share is written once, and the record is cleared.
func TestNormalEndSharedMinimumOnce236c(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, LPP: true, MPC: true})
	f.sharedMin = true
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.Join(f.writes, ","), "-minimum"); n != 1 {
		t.Fatal("Mindestdauer nicht genau einmal geschrieben", f.writes)
	}
	for _, k := range []string{"LPC", "LPP"} {
		for _, name := range []string{k + " Failsafe-Leistung zurückstellen", k + " Failsafe-Mindestdauer zurückstellen"} {
			if s := step047(t, r, name); !s.Passed {
				t.Fatal(s)
			}
		}
	}
	if f.cleared != 1 || r.RestorePending {
		t.Fatal("Datensatz nicht gelöscht", f.cleared, r.RestorePending)
	}
}

// R5-02: two different originals for one shared device value can never both be
// restored; the restore must fail and keep the record.
func TestConflictingSharedMinimumFails236c(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, LPP: true})
	f.sharedMin = true
	p := PendingRestore{ReportID: "x",
		Failsafe: map[string]FailsafeValues{"LPC": {Power: 777, Minimum: 2 * time.Hour}, "LPP": {Power: 666, Minimum: time.Hour}},
		Limits:   map[string]LimitState{"LPC": f.limits["LPC"], "LPP": f.limits["LPP"]}}
	if steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond); ok {
		t.Fatal("Erfolg trotz widersprüchlicher gemeinsamer Mindestdauer", steps)
	}
}

// R5-01: a limit reply without active flag or value is an incomplete read; nothing
// is written although the failsafe values differ.
func TestIncompleteLimitReadWritesNothing236c(t *testing.T) {
	for _, l := range []LimitState{{Watts: 4200, HasValue: true}, {Known: true, Active: true}} {
		f := newDevice047(Capabilities{LPC: true})
		p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": f.failsafe["LPC"]}, Limits: map[string]LimitState{"LPC": f.limits["LPC"]}}
		f.failsafe["LPC"] = FailsafeValues{Power: 5000, Minimum: 3 * time.Hour}
		f.limits["LPC"] = l
		if steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond); ok || len(f.writes) != 0 || f.limitWrites != 0 {
			t.Fatal(ok, steps, f.writes, f.limitWrites)
		}
	}
}

type failsafeReadErr236c struct{ *device047 }

func (failsafeReadErr236c) ReadFailsafe(context.Context, string) (FailsafeValues, error) {
	return FailsafeValues{}, errors.New("Zeitüberschreitung")
}

// R5-01: the read right before a failsafe write must succeed; otherwise nothing is
// written and the step fails.
func TestUnreadableFieldIsNotWritten236c(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true})
	if _, err := restoreFailsafeOnce(context.Background(), failsafeReadErr236c{f}, "LPC", "power", FailsafeValues{Power: 1}, nil); err == nil || len(f.writes) != 0 {
		t.Fatal(err, f.writes)
	}
}

// R6-01: a field that already matched at the first read is re-checked after later
// writes; LPC 3 h matches, LPP 4 h overwrites the shared minimum → no success.
func TestSkippedSharedFieldIsRechecked236c(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, LPP: true})
	f.sharedMin = true
	p := PendingRestore{ReportID: "x",
		Failsafe: map[string]FailsafeValues{"LPC": {Power: 777, Minimum: 3 * time.Hour}, "LPP": {Power: 666, Minimum: 4 * time.Hour}},
		Limits:   map[string]LimitState{"LPC": f.limits["LPC"], "LPP": f.limits["LPP"]}}
	if steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond); ok {
		t.Fatal("Erfolg, obwohl LPC-Mindestdauer überschrieben", steps, f.failsafe)
	}
}
