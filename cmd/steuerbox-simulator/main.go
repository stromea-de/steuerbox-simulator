package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/branding"
	"github.com/stromea-de/steuerbox-simulator/internal/desktop"
	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
	"github.com/stromea-de/steuerbox-simulator/internal/storage"
	"github.com/stromea-de/steuerbox-simulator/internal/webui"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if len(os.Args) == 1 && desktopDefault() {
		code := desktop.Run(ctx, report.BuildVersion(), func(ctx context.Context, l net.Listener, out io.Writer) int {
			base, e := os.UserConfigDir()
			if e != nil {
				fmt.Fprintln(out, e)
				return 1
			}
			state, e := defaultStateDir(base)
			if e != nil {
				fmt.Fprintln(out, e)
				return 1
			}
			s, e := session.New(state)
			if e != nil {
				fmt.Fprintln(out, e)
				return 1
			}
			defer s.Close()
			return serveListener(ctx, s, l, out, out)
		})
		os.Exit(code)
	}
	os.Exit(execute(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func execute(ctx context.Context, args []string, out, errout io.Writer) int {
	return executeWithServices(ctx, args, out, errout, defaultCLIServices())
}
func executeWithServices(ctx context.Context, args []string, out, errout io.Writer, deps cliServices) int {
	usage := func() {
		fmt.Fprintf(errout, "%s\n%s\n%s\nAufruf: %s <identity|discover|ship-check|pair|run|demo|serve> [Optionen]\nHilfe: %s <Befehl> --help\n", branding.Name, branding.Subtitle, branding.Nature, branding.Slug, branding.Slug)
	}
	if len(args) == 0 {
		usage()
		return 2
	}
	loopbackOnly := false
	loopbackPartner := false
	command := args[0]
	if command == "ship-check" {
		return executeShipCheck(ctx, args[1:], out, errout, deps)
	}
	switch command {
	case "identity", "discover", "pair", "run", "demo", "serve":
	case "version", "--version":
		fmt.Fprintf(out, "%s %s\n", branding.Name, report.BuildVersion())
		return 0
	case "help", "--help", "-h":
		usage()
		return 0
	default:
		usage()
		return 2
	}
	base, configErr := os.UserConfigDir()
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(errout)
	f.Usage = func() {
		usage()
		fmt.Fprintf(errout, "Optionen für %s:\n", command)
		f.PrintDefaults()
	}
	state := f.String("state", filepath.Join(base, branding.Slug), "Lokales Datenverzeichnis")
	var ski, listen, pdfPath, jsonPath string
	var deviceMode, authorized bool
	var address, confirmedSKI string
	dc := inspection.DefaultDeviceConfig()
	var mdns bool
	var lan, interfaces bool
	var interfaceName string
	searchTimeout := 2 * time.Second
	c := inspection.DefaultConfig()
	if command == "pair" || command == "run" {
		f.StringVar(&interfaceName, "interface", "", "Adapter für Geräteverbindung (sonst passend zum Subnetz)")
		f.BoolVar(&deviceMode, "device", false, "Gerät vor Ort (ausdrücklicher LAN-Gerätebetrieb)")
		f.StringVar(&address, "address", "", "Geräte-IP:Port aus LAN-Suche oder manuell")
		f.StringVar(&confirmedSKI, "confirm-ski", "", "Am Gerät geprüfte SKI zur ausdrücklichen Bestätigung wiederholen")
	}
	if command == "run" {
		f.StringVar(&ski, "ski", "", "Geräte-SKI")
		f.BoolVar(&authorized, "authorized", false, "Kompatibilitätsflag; nicht mehr erforderlich")
	}
	if command == "pair" || command == "run" {
		f.DurationVar(&dc.StepTimeout, "step-timeout", dc.StepTimeout, "Zeitlimit für Trust/Protokollschritte")
		f.DurationVar(&dc.EffectTimeout, "effect-timeout", dc.EffectTimeout, "Zeitlimit je Leistungswirkung")
		f.BoolVar(&dc.LongTest, "langtest", false, "Wiederkehr nach Failsafe-Mindestdauer prüfen (mehrere Stunden)")
		f.DurationVar(&dc.FailsafeTimeout, "failsafe-timeout", dc.FailsafeTimeout, "Zeitlimit Heartbeat-Ausfall")
		f.DurationVar(&dc.RecoveryTimeout, "recovery-timeout", dc.RecoveryTimeout, "Zeitlimit Wiederkehr einschließlich Mindestdauer")
		f.DurationVar(&dc.FailsafeMinimum, "failsafe-minimum", dc.FailsafeMinimum, "Failsafe-Mindestdauer (mindestens 2h)")
		f.DurationVar(&dc.CleanupTimeout, "cleanup-timeout", dc.CleanupTimeout, "Zeitlimit je Grenzaufhebung/Wiederverbindung")
	}

	switch command {
	case "pair":
		f.StringVar(&ski, "ski", "", "Explizit zu bestätigende Geräte-/Simulator-SKI")
	case "serve":
		f.BoolVar(&loopbackOnly, "loopback-only", false, "Isolierter E2E-Testpartner; keine LAN-Suche/-Verbindung; stdin stop-device beendet nur den Testpartner, drop-connection trennt nur die Verbindung")
		f.BoolVar(&loopbackPartner, "loopback-partner", false, "Nur mit --loopback-only: Gerätebetrieb mit dem Loopback-Testpartner erlauben (E2E); ohne gilt der Server als ohne Netzwerk")
		f.StringVar(&listen, "listen", "127.0.0.1:33287", "Lokale HTTP-Adresse (nur Loopback-IP)")
	case "discover":
		f.Bool("json", false, "Ergebnisliste als JSON (auch Standard); LAN nur mit --lan --interface")
		f.DurationVar(&searchTimeout, "timeout", searchTimeout, "Suchdauer je Adapter (0 < Dauer <= 1m)")
		f.BoolVar(&lan, "lan", false, "Explizite lesende LAN-mDNS-Suche; keine Kopplung/Steuerung")
		f.StringVar(&interfaceName, "interface", "", "Name des freigegebenen LAN-Interfaces (mit --lan)")
		f.BoolVar(&interfaces, "interfaces", false, "Geeignete LAN-Interfaces lokal auflisten (keine Pakete)")
		f.BoolVar(&mdns, "mdns-loopback", false, "mDNS-Multicast-Suche ausschließlich auf Loopback (sonst isolierter DNS-SD-Responder)")
	case "run", "demo":
		f.StringVar(&c.Business, "betrieb", "", "Betrieb")
		f.StringVar(&c.Inspector, "pruefer", "", "Prüfer / Prüferin")
		f.BoolVar(&c.SignOperator, "unterschrift-betreiber", false, "Freiwilliges Empfangsfeld im PDF")
		f.BoolVar(&c.SignInspector, "unterschrift-fachkraft", false, "Freiwilliges Durchführungsfeld im PDF (Übung: testdurchführende Person)")
		f.Func("lpc", "Bezugslimit in W (Komma oder Punkt)", func(raw string) error {
			value, err := inspection.ParsePower(raw)
			if err == nil {
				c.LPC = value
			}
			return err
		})
		f.Func("lpp", "Einspeiselimit in W (Komma oder Punkt)", func(raw string) error {
			value, err := inspection.ParsePower(raw)
			if err == nil {
				c.LPP = value
			}
			return err
		})
		f.Func("failsafe", "Failsafe-Limit in W (Komma oder Punkt)", func(raw string) error {
			value, err := inspection.ParsePower(raw)
			if err == nil {
				c.Failsafe = value
			}
			return err
		})
		f.Func("toleranz", "Messtoleranz in W (Komma oder Punkt)", func(raw string) error {
			value, err := inspection.ParsePower(raw)
			if err == nil {
				c.Tolerance = value
			}
			return err
		})
		f.StringVar(&pdfPath, "pdf", "EEBUS-Protokoll.pdf", "Ziel für PDF-Protokoll")
		f.StringVar(&jsonPath, "json", "", "Optionales JSON-Protokoll")
	}
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if f.NArg() != 0 {
		usage()
		return 2
	}
	if deviceMode {
		explicitLPP := false
		f.Visit(func(option *flag.Flag) {
			if option.Name == "lpp" {
				explicitLPP = true
			}
		})
		if !explicitLPP {
			c.LPP = dc.LPP
		}
		v, e := identity.NormalizeSKI(ski)
		confirmed, e2 := identity.NormalizeSKI(confirmedSKI)
		if e != nil || e2 != nil || v != confirmed || eebus.DeviceAddress(address) != nil {
			fmt.Fprintln(errout, "Gerätebetrieb benötigt --address IP:Port --ski SKI --confirm-ski SKI (am Gerät abgleichen)")
			return 2
		}
		dc.Business = c.Business
		dc.Inspector = c.Inspector
		dc.SignOperator = c.SignOperator
		dc.SignInspector = c.SignInspector
		dc.LPC = c.LPC
		dc.LPP = c.LPP
		dc.Failsafe = c.Failsafe
		dc.Tolerance = c.Tolerance
		dc.Authorized = authorized
		c = dc
		if command == "pair" && (c.StepTimeout <= 0 || c.StepTimeout > 10*time.Minute) {
			fmt.Fprintln(errout, "ungültiges Zeitlimit")
			return 2
		}
	} else if address != "" || confirmedSKI != "" {
		fmt.Fprintln(errout, "Geräteoptionen benötigen --device")
		return 2
	}
	if command == "run" && deviceMode {
		if err := c.ValidateDevice(); err != nil {
			fmt.Fprintln(errout, err)
			return 2
		}
	}
	if command == "serve" {
		if err := eebus.LoopbackAddress(listen); err != nil {
			fmt.Fprintln(errout, err)
			return 2
		}
	}
	if command == "run" || command == "demo" {
		if err := c.Validate(); err != nil {
			fmt.Fprintln(errout, err)
			return 2
		}
		if err := c.ValidateMinimums(deviceMode); err != nil { // 040 F-05
			fmt.Fprintln(errout, err)
			return 2
		}
	}
	if command == "pair" && ski == "" {
		fmt.Fprintln(errout, "--ski erforderlich; zuerst discover aufrufen")
		return 2
	}
	if command == "discover" {
		if searchTimeout <= 0 || searchTimeout > time.Minute {
			fmt.Fprintln(errout, "Ungültige Suchdauer")
			return 2
		}
		if (lan && (interfaceName == "" || mdns || interfaces)) || (!lan && interfaceName != "") || (interfaces && mdns) {
			fmt.Fprintln(errout, "LAN-Suche: --lan --interface <Name>; Suchmodi nicht kombinieren")
			return 2
		}
		if interfaces {
			list, err := deps.interfaces()
			if err != nil {
				fmt.Fprintln(errout, err)
				return 1
			}
			if err := json.NewEncoder(out).Encode(list); err != nil {
				fmt.Fprintln(errout, err)
				return 1
			}
			return 0
		}
	}
	if command == "discover" && (mdns || lan) {
		searchCtx, cancel := context.WithTimeout(ctx, searchTimeout)
		defer cancel()
		var devices []discovery.Device
		var err error
		if lan {
			fmt.Fprintln(errout, "Lesende LAN-Suche auf", interfaceName, "· DNS-SD-Angaben sind unbestätigt; keine Kopplung/Steuerung.")
			devices, err = deps.searchLAN(searchCtx, interfaceName)
		} else {
			devices, err = deps.searchMDNS(searchCtx)
		}
		if ctx.Err() != nil {
			fmt.Fprintln(errout, ctx.Err())
			return 1
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintln(errout, err)
			return 1
		}
		if err := writeDiscoveryJSON(out, devices); err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
		return 0
	}
	explicitState := false
	f.Visit(func(option *flag.Flag) {
		if option.Name == "state" {
			explicitState = true
		}
	})
	var err error
	if !explicitState {
		if configErr != nil {
			fmt.Fprintln(errout, configErr)
			return 1
		}
		*state, err = defaultStateDir(base)
		if err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
	}
	var s *session.Session
	if deviceMode {
		s, err = session.NewController(*state)
	} else {
		s, err = session.New(*state)
	}
	if err != nil {
		fmt.Fprintln(errout, err)
		return 1
	}
	defer s.Close()
	fmt.Fprintf(errout, "%s · %s\n%s\n", branding.Name, branding.Subtitle, branding.Nature)
	if deviceMode {
		if err := s.SelectDevice(discovery.Device{Name: "EEBUS-Gerät " + address, Address: address, SKI: ski, Interface: interfaceName}, true); err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
		fmt.Fprintln(out, "Gerät vor Ort · bestätigte Geräte-SKI:", s.Device.SKI)
		fmt.Fprintln(out, "Eigene SKI:", s.EG.Identity.SKI, "· am Gerät/in dessen App als vertrauenswürdig eintragen.")
		fmt.Fprintf(out, "Nach dem Ausfalltest kann das Gerät bis %s im Notbetrieb bleiben. Langtest: %t.\n", c.FailsafeMinimum, c.LongTest)
		fmt.Fprintf(out, "Grenzen: LPC %.0f W; LPP %.0f W; Failsafe %.0f W; Mindestdauer %s. Wirkung je Schritt %s.\n", c.LPC, c.LPP, c.Failsafe, c.FailsafeMinimum, c.EffectTimeout)
		if command == "pair" {
			pctx, cancel := context.WithTimeout(ctx, c.StepTimeout)
			defer cancel()
			fmt.Fprintln(out, "SHIP-Verbindung und gegenseitiges Vertrauen abwarten …")
			if err := s.Connect(pctx); err != nil {
				fmt.Fprintln(errout, err)
				return 1
			}
			fmt.Fprintln(out, "SHIP/SPINE bereit. Keine Steuerbefehle gesendet.")
			return 0
		}
		fmt.Fprintln(out, "Geräteprüfung angefordert. Verbindung und gegenseitiges Vertrauen abwarten …")
	}
	switch command {
	case "identity":
		_ = json.NewEncoder(out).Encode(map[string]string{"energyguardSki": s.EG.Identity.SKI, "simulatorSki": s.Device.SKI})
		return 0
	case "discover":
		searchCtx, cancel := context.WithTimeout(ctx, searchTimeout)
		defer cancel()
		devices, err := discovery.Search(searchCtx, s.DNS.Address())
		if err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
		if err := writeDiscoveryJSON(out, devices); err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
		return 0
	case "pair":
		if err := s.Pair(ski); err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
		fmt.Fprintln(out, "Beidseitiges Vertrauen für Simulator-SKI", s.Device.SKI, "lokal gespeichert.")
		return 0
	case "serve":
		if loopbackOnly {
			if err := s.CS.Trust(s.EG.Identity.SKI); err != nil {
				fmt.Fprintln(errout, err)
				return 1
			}
			// Only the explicitly started fixture accepts this local process-control input.
			go func() {
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					switch scanner.Text() {
					case "hide-device":
						s.DNS.SetVisible(false)
						fmt.Fprintln(out, "Loopback-Ankündigung verborgen")
					case "show-device":
						s.DNS.SetVisible(true)
						fmt.Fprintln(out, "Loopback-Ankündigung sichtbar")
					case "stop-device":
						s.CS.Close()
						fmt.Fprintln(out, "Loopback-Gerät beendet")
					case "drop-connection":
						// 043: transient loss like a WLAN disassociation; the fixture keeps listening.
						s.CS.Disconnect()
						fmt.Fprintln(out, "Loopback-Verbindung getrennt")
					}
				}
			}()
		}
		return serve(ctx, s, listen, out, errout, loopbackOnly, loopbackPartner)
	case "demo":
		if err := s.Pair(s.Device.SKI); err != nil {
			fmt.Fprintln(errout, err)
			return 1
		}
		fmt.Fprintln(out, "Demo: beidseitige Kopplung mit dem isolierten Simulator ausdrücklich aktiviert.")
	}
	total := 90 * time.Second
	if deviceMode {
		total = c.RunDeadline() // 040 F-06: same formula as the WebUI
	}
	runCtx, cancel := context.WithTimeout(ctx, total)
	defer cancel()
	r, err := s.Run(runCtx, c)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 1
	}
	for _, step := range r.Steps {
		status := "Bestanden"
		if !step.Passed {
			status = "Nicht bestanden"
		}
		if step.Status == inspection.NotChecked {
			status = "Nicht geprüft"
		}
		fmt.Fprintf(out, "%s · %s · %s\n", status, step.Name, step.Detail)
	}
	exportFailed := false
	if jsonPath != "" {
		data, err := report.JSON(r)
		if err == nil {
			err = writePrivate(jsonPath, data)
		}
		if err != nil {
			fmt.Fprintln(errout, "JSON-Export:", err)
			exportFailed = true
		}
	}
	pdf, err := report.PDF(r)
	if err == nil {
		err = writePrivate(pdfPath, pdf)
	}
	if err != nil {
		fmt.Fprintln(errout, "PDF-Export:", err)
		exportFailed = true
	}
	fmt.Fprintln(out, r.ModeLabel()+" · "+r.Outcome())
	if !exportFailed {
		fmt.Fprintln(out, "PDF:", pdfPath)
	}
	fmt.Fprintln(out, inspection.Notice)
	if !r.Passed || exportFailed {
		return 1
	}
	return 0
}
func writePrivate(path string, data []byte) error { return storage.WriteFile(path, data) }
func serve(ctx context.Context, s *session.Session, addr string, out, errout io.Writer, loopbackOnly ...bool) int {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			fmt.Fprintf(errout, "Lokale HTTP-Adresse %s ist bereits belegt. Wählen Sie einen anderen Port mit --listen 127.0.0.1:<port> (z. B. --listen 127.0.0.1:33288).\n", addr)
		} else {
			fmt.Fprintf(errout, "Lokale HTTP-Adresse %s konnte nicht geöffnet werden: %v. Anderen Port mit --listen 127.0.0.1:<port> wählen.\n", addr, err)
		}
		return 1
	}
	return serveListener(ctx, s, listener, out, errout, loopbackOnly...)
}

// newHTTPServer bounds the whole request read (header and body) so a stalled local client cannot hold
// a request open indefinitely (040 F-03). Long-running handlers are unaffected: no WriteTimeout.
func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
}
func serveListener(ctx context.Context, s *session.Session, listener net.Listener, out, errout io.Writer, loopbackOnly ...bool) int {
	// 041: „Simulator beenden“ in der Oberfläche ends exactly this serve loop (and with it this process),
	// after a running inspection has been aborted and cleaned up by the app.
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	options := webui.Options{LoopbackOnly: len(loopbackOnly) > 0 && loopbackOnly[0], LoopbackPartner: len(loopbackOnly) > 1 && loopbackOnly[1], Shutdown: stop}
	app := webui.New(s, options)
	defer app.Close()
	server := newHTTPServer(app)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Fprintln(out, "Lokale Oberfläche: http://"+listener.Addr().String())
	fmt.Fprintln(out, "Übungsmodus oder ausdrücklicher Gerätebetrieb in der Oberfläche. Beenden mit Strg+C.")
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
		return 0
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(errout, err)
			return 1
		}
		return 0
	}
}
