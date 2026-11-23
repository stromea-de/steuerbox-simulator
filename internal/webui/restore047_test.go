package webui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// 236: after a restart the app restores the originals of an interrupted test
// automatically, without a dialog, and drops the record only after readback.
// Loopback counterpart only.
func TestStartupRestoresPendingOriginals047(t *testing.T) {
	s, err := session.New(t.TempDir())
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	original, err := s.ReadFailsafe(ctx, "LPC")
	if err != nil {
		t.Fatal(err)
	}
	limit, err := s.ReadLimit(ctx, "LPC")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SaveRestore(inspection.PendingRestore{ReportID: "alt", Failsafe: map[string]inspection.FailsafeValues{"LPC": original}, Limits: map[string]inspection.LimitState{"LPC": limit}}); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"power", "minimum"} {
		if err = s.WriteFailsafe(ctx, "LPC", field, inspection.FailsafeValues{Power: 500, Minimum: 2 * time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	a := New(s)
	defer a.Close()
	for {
		a.mu.Lock()
		result := a.restoreResult
		a.mu.Unlock()
		if result != nil {
			if !result.OK {
				t.Fatal(result)
			}
			break
		}
		if ctx.Err() != nil {
			t.Fatal("keine automatische Wiederherstellung")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(s.PendingRestores()) != 0 {
		t.Fatal("Datensatz muss nach bestätigter Wiederherstellung gelöscht sein")
	}
	if v, err := s.ReadFailsafe(ctx, "LPC"); err != nil || v.Power != original.Power || v.Minimum != original.Minimum {
		t.Fatal(v, original, err)
	}
}

// R2-06: an unreadable record is never skipped silently; it is the visible outcome.
func TestBrokenJournalIsVisible047(t *testing.T) {
	dir := t.TempDir()
	s, err := session.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = os.MkdirAll(filepath.Join(dir, "pending-restore"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "pending-restore", "kaputt.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := New(s)
	defer a.Close()
	deadline := time.Now().Add(10 * time.Second)
	for {
		a.mu.Lock()
		result := a.restoreResult
		a.mu.Unlock()
		if result != nil {
			if result.OK || result.Error == "" || result.Device != "kaputt.json" {
				t.Fatal(result)
			}
			// R4-05: the status poll names the unreadable record as open, too.
			a.mu.Lock()
			views := a.pendingRestoresLocked()
			a.mu.Unlock()
			if len(views) != 1 || views[0].Device != "kaputt.json" {
				t.Fatal(views)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("unlesbarer Datensatz muss sichtbar werden")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
