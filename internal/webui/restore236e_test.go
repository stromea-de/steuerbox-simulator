package webui

import (
	"context"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// 236d-01: after the automatic restore the session stays connected. The app must
// know the device's control functions from that connection, so "Prüfung starten"
// works without reconnecting (reconnecting sends LPP on a real Opta into failsafe).
// Loopback counterpart only.
func TestRestoreKnowsCapabilities236e(t *testing.T) {
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
	s.Disconnect()
	a := New(s)
	defer a.Close()
	for {
		a.mu.Lock()
		result, busy := a.restoreResult, a.busy
		a.mu.Unlock()
		if result != nil && !busy {
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
	if !s.LiveConnected() {
		t.Fatal("Sitzung muss nach der Wiederherstellung verbunden bleiben")
	}
	a.mu.Lock()
	caps, info := a.capabilities, a.deviceInfo
	a.mu.Unlock()
	if caps == nil || !caps.LPC || !caps.LPP {
		t.Fatalf("Steuerfunktionen nach Wiederherstellung unbekannt: %+v", caps)
	}
	if info == nil {
		t.Fatal("Gerätedaten nach Wiederherstellung fehlen")
	}
	c := inspection.DefaultDeviceConfig()
	if err := deviceConfig(&c, caps, s.Paired()); err != nil {
		t.Fatalf("Prüfstart nach Wiederherstellung abgelehnt: %v", err)
	}
}
