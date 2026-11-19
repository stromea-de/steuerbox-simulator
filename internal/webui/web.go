// Package webui serves the embedded German UI on a loopback HTTP server.
package webui

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

//go:embed assets/*
var assets embed.FS

// Options must be selected by the local server owner, never by an HTTP request.
// LoopbackPartner (039, R039-01) lets the isolated E2E fixture pair with its loopback test partner in device
// mode; without it a --loopback-only server has no network and refuses every device pairing path.
// Shutdown (041) ends this simulator process; only the local server owner provides it.
type Options struct {
	LoopbackOnly, LoopbackPartner bool
	Shutdown                      func()
}

type App struct {
	loopbackOnly       bool
	loopbackPartner    atomic.Bool // read by the session's connection gate outside App.mu
	reportDir          string
	s                  *session.Session
	token              string
	handler            http.Handler
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	mu                 sync.Mutex
	busy               bool
	result             *inspection.Report
	runError           string
	operation          string
	kind               string
	connected          bool
	capabilities       *inspection.Capabilities
	deviceInfo         *inspection.Device
	activeConfig       *inspection.Config
	preparation        *preparation
	connectionAt       time.Time
	connectionState    string
	search             searchProgress
	runCancel          context.CancelFunc
	operationDone      chan struct{}
	connectionProgress session.ConnectionProgress
	searchCancel       context.CancelFunc
	searchDone         chan struct{}
	shutdownHook       func()
	shuttingDown       bool
	importing          bool
	importDone         chan struct{}
}

func New(s *session.Session, options ...Options) *App {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{s: s, token: hex.EncodeToString(b), ctx: ctx, cancel: cancel, connectionAt: time.Now(), connectionState: "disconnected"}
	if len(options) > 0 {
		a.loopbackOnly = options[0].LoopbackOnly
		a.loopbackPartner.Store(options[0].LoopbackOnly && options[0].LoopbackPartner)
		a.shutdownHook = options[0].Shutdown
	}
	if s != nil {
		s.SetNetworkGate(a.networkAvailable) // 039 R039-01: one rule for every device connection setup
		a.reportDir = s.ReportDir()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.index)
	mux.HandleFunc("/protokolle", a.index)
	mux.HandleFunc("/hilfe", a.index)
	mux.HandleFunc("/profil", a.index)
	mux.HandleFunc("/api/reports", a.reports)
	mux.HandleFunc("/api/reports/delete", a.deleteReport)
	mux.HandleFunc("/api/report/site", a.updateSite)
	mux.HandleFunc("/api/reconnect", a.connect)
	mux.HandleFunc("/api/report/versions", a.reportVersions)
	mux.HandleFunc("/api/export", a.exportData)
	mux.HandleFunc("/api/import", a.importData)
	mux.HandleFunc("/api/shutdown", a.shutdown)
	mux.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		a.asset(w, r, "app.js", "text/javascript; charset=utf-8")
	})
	for _, name := range []string{"qr-vendor.js", "input.js", "i18n-data.js", "i18n.js", "result.js", "workspace.js", "saved-report.js", "guide.js", "profile.js", "dialogs.js"} {
		mux.HandleFunc("/"+name, func(w http.ResponseWriter, r *http.Request) { a.asset(w, r, name, "text/javascript; charset=utf-8") })
	}
	mux.HandleFunc("/api/identity/reset", a.resetIdentity)
	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) { a.asset(w, r, "style.css", "text/css; charset=utf-8") })
	// 039 (QA-06): browsers that still request /favicon.ico receive the brand mark instead of a 404.
	for _, name := range []string{"/favicon.ico", "/favicon.svg"} {
		mux.HandleFunc(name, func(w http.ResponseWriter, r *http.Request) { a.asset(w, r, "favicon.svg", "image/svg+xml") })
	}
	mux.HandleFunc("/saved-noscript.css", func(w http.ResponseWriter, r *http.Request) {
		a.asset(w, r, "saved-noscript.css", "text/css; charset=utf-8")
	})
	mux.HandleFunc("/report-list.css", func(w http.ResponseWriter, r *http.Request) {
		a.asset(w, r, "report-list.css", "text/css; charset=utf-8")
	})
	mux.HandleFunc("/api/instance", func(w http.ResponseWriter, r *http.Request) {
		if method(w, r, "GET") {
			a.mu.Lock()
			defer a.mu.Unlock()
			send(w, map[string]any{"application": "stromea-steuerbox-simulator/desktop-v1", "busy": a.busy})
		}
	})
	mux.HandleFunc("/api/state", a.state)
	mux.HandleFunc("/api/prepare", a.prepare)
	mux.HandleFunc("/api/discover", a.discover)
	mux.HandleFunc("/api/pair", a.pair)
	mux.HandleFunc("/api/run", a.run)
	mux.HandleFunc("/api/select", a.selectDevice)
	mux.HandleFunc("/api/simulator", a.selectSimulator)
	mux.HandleFunc("/api/interfaces", a.interfaces)
	mux.HandleFunc("/api/discover-lan", a.discoverLAN)
	mux.HandleFunc("/api/discover-stop", a.stopDiscovery)
	mux.HandleFunc("/api/connect", a.connect)
	mux.HandleFunc("/api/cancel", a.cancelRun)
	mux.HandleFunc("/api/low-load", a.lowLoad)
	mux.HandleFunc("/api/report.pdf", a.pdf)
	mux.HandleFunc("/api/report.json", a.jsonReport)
	mux.HandleFunc("/protokoll", a.downloadPage)
	a.handler = mux
	return a
}
func (a *App) Close() { a.cancel(); a.wg.Wait() }
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		http.Error(w, "Loopback-Host erforderlich", 403)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' blob: data:; media-src 'self' blob:; frame-ancestors 'none'; form-action 'self'; base-uri 'none'")
	if r.Method != "GET" && r.Method != "HEAD" {
		if r.Header.Get("X-Stromea-Token") != a.token || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host) {
			http.Error(w, "Ungültiger lokaler Sitzungstoken/Origin", 403)
			return
		}
	}
	a.handler.ServeHTTP(w, r)
}
func (a *App) asset(w http.ResponseWriter, r *http.Request, name, content string) {
	if !method(w, r, "GET") {
		return
	}
	b, _ := assets.ReadFile("assets/" + name)
	w.Header().Set("Content-Type", content)
	w.Write(b)
}
func (a *App) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/protokolle" && r.URL.Path != "/hilfe" && r.URL.Path != "/profil" {
		http.NotFound(w, r)
		return
	}
	if !method(w, r, "GET") {
		return
	}
	b, _ := assets.ReadFile("assets/index.html")
	t := template.Must(template.New("index").Parse(string(b)))
	var output bytes.Buffer
	if err := t.Execute(&output, struct{ Token, Version string }{a.token, report.BuildVersion()}); err != nil {
		http.Error(w, "Seite nicht verfügbar", 500)
		return
	}
	page := output.Bytes()
	if r.URL.Path == "/protokolle" {
		fallback, err := a.reportListFallback()
		if err != nil {
			http.Error(w, "Protokolle nicht verfügbar", 500)
			return
		}
		// Inject already escaped HTML after template execution; customer braces are data.
		page = bytes.Replace(page, []byte("</header>"), append([]byte("</header>"), fallback...), 1)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

func (a *App) downloadPage(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	data := struct {
		Report   *inspection.Report
		Busy     bool
		Error    string
		Version  string
		Token    string
		Versions []versionRef
	}{a.result, a.busy, a.runError, report.BuildVersion(), a.token, nil}
	if r.URL.Query().Get("id") != "" {
		v, e := a.selectedReport(r)
		if e != nil {
			http.Error(w, "Protokoll nicht gefunden", 404)
			return
		}
		data.Report = v
		data.Busy = false
	}
	if data.Busy {
		data.Report = nil
	}
	if data.Report != nil && data.Report.ID != "" {
		if versions, e := a.versionsOf(data.Report.ID); e == nil && len(versions) > 1 {
			data.Versions = versions // 041: „v2 · v1“
		}
	}
	b, _ := assets.ReadFile("assets/protokoll.html")
	t := template.Must(template.New("protokoll").Funcs(template.FuncMap{"stepName": inspection.StepName, "groupSKI": inspection.GroupSKI, "germanNumber": report.GermanNumber}).Parse(string(b)))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.Execute(w, data)
}
func method(w http.ResponseWriter, r *http.Request, m string) bool {
	if r.Method != m {
		w.Header().Set("Allow", m)
		http.Error(w, "Methode nicht erlaubt", 405)
		return false
	}
	return true
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	body, ok := readBody(w, r)
	return ok && decodeBody(w, body, v)
}

// readBody reads the bounded JSON request body. Handlers call it before taking App.mu (040 F-03):
// a client that stalls while sending its body must never hold the lock that /api/state needs.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "JSON erforderlich", 415)
		return nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1200000))
	if err != nil {
		http.Error(w, "Ungültige Eingabe: "+err.Error(), 400)
		return nil, false
	}
	return body, true
}
func decodeBody(w http.ResponseWriter, body []byte, v any) bool {
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		http.Error(w, "Ungültige Eingabe: "+err.Error(), 400)
		return false
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		http.Error(w, "Nur ein JSON-Objekt erlaubt", 400)
		return false
	}
	return true
}
func send(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func (a *App) state(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ski, paired, real, status := a.s.Snapshot()
	live := a.s.LiveConnected()
	a.connected = live
	phase := "disconnected"
	if live {
		phase = "connected"
	}
	if a.busy && a.kind == "connect" {
		phase = "reconnecting"
	}
	reconnection := a.s.Reconnection()
	if a.kind == "connect" {
		reconnection = a.connectionProgress
	}
	if reconnection.Active {
		phase = "reconnecting"
	}
	if phase != a.connectionState {
		a.connectionState = phase
		a.connectionAt = time.Now()
	}
	outcome := ""
	if a.result != nil {
		outcome = a.result.Outcome()
	}
	caps := a.capabilities
	if !real && paired {
		caps = &inspection.Capabilities{LPC: true, LPP: true, MPC: true}
	}
	send(w, map[string]any{"connectionTimings": a.s.ConnectionTimings(), "reconnection": reconnection, "controlUnresponsive": a.s.ControlError() != nil, "version": report.BuildVersion(), "preparation": a.preparation, "trust": a.s.LiveTrust(), "deviceInfo": a.deviceInfo, "progress": a.s.Progress(), "capabilities": caps, "config": a.activeConfig, "kind": a.kind, "connected": live, "connectionState": phase, "connectionAt": a.connectionAt, "search": a.search, "outcome": outcome, "counts": a.resultCounts(), "device": d, "localSki": ski, "localShipID": a.s.ShipID(), "paired": paired, "deviceMode": real, "connection": status, "operation": a.operation, "busy": a.busy, "report": a.result, "error": a.runError, "shuttingDown": a.shuttingDown})
}
func (a *App) discover(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	devices, err := discovery.Search(ctx, a.s.DNS.Address())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	send(w, devices)
}
func (a *App) pair(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct {
		SKI string `json:"ski"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blockedLocked() {
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	if err := a.s.Pair(in.SKI); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	send(w, map[string]bool{"paired": true})
}
func (a *App) run(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blockedLocked() {
		http.Error(w, "Prüfung läuft bereits", 409)
		return
	}
	if a.s.IsDeviceMode() && !a.offlineAllowed(w) { // Re-Review 1 R039-01: the start may connect, too
		return
	}
	c := inspection.DefaultConfig()
	if a.s.IsDeviceMode() {
		c = inspection.DefaultDeviceConfig()
	}
	if !decodeBody(w, body, &c) {
		return
	}
	if !a.s.IsDeviceMode() {
		if strings.TrimSpace(c.Business) == "" {
			c.Business = "Beispielbetrieb (Übung)"
		}
		if strings.TrimSpace(c.Inspector) == "" {
			c.Inspector = "Beispielperson (Übung)"
		}
	}
	if a.s.IsDeviceMode() {
		if err := deviceConfig(&c, a.capabilities, a.s.Paired()); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
	}
	validate := c.Validate
	if a.s.IsDeviceMode() {
		validate = c.ValidateDevice
	}
	if err := validate(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := c.ValidateMinimums(a.s.IsDeviceMode()); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if !a.s.Paired() {
		http.Error(w, "Bitte zuerst per SKI koppeln", 409)
		return
	}

	if a.s.ControlError() != nil {
		http.Error(w, inspection.ErrControlUnresponsive.Error(), 409)
		return
	}
	c.NormalizeProfile()
	if a.loopbackOnly && a.loopbackPartner.Load() && a.s.IsDeviceMode() {
		// 046: the isolated E2E test partner is the simulated device; its configured
		// timeout replaces the 120 s protocol minimum. Never for real devices.
		c.TestHeartbeatTimeout = c.FailsafeTimeout
	}
	a.activeConfig = &c
	a.s.ResetProgress()
	a.busy = true
	a.operation = "Prüfung läuft"
	a.kind = "run"
	runCtx, runCancel := context.WithCancel(a.ctx)
	a.runCancel = runCancel
	done := make(chan struct{})
	a.operationDone = done
	a.result = nil
	a.runError = ""
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer runCancel()
		defer close(done)
		total := 90 * time.Second
		if a.s.IsDeviceMode() {
			total = c.RunDeadline() // 040 F-06: one formula for WebUI and CLI
		}
		ctx, cancel := context.WithTimeout(runCtx, total)
		defer cancel()
		result, err := a.s.Run(ctx, c)
		if runCtx.Err() != nil {
			a.s.Disconnect()
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if runCtx.Err() != nil {
			a.s.Disconnect()
		}
		a.busy = false
		a.runCancel = nil
		a.operation = ""
		if err != nil {
			a.runError = inspection.UserError(err)
		} else {
			result.Software = report.BuildVersion()
			a.result = &result
			if e := session.SaveReport(a.reportDir, &result); e != nil {
				a.runError = e.Error()
			}
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}
func (a *App) pdf(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.mu.Lock()
	selected, selectionErr := a.selectedReport(r)
	var snapshot inspection.Report
	if selectionErr == nil {
		snapshot = *selected
	}
	a.mu.Unlock()
	if selectionErr != nil {
		http.Error(w, "Noch kein Protokoll", 404)
		return
	}
	b, err := report.PDF(snapshot) // 040 F-03: render outside App.mu
	if err != nil {
		a.pdfFailure(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="EEBUS-Protokoll.pdf"`)
	w.Write(b)
}
func (a *App) jsonReport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.mu.Lock()
	selected, selectionErr := a.selectedReport(r)
	var snapshot inspection.Report
	if selectionErr == nil {
		snapshot = *selected
	}
	a.mu.Unlock()
	if selectionErr != nil {
		http.Error(w, "Noch kein Protokoll", 404)
		return
	}
	data, err := report.JSON(snapshot)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
}

// offlineAllowed is the single server-side rule for network-dependent pairing (039, R039-01): search, device
// selection, connect and reconnect in device mode need an eligible network adapter. The practice path does not.
func (a *App) offlineAllowed(w http.ResponseWriter) bool {
	if a.networkAvailable() {
		return true
	}
	http.Error(w, "Keine Netzwerkverbindung", 409)
	return false
}
func (a *App) networkAvailable() bool {
	if a.loopbackOnly {
		return a.loopbackPartner.Load()
	}
	v, err := discovery.Interfaces()
	return err == nil && len(v) > 0
}
func (a *App) interfaces(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	if a.loopbackOnly {
		send(w, []map[string]any{{"name": "loopback", "address": "127.0.0.1", "loopbackOnly": true}})
		return
	}
	v, err := discovery.Interfaces()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	send(w, v)
}

// LAN discovery requires a CSRF-protected explicit POST with interface selection.
func (a *App) discoverLAN(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct {
		Interface string `json:"interface"`
	}
	if !decode(w, r, &in) {
		return
	}
	if a.loopbackOnly && in.Interface != "loopback" {
		http.Error(w, "Nur isolierte Loopback-Suche erlaubt", 400)
		return
	}
	if !a.offlineAllowed(w) {
		return
	}
	if in.Interface == "" {
		http.Error(w, "LAN-Interface erforderlich", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	stop := context.AfterFunc(a.ctx, cancel)
	defer stop()
	defer cancel()
	a.mu.Lock()
	if a.search.Active || a.shuttingDown || a.importing || (a.busy && a.kind != "run") || ctx.Err() != nil {
		a.mu.Unlock()
		http.Error(w, "Gerätesuche läuft bereits", 409)
		return
	}
	done := make(chan struct{})
	a.searchCancel, a.searchDone = cancel, done
	a.wg.Add(1)
	defer a.wg.Done()
	defer close(done)
	a.search = searchProgress{Active: true, Deadline: time.Now().Add(6 * time.Second), Devices: []discovery.Device{}}
	a.mu.Unlock()
	search := discovery.SearchLANLive
	if a.loopbackOnly {
		search = func(ctx context.Context, _ string, update func([]discovery.Device)) ([]discovery.Device, error) {
			v, err := discovery.Search(ctx, a.s.DNS.Address())
			update(v)
			return v, err
		}
	}
	v, err := search(ctx, in.Interface, func(devices []discovery.Device) {
		a.mu.Lock()
		if ctx.Err() == nil {
			a.search.Devices = append([]discovery.Device{}, devices...)
		}
		a.mu.Unlock()
	})
	a.mu.Lock()
	a.search.Active = false
	a.searchCancel = nil
	if errors.Is(ctx.Err(), context.Canceled) {
		v = append([]discovery.Device{}, a.search.Devices...)
		err = nil
	}
	a.search.Devices = v
	a.mu.Unlock()
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		http.Error(w, err.Error(), 400)
		return
	}
	send(w, v)
}

// Stop only read-only discovery; browsing must preserve a connected device/run.
func (a *App) stopDiscovery(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	a.mu.Lock()
	cancel, done := a.searchCancel, a.searchDone
	if cancel != nil {
		cancel()
	}
	a.mu.Unlock()
	if cancel != nil && done != nil {
		<-done
	}
	send(w, map[string]bool{"stopped": true})
}
func (a *App) selectDevice(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct {
		Device    discovery.Device `json:"device"`
		Confirmed bool             `json:"confirmed"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !a.offlineAllowed(w) {
		return
	}
	if a.loopbackOnly {
		host, _, err := net.SplitHostPort(in.Device.Address)
		ip := net.ParseIP(host)
		if err != nil || ip == nil || !ip.IsLoopback() || in.Device.Interface != "" && in.Device.Interface != "loopback" {
			http.Error(w, "Nur Loopback-Geräte erlaubt", 400)
			return
		}
		// Route selection for literal loopback is performed by the regular backend.
		in.Device.Interface = ""
		in.Device.LocalAddress = ""
	}
	// Stop and join discovery before replacing the session. Its worker must not
	// publish stale results after selection or hold App.mu while we wait.
	a.mu.Lock()
	if a.busy || a.shuttingDown || a.importing {
		a.mu.Unlock()
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	cancel, done := a.searchCancel, a.searchDone
	if cancel != nil {
		cancel()
	}
	a.mu.Unlock()
	if cancel != nil && done != nil {
		<-done
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blockedLocked() || r.Context().Err() != nil {
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	if err := a.s.SelectDevice(in.Device, in.Confirmed); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	a.result = nil
	a.runError = ""
	a.kind = ""
	a.connected = false
	a.capabilities = nil
	a.deviceInfo = nil
	a.preparation = nil
	send(w, map[string]bool{"selected": true})
}
func (a *App) connect(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct {
		Timeout time.Duration `json:"timeout"`
	}
	in.Timeout = time.Minute
	if !decode(w, r, &in) {
		return
	}
	if in.Timeout <= 0 || in.Timeout > 10*time.Minute {
		http.Error(w, "Zeitlimit 0 bis 10 Minuten", 400)
		return
	}
	if a.s.IsDeviceMode() && !a.offlineAllowed(w) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blockedLocked() || !a.s.Paired() {
		http.Error(w, "Bereits aktiv oder Geräte-SKI nicht bestätigt", 409)
		return
	}
	a.busy = true
	a.operation = "Verbindung wird aufgebaut…"
	a.kind = "connect"
	a.connected = false
	a.runError = ""
	ctx, cancel := context.WithTimeout(a.ctx, in.Timeout)
	deadline, _ := ctx.Deadline()
	a.connectionProgress = session.ConnectionProgress{Active: true, Attempt: 1, Deadline: deadline}
	done := make(chan struct{})
	a.operationDone = done
	rebuild := r.URL.Path == "/api/reconnect"
	a.runCancel = cancel
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer cancel()
		defer close(done)
		var err error
		if rebuild {
			err = a.s.Rebuild(ctx)
		} else {
			err = a.s.Connect(ctx)
		}
		var caps *inspection.Capabilities
		if err == nil {
			if v, e := a.s.Capabilities(ctx); e == nil {
				caps = &v
			}
		}
		var info *inspection.Device
		if err == nil {
			v := a.s.DeviceInfo()
			info = &v
		}
		if ctx.Err() != nil && (err == nil || errors.Is(ctx.Err(), context.Canceled)) {
			err = ctx.Err()
		}
		if err != nil {
			a.s.Disconnect()
		}
		var preview *preparation
		a.mu.Lock()
		defer a.mu.Unlock()
		if ctx.Err() != nil && (err == nil || errors.Is(ctx.Err(), context.Canceled)) {
			err = ctx.Err()
			a.s.Disconnect()
		}
		// 040 F-01: a saved report is immutable (its handed-out SHA-256 must stay valid). Reconnect events
		// stay in the live session; only an unsaved result shows them.
		if rebuild && a.result != nil && a.result.ID == "" {
			a.result.Events = a.s.ConnectionEvents()
		}
		a.connectionProgress.Active = false
		a.connectionProgress.TimedOut = errors.Is(err, context.DeadlineExceeded)
		a.preparation = preview
		a.deviceInfo = info
		a.busy = false
		a.runCancel = nil
		if errors.Is(err, context.Canceled) {
			a.kind = ""
			a.runError = ""
			a.operation = "Abgebrochen"
			a.capabilities = nil
		} else if err != nil {
			a.runError = err.Error()
			a.operation = "Verbindung fehlgeschlagen"
		} else {
			a.operation = "Gerät erfolgreich gekoppelt."
			a.connected = true
			a.capabilities = caps
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}
func (a *App) cancelRun(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	// 040 F-04: {"read":true} cancels only the reading of device values; the connection stays.
	var in struct {
		Read bool `json:"read"`
	}
	if r.ContentLength != 0 && !decode(w, r, &in) {
		return
	}
	a.mu.Lock()
	cancel, done := a.runCancel, a.operationDone
	searchCancel, searchDone := a.searchCancel, a.searchDone
	if in.Read && a.kind != "prepare" {
		a.mu.Unlock()
		send(w, map[string]bool{"cancelRequested": false})
		return
	}
	if !a.busy && !a.search.Active {
		if a.s != nil {
			a.s.Disconnect()
		}
		a.connectionProgress = session.ConnectionProgress{}
		if a.kind == "connect" {
			a.kind = ""
			a.runError = ""
			a.operation = "Abgebrochen"
		}
	}
	if cancel != nil {
		if a.kind == "run" {
			a.s.CancelRecovery()
		}
		cancel()
		if a.kind != "prepare" { // 040 F-07: reading sets no limits
			a.operation = "Abbruch angefordert; Grenzen werden aufgehoben"
		}
	}
	if searchCancel != nil {
		searchCancel()
	}
	a.mu.Unlock()
	// Completion is the barrier: a subsequent selection can never be overwritten
	// by the canceled worker. Do not hold App.mu while waiting for its final state.
	if done != nil && cancel != nil {
		<-done
	}
	if searchDone != nil && searchCancel != nil {
		<-searchDone
	}
	// An explicit user cancel keeps the 025 disconnect guarantee. Selection uses
	// its own search barrier and must not disconnect the newly selected endpoint.
	if searchCancel != nil {
		a.mu.Lock()
		if !a.busy && a.s != nil {
			a.s.Disconnect()
			a.connectionProgress = session.ConnectionProgress{}
		}
		a.mu.Unlock()
	}
	send(w, map[string]bool{"cancelRequested": true})
}

func (a *App) selectSimulator(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blockedLocked() {
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	if err := a.s.SelectSimulator(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	a.result = nil
	a.runError = ""
	a.operation = "Übungsmodus (Simulator)"
	a.kind = ""
	a.connected = false
	send(w, map[string]bool{"simulator": true})
}

func (a *App) resetIdentity(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct {
		Confirmed bool `json:"confirmed"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blockedLocked() {
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	if err := a.s.ResetIdentity(in.Confirmed); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	a.connected = false
	a.kind = ""
	a.runError = ""
	send(w, map[string]bool{"reset": true})
}

func (a *App) lowLoad(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct {
		Retry bool `json:"retry"`
	}
	if !decode(w, r, &in) {
		return
	}
	if err := a.s.ResolveLowLoad(in.Retry); err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	send(w, map[string]bool{"accepted": true})
}

// deviceConfig normalizes a single reported direction before validating unused fields.
func deviceConfig(c *inspection.Config, caps *inspection.Capabilities, connected bool) error {
	if !connected || caps == nil || (!caps.LPC && !caps.LPP) {
		return errors.New("Das Gerät meldet keine Steuerfunktion. Eine Prüfung ist nicht möglich.")
	}
	if !caps.LPC {
		c.Scope = inspection.ScopeProduction
		c.LPC = 0
	} else if !caps.LPP {
		c.Scope = inspection.ScopeConsumption
		c.LPP = 0
	} else if c.Scope == "" {
		c.Scope = inspection.ScopeBoth
	}
	return nil
}

func (a *App) pdfFailure(w http.ResponseWriter, r *http.Request) {
	const page = `<!doctype html><html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>PDF · Stromea</title><link rel="stylesheet" href="/style.css"><script src="/i18n-data.js" defer></script><script src="/i18n.js" defer></script></head><body><main><header><div class="brand-heading"><h1>Stromea Steuerbox-Simulator</h1></div><div class="preferences"><label for="language">Sprache</label><select id="language"></select></div><nav class="main-menu"><a href="/?new=1">Neue Prüfung</a><a href="/protokolle">Protokolle</a><a href="/hilfe">Hilfe</a></nav></header><section class="card local-page"><p role="alert">PDF konnte nicht erstellt werden. Erneut versuchen oder Messdaten als JSON sichern.</p><p><a class="button" href="/api/report.pdf{{if .}}?id={{.}}{{end}}">Erneut versuchen</a></p><p><a href="/api/report.json{{if .}}?id={{.}}{{end}}">Messdaten sichern (JSON)</a></p><p><a href="/protokoll{{if .}}?id={{.}}{{end}}">Berichtsübersicht</a></p></section></main></body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_ = template.Must(template.New("pdf-failure").Parse(page)).Execute(w, r.URL.Query().Get("id"))
}

type searchProgress struct {
	Active   bool               `json:"active"`
	Deadline time.Time          `json:"deadline"`
	Devices  []discovery.Device `json:"devices"`
}

func (a *App) resultCounts() *inspection.Counts {
	if a.result == nil {
		return nil
	}
	v := a.result.CheckCounts()
	return &v
}
