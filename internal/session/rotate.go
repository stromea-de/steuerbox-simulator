package session

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/storage"
)

// ResetIdentity prepares both loopback endpoints before committing the new own
// identity atomically. The simulated device retains its installation identity.
func (s *Session) ResetIdentity(confirmed bool) error {
	if !confirmed {
		return errors.New("Neue Identität muss bestätigt werden; alte Kopplungen werden ungültig")
	}
	return s.replaceIdentity(nil)
}

// ImportIdentity (041) replaces the own simulator identity with an exported one (certificate and private
// key as PEM). Like ResetIdentity, existing pairings with other identities become invalid.
func (s *Session) ImportIdentity(pemData []byte) error {
	if _, err := identity.Parse(pemData); err != nil {
		return errors.New("Simulator-Kennung ungültig")
	}
	return s.replaceIdentity(pemData)
}

func (s *Session) replaceIdentity(pemData []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return errors.New("Prüfung läuft")
	}
	dir, err := os.MkdirTemp(s.state, ".identity-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if pemData != nil {
		if err = storage.WriteFile(filepath.Join(dir, "identity.pem"), pemData); err != nil {
			return err
		}
	}
	id, err := identity.LoadOrCreate(dir)
	if err != nil {
		return err
	}
	eg, err := eebus.NewEndpoint(id, false)
	if err != nil {
		return err
	}
	simID, err := identity.LoadOrCreate(filepath.Join(s.state, "simulator"))
	if err != nil {
		eg.Close()
		return err
	}
	cs, err := eebus.NewEndpoint(simID, true)
	if err != nil {
		eg.Close()
		return err
	}
	d := discovery.Device{Name: "Stromea EEBUS Loopback-Simulator", SKI: simID.SKI, Brand: "Stromea", Model: "EEBUS Simulator", Address: cs.Address()}
	dns, err := discovery.StartResponder(d)
	if err != nil {
		eg.Close()
		cs.Close()
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, "identity.pem"))
	if err == nil {
		err = storage.WriteFile(filepath.Join(s.state, "energyguard", "identity.pem"), data)
	}
	if err != nil {
		eg.Close()
		cs.Close()
		dns.Close()
		return err
	}
	oldEG, oldCS, oldDNS := s.EG, s.CS, s.DNS
	s.EG, s.CS, s.DNS, s.Device = eg, cs, dns, d
	s.paired, s.connected, s.deviceMode = false, false, false
	s.controlErr = nil
	s.limitDuration = 0
	s.progress = ""
	s.completed = nil
	s.connectionPhases = nil
	s.events = nil
	oldEG.Close()
	if oldCS != nil {
		oldCS.Close()
	}
	if oldDNS != nil {
		oldDNS.Close()
	}
	// Existing trust.json is bound to the old EG SKI; New deliberately ignores it.
	return nil
}
