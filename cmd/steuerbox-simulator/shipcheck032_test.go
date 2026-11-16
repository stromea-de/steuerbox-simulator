package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
)

type closePeer032 struct {
	*checkPeer031
	closed   bool
	closeErr error
	order    []string
}

type rejectedPeer032 struct{ *checkPeer031 }

func (p *rejectedPeer032) Connect(context.Context) error {
	return errors.New("SHIP readiness deadline exceeded")
}
func (p *rejectedPeer032) ConnectionTimings() []eebus.ConnectionTiming {
	return []eebus.ConnectionTiming{{Phase: "TLS-Handshake", Status: "complete"}, {Phase: "SHIP-Handshake", Status: "failed", DurationMS: 10, Detail: "websocket: close 1006 (abnormal closure): unexpected EOF"}}
}
func TestShipCheck032DuplicateJSON(t *testing.T) {
	p := &rejectedPeer032{&checkPeer031{ski: ski031}}
	deps := checkServices031(t, p.checkPeer031)
	deps.newController = func(string) (shipCheckSession, error) { return p, nil }
	var out, errs bytes.Buffer
	code := executeWithServices(context.Background(), checkArgs031(t), &out, &errs, deps)
	r := readCheck031(t, &out)
	if code != 1 || r.Attempts[0].Disconnect.Reason != "peer_rejected_possible_duplicate_session" || !strings.Contains(r.Attempts[0].Disconnect.Detail, "Möglicherweise") || r.Passed {
		t.Fatal(code, r)
	}
}

func TestShipCheck032PauseTimingAndCancellation(t *testing.T) {
	peer := &checkPeer031{ski: ski031}
	deps := checkServices031(t, peer)
	var out, errs bytes.Buffer
	args := append(checkArgs031(t), "--repeat", "2", "--pause", "20ms")
	if code := executeWithServices(context.Background(), args, &out, &errs, deps); code != 0 {
		t.Fatal(code, errs.String())
	}
	r := readCheck031(t, &out)
	if len(r.Attempts) != 2 || r.Attempts[1].Started.Sub(r.Attempts[0].Finished) < 20*time.Millisecond {
		t.Fatal("next attempt started before close plus pause", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps.waitPause = func(ctx context.Context, d time.Duration) error { cancel(); return waitShipPause(ctx, d) }
	out.Reset()
	errs.Reset()
	if code := executeWithServices(ctx, args, &out, &errs, deps); code != 1 {
		t.Fatal(code)
	}
	r = readCheck031(t, &out)
	if len(r.Attempts) != 1 || r.Passed || r.Error != "context canceled" {
		t.Fatal(r)
	}
}

func (p *closePeer032) DisconnectAndWait(ctx context.Context) error {
	p.order = append(p.order, "closed")
	p.Disconnect()
	p.closed = true
	return p.closeErr
}

func TestShipCheck032CloseBeforeDefaultPause(t *testing.T) {
	p := &closePeer032{checkPeer031: &checkPeer031{ski: ski031}}
	deps := checkServices031(t, p.checkPeer031)
	deps.newController = func(string) (shipCheckSession, error) { return p, nil }
	deps.waitPause = func(ctx context.Context, d time.Duration) error {
		if !p.closed || d != 3*time.Second {
			t.Fatal("pause before close or wrong default", p.closed, d)
		}
		p.order = append(p.order, "pause")
		return nil
	}
	var out, errs bytes.Buffer
	if code := executeWithServices(context.Background(), append(checkArgs031(t), "--repeat", "2"), &out, &errs, deps); code != 0 {
		t.Fatal(code, errs.String())
	}
	r := readCheck031(t, &out)
	if r.PauseMS != 3000 || len(p.order) != 3 || p.order[0] != "closed" || p.order[1] != "pause" || p.order[2] != "closed" || !r.Attempts[0].Disconnect.Closed {
		t.Fatal(r, p.order)
	}
}
func TestShipCheck032CloseFailureStopsRepetition(t *testing.T) {
	p := &closePeer032{checkPeer031: &checkPeer031{ski: ski031}, closeErr: errors.New("close timed out")}
	deps := checkServices031(t, p.checkPeer031)
	deps.newController = func(string) (shipCheckSession, error) { return p, nil }
	deps.waitPause = func(context.Context, time.Duration) error { t.Fatal("pause after failed close"); return nil }
	var out, errs bytes.Buffer
	code := executeWithServices(context.Background(), append(checkArgs031(t), "--repeat", "3"), &out, &errs, deps)
	r := readCheck031(t, &out)
	if code != 1 || r.Passed || len(r.Attempts) != 1 || r.Attempts[0].Disconnect.Reason != "close_failed" || r.Attempts[0].Disconnect.Closed {
		t.Fatal(code, r)
	}
}
func TestShipCheck032DuplicateSessionEvidence(t *testing.T) {
	for _, tc := range []struct {
		tls           bool
		duration      int64
		phase, detail string
		want          bool
	}{
		{true, 15, "SHIP-Handshake", "websocket: close 1006 (abnormal closure): unexpected EOF", true},
		{true, 15, "WebSocket-Upgrade", "websocket: close 1006 (abnormal closure)", true},
		{false, 15, "SHIP-Handshake", "websocket: close 1006 (abnormal closure)", false},
		{true, 5000, "SHIP-Handshake", "websocket: close 1006 (abnormal closure)", false},
		{true, 15, "SHIP-Handshake", "websocket: close 4452 (rejected)", false},
	} {
		rows := []eebus.ConnectionTiming{{Phase: "TLS-Handshake", Status: "failed"}, {Phase: tc.phase, Status: "failed", DurationMS: tc.duration, Detail: tc.detail}}
		if tc.tls {
			rows[0].Status = "complete"
		}
		if got := possibleDuplicateSession(nil, rows); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	var out bytes.Buffer
	if code := execute(context.Background(), append(checkArgs031(t), "--pause", "-1s"), &out, &out); code != 2 {
		t.Fatal(code, out.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitShipPause(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
