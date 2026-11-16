package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
)

func TestConnectionEvidence027(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	s.ResetProgress() // Preparation evidence must survive starting the inspection.
	events := s.ConnectionEvents()
	if len(events) != 4 {
		t.Fatal(events)
	}
	for i, phase := range []string{"TCP-Verbindung", "TLS-Handshake", "WebSocket-Upgrade", "SHIP-Handshake"} {
		if events[i].Name != phase || !events[i].Passed || !strings.Contains(events[i].Detail, " ms") {
			t.Fatal(events)
		}
	}
	r := inspection.Report{Events: events}
	pdf, err := report.PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"TCP-Verbindung", "TLS-Handshake", "WebSocket-Upgrade", "SHIP-Handshake"} {
		if !strings.Contains(string(pdf), phase) {
			t.Fatal("missing PDF phase", phase)
		}
	}
	// A subsequent device selection must not carry another device's diagnostics.
	other := s.Device
	other.Address = "127.0.0.1:4711"
	if err = s.SelectDevice(other, true); err != nil {
		t.Fatal(err)
	}
	if len(s.ConnectionEvents()) != 0 {
		t.Fatal("previous device's evidence retained")
	}
}
