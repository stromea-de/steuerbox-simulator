package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// 043 (Echtgerät 2026-11-14, Bericht VWAM…): a connection lost during the power decision must be recorded as
// connection_lost, so the result names the cause instead of „unvollständig“ without a reason.
func TestLowLoadConnectionLossIsConnectionLost043(t *testing.T) {
	s, err := NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.EG.Close()
	if s.EG.Alive() {
		t.Fatal("fresh endpoint must not be alive")
	}
	_, e := s.LowLoad(context.Background(), inspection.LowLoad{Direction: "LPP"})
	if !errors.Is(e, inspection.ErrConnectionLost) {
		t.Fatalf("want ErrConnectionLost, got %v", e)
	}
	if got := inspection.AbortReason(e); got != "connection_lost" {
		t.Fatalf("abort reason %q", got)
	}
}

// R043-04: an expired power decision is marked as such (not a measurement timeout), still classified as timeout.
func TestLowLoadDecisionTimeoutIsMarked043(t *testing.T) {
	s := &Session{lowLoadTimeout: 20 * time.Millisecond}
	_, e := s.LowLoad(context.Background(), inspection.LowLoad{Direction: "LPP"})
	if !errors.Is(e, inspection.ErrNoLoadDecision) || !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("want ErrNoLoadDecision and DeadlineExceeded, got %v", e)
	}
	if got := inspection.AbortReason(e); got != "timeout" {
		t.Fatalf("abort reason %q", got)
	}
}
