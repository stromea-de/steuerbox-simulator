package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stromea-de/steuerbox-simulator/internal/branding"
	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// The CLI uses the same session as the WebUI. This narrow boundary permits
// deterministic orchestration tests without opening sockets or contacting a LAN.
type shipCheckSession interface {
	SelectDevice(discovery.Device, bool) error
	Connect(context.Context) error
	Disconnect()
	DisconnectAndWait(context.Context) error
	Close()
	Snapshot() (discovery.Device, string, bool, bool, string)
	ConnectionTimings() []eebus.ConnectionTiming
	LiveTrust() eebus.TrustState
}

type cliServices struct {
	waitPause     func(context.Context, time.Duration) error
	configDir     func() (string, error)
	newController func(string) (shipCheckSession, error)
	interfaces    func() ([]discovery.Interface, error)
	searchLAN     func(context.Context, string) ([]discovery.Device, error)
	searchMDNS    func(context.Context) ([]discovery.Device, error)
}

func defaultCLIServices() cliServices {
	return cliServices{
		configDir:     os.UserConfigDir,
		waitPause:     waitShipPause,
		newController: func(state string) (shipCheckSession, error) { return session.NewController(state) },
		interfaces:    discovery.Interfaces, searchLAN: discovery.SearchLAN, searchMDNS: discovery.SearchMDNS,
	}
}

type checkTarget struct{ address, name, port string }
type shipCheckOptions struct {
	target                                     checkTarget
	ski, adapter, state, jsonPath              string
	hold, connectTimeout, searchTimeout, pause time.Duration
	repeat                                     int
}

type shipCheckReport struct {
	SchemaVersion      int                `json:"schemaVersion"`
	Command            string             `json:"command"`
	Target             string             `json:"target"`
	ExpectedSKI        string             `json:"expectedSki"`
	LocalSKI           string             `json:"localSki,omitempty"`
	Interface          string             `json:"interface,omitempty"`
	PauseMS            int64              `json:"pauseMs"`
	RequestedAttempts  int                `json:"requestedAttempts"`
	RequestedHoldMS    int64              `json:"requestedHoldMs"`
	ConnectTimeoutMS   int64              `json:"connectTimeoutMs"`
	DiscoveryTimeoutMS int64              `json:"discoveryTimeoutMs"`
	Started            time.Time          `json:"started"`
	Finished           time.Time          `json:"finished"`
	Passed             bool               `json:"passed"`
	Attempts           []shipCheckAttempt `json:"attempts"`
	Error              string             `json:"error,omitempty"`
}
type shipCheckAttempt struct {
	Index             int                      `json:"index"`
	Started           time.Time                `json:"started"`
	Finished          time.Time                `json:"finished"`
	Device            discovery.Device         `json:"device"`
	ConnectionPassed  bool                     `json:"connectionPassed"`
	ConnectDurationMS int64                    `json:"connectDurationMs"`
	Phases            []eebus.ConnectionTiming `json:"phases"`
	Hold              shipCheckHold            `json:"hold"`
	Disconnect        shipCheckDisconnect      `json:"disconnect"`
	Passed            bool                     `json:"passed"`
	Error             string                   `json:"error,omitempty"`
}
type shipCheckHold struct {
	RequestedMS int64 `json:"requestedMs"`
	ActualMS    int64 `json:"actualMs"`
	Passed      bool  `json:"passed"`
}
type shipCheckDisconnect struct {
	Closed bool      `json:"closed"`
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
	Detail string    `json:"detail,omitempty"`
}

func parseCheckTarget(raw string) (checkTarget, error) {
	raw = strings.TrimSpace(raw)
	bad := errors.New("--address benötigt lokale IP:Port oder einen angekündigten mDNS-Namen (optional :Port)")
	if raw == "" || len(raw) > 300 || strings.ContainsAny(raw, "/\\@\r\n\t") {
		return checkTarget{}, bad
	}
	if eebus.DeviceAddress(raw) == nil {
		return checkTarget{address: raw}, nil
	}
	host, port := raw, ""
	if strings.Contains(raw, ":") {
		var err error
		host, port, err = net.SplitHostPort(raw)
		if err != nil {
			return checkTarget{}, bad
		}
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return checkTarget{}, bad
		}
		port = strconv.Itoa(p)
	}
	ip, _, _ := strings.Cut(host, "%")
	if net.ParseIP(ip) != nil || strings.ContainsAny(host, "%[]") {
		return checkTarget{}, bad
	}
	name := strings.TrimSuffix(host, ".")
	if name == "" || strings.Contains(name, ".") && !strings.HasSuffix(strings.ToLower(name), ".local") {
		return checkTarget{}, bad
	}
	return checkTarget{name: name, port: port}, nil
}

func executeShipCheck(ctx context.Context, args []string, out, errout io.Writer, deps cliServices) int {
	base, configErr := deps.configDir()
	f := flag.NewFlagSet("ship-check", flag.ContinueOnError)
	f.SetOutput(errout)
	f.Usage = func() {
		fmt.Fprintf(errout, "Aufruf: %s ship-check --address IP:Port|mDNS-Name --ski SKI [Optionen]\n", branding.Slug)
		f.PrintDefaults()
	}
	var o shipCheckOptions
	raw := f.String("address", "", "Ziel: lokale IP:Port oder mDNS-Name (ohne Port: aus DNS-SD)")
	f.StringVar(&o.ski, "ski", "", "Erwartete, am Gerät geprüfte SKI (40 Hex-Zeichen)")
	f.StringVar(&o.adapter, "interface", "", "Adapterbindung wie WebUI; bei mDNS ohne Angabe auf allen geeigneten Adaptern suchen")
	f.StringVar(&o.state, "state", filepath.Join(base, branding.Slug), "Datenverzeichnis wie WebUI; vorhandene energyguard-Identität verwenden")
	f.DurationVar(&o.hold, "hold", 60*time.Second, "Haltedauer je erfolgreicher Verbindung (0 < Dauer <= 24h)")
	f.DurationVar(&o.pause, "pause", 3*time.Second, "Pause nach vollständigem lokalem Schließen vor dem nächsten Versuch (0 bis 10m)")
	f.IntVar(&o.repeat, "repeat", 1, "Anzahl unabhängiger Verbindungen (1 bis 1000)")
	f.DurationVar(&o.connectTimeout, "connect-timeout", time.Minute, "Zeitlimit je Sitzungsaufbau (0 < Dauer <= 10m; Transportbudget weiterhin 15s)")
	f.DurationVar(&o.searchTimeout, "discovery-timeout", 6*time.Second, "mDNS-Suchdauer je Adapter (0 < Dauer <= 1m)")
	f.StringVar(&o.jsonPath, "json", "", "Zusätzliches JSON-Ziel; stdout enthält immer genau ein Ergebnisobjekt")
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	var err error
	o.target, err = parseCheckTarget(*raw)
	if err == nil {
		o.ski, err = identity.NormalizeSKI(o.ski)
	}
	if err != nil || f.NArg() != 0 || o.hold <= 0 || o.hold > 24*time.Hour || o.repeat < 1 || o.repeat > 1000 || o.pause < 0 || o.pause > 10*time.Minute || o.connectTimeout <= 0 || o.connectTimeout > 10*time.Minute || o.searchTimeout <= 0 || o.searchTimeout > time.Minute {
		if err != nil {
			fmt.Fprintln(errout, err)
		} else {
			fmt.Fprintln(errout, "Ungültige Optionen: Haltedauer, Wiederholungen und Zeitlimits prüfen; keine Positionsargumente.")
		}
		return 2
	}
	explicitState := false
	f.Visit(func(v *flag.Flag) {
		if v.Name == "state" {
			explicitState = true
		}
	})
	if !explicitState {
		if configErr != nil {
			err = configErr
		} else {
			o.state, err = defaultStateDir(base)
		}
	}
	r := shipCheckReport{SchemaVersion: 1, Command: "ship-check", Target: *raw, ExpectedSKI: o.ski, Interface: o.adapter, RequestedAttempts: o.repeat, PauseMS: o.pause.Milliseconds(), RequestedHoldMS: o.hold.Milliseconds(), ConnectTimeoutMS: o.connectTimeout.Milliseconds(), DiscoveryTimeoutMS: o.searchTimeout.Milliseconds(), Started: time.Now(), Attempts: []shipCheckAttempt{}}
	if err != nil {
		r.Error = err.Error()
	} else {
		runShipCheck(ctx, o, deps, &r)
	}
	r.Finished = time.Now()
	data, _ := json.MarshalIndent(r, "", "  ")
	data = append(data, '\n')
	if o.jsonPath != "" {
		if err := writePrivate(o.jsonPath, data); err != nil {
			r.Passed = false
			r.Error = "JSON-Export: " + err.Error()
			data, _ = json.MarshalIndent(r, "", "  ")
			data = append(data, '\n')
		}
	}
	if _, err := out.Write(data); err != nil {
		fmt.Fprintln(errout, "JSON-Ausgabe:", err)
		return 1
	}
	if !r.Passed {
		if r.Error != "" {
			fmt.Fprintln(errout, r.Error)
		}
		return 1
	}
	return 0
}

func runShipCheck(ctx context.Context, o shipCheckOptions, deps cliServices, r *shipCheckReport) {
	if err := ctx.Err(); err != nil {
		r.Error = err.Error()
		return
	}
	s, err := deps.newController(o.state)
	if err != nil {
		r.Error = err.Error()
		return
	}
	defer s.Close()
	_, r.LocalSKI, _, _, _ = s.Snapshot()
	r.Passed = true
	for i := 1; i <= o.repeat; i++ {
		if err := ctx.Err(); err != nil {
			r.Passed = false
			r.Error = err.Error()
			break
		}
		if i > 1 {
			if err := deps.waitPause(ctx, o.pause); err != nil {
				r.Passed = false
				r.Error = err.Error()
				break
			}
		}
		a := checkShipAttempt(ctx, o, deps, s, i)
		r.Attempts = append(r.Attempts, a)
		if !a.Passed {
			r.Passed = false
		}
		if !a.Disconnect.Closed {
			break
		}
	}
}

func checkShipAttempt(ctx context.Context, o shipCheckOptions, deps cliServices, s shipCheckSession, index int) (a shipCheckAttempt) {
	a = shipCheckAttempt{Index: index, Started: time.Now(), Phases: []eebus.ConnectionTiming{}, Hold: shipCheckHold{RequestedMS: o.hold.Milliseconds()}}
	defer func() {
		if ctx.Err() != nil && !a.Passed {
			a.Disconnect.Reason = "canceled"
		}
		a.Disconnect.At = time.Now()
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := s.DisconnectAndWait(closeCtx)
		cancel()
		a.Disconnect.Closed = err == nil
		if err != nil {
			a.Passed = false
			a.Disconnect.Reason = "close_failed"
			a.Error = strings.TrimSpace(a.Error + " Schließen: " + err.Error())
		}
		a.Finished = time.Now()
	}()
	d, err := resolveCheckDevice(ctx, o, deps)
	a.Device = d
	if err != nil {
		a.Error = err.Error()
		a.Disconnect.Reason = "resolution_failed"
		return
	}
	if err = s.SelectDevice(d, true); err != nil {
		a.Error = err.Error()
		a.Disconnect.Reason = "selection_failed"
		return
	}
	a.Device, _, _, _, _ = s.Snapshot()
	connectCtx, cancel := context.WithTimeout(ctx, o.connectTimeout)
	start := time.Now()
	err = s.Connect(connectCtx)
	if err == nil {
		err = connectCtx.Err()
	}
	a.ConnectDurationMS = time.Since(start).Milliseconds()
	a.Phases = append(a.Phases, s.ConnectionTimings()...)
	cancel()
	if err != nil {
		a.Error = err.Error()
		a.Disconnect.Reason = "connect_failed"
		if possibleDuplicateSession(err, a.Phases) {
			a.Disconnect.Reason = "peer_rejected_possible_duplicate_session"
			a.Disconnect.Detail = "Die Verbindung wurde kurz nach TLS beendet. Möglicherweise besteht bereits eine Sitzung mit derselben SKI. Andere Sitzung beenden oder eine längere --pause wählen."
		}
		if ctx.Err() != nil {
			a.Disconnect.Reason = "canceled"
		}
		return
	}
	trust := s.LiveTrust()
	if !trust.Device || !trust.Simulator {
		a.Error = "Gegenseitiges SHIP-Vertrauen nicht bestätigt"
		a.Disconnect.Reason = "connect_failed"
		return
	}
	a.ConnectionPassed = true
	held, err := holdSHIP(ctx, s, o.hold)
	a.Hold.ActualMS = held.Milliseconds()
	a.Hold.Passed = err == nil
	a.Passed = err == nil
	a.Disconnect.Reason = "hold_complete"
	if err != nil {
		a.Error = err.Error()
		a.Disconnect.Reason = "connection_lost"
		_, _, _, _, a.Disconnect.Detail = s.Snapshot()
		if ctx.Err() != nil {
			a.Disconnect.Reason = "canceled"
		}
	}
	return
}

func holdSHIP(ctx context.Context, s shipCheckSession, duration time.Duration) (time.Duration, error) {
	start := time.Now()
	end := time.NewTimer(duration)
	defer end.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		t := s.LiveTrust()
		if !t.Device || !t.Simulator {
			return errors.New("SHIP-Verbindung während der Haltezeit verloren")
		}
		return nil
	}
	for {
		if err := check(); err != nil {
			return time.Since(start), err
		}
		select {
		case <-ctx.Done():
			return time.Since(start), ctx.Err()
		case <-end.C:
			return time.Since(start), check()
		case <-tick.C:
		}
	}
}

func resolveCheckDevice(ctx context.Context, o shipCheckOptions, deps cliServices) (discovery.Device, error) {
	d := discovery.Device{Address: o.target.address, SKI: o.ski, Interface: o.adapter}
	if o.target.address != "" {
		return d, nil
	}
	names := []string{o.adapter}
	if o.adapter == "" {
		list, err := deps.interfaces()
		if err != nil {
			return d, err
		}
		names = nil
		seen := map[string]bool{}
		for _, iface := range list {
			if !seen[iface.Name] {
				names = append(names, iface.Name)
				seen[iface.Name] = true
			}
		}
	}
	matches := map[string]discovery.Device{}
	for _, name := range names {
		searchCtx, cancel := context.WithTimeout(ctx, o.searchTimeout)
		found, err := deps.searchLAN(searchCtx, name)
		cancel()
		if ctx.Err() != nil {
			return d, ctx.Err()
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return d, fmt.Errorf("mDNS auf %s: %w", name, err)
		}
		for _, candidate := range found {
			ski, err := identity.NormalizeSKI(candidate.SKI)
			if err != nil || ski != o.ski {
				continue
			}
			matched := false
			for _, value := range []string{candidate.Name, candidate.Host, candidate.ServiceName} {
				matched = matched || strings.EqualFold(strings.TrimSuffix(value, "."), o.target.name)
			}
			_, port, err := net.SplitHostPort(candidate.Address)
			if !matched || err != nil || o.target.port != "" && port != o.target.port {
				continue
			}
			candidate.SKI = ski
			candidate.Interface = name
			matches[name+"\x00"+candidate.LocalAddress+"\x00"+candidate.Address] = candidate
		}
	}
	if len(matches) != 1 {
		return d, fmt.Errorf("mDNS-Ziel %q mit erwarteter SKI: %d Treffer; Name prüfen bzw. --interface eindeutig wählen", o.target.name, len(matches))
	}
	for _, candidate := range matches {
		return candidate, nil
	}
	return d, errors.New("mDNS-Ziel nicht gefunden")
}

func writeDiscoveryJSON(out io.Writer, devices []discovery.Device) error {
	if devices == nil {
		devices = []discovery.Device{}
	}
	return json.NewEncoder(out).Encode(devices)
}

func waitShipPause(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}
func possibleDuplicateSession(err error, rows []eebus.ConnectionTiming) bool {
	tlsOK := false
	var closeErr *websocket.CloseError
	code1006 := errors.As(err, &closeErr) && closeErr.Code == websocket.CloseAbnormalClosure
	for _, row := range rows {
		if row.Phase == "TLS-Handshake" && row.Status == "complete" {
			tlsOK = true
		}
		if tlsOK && (row.Phase == "SHIP-Handshake" || row.Phase == "WebSocket-Upgrade") && row.Status == "failed" && row.DurationMS <= 1000 && (code1006 || strings.Contains(row.Detail, "websocket: close 1006 (")) {
			return true
		}
	}
	return false
}
