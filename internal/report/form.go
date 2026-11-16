package report

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
)

// formChecks groups the internal evidence without changing the stored report or
// evaluation. Every required member must pass; a later success cannot hide failure.
func formChecks(r inspection.Report, kind string) []checkpoint {
	baseline := "Ausgangsmessung MPC"
	if kind == "LPP" {
		baseline = "LPP Ausgangsmessung MPC"
	}
	effect := []string{kind + " gemessene Wirkung"}
	// Legacy both-direction simulator reports have one common baseline only.
	if kind == "LPC" || r.Config.Scope == inspection.ScopeProduction || r.SchemaVersion >= 1 && r.Mode == inspection.ModeOnSite {
		effect = append([]string{baseline}, effect...)
	}
	recovery := []string{kind + " Wiederkehr"}
	if r.Mode == inspection.ModeOnSite && r.Config.Scope != "" {
		if r.Config.FullTest() {
			recovery = append(recovery, kind+" Wiederkehr gemessene Wirkung")
		} else {
			recovery = []string{kind + " Vorzeitige Freigabe", kind + " Vorzeitige Rückkehr gemessene Wirkung"}
		}
	}
	checks := []checkpoint{{"Grenze angenommen", []string{kind + " Grenze + Quittung"}}, {"Grenze wirkt", effect}, {"Notbetrieb bei Ausfall", []string{kind + " Heartbeat-Ausfall", kind + " Failsafe gemessene Wirkung"}}, {"Rückkehr zum Normalbetrieb", recovery}}
	if r.Config.FullTest() {
		checks = append(checks, checkpoint{"Notbetrieb hält Mindestdauer", []string{kind + " Wiederkehr"}})
	}
	return checks
}
func connectionLost(r inspection.Report) bool {
	if r.AbortReason == "connection_lost" {
		return true
	}
	for _, e := range r.Events {
		if e.Name == "Verbindungsabbruch" {
			return true
		}
	}
	return false
}
func stepConnectionLost(r inspection.Report, s inspection.Step) bool {
	if s.ReasonCode != "" {
		return s.ReasonCode == "connection_lost"
	}
	return connectionLost(r) && (strings.Contains(s.Detail, "context canceled") || strings.Contains(s.Detail, "Verbindung") || strings.Contains(s.Detail, "connection_lost"))
}
func lowBaseline(r inspection.Report, kind string) *inspection.Step {
	name := "Ausgangsmessung MPC"
	if kind == "LPP" {
		name = "LPP Ausgangsmessung MPC"
	}
	for _, s := range r.Steps {
		if s.Name == name && (s.ReasonCode == "insufficient_load" || s.Status == inspection.NotChecked && strings.Contains(s.Detail, "zu wenig Leistung")) {
			return &s
		}
	}
	return nil
}
func formRemark(r inspection.Report, c checkpoint) string {
	status := checkpointStatus(r, c)
	if status == "bestanden" {
		for i := len(c.steps) - 1; i >= 0; i-- {
			for _, s := range r.Steps {
				if s.Name == c.steps[i] && s.Watts != nil {
					return "Gemessen: " + powerLabel(*s.Watts) + "."
				}
			}
		}
		return "Bestätigt."
	}
	if status == "nicht bestanden" {
		for _, name := range c.steps {
			for _, s := range r.Steps {
				if s.Name != name || s.Passed || s.Status == inspection.NotChecked {
					continue
				}
				if stepConnectionLost(r, s) {
					return "Keine Antwort. Verbindung riss ab."
				}
				if s.ReasonCode == "timeout" || strings.Contains(s.Detail, "deadline") {
					return "Keine Antwort im Zeitlimit."
				}
				return "Gerät bestätigt den Prüfpunkt nicht."
			}
		}
	}
	kind := "LPC"
	if strings.HasPrefix(c.steps[0], "LPP ") {
		kind = "LPP"
	}
	if c.text == "Grenze wirkt" {
		if s := lowBaseline(r, kind); s != nil {
			note := "Zu wenig Last"
			if s.Watts != nil {
				if *s.Watts == 0 {
					note = "Keine Last (0 W)"
				} else {
					note += " (" + powerLabel(*s.Watts) + ")"
				}
			}
			if connectionLost(r) {
				return "Abbruch. Zudem " + strings.ToLower(note[:1]) + note[1:] + "."
			}
			return note + "."
		}
	}
	for _, name := range c.steps {
		for _, s := range r.Steps {
			if s.Name == name && s.Status == inspection.NotChecked {
				if s.ReasonCode == "connection_lost" {
					return "Abbruch durch Verbindungsverlust."
				}
				if strings.Contains(s.Detail, "MPC nicht") {
					return "Gerät meldet keine Leistung."
				}
				if strings.Contains(s.Detail, "zu wenig Leistung") {
					return "Leistung reicht für den Nachweis nicht aus."
				}
			}
		}
	}
	if displayAbort(r) != "" {
		return "Abbruch vorher."
	}
	return "Nicht geprüft: Nachweis fehlt."
}
func nextActions(r inspection.Report) []string {
	var actions []string
	if connectionLost(r) {
		actions = append(actions, "Verbindung stabil machen, etwa per Netzwerkkabel.")
	} else if r.AbortReason == "timeout" {
		actions = append(actions, "Verbindung und Antwort des Geräts prüfen.")
	} else if r.AbortReason == "user_cancel" {
		actions = append(actions, "Abbruchursache klären und Gerät für den nächsten Start vorbereiten.")
	}
	for _, kind := range []string{"LPC", "LPP"} {
		if !r.Config.Includes(kind) {
			continue
		}
		if lowBaseline(r, kind) != nil {
			if kind == "LPC" {
				actions = append(actions, "Vor dem Start Last einschalten: mehr als "+watts(r.Config.LPC+r.Config.Tolerance)+".")
			} else {
				actions = append(actions, "Vor dem Start Einspeisung bereitstellen: mehr als "+watts(r.Config.LPP+r.Config.Tolerance)+".")
			}
		}
	}
	for _, s := range r.Steps {
		if s.Status == inspection.NotChecked && strings.Contains(s.Detail, "MPC nicht") {
			actions = append(actions, "Leistungsmeldung (MPC) am Gerät prüfen.")
			break
		}
	}
	if !strings.Contains(r.ReleaseSummary(), "Ja") {
		actions = append(actions, "Testgrenzen am Gerät aufheben und Freigabe prüfen.")
	}
	if r.Mode == inspection.ModeOnSite && r.FailsafeRestore() == "unconfirmed" {
		actions = append(actions, "Notbetriebswerte am Gerät prüfen und wieder wie vor der Prüfung einstellen.")
	}
	if len(actions) == 0 {
		for _, s := range r.Steps {
			if !s.Passed && s.Status != inspection.NotChecked {
				actions = append(actions, "Einstellungen für „"+inspection.StepName(s.Name)+"“ am Gerät prüfen.")
				break
			}
		}
	}
	if len(actions) == 0 {
		actions = append(actions, "Fehlende Prüfnachweise in der Prüfübersicht prüfen.")
	}
	return append(actions, "Prüfung wiederholen.")
}
