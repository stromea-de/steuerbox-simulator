package session

import (
	"context"
	"errors"
	"fmt"
	"github.com/enbility/spine-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"sort"
	"time"
)

func (s *Session) ControlError() error { s.mu.Lock(); defer s.mu.Unlock(); return s.controlErr }
func (s *Session) waitControlResult(ctx context.Context, counter model.MsgCounterType, result <-chan model.ResultDataType) error {
	select {
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			s.mu.Lock()
			s.controlErr = inspection.ErrControlUnresponsive
			s.mu.Unlock()
			s.RecordConnection("Steuerbefehle ohne Antwort", "Steuerquittung fehlt; weitere Schreibversuche bis zum Neuaufbau gesperrt", false)
			return fmt.Errorf("%w: Quittung für Nachricht %d fehlt: %w", inspection.ErrControlUnresponsive, counter, ctx.Err())
		}
		return fmt.Errorf("Quittung für Nachricht %d fehlt: %w", counter, ctx.Err())
	case v := <-result:
		if v.ErrorNumber == nil {
			return fmt.Errorf("ungültige SPINE-Quittung für Nachricht %d: Fehlernummer fehlt", counter)
		}
		if *v.ErrorNumber == model.ErrorNumberTypeCommandRejected {
			return fmt.Errorf("SPINE-Quittung für Nachricht %d: vom Gerät abgelehnt (keine Bindung/abgelehnt; commandRejected, Fehlernummer %d)", counter, *v.ErrorNumber)
		}
		if *v.ErrorNumber != 0 {
			return fmt.Errorf("negative SPINE-Quittung für Nachricht %d: vom Gerät abgelehnt (Fehlernummer %d)", counter, *v.ErrorNumber)
		}
		return nil
	}
}
func (s *Session) RecordConnection(name, detail string, passed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, inspection.Step{Name: name, Detail: detail, Passed: passed, At: time.Now()})
}
func (s *Session) ConnectionEvents() []inspection.Step {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := append(append([]inspection.Step(nil), s.connectionPhases...), s.events...)
	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })
	return events
}

// Rebuild replaces the entire endpoint (including stale subscriptions/requests),
// retaining only the explicitly selected identity and peer. Caller must be idle.
func (s *Session) Rebuild(ctx context.Context) error {
	s.RecordConnection("Verbindung neu aufbauen", "Bisherige SHIP/SPINE-Sitzung wird geschlossen", true)
	if s.IsDeviceMode() {
		if e := s.SelectDevice(s.Device, true); e != nil {
			return e
		}
	} else {
		s.EG.Disconnect()
		s.mu.Lock()
		s.connected = false
		s.controlErr = nil
		s.mu.Unlock()
	}
	e := s.Connect(ctx)
	if e == nil {
		_, e = s.Capabilities(ctx)
	}
	detail := "SHIP/SPINE neu verbunden; Gerätefunktionen erneut erkannt"
	if e != nil {
		detail = "Neuaufbau nicht bestätigt"
	}
	s.RecordConnection("Verbindung neu aufgebaut", detail, e == nil)
	return e
}

// Keep preparation/reconnect timings apart from per-run progress, which is reset
// on Start. SelectDevice clears them when the selected peer changes.
func (s *Session) recordConnectionTimings(rows []eebus.ConnectionTiming) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range rows {
		detail := fmt.Sprintf("%d ms · %s", row.DurationMS, row.Status)
		if row.Detail != "" {
			detail += " · " + row.Detail
		}
		s.connectionPhases = append(s.connectionPhases, inspection.Step{Category: "event", Name: row.Phase, At: row.Started, Passed: row.Status == "complete", Detail: detail})
	}
}
func (s *Session) ConnectionTimings() []eebus.ConnectionTiming {
	s.mu.Lock()
	endpoint := s.EG
	s.mu.Unlock()
	return endpoint.ConnectionTimings()
}
