package session

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/enbility/spine-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func connected047(t *testing.T) (*Session, string, context.Context) {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	if err = s.SelectDevice(s.Device, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	return s, dir, ctx
}

// originals047 reads the device's failsafe values and limit like a run does.
func originals047(t *testing.T, s *Session, ctx context.Context, id string) inspection.PendingRestore {
	t.Helper()
	original, err := s.ReadFailsafe(ctx, "LPC")
	if err != nil {
		t.Fatal(err)
	}
	limit, err := s.ReadLimit(ctx, "LPC")
	if err != nil || !limit.Known {
		t.Fatal(limit, err)
	}
	return inspection.PendingRestore{ReportID: id, Saved: time.Now(), Failsafe: map[string]inspection.FailsafeValues{"LPC": original}, Limits: map[string]inspection.LimitState{"LPC": limit}}
}

// 236: a crash after the first device write leaves the originals in the state
// directory; an explicit restore writes them back, reads them back and only then
// deletes the record. Loopback counterpart only, no real device.
func TestPendingRestoreAfterCrash047(t *testing.T) {
	s, _, ctx := connected047(t)
	original, err := s.ReadFailsafe(ctx, "LPC")
	if err != nil {
		t.Fatal(err)
	}
	limit, err := s.ReadLimit(ctx, "LPC")
	if err != nil || !limit.Known {
		t.Fatal(limit, err)
	}
	if err = s.SaveRestore(inspection.PendingRestore{ReportID: "r1", Saved: time.Now(), Failsafe: map[string]inspection.FailsafeValues{"LPC": original}, Limits: map[string]inspection.LimitState{"LPC": limit}}); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(s.restorePath(s.Device.SKI)); err != nil {
		t.Fatal(err)
	}
	// The interrupted test left its values in the device.
	for _, field := range []string{"power", "minimum"} {
		if err = s.WriteFailsafe(ctx, "LPC", field, inspection.FailsafeValues{Power: 500, Minimum: 2 * time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = s.Limit(ctx, "LPC", 4200, true); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.ReadLimit(ctx, "LPC"); !v.Active {
		t.Fatal("Testgrenze muss vor der Wiederherstellung aktiv gelesen werden", v)
	}
	p, err := s.LoadRestore(s.Device.SKI)
	if err != nil || p == nil || p.Device.SKI != s.Device.SKI || p.Failsafe["LPC"].Power != original.Power {
		t.Fatal(p, err)
	}
	steps, ok, err := s.RestorePending(ctx, s.Device.SKI)
	if err != nil || !ok {
		t.Fatal(err, steps)
	}
	got, err := s.ReadFailsafe(ctx, "LPC")
	if err != nil || got.Power != original.Power || got.Minimum != original.Minimum {
		t.Fatal(got, original, err)
	}
	if v, err := s.ReadLimit(ctx, "LPC"); err != nil || v.Active {
		t.Fatal(v, err)
	}
	if p, err := s.LoadRestore(s.Device.SKI); p != nil || err != nil {
		t.Fatal("Datensatz erst nach Bestätigung löschen", p, err)
	}
}

// A regular run deletes the record after the confirmed restore and proves no active limit.
func TestRunClearsPendingRestore047(t *testing.T) {
	s, _, ctx := connected047(t)
	c := inspection.DefaultDeviceConfig()
	c.Business, c.Inspector, c.Authorized = "Test", "Test", true
	c.Scope = inspection.ScopeConsumption
	c.PauseLowLoad = false
	c.StepTimeout = 2*eebus.HeartbeatInterval + time.Second
	c.CleanupTimeout = c.StepTimeout
	runCtx, cancel := context.WithCancel(ctx)
	go func() {
		for runCtx.Err() == nil {
			for _, v := range s.Progress().Steps {
				if v.Name == "Failsafe-Werte setzen" && v.Passed {
					if _, err := os.Stat(s.restorePath(s.Device.SKI)); err != nil {
						t.Error("Originalwerte müssen während der Prüfung gespeichert sein", err)
					}
					cancel()
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	r, err := s.Run(runCtx, c)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	if r.OriginalsRestored() != "yes" || r.ReleaseVerified == nil || !*r.ReleaseVerified || r.RestorePending {
		t.Fatal(r.OriginalsRestored(), r.ReleaseVerified, r.Steps)
	}
	if _, err := os.Stat(s.restorePath(s.Device.SKI)); !os.IsNotExist(err) {
		t.Fatal("Datensatz muss nach bestätigter Rückstellung gelöscht sein", err)
	}
}

// A new run on a device with unrestored originals restores them first and never
// records the test values as originals.
func TestRunRestoresPendingFirst047(t *testing.T) {
	s, _, ctx := connected047(t)
	journal := originals047(t, s, ctx, "alt")
	original := journal.Failsafe["LPC"]
	var err error
	if err = s.SaveRestore(journal); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"power", "minimum"} {
		if err = s.WriteFailsafe(ctx, "LPC", field, inspection.FailsafeValues{Power: 500, Minimum: 2 * time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	c := inspection.DefaultDeviceConfig()
	c.Business, c.Inspector, c.Authorized = "Test", "Test", true
	c.Scope = inspection.ScopeConsumption
	c.PauseLowLoad = false
	c.StepTimeout = 2*eebus.HeartbeatInterval + time.Second
	c.CleanupTimeout = c.StepTimeout
	runCtx, cancel := context.WithCancel(ctx)
	go func() {
		for runCtx.Err() == nil {
			for _, v := range s.Progress().Steps {
				if v.Name == "Failsafe-Werte setzen" {
					cancel()
					return
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	r, err := s.Run(runCtx, c)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	if got := r.FailsafeOriginals["LPC"]; got.Power != original.Power || got.Minimum != original.Minimum {
		t.Fatal("Ursprungswerte müssen die echten Originale sein", got, original)
	}
	if v, _ := s.ReadFailsafe(ctx, "LPC"); v.Power != original.Power {
		t.Fatal(v)
	}
}

// R236-02: open originals are never replaced, and only their own test deletes them.
func TestJournalIsExclusive047(t *testing.T) {
	s, _, _ := connected047(t)
	var err error
	first := inspection.PendingRestore{ReportID: "a", Failsafe: map[string]inspection.FailsafeValues{"LPC": {Power: 777, Minimum: 3 * time.Hour}}}
	if err = s.SaveRestore(first); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveRestore(inspection.PendingRestore{ReportID: "b", Failsafe: map[string]inspection.FailsafeValues{"LPC": {Power: 500, Minimum: 2 * time.Hour}}}); err == nil {
		t.Fatal("zweiter Datensatz darf die Originale nicht ersetzen")
	}
	if err = s.ClearRestore("b"); err == nil {
		t.Fatal("fremde Prüfung darf den Datensatz nicht löschen")
	}
	if p, _ := s.LoadRestore(s.Device.SKI); p == nil || p.Failsafe["LPC"].Power != 777 {
		t.Fatal(p)
	}
	if err = s.ClearRestore("a"); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.LoadRestore(s.Device.SKI); p != nil {
		t.Fatal(p)
	}
}

// R2-02: restoring an older record never deletes a newer one that replaced it.
func TestRestoreKeepsNewerJournal047(t *testing.T) {
	s, _, ctx := connected047(t)
	journal := originals047(t, s, ctx, "a")
	var err error
	if err = s.SaveRestore(journal); err != nil {
		t.Fatal(err)
	}
	stale, err := s.LoadRestore(s.Device.SKI)
	if err != nil || stale == nil {
		t.Fatal(stale, err)
	}
	// Another process restored A, deleted it and a new run journaled B.
	if err = s.ClearRestore("a"); err != nil {
		t.Fatal(err)
	}
	journal.ReportID = "b"
	if err = s.SaveRestore(journal); err != nil {
		t.Fatal(err)
	}
	// The stale record must look changed, otherwise nothing is written or deleted.
	for _, field := range []string{"power", "minimum"} {
		if err = s.WriteFailsafe(ctx, "LPC", field, inspection.FailsafeValues{Power: 500, Minimum: 2 * time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err = s.restoreConnected(ctx, stale); err == nil {
		t.Fatal("Löschen eines fremden Datensatzes muss scheitern")
	}
	if p, _ := s.LoadRestore(s.Device.SKI); p == nil || p.ReportID != "b" {
		t.Fatal("neuerer Datensatz gelöscht", p)
	}
}

// R2-05: the record pins the device entity; a different entity is never written.
func TestRestoreRefusesOtherEntity047(t *testing.T) {
	s, _, ctx := connected047(t)
	var err error
	if err = s.SaveRestore(originals047(t, s, ctx, "a")); err != nil {
		t.Fatal(err)
	}
	s.setPins(nil)
	p, err := s.LoadRestore(s.Device.SKI)
	if err != nil || p == nil || len(p.Entities["LPC"]) == 0 {
		t.Fatal("Entität muss gespeichert sein", p, err)
	}
	for _, field := range []string{"power", "minimum"} {
		if err = s.WriteFailsafe(ctx, "LPC", field, inspection.FailsafeValues{Power: 500, Minimum: 2 * time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	p.Entities["LPC"] = append(p.Entities["LPC"], 99)
	steps, ok, _ := s.restoreConnected(ctx, p)
	if ok {
		t.Fatal("andere Entität darf nicht beschrieben werden", steps)
	}
	if v, _ := s.ReadFailsafe(ctx, "LPC"); v.Power != 500 {
		t.Fatal("nichts schreiben", v)
	}
	if p, _ := s.LoadRestore(s.Device.SKI); p == nil {
		t.Fatal("Datensatz muss bleiben")
	}
}

// R3-01: while pinned, reads and writes reach only the pinned entity.
func TestPinnedEntityOnly047(t *testing.T) {
	s, _, ctx := connected047(t)
	real, err := s.entityAddress("LPC")
	if err != nil {
		t.Fatal(err)
	}
	s.setPins(map[string][]model.AddressEntityType{"LPC": append(append([]model.AddressEntityType(nil), real...), 99)})
	if _, err = s.ReadFailsafe(ctx, "LPC"); err == nil {
		t.Fatal("Lesen von anderer Entität")
	}
	if err = s.WriteFailsafe(ctx, "LPC", "power", inspection.FailsafeValues{Power: 500}); err == nil {
		t.Fatal("Schreiben auf andere Entität")
	}
	if _, err = s.ReadLimit(ctx, "LPC"); err == nil {
		t.Fatal("Grenze von anderer Entität")
	}
	if _, err = s.Limit(ctx, "LPC", 4200, true); err == nil {
		t.Fatal("Grenze auf andere Entität geschrieben")
	}
	s.setPins(map[string][]model.AddressEntityType{"LPC": real})
	if _, err = s.ReadFailsafe(ctx, "LPC"); err != nil {
		t.Fatal(err)
	}
	s.setPins(nil)
}

// R3-01: a cold session (no connection yet) connects before restoring.
func TestColdRunRestoresFirst047(t *testing.T) {
	s, _, ctx := connected047(t)
	journal := originals047(t, s, ctx, "alt")
	if err := s.SaveRestore(journal); err != nil {
		t.Fatal(err)
	}
	s.setPins(nil)
	for _, field := range []string{"power", "minimum"} {
		if err := s.WriteFailsafe(ctx, "LPC", field, inspection.FailsafeValues{Power: 500, Minimum: 2 * time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	s.Disconnect()
	steps, ok, err := s.RestorePending(ctx, s.Device.SKI)
	if err != nil || !ok {
		t.Fatal(err, steps)
	}
	if v, _ := s.ReadFailsafe(ctx, "LPC"); v.Power != journal.Failsafe["LPC"].Power {
		t.Fatal(v)
	}
}

// R3-02: a record left in a claim file is put back and restored, never ignored; a
// claim beside another record blocks new records and runs on that device.
func TestClaimFilesAreRecoveredOrBlock047(t *testing.T) {
	s, _, ctx := connected047(t)
	journal := originals047(t, s, ctx, "a")
	if err := s.SaveRestore(journal); err != nil {
		t.Fatal(err)
	}
	s.setPins(nil)
	path := s.restorePath(s.Device.SKI)
	if err := os.Rename(path, path+".claim-1-1"); err != nil {
		t.Fatal(err)
	}
	if p, err := s.LoadRestore(s.Device.SKI); err != nil || p == nil || p.ReportID != "a" || len(s.claims(s.Device.SKI)) != 0 {
		t.Fatal("Claim-Datei muss zurückgelegt werden", p, err, s.claims(s.Device.SKI))
	}
	if err := os.WriteFile(path+".claim-2-2", []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if p, err := s.LoadRestore(s.Device.SKI); err == nil {
		t.Fatal("weiterer offener Datensatz muss blockieren", p)
	}
	if err := s.ClearRestore("a"); err != nil {
		t.Fatal(err)
	}
	// The remaining claim is put back first; being unreadable it blocks the device.
	journal.ReportID = "b"
	if err := s.SaveRestore(journal); err == nil {
		t.Fatal("neuer Datensatz trotz offener Claim-Datei")
	}
	if p, err := s.LoadRestore(s.Device.SKI); err == nil || p != nil {
		t.Fatal("unlesbarer Datensatz muss blockieren", p, err)
	}
	if b := s.BrokenRestores(); len(b) != 1 || b[0] != s.Device.SKI+".json" {
		t.Fatal("unlesbarer Datensatz sichtbar", b)
	}
}

// R4-01: while a test holds the device, a second restore neither reads nor deletes
// its record.
func TestDeviceLockKeepsRunningJournal047(t *testing.T) {
	s, _, ctx := connected047(t)
	journal := originals047(t, s, ctx, "laufend")
	unlock, err := s.lockDevice(s.Device.SKI)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveRestore(journal); err != nil {
		t.Fatal(err)
	}
	s.setPins(nil)
	if _, _, err = s.RestorePending(ctx, s.Device.SKI); err == nil {
		t.Fatal("zweiter Vorgang trotz belegtem Gerät")
	}
	if p, err := s.LoadRestore(s.Device.SKI); err != nil || p == nil || p.ReportID != "laufend" {
		t.Fatal("Datensatz des laufenden Tests verloren", p, err)
	}
	if _, err = s.Run(ctx, inspection.Config{}); err == nil {
		t.Fatal("Prüfung trotz belegtem Gerät")
	}
	unlock()
	if steps, ok, err := s.RestorePending(ctx, s.Device.SKI); err != nil || !ok {
		t.Fatal(steps, ok, err)
	}
}

// R4-02: originals, journal and writes use the entity pinned before the read; a
// function outside the pinned set is not configured.
func TestPinBeforeOriginals047(t *testing.T) {
	s, _, ctx := connected047(t)
	if err := s.PinEntities([]string{"LPC"}); err != nil {
		t.Fatal(err)
	}
	pinned := s.pins["LPC"]
	journal := originals047(t, s, ctx, "pin")
	s.setPins(map[string][]model.AddressEntityType{"LPC": append(append([]model.AddressEntityType(nil), pinned...), 7)})
	if err := s.SaveRestore(journal); err != nil {
		t.Fatal(err)
	}
	p, err := s.LoadRestore(s.Device.SKI)
	if err != nil || p == nil || len(p.Entities["LPC"]) != len(pinned)+1 {
		t.Fatal("Journal nicht mit der vorab festgelegten Entität", p, err)
	}
	s.setPins(map[string][]model.AddressEntityType{"LPP": pinned})
	if s.useCaseEntity("LPC") != nil || !s.excluded("LPC") {
		t.Fatal("nicht festgelegte Funktion erreichbar")
	}
	if err = s.configureDevice(ctx, inspection.Config{Scope: inspection.ScopeConsumption, Failsafe: 1, FailsafeMinimum: 2 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	s.setPins(nil)
	if v, _ := s.ReadFailsafe(ctx, "LPC"); v.Power == 1 {
		t.Fatal("nicht festgelegte Funktion beschrieben")
	}
	if err := s.SaveRestore(journal); err == nil {
		// the earlier record is still open; a second one is refused (R236-02)
		t.Fatal("zweites Journal")
	}
}

// R5-04: claim recovery at startup leaves a device alone while a test or restore
// holds its lock; the claim belongs to that owner's delete check.
func TestRecoverAllClaimsSkipsLockedDevice236c(t *testing.T) {
	s, _, ctx := connected047(t)
	journal := originals047(t, s, ctx, "claim")
	if err := s.SaveRestore(journal); err != nil {
		t.Fatal(err)
	}
	path := s.restorePath(s.Device.SKI)
	claim := path + ".claim-1-1"
	if err := os.Rename(path, claim); err != nil {
		t.Fatal(err)
	}
	unlock, err := s.lockDevice(s.Device.SKI)
	if err != nil {
		t.Fatal(err)
	}
	s.RecoverAllClaims()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Claim trotz Gerätesperre zurückgelegt", err)
	}
	unlock()
	s.RecoverAllClaims()
	if _, err := os.Stat(path); err != nil {
		t.Fatal("Claim nach Freigabe nicht zurückgelegt", err)
	}
	if _, err := os.Stat(claim); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Claim-Datei bleibt", err)
	}
}
