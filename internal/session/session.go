// Package session owns an isolated EG/CS pair and the inspection backend.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/enbility/spine-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
)

type Session struct {
	state            string
	EG, CS           *eebus.Endpoint
	DNS              *discovery.Responder
	Device           discovery.Device
	mu               sync.Mutex
	paired           bool
	connected        bool
	busy             bool
	deviceMode       bool
	networkGate      func() bool
	limitDuration    time.Duration
	progress         string
	completed        []inspection.Step
	samples          []inspection.Sample
	limits           []inspection.LimitChange
	lowLoad          *inspection.LowLoad
	controlErr       error
	events           []inspection.Step
	connectionPhases []inspection.Step
	progressStarted  time.Time
	progressDeadline time.Time
	lowLoadTimeout   time.Duration
	lowLoadChoice    chan bool
	reconnection     ConnectionProgress
	recoveryCancel   context.CancelFunc
	recoveryStopped  bool
}

func New(state string) (*Session, error) {
	a, err := identity.LoadOrCreate(filepath.Join(state, "energyguard"))
	if err != nil {
		return nil, err
	}
	b, err := identity.LoadOrCreate(filepath.Join(state, "simulator"))
	if err != nil {
		return nil, err
	}
	s := &Session{state: state}
	s.EG, err = eebus.NewEndpointWithOptions(a, false, eebus.EndpointOptions{PracticeSpeed: 10})
	if err != nil {
		return nil, err
	}
	s.CS, err = eebus.NewEndpointWithOptions(b, true, eebus.EndpointOptions{PracticeSpeed: 10})
	if err != nil {
		s.EG.Close()
		return nil, err
	}
	s.Device = discovery.Device{Name: "Stromea EEBUS Loopback-Simulator", SKI: b.SKI, Brand: "Stromea", Model: "EEBUS Simulator", Address: s.CS.Address()}
	s.DNS, err = discovery.StartResponder(s.Device)
	if err != nil {
		s.Close()
		return nil, err
	}
	data, readErr := os.ReadFile(filepath.Join(state, "trust.json"))
	if readErr == nil {
		var trust struct {
			EG string
			CS string
		}
		if err = json.Unmarshal(data, &trust); err != nil {
			s.Close()
			return nil, fmt.Errorf("Vertrauensspeicher beschädigt: %w", err)
		}
		if trust.EG == a.SKI && trust.CS == b.SKI {
			if err = s.Pair(b.SKI); err != nil {
				s.Close()
				return nil, err
			}
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		s.Close()
		return nil, readErr
	}
	return s, nil
}
func (s *Session) Close() {
	if s.DNS != nil {
		s.DNS.Close()
	}
	if s.EG != nil {
		s.EG.Close()
	}
	if s.CS != nil {
		s.CS.Close()
	}
}
func (s *Session) Pair(ski string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return errors.New("Prüfung läuft")
	}
	if s.deviceMode {
		return errors.New("Gerätemodus: Geräte-SKI über Gerätewahl bestätigen")
	}
	v, err := identity.NormalizeSKI(ski)
	if err != nil {
		return err
	}
	if v != s.Device.SKI {
		return errors.New("SKI stimmt nicht mit dem isolierten Simulator überein")
	}
	if err = s.EG.Trust(v); err != nil {
		return err
	}
	if err = s.CS.Trust(s.EG.Identity.SKI); err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		EG string
		CS string
	}{s.EG.Identity.SKI, s.Device.SKI})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.state, "trust-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(s.state, "trust.json")); err != nil {
		return err
	}
	s.paired = true
	return nil
}
func (s *Session) Run(ctx context.Context, c inspection.Config) (inspection.Report, error) {
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return inspection.Report{}, errors.New("Prüfung läuft bereits")
	}
	s.busy = true
	if ctx.Err() == nil {
		s.recoveryStopped = false
	}
	s.progress = ""
	s.completed = nil
	s.samples = nil
	s.limits = nil
	s.events = nil
	s.lowLoad = nil
	s.lowLoadChoice = nil
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.busy = false; s.progress = ""; s.mu.Unlock() }()
	if s.IsDeviceMode() {
		runCtx, cancel := context.WithCancelCause(ctx)
		done := make(chan struct{})
		monitored := make(chan struct{})
		go func() {
			seenAlive := s.EG.Alive()
			defer close(monitored)
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-runCtx.Done():
					return
				case <-ticker.C:
					if s.EG.Alive() {
						seenAlive = true
					} else if seenAlive {
						s.RecordConnection("Verbindungsabbruch", "SHIP-Verbindung während der Prüfung getrennt", false)
						cancel(inspection.ErrConnectionLost)
						return
					}
				}
			}
		}()
		defer cancel(nil)
		ctx = runCtx
		s.mu.Lock()
		s.limitDuration = c.LimitLease()
		s.mu.Unlock()
		r, err := inspection.RunOnSite(ctx, c, inspection.Device{Name: s.Device.Name, SKI: s.Device.SKI}, s)
		close(done)
		cancel(nil)
		<-monitored
		r.Software = report.BuildVersion()
		r.LocalSKI = s.EG.Identity.SKI
		r.Events = s.ConnectionEvents()
		return r, err
	}
	r, err := inspection.Run(ctx, c, inspection.Device{Name: s.Device.Name, SKI: s.Device.SKI, Brand: s.Device.Brand, Model: s.Device.Model}, s)
	r.Software = report.BuildVersion()
	r.Events = s.ConnectionEvents()
	if err == nil {
		// Cached discovery data also belongs to a practice report; no new network read.
		r.Device.Type = s.DeviceInfo().Type
		if caps, readErr := s.Capabilities(context.Background()); readErr == nil {
			r.Capabilities = &caps
		}
	}
	return r, err
}
func (s *Session) Connect(ctx context.Context) (err error) {
	var traced *eebus.Endpoint
	defer func() {
		if traced != nil {
			s.recordConnectionTimings(traced.ConnectionTimings())
		}
	}()
	defer func() {
		if err != nil {
			s.Disconnect()
		}
	}()
	s.mu.Lock()
	paired, connected, running, deviceMode := s.paired, s.connected, s.busy, s.deviceMode
	s.mu.Unlock()
	if !paired {
		return errors.New("beidseitige SKI-Kopplung fehlt")
	}
	// A run performs one active, read-only probe with a short deadline. Cached
	// discovery/readiness alone cannot prove the peer still answers.
	if deviceMode && running {
		probeCtx, probeCancel := context.WithTimeout(ctx, 8*time.Second)
		defer probeCancel()
		ctx = probeCtx
	}
	if !connected || (deviceMode && !s.EG.Alive()) {
		traced = s.EG
		if err := s.dialDevice(ctx); err != nil {
			if errors.Is(err, ErrOffline) {
				return err
			}
			return fmt.Errorf("SHIP-Verbindung fehlgeschlagen; Geräte-SKI prüfen und eigene SKI am Gerät freigeben: %w", err)
		}
		s.mu.Lock()
		s.connected = true
		s.mu.Unlock()
	}
	if err := s.EG.Ready(ctx); err != nil {
		return fmt.Errorf("SHIP/SPINE nicht bereit (%s): %w", s.EG.ConnectionState(), err)
	}
	if deviceMode {
		s.readDeviceClassification(ctx)
		// Only during an inspection: the preparation connection already obtains
		// fresh discovery; the run must actively validate current reachability.
		if running {
			caps, err := s.Capabilities(ctx)
			if err != nil {
				return err
			}
			var probeErr error
			if caps.MPC {
				_, probeErr = s.Power(ctx)
			} else if caps.LPC {
				_, probeErr = s.ReadFailsafe(ctx, "LPC")
			} else if caps.LPP {
				_, probeErr = s.ReadFailsafe(ctx, "LPP")
			}
			if probeErr != nil {
				return fmt.Errorf("Verbindung vor Start nicht bestätigt: %w", probeErr)
			}
		}
	}
	return nil
}
func (s *Session) Supports(ctx context.Context) error {
	r := s.EG.RemoteEntity()
	if r == nil {
		return errors.New("keine SPINE-Entität")
	}
	for _, scenario := range []uint{1, 2, 3} {
		if !s.EG.LPC.IsScenarioAvailableAtEntity(r, scenario) || !s.EG.LPP.IsScenarioAvailableAtEntity(r, scenario) {
			return fmt.Errorf("LPC/LPP Szenario %d fehlt", scenario)
		}
	}
	if !s.EG.MPC.IsScenarioAvailableAtEntity(r, 1) {
		return errors.New("MPC Szenario 1 fehlt")
	}
	return ctx.Err()
}
func (s *Session) Configure(ctx context.Context, c inspection.Config) error {
	if s.IsDeviceMode() {
		return s.configureDevice(ctx, c)
	}
	// Start each repeat with both limits released and consumption as the simulator's load direction.
	for _, kind := range []string{"LPP", "LPC"} {
		if !c.Includes(kind) {
			continue
		}
		if _, err := s.Limit(ctx, kind, 0, false); err != nil {
			return err
		}
	}
	r := s.EG.RemoteEntity()
	for _, kind := range []string{"LPC", "LPP"} {
		if !c.Includes(kind) {
			continue
		}
		var err error
		if kind == "LPC" {
			_, err = s.EG.LPC.WriteFailsafeConsumptionActivePowerLimit(r, c.Failsafe)
		} else {
			_, err = s.EG.LPP.WriteFailsafeProductionActivePowerLimit(r, c.Failsafe)
		}
		if err != nil {
			return err
		}
	}
	if c.Includes("LPC") {
		if _, err := s.EG.LPC.WriteFailsafeDurationMinimum(r, 2*time.Hour); err != nil {
			return err
		}
	} else {
		if _, err := s.EG.LPP.WriteFailsafeDurationMinimum(r, 2*time.Hour); err != nil {
			return err
		}
	}
	return eebus.Poll(ctx, func() bool {
		if c.Includes("LPC") {
			a, err := s.EG.LPC.FailsafeConsumptionActivePowerLimit(r)
			if err != nil || a != c.Failsafe {
				return false
			}
		}
		if c.Includes("LPP") {
			b, err := s.EG.LPP.FailsafeProductionActivePowerLimit(r)
			if err != nil || b != c.Failsafe {
				return false
			}
		}
		if c.Includes("LPC") {
			d, err := s.EG.LPC.FailsafeDurationMinimum(r)
			return err == nil && d == 2*time.Hour
		}
		d, err := s.EG.LPP.FailsafeDurationMinimum(r)
		return err == nil && d == 2*time.Hour
	})
}
func (s *Session) Limit(ctx context.Context, kind string, value float64, active bool) (string, error) {
	if e := s.ControlError(); e != nil {
		return "", e
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r := s.EG.RemoteEntity()
	deviceMode := s.IsDeviceMode() // 040 F-10: read under s.mu
	if deviceMode {
		r = s.EG.EntityForUseCase(kind)
	}
	// A remote device left over from a closed transport never acknowledges.
	if r == nil || !s.EG.RemoteCurrent() {
		return "", errors.New("keine Verbindung")
	}
	result := make(chan model.ResultDataType, 1)
	cb := func(v model.ResultDataType) {
		select {
		case result <- v:
		default:
		}
	}
	var counter *model.MsgCounterType
	var err error
	duration := time.Minute
	s.mu.Lock()
	if deviceMode && s.limitDuration > 0 {
		duration = s.limitDuration
	}
	s.mu.Unlock()
	if kind == "LPP" {
		value = -value
	} // LPP APPL is non-positive; the API preserves its sign.
	if kind != "LPC" && kind != "LPP" {
		return "", errors.New("unbekannter Use Case")
	}
	counter, err = s.writeLimit(r, kind, value, active, duration, cb)
	if err != nil {
		return "", err
	}
	if counter == nil {
		return "", errors.New("keine Nachrichtennummer")
	}
	if err := s.waitControlResult(ctx, *counter, result); err != nil {
		return "", err
	}
	return fmt.Sprintf("SPINE result 0, Nachricht %d", *counter), nil
}
func (s *Session) Heartbeat(ctx context.Context, on bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h := s.EG.Entity.HeartbeatManager()
	deviceMode := s.IsDeviceMode() // 040 F-10
	if on {
		s.EG.AllowHeartbeats()
		if !h.IsHeartbeatRunning() {
			if err := h.StartHeartbeat(); err != nil {
				return err
			}
		}
		// Allow two heartbeat intervals before reading a device's baseline or recovery.
		// StartHeartbeat only starts a goroutine; it does not synchronously send data.
		if deviceMode {
			timer := time.NewTimer(2 * eebus.HeartbeatInterval)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
		return nil
	}
	if deviceMode {
		return s.EG.StopHeartbeat(ctx)
	}
	h.StopHeartbeat()
	return nil
}

// Power uses an owned value captured at the SHIP/SPINE receive boundary.
func (s *Session) Power(ctx context.Context) (float64, error) {
	return s.EG.ReadPower(ctx)
}

// PowerMeasuredAt is the device's own time of the last power reading (046).
func (s *Session) PowerMeasuredAt() (time.Time, bool) {
	t := s.EG.LastPowerMeasured()
	return t, !t.IsZero()
}

func (s *Session) Paired() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.paired }

// The result receipt is an upper bound for reception at the controlled device.
func (s *Session) LastHeartbeatReceipt() (time.Time, string) {
	return s.EG.LastHeartbeatReceipt(), "SPINE-Quittung des letzten Heartbeats lokal empfangen (obere Grenze für Empfang am Gerät)"
}

// ErrOffline is returned when a device-mode connection would need a network that is not available (039, R039-01).
var ErrOffline = errors.New("Keine Netzwerkverbindung")

// SetNetworkGate installs the availability rule checked before every new device-mode connection setup.
func (s *Session) SetNetworkGate(gate func() bool) { s.mu.Lock(); s.networkGate = gate; s.mu.Unlock() }

// dialDevice is the only place that opens a device-mode SHIP connection; the practice simulator is not gated.
func (s *Session) dialDevice(ctx context.Context) error {
	s.mu.Lock()
	gate, deviceMode := s.networkGate, s.deviceMode
	s.mu.Unlock()
	if deviceMode && gate != nil && !gate() {
		return ErrOffline
	}
	return s.EG.ConnectOnRoute(ctx, s.Device.Address, s.Device.SKI, eebus.DialRoute{Interface: s.Device.Interface, LocalAddress: s.Device.LocalAddress})
}
