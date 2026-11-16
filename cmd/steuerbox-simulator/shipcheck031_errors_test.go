package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
)

func TestShipCheck031HoldAndDeadline(t *testing.T) {
	for _, tc := range []struct {
		name    string
		peer    checkPeer031
		options []string
		reason  string
	}{
		{"observed drop", checkPeer031{loseAfter: 5 * time.Millisecond}, []string{"--hold", "200ms"}, "connection_lost"},
		{"connect deadline cannot become success", checkPeer031{waitDeadline: true}, []string{"--connect-timeout", "5ms"}, "connect_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &tc.peer
			p.ski = ski031
			var out, errs bytes.Buffer
			code := executeWithServices(context.Background(), append(checkArgs031(t), tc.options...), &out, &errs, checkServices031(t, p))
			r := readCheck031(t, &out)
			if code != 1 || r.Passed || r.Attempts[0].Disconnect.Reason != tc.reason {
				t.Fatal(code, r)
			}
			if tc.reason == "connection_lost" && (r.Attempts[0].Hold.ActualMS < 5 || r.Attempts[0].Hold.ActualMS >= 200) {
				t.Fatal(r.Attempts[0].Hold)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	p := &checkPeer031{live: true}
	held, err := holdSHIP(ctx, p, time.Second)
	if !errors.Is(err, context.DeadlineExceeded) || held >= time.Second {
		t.Fatal(held, err)
	}
}

type failedWriter031 struct{}

func (failedWriter031) Write([]byte) (int, error) { return 0, errors.New("output closed") }

func TestShipCheck031InitializationAndOutputFailures(t *testing.T) {
	p := &checkPeer031{ski: ski031}
	deps := checkServices031(t, p)
	deps.newController = func(string) (shipCheckSession, error) { return nil, errors.New("initialization failed") }
	var out, errs bytes.Buffer
	if code := executeWithServices(context.Background(), checkArgs031(t), &out, &errs, deps); code != 1 {
		t.Fatal(code)
	}
	r := readCheck031(t, &out)
	if r.Passed || r.Error != "initialization failed" || r.Attempts == nil || len(r.Attempts) != 0 {
		t.Fatal(r)
	}
	if code := executeWithServices(context.Background(), checkArgs031(t), failedWriter031{}, &errs, checkServices031(t, p)); code != 1 {
		t.Fatal("stdout error ignored", code)
	}
}

func TestShipCheck031MDNSPinAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		device    discovery.Device
		searchErr error
	}{
		{"wrong announced key", discovery.Device{Name: "Opta", SKI: "1123456789abcdef0123456789abcdef01234567", Address: "127.0.0.1:4711"}, nil},
		{"wrong port", discovery.Device{Name: "Opta", SKI: ski031, Address: "127.0.0.1:4712"}, nil},
		{"adapter error", discovery.Device{}, errors.New("adapter gone")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &checkPeer031{ski: ski031}
			deps := checkServices031(t, p)
			deps.searchLAN = func(context.Context, string) ([]discovery.Device, error) {
				return []discovery.Device{tc.device}, tc.searchErr
			}
			var out, errs bytes.Buffer
			code := executeWithServices(context.Background(), append(checkArgs031(t), "--address", "Opta:4711", "--interface", "mock-a"), &out, &errs, deps)
			r := readCheck031(t, &out)
			if code != 1 || r.Passed || p.connects != 0 || r.Attempts[0].Disconnect.Reason != "resolution_failed" {
				t.Fatal(code, r, p.connects)
			}
		})
	}
}

func TestDiscover031JSONEmptyErrorsAndCancel(t *testing.T) {
	deps := checkServices031(t, &checkPeer031{})
	deps.newController = func(string) (shipCheckSession, error) {
		t.Fatal("read-only discovery initialized identity/session")
		return nil, nil
	}
	deps.searchLAN = func(context.Context, string) ([]discovery.Device, error) { return nil, context.DeadlineExceeded }
	var out, errs bytes.Buffer
	args := []string{"discover", "--json", "--lan", "--interface", "mock-a"}
	if code := executeWithServices(context.Background(), args, &out, &errs, deps); code != 0 || out.String() != "[]\n" {
		t.Fatal(code, out.String(), errs.String())
	}
	if code := executeWithServices(context.Background(), args, failedWriter031{}, &errs, deps); code != 1 {
		t.Fatal(code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	if code := executeWithServices(ctx, args, &out, &errs, deps); code != 1 || out.Len() != 0 {
		t.Fatal(code, out.String())
	}
	for _, extra := range [][]string{{"--timeout", "0s"}, {"--timeout", "2m"}} {
		if code := executeWithServices(context.Background(), append(args, extra...), &out, &errs, deps); code != 2 {
			t.Fatal(code)
		}
	}
}
