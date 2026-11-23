package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/storage"
)

// 236: original device values survive a crash in the state directory, one file
// per device SKI. The files hold no secrets: device SKI/address and the values
// read before the test.
const pendingRestoreDir = "pending-restore"

type PendingRestore struct {
	Version int              `json:"version"`
	Device  discovery.Device `json:"device"`
	// Entities pins the SPINE entity of each use case (R2-05): a restore never
	// writes to another entity of the same device.
	Entities map[string][]model.AddressEntityType `json:"entities"`
	inspection.PendingRestore
}

// entityAddress is the entity the session writes to for kind.
func (s *Session) entityAddress(kind string) ([]model.AddressEntityType, error) {
	r := s.EG.EntityForUseCase(kind)
	if r == nil || r.Address() == nil || len(r.Address().Entity) == 0 {
		return nil, fmt.Errorf("%s-Gerätefunktion nicht gefunden", kind)
	}
	return append([]model.AddressEntityType(nil), r.Address().Entity...), nil
}

// setPins fixes the entity per use case for all following reads and writes (R3-01).
func (s *Session) setPins(p map[string][]model.AddressEntityType) {
	s.mu.Lock()
	s.pins = p
	s.mu.Unlock()
}

// PinEntities fixes the entity of every function the test may change before its
// originals are read (R4-02): originals, journal and every later write refer to the
// same entity; a function that was not pinned is not written at all.
func (s *Session) PinEntities(kinds []string) error {
	entities := map[string][]model.AddressEntityType{}
	for _, k := range kinds {
		e, err := s.entityAddress(k)
		if err != nil {
			return err
		}
		entities[k] = e
	}
	s.setPins(entities)
	return nil
}

// excluded reports a function outside the pinned set while pins are active (R4-02).
func (s *Session) excluded(kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.pins[kind]
	return s.pins != nil && !ok
}

// useCaseEntity is the remote entity for kind. While pinned, only the pinned
// entity qualifies, also after a reconnect re-ran discovery (R3-01); a missing or
// moved function yields nil, so nothing is read from or written to another entity.
func (s *Session) useCaseEntity(kind string) spineapi.EntityRemoteInterface {
	s.mu.Lock()
	pins := s.pins
	pin, pinned := pins[kind]
	s.mu.Unlock()
	r := s.EG.EntityForUseCase(kind)
	if pins == nil {
		return r
	}
	if !pinned {
		return nil // R4-02: a function without saved originals is never written to
	}
	if r == nil || r.Address() == nil || !slices.Equal(r.Address().Entity, pin) {
		return nil
	}
	return r
}

// lockDevice reserves the device for this test or restore across processes using the
// same state directory (R4-01): while held, no other simulator can read, save, restore
// or delete the originals of this device. The operating system frees it on a crash.
func (s *Session) lockDevice(ski string) (func(), error) {
	s.mu.Lock()
	state := s.state
	s.mu.Unlock()
	if state == "" {
		return func() {}, nil
	}
	if _, err := identity.NormalizeSKI(ski); err != nil {
		return nil, err
	}
	dir := filepath.Join(state, "device-lock")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	unlock, err := storage.TryLock(filepath.Join(dir, ski+".lock"))
	if errors.Is(err, storage.ErrLocked) {
		return nil, errors.New("Gerät wird gerade von einem anderen Simulatorfenster geprüft; nichts verändert")
	}
	return unlock, err
}

func (s *Session) restorePath(ski string) string {
	return filepath.Join(s.state, pendingRestoreDir, ski+".json")
}

func (s *Session) SaveRestore(p inspection.PendingRestore) error {
	s.mu.Lock()
	d, state := s.Device, s.state
	s.mu.Unlock()
	if state == "" || d.SKI == "" {
		return errors.New("kein Speicherort für Originalwerte")
	}
	s.mu.Lock()
	pins := s.pins
	s.mu.Unlock()
	entities := map[string][]model.AddressEntityType{}
	for k := range p.Failsafe {
		// R4-02: the entity pinned before the originals were read, never a newer one.
		if pins != nil {
			e, ok := pins[k]
			if !ok {
				return fmt.Errorf("%s-Gerätefunktion vor dem Lesen der Originalwerte nicht festgelegt", k)
			}
			entities[k] = e
			continue
		}
		e, err := s.entityAddress(k)
		if err != nil {
			return err
		}
		entities[k] = e
	}
	data, err := json.MarshalIndent(PendingRestore{Version: 1, Device: d, Entities: entities, PendingRestore: p}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.restorePath(d.SKI)), 0o700); err != nil {
		return err
	}
	s.recoverClaims(d.SKI)
	// R236-02: never replace open originals of an earlier test.
	if err := storage.CreateExclusive(s.restorePath(d.SKI), data); err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("für dieses Gerät sind noch Originalwerte einer früheren Prüfung offen")
		}
		return err
	}
	// R3-02: another record may sit in a claim file while its owner checks it. Any
	// such record makes this one void: it is removed again and nothing is written.
	if claims := s.claims(d.SKI); len(claims) > 0 {
		if err := s.clearRestoreIf(d.SKI, p.ReportID); err != nil {
			return fmt.Errorf("Originalwerte einer anderen Prüfung in Bearbeitung; eigener Datensatz nicht entfernt: %w", err)
		}
		return errors.New("für dieses Gerät sind noch Originalwerte einer früheren Prüfung offen")
	}
	s.setPins(entities)
	return nil
}

// claims lists claim files of ski (records taken out for a delete check).
func (s *Session) claims(ski string) []string {
	matches, _ := filepath.Glob(s.restorePath(ski) + ".claim-*")
	return matches
}

// recoverClaims puts records back that a crashed or failed delete check left as
// claim files (R3-02), so the next run restores them instead of ignoring them. A
// claim is removed only when the record at its place has the same content.
func (s *Session) recoverClaims(ski string) {
	path := s.restorePath(ski)
	for _, claim := range s.claims(ski) {
		if err := os.Link(claim, path); err != nil && !errors.Is(err, os.ErrExist) {
			continue
		}
		a, errA := os.ReadFile(claim)
		b, errB := os.ReadFile(path)
		if errA == nil && errB == nil && string(a) == string(b) {
			_ = os.Remove(claim)
		}
	}
}

// ClearRestore deletes the record of the selected device only when it still belongs
// to reportID (R236-02).
func (s *Session) ClearRestore(reportID string) error {
	s.mu.Lock()
	ski := s.Device.SKI
	s.mu.Unlock()
	return s.clearRestoreIf(ski, reportID)
}

// clearRestoreIf claims the record by an atomic rename before checking its owner
// (R2-02), so a newer record written by another process is never deleted. A
// foreign record is linked back without replacing anything; if that is not
// possible, it stays as a visible claim file.
func (s *Session) clearRestoreIf(ski, reportID string) error {
	path := s.restorePath(ski)
	claim := fmt.Sprintf("%s.claim-%d-%d", path, os.Getpid(), time.Now().UnixNano())
	if err := os.Rename(path, claim); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	data, err := os.ReadFile(claim)
	var p PendingRestore
	if err == nil {
		err = json.Unmarshal(data, &p)
	}
	if err == nil && p.ReportID == reportID {
		return os.Remove(claim)
	}
	if lerr := os.Link(claim, path); lerr != nil {
		return fmt.Errorf("Originalwerte einer anderen Prüfung nicht zurückgelegt (%v); Datei %s bleibt erhalten", lerr, filepath.Base(claim))
	}
	_ = os.Remove(claim)
	if err != nil {
		return err
	}
	return errors.New("Originalwerte gehören zu einer anderen Prüfung")
}

// RecoverAllClaims puts back claim files of every device; only at startup, before
// the automatic restore (R3-02), never while polling.
func (s *Session) RecoverAllClaims() {
	matches, _ := filepath.Glob(filepath.Join(s.state, pendingRestoreDir, "*.json.claim-*"))
	for _, m := range matches {
		ski, _, _ := strings.Cut(filepath.Base(m), ".json.claim-")
		if _, err := identity.NormalizeSKI(ski); err != nil {
			continue
		}
		// R5-04: a claim belongs to a running delete check while its device is
		// locked; it is put back only when no test or restore holds the device.
		unlock, err := s.lockDevice(ski)
		if err != nil {
			continue
		}
		s.recoverClaims(ski)
		unlock()
	}
}

// LoadRestore returns the saved originals of an interrupted test on this device, or nil.
func (s *Session) LoadRestore(ski string) (*PendingRestore, error) {
	if _, err := identity.NormalizeSKI(ski); err != nil {
		return nil, err
	}
	return s.loadRestore(ski, true)
}

// loadRestore reads the record; with recover it first puts back claim files
// (R3-02) and treats a remaining one as an open record of unknown content.
func (s *Session) loadRestore(ski string, recover bool) (*PendingRestore, error) {
	if recover {
		s.recoverClaims(ski)
	}
	data, err := os.ReadFile(s.restorePath(ski))
	if errors.Is(err, os.ErrNotExist) {
		if recover && len(s.claims(ski)) > 0 {
			return nil, errors.New("gespeicherte Originalwerte in Bearbeitung")
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p PendingRestore
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("gespeicherte Originalwerte beschädigt: %w", err)
	}
	if p.Version != 1 || len(p.Failsafe) == 0 || p.Device.SKI != ski || len(p.Entities) == 0 {
		return nil, errors.New("gespeicherte Originalwerte unvollständig")
	}
	if recover && len(s.claims(ski)) > 0 {
		return nil, errors.New("weitere gespeicherte Originalwerte für dieses Gerät offen")
	}
	return &p, nil
}

// BrokenRestores names saved records that cannot be read or were left as claim
// files (R2-06); they block nothing silently and are shown as failures.
func (s *Session) BrokenRestores() []string {
	entries, _ := os.ReadDir(filepath.Join(s.state, pendingRestoreDir))
	var out []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".stromea-") {
			continue // temporary file of an unfinished save; nothing was written then
		}
		ski, ok := strings.CutSuffix(name, ".json")
		if ok {
			if _, err := identity.NormalizeSKI(ski); err == nil {
				if p, err := s.loadRestore(ski, false); err == nil && p != nil {
					continue
				}
			}
		}
		out = append(out, name)
	}
	return out
}

// PendingRestores lists every device whose originals are not confirmed restored yet.
func (s *Session) PendingRestores() []PendingRestore {
	entries, _ := os.ReadDir(filepath.Join(s.state, pendingRestoreDir))
	var out []PendingRestore
	for _, e := range entries {
		ski, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok {
			continue
		}
		if _, err := identity.NormalizeSKI(ski); err != nil {
			continue
		}
		if p, err := s.loadRestore(ski, false); err == nil && p != nil {
			out = append(out, *p)
		}
	}
	return out
}

// RestoreTimeout covers a reconnect plus the device-mode heartbeat start.
var RestoreTimeout = 2*eebus.HeartbeatInterval + 30*time.Second

// RestorePending reconnects to the device of an interrupted test, writes the saved
// originals back and reads them back (236). It runs automatically after a restart;
// the record is deleted only when everything is confirmed.
func (s *Session) RestorePending(ctx context.Context, ski string) ([]inspection.Step, bool, error) {
	unlock, err := s.lockDevice(ski)
	if err != nil {
		return nil, false, err
	}
	defer unlock()
	p, err := s.LoadRestore(ski)
	if err != nil {
		return nil, false, err
	}
	if p == nil {
		return nil, false, errors.New("keine offenen Originalwerte")
	}
	if err := s.SelectDevice(p.Device, true); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return nil, false, errors.New("Prüfung läuft")
	}
	s.busy = true
	s.recoveryStopped = false
	s.limitDuration = time.Minute
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.busy = false; s.mu.Unlock() }()
	cctx, cancel := context.WithTimeout(ctx, RestoreTimeout)
	err = s.Connect(cctx)
	cancel()
	if err != nil {
		return nil, false, err
	}
	return s.restoreConnected(ctx, p)
}

// restoreConnected restores on the already selected device; used before a new run
// so a new test can never overwrite unrestored originals.
func (s *Session) restoreConnected(ctx context.Context, p *PendingRestore) ([]inspection.Step, bool, error) {
	for k := range p.Failsafe {
		cur, err := s.entityAddress(k)
		if err == nil && !slices.Equal(cur, p.Entities[k]) {
			err = fmt.Errorf("%s-Gerätefunktion liegt jetzt auf einer anderen Entität", k)
		}
		if err != nil {
			step := inspection.Step{Name: "Originalwerte prüfen", Detail: inspection.UserError(err) + "; nichts geschrieben, am Gerät prüfen", At: time.Now()}
			return []inspection.Step{step}, false, nil
		}
	}
	// R3-01: every read and write of the restore goes to the pinned entity, also
	// after a reconnect inside RestoreOriginals.
	s.setPins(p.Entities)
	defer s.setPins(nil)
	steps, ok := inspection.RestoreOriginals(ctx, s, p.PendingRestore, RestoreTimeout)
	if ok {
		if err := s.clearRestoreIf(p.Device.SKI, p.ReportID); err != nil {
			return steps, false, err
		}
	}
	return steps, ok, nil
}
