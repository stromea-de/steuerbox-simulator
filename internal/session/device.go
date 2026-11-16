package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/enbility/eebus-go/features/client"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// SelectDevice pins user-confirmed identity without connecting or controlling.
// Real trust is intentionally not auto-restored on the next invocation.
func (s *Session) SelectDevice(d discovery.Device, confirmed bool) error {
	if !confirmed {
		return errors.New("Geräte-SKI muss ausdrücklich bestätigt werden")
	}
	if err := eebus.DeviceAddress(d.Address); err != nil {
		return err
	}
	ski, err := identity.NormalizeSKI(d.SKI)
	if err != nil {
		return err
	}
	d.SKI = ski
	d, err = discovery.ResolveDeviceRoute(d)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return errors.New("Prüfung läuft")
	}
	id, err := identity.LoadOrCreate(filepath.Join(s.state, "energyguard"))
	if err != nil {
		return err
	}
	eg, err := eebus.NewEndpointWithOptions(id, false, eebus.EndpointOptions{DeviceMode: true})
	if err != nil {
		return err
	}
	if err = eg.Trust(ski); err != nil {
		eg.Close()
		return err
	}
	old := s.EG
	if s.Device.SKI != d.SKI || s.Device.Address != d.Address {
		s.connectionPhases = nil
		s.events = nil
	}
	s.EG = eg
	if s.CS != nil {
		s.CS.SetPracticeSpeed(1)
	}
	s.Device = d
	s.deviceMode = true
	s.controlErr = nil
	s.connected = false
	s.paired = true
	old.Close()
	return nil
}
func (s *Session) ShipID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.EG.Service.Configuration().Identifier()
}
func (s *Session) IsDeviceMode() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.deviceMode }
func (s *Session) Snapshot() (discovery.Device, string, bool, bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.EG.ConnectionState()
	if s.progress != "" {
		status = s.progress + " · " + status
	}
	return s.Device, s.EG.Identity.SKI, s.paired, s.deviceMode, status
}
func (s *Session) Capabilities(ctx context.Context) (inspection.Capabilities, error) {
	r := s.EG.RemoteEntity()
	if r == nil {
		return inspection.Capabilities{}, errors.New("keine SPINE-Entität")
	}
	if err := ctx.Err(); err != nil {
		return inspection.Capabilities{}, err
	}
	scenarios := map[string][]uint{"LPC": {}, "LPP": {}, "MPC": {}}
	collect := func(kind string, check func(spineapi.EntityRemoteInterface, uint) bool, count uint) bool {
		entity := s.EG.EntityForUseCase(kind)
		if entity == nil {
			return false
		}
		for n := uint(1); n <= count; n++ {
			if check(entity, n) {
				scenarios[kind] = append(scenarios[kind], n)
			}
		}
		return uint(len(scenarios[kind])) == count
	}
	lpc := collect("LPC", s.EG.LPC.IsScenarioAvailableAtEntity, 3)
	lpp := collect("LPP", s.EG.LPP.IsScenarioAvailableAtEntity, 3)
	mpc := collect("MPC", s.EG.MPC.IsScenarioAvailableAtEntity, 1)
	return inspection.Capabilities{LPC: lpc, LPP: lpp, MPC: mpc, Scenarios: scenarios}, nil
}
func (s *Session) DeviceInfo() inspection.Device {
	d := inspection.Device{Name: s.Device.Name, SKI: s.Device.SKI, Brand: s.Device.Brand, Model: s.Device.Model, Source: "Gerätedaten: vom Gerät gemeldet"}
	r := s.EG.RemoteEntity()
	if r == nil {
		return d
	}
	// spine-go v0.7.0 api.DeviceInterface.DeviceType(), populated by NodeManagement detailed discovery.
	if typ := r.Device().DeviceType(); typ != nil {
		d.Type = string(*typ)
	}
	// eebus-go v0.7.0 features/client/deviceclassification.go: GetManufacturerDetails.
	// Classification normally lives on the device-information entity, not the CS.
	// 040 F-11: the device-information entity wins, then the first entity reporting a value.
	// Sub-components (e.g. an EMS with several classified entities) never overwrite it.
	var details []model.DeviceClassificationManufacturerDataType
	entities := append([]spineapi.EntityRemoteInterface(nil), r.Device().Entities()...)
	sort.SliceStable(entities, func(i, j int) bool {
		return entities[i].EntityType() == model.EntityTypeTypeDeviceInformation && entities[j].EntityType() != model.EntityTypeTypeDeviceInformation
	})
	for _, entity := range entities {
		f, err := client.NewDeviceClassification(s.EG.Entity, entity)
		if err != nil {
			continue
		}
		data, err := f.GetManufacturerDetails()
		if err != nil || data == nil {
			continue
		}
		details = append(details, *data)
	}
	return applyManufacturer(d, details)
}

// applyManufacturer takes brand, serial and firmware from ordered manufacturer details (first value wins)
// and lets them replace discovery data, as before; the model is only filled when still empty.
func applyManufacturer(d inspection.Device, details []model.DeviceClassificationManufacturerDataType) inspection.Device {
	var brand, serial, firmware string
	first := func(field *string, v *string) {
		if *field == "" && v != nil && *v != "" {
			*field = *v
		}
	}
	for _, data := range details {
		if data.VendorName != nil {
			first(&brand, (*string)(data.VendorName))
		} else if data.BrandName != nil {
			first(&brand, (*string)(data.BrandName))
		}
		first(&serial, (*string)(data.SerialNumber))
		first(&firmware, (*string)(data.SoftwareRevision))
	}
	for _, field := range []struct{ target, value *string }{{&d.Brand, &brand}, {&d.Serial, &serial}, {&d.Firmware, &firmware}} {
		if *field.value != "" {
			*field.target = *field.value
		}
	}
	for _, data := range details {
		for _, name := range []*string{(*string)(data.DeviceName), (*string)(data.DeviceCode)} {
			if name == nil {
				continue
			}
			candidate := inspection.Device{Model: *name, SKI: d.SKI, Serial: d.Serial}
			if d.ModelLabel() == "" && candidate.ModelLabel() != "" {
				d.Model = candidate.ModelLabel()
			}
		}
	}
	return d
}

// Disconnect is called only after the operation worker has stopped. No session
// replacement can race an old dial/handshake/read completing after cancellation.
func (s *Session) Disconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EG.Disconnect()
	s.connected = false
	s.reconnection.Active = false
	s.reconnection.TimedOut = false
}

// DisconnectAndWait is used by unattended repetition before choosing a new endpoint.
func (s *Session) DisconnectAndWait(ctx context.Context) error {
	s.mu.Lock()
	endpoint := s.EG
	s.connected = false
	s.reconnection.Active = false
	s.reconnection.TimedOut = false
	s.mu.Unlock()
	return endpoint.DisconnectAndWait(ctx)
}

// An explicit user abort also stops recovery in the independent cleanup
// context. A failed release remains unconfirmed in the inspection report.
func (s *Session) CancelRecovery() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recoveryStopped = true
	if s.recoveryCancel != nil {
		s.recoveryCancel()
	}
}

type ConnectionProgress struct {
	Active   bool      `json:"active"`
	TimedOut bool      `json:"timedOut"`
	Attempt  int       `json:"attempt"`
	Deadline time.Time `json:"deadline"`
}

func (s *Session) Reconnection() ConnectionProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reconnection
}

func (s *Session) Recover(ctx context.Context) (err error) {
	// A user abort stops new connection attempts. An existing transport may still
	// release limits and restore settings using the bounded cleanup context.
	if e := s.ControlError(); e != nil {
		return e
	}
	if s.EG.Alive() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	s.mu.Lock()
	if s.recoveryStopped {
		s.mu.Unlock()
		return context.Canceled
	}
	s.recoveryCancel = cancel
	s.reconnection = ConnectionProgress{Active: true, Deadline: deadline}
	s.mu.Unlock()
	defer func() {
		if err != nil {
			s.Disconnect()
		}
		s.mu.Lock()
		s.reconnection.Active = false
		s.reconnection.TimedOut = errors.Is(err, context.DeadlineExceeded)
		s.recoveryCancel = nil
		s.mu.Unlock()
	}()
	s.RecordConnection("Verbindungsabbruch", "SHIP-Verbindung getrennt; Wiederverbindung wird versucht", false)
	s.EG.Disconnect()
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("Verbindung nicht wiederhergestellt (%v): %w", last, err)
		}
		s.mu.Lock()
		s.reconnection.Attempt++
		s.mu.Unlock()
		if err := s.dialDevice(ctx); err != nil {
			last = err
		} else {
			if err = s.EG.Ready(ctx); err == nil {
				s.RecordConnection("Verbindung neu aufgebaut", "SHIP/SPINE erneut bereit", true)
				return nil
			} else {
				last = err
				s.EG.Disconnect()
			}
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
func NewController(state string) (*Session, error) {
	id, err := identity.LoadOrCreate(filepath.Join(state, "energyguard"))
	if err != nil {
		return nil, err
	}
	eg, err := eebus.NewEndpoint(id, false)
	if err != nil {
		return nil, err
	}
	return &Session{state: state, EG: eg}, nil
}

func (s *Session) readDeviceClassification(ctx context.Context) {
	r := s.EG.RemoteEntity()
	if r == nil {
		return
	}
	for _, entity := range r.Device().Entities() {
		if entity.FeatureOfTypeAndRole(model.FeatureTypeTypeDeviceClassification, model.RoleTypeServer) == nil {
			continue
		}
		f, err := client.NewDeviceClassification(s.EG.Entity, entity)
		if err != nil {
			continue
		}
		if _, err = f.RequestManufacturerDetails(); err != nil {
			continue
		}
		mctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_ = eebus.Poll(mctx, func() bool { _, err := f.GetManufacturerDetails(); return err == nil })
		cancel()
	}
}

func (s *Session) SelectSimulator() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return errors.New("Prüfung läuft")
	}
	if !s.deviceMode {
		return nil
	}
	fresh, err := New(s.state)
	if err != nil {
		return err
	}
	old := s.EG
	s.EG = fresh.EG
	s.Device = fresh.Device
	s.paired = fresh.paired
	s.connected = false
	s.deviceMode = false
	s.connectionPhases = nil
	s.events = nil
	s.controlErr = nil
	s.limitDuration = 0
	if s.CS != nil {
		s.CS.Close()
	}
	if s.DNS != nil {
		s.DNS.Close()
	}
	s.CS = fresh.CS
	s.DNS = fresh.DNS
	old.Close()
	return nil
}

func (s *Session) StepStarted(name string) { s.StepWaiting(name, time.Time{}) }
func (s *Session) StepWaiting(name string, deadline time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = name
	s.progressStarted = time.Now()
	s.progressDeadline = deadline
}

func (s *Session) StepFinished(step inspection.Step) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed = append(s.completed, step)
	s.progress = ""
	s.progressDeadline = time.Time{}
}
func (s *Session) Progress() inspection.Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	var low *inspection.LowLoad
	if s.lowLoad != nil {
		v := *s.lowLoad
		low = &v
	}
	return inspection.Progress{Started: s.progressStarted, Deadline: s.progressDeadline, Events: append([]inspection.Step(nil), s.events...), Current: s.progress, Steps: append([]inspection.Step(nil), s.completed...), Samples: cloneSamples(s.samples), Limits: cloneLimits(s.limits), LowLoad: low}
}

// ResetProgress prevents an earlier run from leaking into the first HTTP poll.
func (s *Session) ResetProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = ""
	s.completed = nil
	s.samples = nil
	s.limits = nil
	s.events = nil
	s.lowLoad = nil
	s.lowLoadChoice = nil
}

func cloneSamples(samples []inspection.Sample) []inspection.Sample {
	out := append([]inspection.Sample(nil), samples...)
	for i := range out {
		if out[i].Watts != nil {
			value := *out[i].Watts
			out[i].Watts = &value
		}
	}
	return out
}
func cloneLimits(limits []inspection.LimitChange) []inspection.LimitChange {
	out := append([]inspection.LimitChange(nil), limits...)
	for i := range out {
		if out[i].Watts != nil {
			value := *out[i].Watts
			out[i].Watts = &value
		}
	}
	return out
}

// LimitChanged keeps the live limit timeline (UX-S21) next to the samples.
func (s *Session) LimitChanged(change inspection.LimitChange) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.limits = append(s.limits, cloneLimits([]inspection.LimitChange{change})[0])
}
func (s *Session) PowerSample(sample inspection.Sample) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.samples) == inspection.SampleCapacity {
		copy(s.samples, s.samples[1:])
		s.samples = s.samples[:inspection.SampleCapacity-1]
	}
	s.samples = append(s.samples, cloneSamples([]inspection.Sample{sample})[0])
}
func (s *Session) LowLoad(ctx context.Context, v inspection.LowLoad) (bool, error) {
	timeout := s.lowLoadTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	s.StepWaiting("Lastentscheidung vor Start", deadline)
	choice := make(chan bool, 1)
	s.mu.Lock()
	s.lowLoad = &v
	s.lowLoadChoice = choice
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.lowLoad = nil
		s.lowLoadChoice = nil
		s.progressDeadline = time.Time{}
		s.mu.Unlock()
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return false, context.Canceled
			}
			return false, fmt.Errorf("Lastentscheidung abgelaufen: %w: %w", inspection.ErrNoLoadDecision, ctx.Err())
		case retry := <-choice:
			return retry, nil
		case <-ticker.C:
			if s.EG != nil && !s.EG.Alive() {
				s.RecordConnection("Verbindungsabbruch", "Verbindung während Lastentscheidung getrennt", false)
				// 043: classified as connection_lost (the result names the cause; the report has abortReason).
				return false, fmt.Errorf("Verbindung während Lastentscheidung getrennt: %w", inspection.ErrConnectionLost)
			}
		}
	}
}
func (s *Session) ResolveLowLoad(retry bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lowLoadChoice == nil {
		return errors.New("Keine Lastentscheidung offen")
	}
	select {
	case s.lowLoadChoice <- retry:
		return nil
	default:
		return errors.New("Entscheidung bereits übernommen")
	}
}

// LiveTrust serializes the endpoint pointer with explicit session replacement.
func (s *Session) LiveTrust() eebus.TrustState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.EG.ConnectionTrust()
}
func (s *Session) LiveConnected() bool { return s.LiveTrust().Simulator }

// FailsafeDelay reports the simulated device's heartbeat timeout (UX-S21);
// without a simulated device the configured timeout applies.
func (s *Session) FailsafeDelay() time.Duration {
	if s.CS == nil {
		return 0
	}
	return s.CS.FailsafeDelay()
}
