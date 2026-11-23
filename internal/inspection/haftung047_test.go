package inspection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// device047 keeps failsafe values and limits like a real device (236): writes
// change state, reads return what the device holds.
type device047 struct {
	onsiteFake
	failsafe    map[string]FailsafeValues
	limits      map[string]LimitState
	dropRestore bool // acknowledges restore writes without storing them
	stuckActive bool // a released limit stays active
	saveErr     error
	saved       []PendingRestore
	cleared     int
	writes      []string
	onLimit     func()
	keepValue   bool  // an inactive write leaves the old value in place
	limitErr    error // limit reads fail
	limitWrites int
	inFailsafe  map[string]bool // device rejects failsafe values until a limit write
	sharedMin   bool            // one failsafe minimum for LPC and LPP; a second write times out
}

func newDevice047(caps Capabilities) *device047 {
	return &device047{onsiteFake: onsiteFake{caps: caps, fake: fake{powers: []float64{8000, 3000, 400, 8000}}},
		failsafe: map[string]FailsafeValues{"LPC": {Power: 777, Minimum: 3 * time.Hour}, "LPP": {Power: 666, Minimum: 3 * time.Hour}},
		limits:   map[string]LimitState{"LPC": {Watts: 4200, HasValue: true, Known: true}, "LPP": {Watts: 9000, HasValue: true, Known: true}}}
}
func (f *device047) Configure(_ context.Context, c Config) error {
	f.calls = append(f.calls, "configure")
	for k := range f.failsafe {
		f.failsafe[k] = FailsafeValues{Power: c.Failsafe, Minimum: c.FailsafeMinimum}
	}
	return nil
}
func (f *device047) ReadFailsafe(_ context.Context, k string) (FailsafeValues, error) {
	return f.failsafe[k], nil
}
func (f *device047) WriteFailsafe(ctx context.Context, k, field string, v FailsafeValues) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	f.writes = append(f.writes, k+"-"+field)
	if f.inFailsafe[k] {
		return errors.New("commandRejected, Fehlernummer 7")
	}
	if f.dropRestore {
		return nil
	}
	cur := f.failsafe[k]
	if field == "power" {
		cur.Power = v.Power
	} else {
		if f.sharedMin {
			if cur.Minimum == v.Minimum {
				return errors.New("negative SPINE-Quittung: Fehlernummer 2")
			}
			for o, fv := range f.failsafe {
				fv.Minimum = v.Minimum
				f.failsafe[o] = fv
			}
		}
		cur.Minimum = v.Minimum
	}
	f.failsafe[k] = cur
	return nil
}
func (f *device047) Limit(ctx context.Context, k string, v float64, on bool) (string, error) {
	if on && f.onLimit != nil {
		hook := f.onLimit
		f.onLimit = nil
		hook()
		return "", context.Canceled
	}
	ack, err := f.onsiteFake.Limit(ctx, k, v, on)
	if err == nil {
		f.limitWrites++
		delete(f.inFailsafe, k)
		active := on || (f.stuckActive && f.limits[k].Active)
		if f.keepValue && !on {
			v = f.limits[k].Watts
		}
		f.limits[k] = LimitState{Watts: v, HasValue: true, Known: true, Active: active}
	}
	return ack, err
}
func (f *device047) ReadLimit(_ context.Context, k string) (LimitState, error) {
	if f.limitErr != nil {
		return LimitState{}, f.limitErr
	}
	return f.limits[k], nil
}
func (f *device047) SaveRestore(p PendingRestore) error {
	f.calls = append(f.calls, "journal")
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, p)
	return nil
}
func (f *device047) ClearRestore(id string) error {
	if len(f.saved) == 0 || f.saved[len(f.saved)-1].ReportID != id {
		return errors.New("fremder Datensatz")
	}
	f.cleared++
	f.calls = append(f.calls, "journal-clear")
	return nil
}

func step047(t *testing.T, r Report, name string) Step {
	t.Helper()
	for _, s := range r.Steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("Schritt %q fehlt: %+v", name, r.Steps)
	return Step{}
}

func TestRestoreIsReadBackAndJournalCleared047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, MPC: true})
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"LPC Failsafe-Leistung zurückstellen", "LPC Failsafe-Mindestdauer zurückstellen"} {
		if s := step047(t, r, name); !s.Passed || !strings.Contains(s.Detail, "zurückgelesen") {
			t.Fatal(s)
		}
	}
	if r.FailsafeRestore() != "confirmed" || f.failsafe["LPC"] != (FailsafeValues{Power: 777, Minimum: 3 * time.Hour}) {
		t.Fatal(r.FailsafeRestore(), f.failsafe)
	}
	if s := step047(t, r, "Grenzen aufheben"); !s.Passed || !strings.Contains(s.Detail, "keine aktive Begrenzung") {
		t.Fatal(s)
	}
	if r.ReleaseVerified == nil || !*r.ReleaseVerified || f.limits["LPC"].Active || f.limits["LPC"].Watts != 4200 {
		t.Fatal("Grenze muss inaktiv auf Ursprungswert stehen", r.ReleaseVerified, f.limits)
	}
	if len(f.saved) != 1 || f.saved[0].Failsafe["LPC"].Power != 777 || f.saved[0].Limits["LPC"].Watts != 4200 || f.saved[0].ReportID != r.ID {
		t.Fatal(f.saved)
	}
	calls := strings.Join(f.calls, ",")
	if strings.Index(calls, "journal") > strings.Index(calls, "configure") || f.cleared != 1 || r.RestorePending {
		t.Fatal("Journal muss vor dem ersten Schreiben stehen und nach bestätigter Rückstellung gelöscht werden", calls, f.cleared)
	}
	if r.OriginalsRestored() != "yes" {
		t.Fatal(r.OriginalsRestored())
	}
}

func TestAcknowledgedButUnchangedRestoreFails047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, MPC: true})
	f.dropRestore = true
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s := step047(t, r, "LPC Failsafe-Leistung zurückstellen"); s.Passed || !strings.Contains(s.Detail, "500") {
		t.Fatal("Quittung ohne passenden Rücklesewert darf nicht bestehen", s)
	}
	if r.FailsafeRestore() != "unconfirmed" || r.OriginalsRestored() != "no" || f.cleared != 0 || !r.RestorePending {
		t.Fatal(r.FailsafeRestore(), r.OriginalsRestored(), f.cleared, r.RestorePending)
	}
}

func TestStillActiveLimitFailsRelease047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, MPC: true})
	f.stuckActive = true
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s := step047(t, r, "Grenzen aufheben"); s.Passed || !strings.Contains(s.Detail, "aktiv") {
		t.Fatal(s)
	}
	if r.ReleaseVerified == nil || *r.ReleaseVerified || f.cleared != 0 || !r.RestorePending {
		t.Fatal(r.ReleaseVerified, f.cleared)
	}
}

func TestUnsavedJournalBlocksDeviceWrites047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, MPC: true})
	f.saveErr = errors.New("Datenträger voll")
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(f.calls, ","), "configure") || len(f.writes) != 0 {
		t.Fatal("ohne gespeicherte Originalwerte keine Schreibbefehle", f.calls, f.writes)
	}
	if s := step047(t, r, "Failsafe-Werte setzen"); s.Passed {
		t.Fatal(s)
	}
	if r.FailsafeRestore() != "unchanged" || r.OriginalsRestored() != "unchanged" {
		t.Fatal(r.FailsafeRestore(), r.OriginalsRestored())
	}
}

func TestCancelStillRestoresAndReadsBack047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, MPC: true})
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.onLimit = cancel
	r, err := RunOnSite(ctx, c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if r.FailsafeRestore() != "confirmed" || f.failsafe["LPC"].Power != 777 || f.limits["LPC"].Active || f.cleared != 1 {
		t.Fatal(r.FailsafeRestore(), f.failsafe, f.limits, f.cleared)
	}
}

func TestRestoreOriginalsAfterRestart047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, LPP: true})
	f.failsafe["LPC"] = FailsafeValues{Power: 500, Minimum: 2 * time.Hour}
	f.limits["LPC"] = LimitState{Watts: 4200, HasValue: true, Known: true, Active: true}
	p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": {Power: 777, Minimum: 3 * time.Hour}}, Limits: map[string]LimitState{"LPC": {Watts: 4200, HasValue: true}}}
	steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond)
	if !ok || f.failsafe["LPC"] != (FailsafeValues{Power: 777, Minimum: 3 * time.Hour}) || f.limits["LPC"].Active || len(steps) == 0 {
		t.Fatal(ok, steps, f.failsafe, f.limits)
	}
	f.dropRestore = true
	f.failsafe["LPC"] = FailsafeValues{Power: 500, Minimum: 2 * time.Hour}
	if _, ok = RestoreOriginals(context.Background(), f, p, 50*time.Millisecond); ok {
		t.Fatal("ohne Rücklesebestätigung nicht ok")
	}
}

// R236-01: an inactive limit that still carries the test value is not restored.
func TestInactiveTestValueIsNotRestored047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, MPC: true})
	f.keepValue = true
	f.limits["LPC"] = LimitState{Watts: 6000, HasValue: true, Known: true}
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s := step047(t, r, "Grenzen aufheben"); s.Passed || !strings.Contains(s.Detail, "statt 6000 W") {
		t.Fatal(s)
	}
	if r.OriginalsRestored() != "no" || !r.RestorePending || f.cleared != 0 {
		t.Fatal(r.OriginalsRestored(), r.RestorePending, f.cleared)
	}
}

// R236-01: without a readable original limit nothing is written.
func TestUnreadableLimitBlocksWrites047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, MPC: true})
	f.limitErr = errors.New("keine Antwort")
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(f.calls, ","), "configure") || len(f.saved) != 0 || f.limitWrites != 0 || r.OriginalsRestored() != "unchanged" {
		t.Fatal(f.calls, f.saved, f.limitWrites, r.OriginalsRestored())
	}
}

// R236-04: after a crash before the first write the device already holds the
// originals; the restart restore writes nothing (not even a heartbeat).
func TestRestoreWritesOnlyDifferences047(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true})
	p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": f.failsafe["LPC"]}, Limits: map[string]LimitState{"LPC": f.limits["LPC"]}}
	steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond)
	if !ok || len(f.writes) != 0 || f.limitWrites != 0 || strings.Contains(strings.Join(f.calls, ","), "heartbeat") {
		t.Fatal(ok, steps, f.writes, f.limitWrites, f.calls)
	}
	// Only the changed minimum is written back; the limit release before it ends a
	// device failsafe state (236b).
	f.failsafe["LPC"] = FailsafeValues{Power: 777, Minimum: 2 * time.Hour}
	if _, ok = RestoreOriginals(context.Background(), f, p, 50*time.Millisecond); !ok || strings.Join(f.writes, ",") != "LPC-minimum" || f.limitWrites != 1 {
		t.Fatal(ok, f.writes, f.limitWrites)
	}
}

// 236b real-device finding: after a disconnect the Opta keeps LPP in failsafe state and
// rejects new failsafe values until a limit write; the restore must release the
// (already inactive) limit first, then write the failsafe values.
func TestRestoreLeavesDeviceFailsafeFirst236b(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, LPP: true})
	orig := map[string]LimitState{"LPC": f.limits["LPC"], "LPP": {Watts: 0, HasValue: true, Known: true}}
	p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": f.failsafe["LPC"], "LPP": f.failsafe["LPP"]}, Limits: orig}
	f.limits["LPP"] = orig["LPP"]
	f.failsafe["LPP"] = FailsafeValues{Power: 5000, Minimum: 3 * time.Hour}
	f.inFailsafe = map[string]bool{"LPP": true}
	steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond)
	if !ok || f.failsafe["LPP"] != p.Failsafe["LPP"] || f.limits["LPP"].Active || f.limitWrites != 1 {
		t.Fatal(ok, steps, f.failsafe, f.limits, f.limitWrites)
	}
}

// 236b real-device finding: the Opta shares one failsafe minimum between LPC and LPP.
// The LPC write already restores LPP's minimum; a second write is not sent.
func TestRestoreSharedMinimumWrittenOnce236b(t *testing.T) {
	f := newDevice047(Capabilities{LPC: true, LPP: true})
	p := PendingRestore{ReportID: "x", Failsafe: map[string]FailsafeValues{"LPC": f.failsafe["LPC"], "LPP": f.failsafe["LPP"]},
		Limits: map[string]LimitState{"LPC": f.limits["LPC"], "LPP": f.limits["LPP"]}}
	f.sharedMin = true
	f.failsafe["LPC"] = FailsafeValues{Power: 5000, Minimum: 2 * time.Hour}
	f.failsafe["LPP"] = FailsafeValues{Power: 5000, Minimum: 2 * time.Hour}
	steps, ok := RestoreOriginals(context.Background(), f, p, 50*time.Millisecond)
	if !ok || strings.Join(f.writes, ",") != "LPC-power,LPC-minimum,LPP-power" || f.failsafe["LPP"] != p.Failsafe["LPP"] {
		t.Fatal(ok, steps, f.writes, f.failsafe)
	}
}
