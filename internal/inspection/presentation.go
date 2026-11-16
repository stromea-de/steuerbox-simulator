package inspection

import "strings"

func GroupSKI(value string) string {
	compact := strings.NewReplacer(" ", "", ":", "", "-", "", "\n", "", "\t", "").Replace(value)
	var blocks []string
	for len(compact) > 4 {
		blocks = append(blocks, strings.ToUpper(compact[:4]))
		compact = compact[4:]
	}
	if compact != "" {
		blocks = append(blocks, strings.ToUpper(compact))
	}
	return strings.Join(blocks, " ")
}
func StepName(name string) string {
	names := map[string]string{"Verbindung": "Datenverbindung prüfen", "Unterstützte Use Cases": "Gerätefunktionen prüfen", "Failsafe-Werte setzen": "Notbetrieb konfigurieren", "Heartbeat starten": "Lebenszeichen senden (Heartbeat)", "Ausgangsmessung MPC": "Ausgangsleistung messen", "LPP Ausgangsmessung MPC": "Einspeisung messen", "Grenzen aufheben": "Testgrenzen aufheben"}
	if title, ok := names[name]; ok {
		return title
	}
	direction := "Bezug"
	if strings.HasPrefix(name, "LPP ") {
		direction = "Einspeisung"
	}
	suffix := strings.TrimPrefix(strings.TrimPrefix(name, "LPC "), "LPP ")
	actions := map[string]string{"Ausgangsmessung MPC": "Ausgangsleistung messen", "Grenze + Quittung": "Grenze setzen", "gemessene Wirkung": "Leistungsanpassung messen", "Heartbeat-Ausfall": "Lebenszeichen unterbrechen (Heartbeat)", "Failsafe gemessene Wirkung": "Notbetrieb messen", "Wiederkehr": "Mindestdauer abwarten", "Wiederkehr gemessene Wirkung": "Rückkehr nach Mindestdauer messen", "Heartbeat wieder starten": "Lebenszeichen senden (Heartbeat)", "Grenze freigeben": "Grenze freigeben", "Grenzen aufheben": "Testgrenzen aufheben", "Vorzeitige Freigabe": "Freigabe versuchen", "Vorzeitige Rückkehr gemessene Wirkung": "Rückkehr messen", "Ausfallfrist abwarten": "Ausfallfrist abwarten", "Wiederkehr Protokoll": "Rückkehr bestätigen lassen", "Failsafe Ursprungswerte": "Notbetriebs-Werte sichern", "Failsafe-Leistung zurückstellen": "Notbetriebs-Leistung zurückstellen", "Failsafe-Mindestdauer zurückstellen": "Notbetriebs-Dauer zurückstellen"}
	if action, ok := actions[suffix]; ok {
		return direction + ": " + action
	}
	return name
}

func (r Report) ReleaseSummary() string {
	for _, s := range r.Steps {
		if s.Name == "Grenzen aufheben" && s.Passed && s.Status != NotChecked {
			return "Testgrenzen aufgehoben: Ja, vom Gerät bestätigt"
		}
	}
	return "Testgrenzen aufgehoben: Nicht bestätigt · am Gerät kontrollieren"
}

// FailsafeRestore classifies restoring the device's fallback values (EEBUS LPC/LPP failsafe power limit
// and failsafe duration minimum) after the test (042): "simulated" (practice), "confirmed" (every restore
// write acknowledged and read back for every saved direction), "unchanged" (the test never tried to write
// fallback values, e.g. cancelled before or original values unreadable; R042-01) or "unconfirmed".
func (r Report) FailsafeRestore() string {
	if r.Mode != ModeOnSite {
		return "simulated"
	}
	count, touched := 0, false
	for _, s := range r.Steps {
		if (s.Name == "Failsafe-Werte setzen" && s.Status != NotChecked) || strings.Contains(s.Name, "zurückstellen") {
			touched = true
		}
	}
	if !touched {
		return "unchanged"
	}
	for _, s := range r.Steps {
		if strings.Contains(s.Name, "zurückstellen") {
			if !s.Passed || s.Status == NotChecked {
				return "unconfirmed"
			}
			count++
		}
	}
	if count > 0 && count >= 2*len(r.FailsafeOriginals) {
		return "confirmed"
	}
	return "unconfirmed"
}

// FailsafeWarningWeb is the single action hint in the web result (UX-S16: one action only).
const FailsafeWarningWeb = "Wiederherstellung der Notbetriebswerte nicht bestätigt. Stelle am Gerät Leistungsgrenze und Mindestdauer für Kommunikationsausfall wieder wie vor der Prüfung ein."

func (r Report) OpenEvidence() []Step {
	var out []Step
	for _, s := range r.Steps {
		if s.Status == NotChecked && !r.optionalStep(s.Name) {
			out = append(out, s)
		}
	}
	return out
}

// Reason separates decision-relevant explanations from recorded protocol diagnostics.
func (s Step) Reason() string {
	switch {
	case strings.Contains(s.Detail, "Langtest"):
		return "Mindestdauer nicht geprüft: In der Funktionsprüfung wird sie nicht abgewartet"
	case strings.Contains(s.Detail, "MPC nicht"), strings.Contains(s.Detail, "MPC fehlt"):
		return "Das Gerät meldet keine Leistungsmessung."
	case strings.Contains(s.Detail, "nicht vom Gerät gemeldet"):
		return "vom Gerät nicht gemeldet"
	case strings.Contains(s.Detail, "zu wenig Leistung"), strings.Contains(s.Detail, "Zu wenig Leistung"):
		return "Die Leistung reicht für diesen Nachweis nicht aus."
	case strings.Contains(s.Detail, "Vorzeitige Rückkehr nicht nachgewiesen"):
		return "Die Rückkehr wurde nicht bestätigt. Notbetrieb am Gerät prüfen."
	case strings.Contains(s.Detail, "Keine Änderung ohne lesbare Ursprungswerte"):
		return "Die ursprünglichen Notbetriebs-Werte sind nicht lesbar. Wir ändern keine Werte."
	case s.Status == NotChecked:
		return "Dieser Nachweis fehlt. Prüfschritt öffnen."
	case !s.Passed:
		return "Der Prüfschritt ist fehlgeschlagen. Einstellungen und Gerät prüfen."
	default:
		return "Bestätigt"
	}
}

func (s Step) UserDetail() string { return UserDetail(s.Detail) }

// Display-only clarification; recorded step names and decisions remain unchanged.
func (r Report) StepLabel(s Step) string {
	if r.Mode == ModeSimulator && (s.Name == "LPC Wiederkehr" || s.Name == "LPP Wiederkehr") {
		direction := "Bezug"
		if strings.HasPrefix(s.Name, "LPP") {
			direction = "Einspeisung"
		}
		return direction + ": Mindestdauer verkürzt simulieren"
	}
	return StepName(s.Name)
}
func (r Report) ProfileSummary() string {
	if r.Mode == ModeSimulator {
		return "Übung: Eintritt und Rückkehr werden simuliert; die Mindestdauer wird nur verkürzt simuliert, nicht zeitlich nachgewiesen"
	}
	if r.Config.FullTest() {
		return "Eintritt in den Notbetrieb und Rückkehr; die eingestellte Mindestdauer wird beobachtet"
	}
	return "Eintritt in den Notbetrieb und Versuch, ihn auf Befehl der Steuerbox vorzeitig zu beenden; Mindestdauer nicht abgewartet"
}

// FailsafeNote is a neutral line only for an unconfirmed restore on a real device.
func (r Report) FailsafeNote() string {
	if r.Mode == ModeOnSite && r.FailsafeRestore() == "unconfirmed" {
		return FailsafeWarningWeb
	}
	return ""
}

// ReleaseStatus classifies releasing the test limits like a table cell (UX-S16, 038):
// failed beats not checked, a missing attempt is still open.
func (r Report) ReleaseStatus() string {
	status := "open"
	for _, s := range r.Steps {
		if s.Name != "Grenzen aufheben" {
			continue
		}
		switch {
		case !s.Passed && s.Status != NotChecked:
			return "failed"
		case s.Status == NotChecked:
			status = "not_checked"
		case status == "open":
			status = "passed"
		}
	}
	return status
}

// ReleaseRow is the last row of the web result table; out-of-scope columns stay excluded.
func (r Report) ReleaseRow() MatrixRow {
	row := MatrixRow{Key: "release", Title: "Testgrenzen aufgehoben", LPC: r.ReleaseStatus(), LPP: r.ReleaseStatus()}
	if !r.Config.Includes("LPC") {
		row.LPC = "excluded"
	}
	if !r.Config.Includes("LPP") {
		row.LPP = "excluded"
	}
	return row
}

// ReleaseNote reports the cleanup acknowledgement without classifying it as a tested function.
func (r Report) ReleaseNote() string {
	seen := false
	for _, s := range r.Steps {
		if s.Name != "Grenzen aufheben" {
			continue
		}
		seen = true
		if !s.Passed || s.Status == NotChecked {
			return "Aufheben der Testgrenzen gesendet, keine Bestätigung vom Gerät."
		}
	}
	if seen {
		return "Die Testgrenzen wurden wieder aufgehoben."
	}
	return "Aufheben der Testgrenzen gesendet, keine Bestätigung vom Gerät."
}

// ScopeLabel names the checked use cases (042): LPC (consumption), LPP (production) or both.
// Legacy reports without a recorded scope return "".
func (c Config) ScopeLabel() string {
	switch c.Scope {
	case ScopeConsumption:
		return "LPC"
	case ScopeProduction:
		return "LPP"
	case ScopeBoth:
		return "LPC/LPP"
	}
	return ""
}

// ResultTitle is the result heading in web and saved views (042, Nutzer 2026-11-14):
// "LPC-Funktionsprüfung bestanden", "LPC/LPP-Langzeitprüfung nicht bestanden", "… unvollständig",
// "… abgebrochen" (cancelled by the tester). Legacy reports without scope keep the previous title.
func ResultTitle(c Config, outcome, abortReason string) string {
	scope := c.ScopeLabel()
	if scope == "" {
		if outcome == "Bestanden" {
			return c.ProfileLabel() + " bestanden"
		}
		return outcome
	}
	state := "unvollständig"
	switch {
	case outcome == "Bestanden":
		state = "bestanden"
	case outcome == "Nicht bestanden":
		state = "nicht bestanden"
	case abortReason == "user_cancel":
		state = "abgebrochen"
	}
	return scope + "-" + c.ProfileLabel() + " " + state
}

// ReportTitle is the PDF page title (042): "LPC-Prüfung am Gerät", "LPC/LPP-Prüfung im Simulator".
func (r Report) ReportTitle() string {
	title := "Prüfung am Gerät"
	if r.Mode != ModeOnSite {
		title = "Prüfung im Simulator"
	}
	if scope := r.Config.ScopeLabel(); scope != "" {
		return scope + "-" + title
	}
	return title
}
