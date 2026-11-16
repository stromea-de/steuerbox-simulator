package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
)

const ski031 = "0123456789abcdef0123456789abcdef01234567"

type checkPeer031 struct {
	loseAfter                     time.Duration
	connectedAt                   time.Time
	waitDeadline                  bool
	trustReads                    int
	device                        discovery.Device
	ski                           string
	connects, disconnects, closes int
	live                          bool
	failAt                        int
	lost                          bool
	cancel                        context.CancelFunc
}

func (p *checkPeer031) SelectDevice(d discovery.Device, confirmed bool) error {
	if !confirmed {
		return errors.New("not confirmed")
	}
	p.device = d
	return nil
}
func (p *checkPeer031) Connect(ctx context.Context) error {
	p.connects++
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing deadline")
	}
	if p.cancel != nil {
		p.cancel()
		return context.Canceled
	}
	if p.waitDeadline {
		<-ctx.Done()
		return nil
	}
	if p.connects == p.failAt {
		return &eebus.ConnectionError{Phase: "TLS-Handshake", Duration: 3 * time.Millisecond, Err: errors.New("SKI mismatch")}
	}
	p.live = true
	p.connectedAt = time.Now()
	p.trustReads = 0
	return nil
}
func (p *checkPeer031) DisconnectAndWait(context.Context) error { p.Disconnect(); return nil }
func (p *checkPeer031) Disconnect()                             { p.disconnects++; p.live = false }
func (p *checkPeer031) Close()                                  { p.closes++ }
func (p *checkPeer031) Snapshot() (discovery.Device, string, bool, bool, string) {
	return p.device, p.ski, true, true, "test transport closed"
}
func (p *checkPeer031) LiveTrust() eebus.TrustState {
	p.trustReads++
	live := p.live && (!p.lost || p.trustReads == 1)
	if p.loseAfter > 0 && time.Since(p.connectedAt) >= p.loseAfter {
		live = false
	}
	return eebus.TrustState{Device: live, Simulator: live}
}
func (p *checkPeer031) ConnectionTimings() []eebus.ConnectionTiming {
	if p.connects == p.failAt {
		return []eebus.ConnectionTiming{{Phase: "TCP-Verbindung", Status: "complete"}, {Phase: "TLS-Handshake", Status: "failed", DurationMS: 3}}
	}
	return []eebus.ConnectionTiming{{Phase: "TCP-Verbindung", Status: "complete"}, {Phase: "TLS-Handshake", Status: "complete"}, {Phase: "WebSocket-Upgrade", Status: "complete"}, {Phase: "SHIP-Handshake", Status: "complete"}}
}
func checkServices031(t *testing.T, p *checkPeer031) cliServices {
	t.Helper()
	config := t.TempDir()
	return cliServices{
		waitPause:     waitShipPause,
		configDir:     func() (string, error) { return config, nil },
		newController: func(string) (shipCheckSession, error) { return p, nil },
		interfaces: func() ([]discovery.Interface, error) {
			return []discovery.Interface{{Name: "mock-a"}, {Name: "mock-b"}}, nil
		},
		searchLAN: func(context.Context, string) ([]discovery.Device, error) {
			t.Fatal("unexpected LAN search")
			return nil, nil
		},
	}
}
func checkArgs031(t *testing.T) []string {
	t.Helper()
	return []string{"ship-check", "--address", "127.0.0.1:4711", "--ski", ski031, "--hold", "2ms", "--state", t.TempDir()}
}
func readCheck031(t *testing.T, b *bytes.Buffer) shipCheckReport {
	t.Helper()
	var r shipCheckReport
	d := json.NewDecoder(b)
	if err := d.Decode(&r); err != nil {
		t.Fatal(err, b.String())
	}
	if err := d.Decode(new(any)); err != io.EOF {
		t.Fatal("stdout contains more than one JSON document", err)
	}
	return r
}
func TestShipCheck031Usage(t *testing.T) {
	for _, args := range [][]string{
		{}, {"--address", "127.0.0.1:4711"}, {"--address", "127.0.0.1:4711", "--ski", "bad"},
		{"--address", "https://opta.local/ship/", "--ski", ski031}, {"--address", "127.0.0.1:0", "--ski", ski031},
		{"--address", "8.8.8.8:4711", "--ski", ski031}, {"--address", "127.0.0.1", "--ski", ski031},
	} {
		var out, errs bytes.Buffer
		if code := execute(context.Background(), append([]string{"ship-check"}, args...), &out, &errs); code != 2 {
			t.Fatalf("%v: %d %s", args, code, errs.String())
		}
		if out.Len() != 0 {
			t.Fatal("usage produced result", out.String())
		}
	}
	for _, extra := range [][]string{{"--hold", "0s"}, {"--repeat", "0"}, {"--repeat", "1001"}, {"--connect-timeout", "0s"}, {"--discovery-timeout", "-1s"}, {"extra"}} {
		var out, errs bytes.Buffer
		if code := execute(context.Background(), append(checkArgs031(t), extra...), &out, &errs); code != 2 {
			t.Fatal(extra, code, errs.String())
		}
	}
	var b bytes.Buffer
	if code := execute(context.Background(), []string{"ship-check", "--help"}, &b, &b); code != 0 || !strings.Contains(b.String(), "--hold") && !strings.Contains(b.String(), "-hold") {
		t.Fatal(code, b.String())
	}
}
func TestShipCheck031RepeatAndPrivateJSON(t *testing.T) {
	p := &checkPeer031{ski: ski031}
	deps := checkServices031(t, p)
	path := filepath.Join(t.TempDir(), "result.json")
	args := append(checkArgs031(t), "--repeat", "2", "--pause", "0s", "--interface", "mock-a", "--json", path)
	var out, errs bytes.Buffer
	if code := executeWithServices(context.Background(), args, &out, &errs, deps); code != 0 {
		t.Fatal(code, errs.String(), out.String())
	}
	r := readCheck031(t, &out)
	if !r.Passed || len(r.Attempts) != 2 || r.LocalSKI != ski031 || p.connects != 2 || p.disconnects != 2 || p.closes != 1 {
		t.Fatal(r, p)
	}
	for _, a := range r.Attempts {
		if !a.Passed || !a.Hold.Passed || a.Hold.ActualMS < 2 || a.Disconnect.Reason != "hold_complete" || len(a.Phases) != 4 || a.Device.Interface != "mock-a" {
			t.Fatal(a)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved shipCheckReport
	if err = json.Unmarshal(data, &saved); err != nil || !saved.Passed {
		t.Fatal(err, string(data))
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	if strings.Contains(string(data), "PRIVATE KEY") || strings.Contains(string(data), "Certificate") {
		t.Fatal("credentials in JSON")
	}
}
func TestShipCheck031FailureDoesNotBecomePass(t *testing.T) {
	for _, tc := range []struct {
		name   string
		peer   checkPeer031
		reason string
	}{{"TLS failure", checkPeer031{failAt: 1}, "connect_failed"}, {"drop during hold", checkPeer031{lost: true}, "connection_lost"}} {
		t.Run(tc.name, func(t *testing.T) {
			p := &tc.peer
			p.ski = ski031
			var out, errs bytes.Buffer
			code := executeWithServices(context.Background(), append(checkArgs031(t), "--repeat", "2", "--pause", "0s"), &out, &errs, checkServices031(t, p))
			r := readCheck031(t, &out)
			if code != 1 || r.Passed || len(r.Attempts) != 2 || r.Attempts[0].Passed || r.Attempts[0].Disconnect.Reason != tc.reason {
				t.Fatal(code, r)
			}
			if tc.reason == "connect_failed" && (r.Attempts[0].Phases[1].Status != "failed" || !r.Attempts[1].Passed) {
				t.Fatal(r)
			}
		})
	}
}
func TestShipCheck031CancelAndIOFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &checkPeer031{ski: ski031, cancel: cancel}
	var out, errs bytes.Buffer
	if code := executeWithServices(ctx, append(checkArgs031(t), "--repeat", "3"), &out, &errs, checkServices031(t, p)); code != 1 {
		t.Fatal(code)
	}
	r := readCheck031(t, &out)
	if len(r.Attempts) != 1 || r.Attempts[0].Disconnect.Reason != "canceled" || r.Passed {
		t.Fatal(r)
	}
	p = &checkPeer031{ski: ski031}
	out.Reset()
	errs.Reset()
	if code := executeWithServices(context.Background(), append(checkArgs031(t), "--json", filepath.Join(t.TempDir(), "absent", "result.json")), &out, &errs, checkServices031(t, p)); code != 1 {
		t.Fatal(code)
	}
	r = readCheck031(t, &out)
	if r.Passed || !strings.Contains(r.Error, "JSON") {
		t.Fatal(r)
	}
}
func TestShipCheck031DefaultIdentityIsReused(t *testing.T) {
	base := t.TempDir()
	state, err := defaultStateDir(base)
	if err != nil {
		t.Fatal(err)
	}
	id, err := identity.LoadOrCreate(filepath.Join(state, "energyguard"))
	if err != nil {
		t.Fatal(err)
	}
	p := &checkPeer031{ski: id.SKI}
	deps := checkServices031(t, p)
	deps.configDir = func() (string, error) { return base, nil }
	deps.newController = func(path string) (shipCheckSession, error) {
		if path != state {
			t.Fatalf("state %s want %s", path, state)
		}
		got, e := identity.LoadOrCreate(filepath.Join(path, "energyguard"))
		if e != nil || got.SKI != id.SKI {
			t.Fatal(got.SKI, e)
		}
		return p, nil
	}
	var out, errs bytes.Buffer
	args := []string{"ship-check", "--address", "127.0.0.1:4711", "--ski", ski031, "--hold", "1ms"}
	if code := executeWithServices(context.Background(), args, &out, &errs, deps); code != 0 {
		t.Fatal(code, errs.String())
	}
	if r := readCheck031(t, &out); r.LocalSKI != id.SKI {
		t.Fatal(r)
	}
}
func TestShipCheck031MDNSAndDiscoveryJSON(t *testing.T) {
	p := &checkPeer031{ski: ski031}
	deps := checkServices031(t, p)
	var searched []string
	deps.searchLAN = func(_ context.Context, name string) ([]discovery.Device, error) {
		searched = append(searched, name)
		return []discovery.Device{{Name: "Opta", Host: "opta.local.", ServiceName: "Opta._ship._tcp.local.", Address: "127.0.0.1:4711", SKI: ski031, Interface: name, LocalAddress: "127.0.0.1"}}, nil
	}
	for _, target := range []string{"Opta", "OPTA.local:4711", "Opta._ship._tcp.local."} {
		args := append(checkArgs031(t), "--address", target, "--interface", "mock-a")
		var out, errs bytes.Buffer
		if code := executeWithServices(context.Background(), args, &out, &errs, deps); code != 0 {
			t.Fatal(code, errs.String(), out.String())
		}
		r := readCheck031(t, &out)
		if r.Attempts[0].Device.Address != "127.0.0.1:4711" {
			t.Fatal(r)
		}
	}
	var out, errs bytes.Buffer
	if code := executeWithServices(context.Background(), append(checkArgs031(t), "--address", "opta.local"), &out, &errs, deps); code != 1 {
		t.Fatal("ambiguous route accepted", code, out.String())
	}
	out.Reset()
	errs.Reset()
	if code := executeWithServices(context.Background(), []string{"discover", "--json", "--lan", "--interface", "mock-b"}, &out, &errs, deps); code != 0 {
		t.Fatal(code, errs.String())
	}
	var devices []discovery.Device
	if err := json.Unmarshal(out.Bytes(), &devices); err != nil || len(devices) != 1 || devices[0].Interface != "mock-b" {
		t.Fatal(err, out.String())
	}
	if len(searched) < 5 {
		t.Fatal(searched)
	}
}
