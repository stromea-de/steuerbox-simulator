package report

import (
	"fmt"
	"strings"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

const operatorDeclaration = "Ich bestätige den Erhalt dieses Testberichts (Protokoll-ID: %s)."
const inspectorDeclaration = "Ich bestätige die Durchführung der aufgeführten Tests im dokumentierten Prüfaufbau und die Aufzeichnung ihrer Ergebnisse. Die Erklärung bezieht sich auf den angegebenen Testumfang und Testzeitpunkt."

type checkpoint struct {
	text  string
	steps []string
}

func checkpoints(r inspection.Report) []checkpoint {
	texts := []string{
		"Verbindung und gemeldete Funktionen.",
		"Verbrauchsgrenze: Annahme der Vorgabe und zurückgemeldete Leistung (" + watts(r.Config.LPC) + ").",
		"Einspeisegrenze: Annahme der Vorgabe und zurückgemeldete Leistung (" + watts(r.Config.LPP) + ").",
		"Verbindungsausfall: Ersatzgrenze " + watts(r.Config.Failsafe) + " für Bezug und Einspeisung.",
		"Rückkehr und Aufhebung der Testgrenzen.",
	}
	var checks []checkpoint
	for i, steps := range r.RequiredStepGroups() {
		if len(steps) == 0 {
			continue
		}
		if r.Config.Scope == inspection.ScopeConsumption && i == 3 {
			texts[i] = "Verbindungsausfall: Ersatzgrenze " + watts(r.Config.Failsafe) + " für Bezug."
		}
		if r.Config.Scope == inspection.ScopeProduction && i == 3 {
			texts[i] = "Verbindungsausfall: Ersatzgrenze " + watts(r.Config.Failsafe) + " für Einspeisung."
		}
		checks = append(checks, checkpoint{texts[i], steps})
	}
	return checks
}

// Every occurrence of a required step counts. A later success cannot hide a
// failure. Never infer successful checks from a free-form detail or step prefix.
func checkpointStatus(r inspection.Report, c checkpoint) string {
	missing := false
	for _, name := range c.steps {
		found := false
		for _, s := range r.Steps {
			if s.Name != name {
				continue
			}
			found = true
			if s.Status == inspection.NotChecked {
				missing = true
				continue
			}
			if !s.Passed {
				return "nicht bestanden"
			}
		}
		if !found {
			missing = true
		}
	}
	if missing {
		return "nicht geprüft"
	}
	return "bestanden"
}

func outcome(r inspection.Report) (color, label string) {
	label = r.Outcome()
	switch label {
	case "Nicht bestanden":
		return red, label
	case "Unvollständig":
		return muted, label
	default:
		return green, label
	}
}

func (d *document) operator(r inspection.Report) {
	d.formHeader(r)
	d.summary(r)
	d.reserve(45)
	d.heading("2 · Prüfpunkte · ergebnisrelevant")
	d.resultMatrix(r)
	d.reserve(65)
	d.heading("3 · Ablauf · informativ")
	// UX-S16: the release statement stands once, below the matrix; no separate field repeats it.
	d.failsafeRestore(r)

	if r.Outcome() != "Bestanden" {
		d.reserve(60)
		d.heading("4 · Nächste Schritte")
		next := nextActions(r)
		for _, text := range next {
			lines := wrapText("• "+text, bodyWidth, 8.5, "F1")
			d.reserve(float64(len(lines)) * 11)
			d.lines(margin, d.y, 8.5, 11, "F1", ink, lines)
			d.y += float64(len(lines)) * 11
		}
		d.y += 12
	}
	if strings.TrimSpace(r.Config.Site.Note) != "" {
		d.reserve(120)
		d.gridRow([]field{{"Notiz", r.Config.Site.Note, false}})
		d.y += 18
	}
	d.signatures(r)
}

// gridRow wraps each value in a bordered field. Extremely long values flow in
// full-width framed rows, retaining all assignment data across page breaks.
func (d *document) gridRow(fields []field) {
	if len(fields) == 0 {
		return
	}
	width := bodyWidth / float64(len(fields))
	height := 0.0
	for _, f := range fields {
		height = max(height, 22+float64(len(wrapText(f.value, width-14, 8.5, "F1")))*11)
	}
	if height > 180 {
		for _, f := range fields {
			lines := wrapText(f.value, bodyWidth-14, 8.5, "F1")
			for len(lines) > 0 {
				n := min(len(lines), 12)
				d.reserve(22 + float64(n)*11)
				d.frame(margin, d.y, bodyWidth, 22+float64(n)*11, rule)
				d.text(margin+7, d.y+4, 6.8, "F1", muted, f.key)
				d.lines(margin+7, d.y+17, 8.5, 11, "F1", ink, lines[:n])
				d.y += 22 + float64(n)*11
				lines = lines[n:]
			}
		}
		return
	}
	d.reserve(height)
	for i, f := range fields {
		x := margin + float64(i)*width
		d.frame(x, d.y, width, height, rule)
		d.text(x+7, d.y+4, 6.8, "F1", muted, f.key)
		d.lines(x+7, d.y+17, 8.5, 11, "F1", ink, wrapText(f.value, width-14, 8.5, "F1"))
	}
	d.y += height
}
func (d *document) formHeader(r inspection.Report) {
	version := "Version 1"
	if r.Revision != nil {
		version = fmt.Sprintf("Version %d – ersetzt Version %d vom %s · vorherige Protokoll-Nr. %s", r.VersionNumber(), r.VersionNumber()-1, r.Revision.PreviousDate.Format("02.01.2006 15:04 -07:00"), r.Revision.PreviousID)
	}
	d.gridRow([]field{{"Berichtsversion", version, false}})
	s, p := r.Config.Site, r.Config.Profile
	model := r.Device.ModelLabel()
	if model == "" {
		model = r.Device.TypeLabel()
	}
	device := strings.TrimSpace(r.Device.Brand + " " + model)
	if strings.HasPrefix(strings.ToLower(model), strings.ToLower(r.Device.Brand)+" ") {
		device = model
	}
	d.gridRow([]field{{"Datum", r.Started.Format("02.01.2006"), false}, {"Uhrzeit", r.Started.Format("15:04") + " – " + r.Finished.Format("15:04 -07:00"), false}, {"Prüfart", r.Config.ProfileLabel(), false}})
	blocks := []field{}
	if value := strings.TrimSpace(strings.Join([]string{s.Customer, s.CustomerAddress}, "\n")); value != "" {
		blocks = append(blocks, field{"Anlagenbetreiber", value, false})
	}
	plant := []string{}
	if s.Address != "" && s.Address != s.CustomerAddress {
		plant = append(plant, s.Address)
	}
	if s.Meter != "" {
		plant = append(plant, "Zähler- oder Anlagennummer: "+s.Meter)
	}
	if len(plant) > 0 {
		blocks = append(blocks, field{"Anlage", strings.Join(plant, "\n"), false})
	}
	country := strings.TrimSpace(p.Country)
	if strings.EqualFold(country, "Deutschland") {
		country = "" // 041: the German report names the country only abroad
	}
	address := strings.TrimSpace(strings.Join([]string{p.Street, strings.TrimSpace(p.PostalCode + " " + p.City), country}, "\n"))
	if value := strings.TrimSpace(strings.Join([]string{r.Business, r.Inspector, address}, "\n")); value != "" {
		blocks = append(blocks, field{"Prüfer", value, false})
	}
	d.gridRow(blocks)
	d.gridRow([]field{{"Gerät", device, false}})
	d.y += 12
}

func (d *document) signatures(r inspection.Report) {
	type block struct{ title, declaration, name string }
	var blocks []block
	if r.Config.SignOperator {
		blocks = append(blocks, block{"Empfangsbestätigung", fmt.Sprintf(operatorDeclaration, d.id), ""})
	}
	if r.Config.SignInspector {
		blocks = append(blocks, block{"Bestätigung der Fachkraft", inspectorDeclaration, r.Inspector})
	}
	if len(blocks) == 0 {
		return
	}
	width := (bodyWidth - 24) / 2
	height := 0.0
	for _, item := range blocks {
		height = max(height, 38+float64(len(wrapText(item.declaration, width, 8, "F1")))*10+86)
	}
	// 038: same section design as "Dokumentierter Prüfumfang".
	d.reserve(height + 26)
	d.heading("Unterschriften")
	start := d.y
	for i, item := range blocks {
		x := margin + float64(i)*(width+24)
		title := wrapText(item.title, width, 8.5, "F2")
		d.lines(x, start, 8.5, 10.5, "F2", ink, title)
		lines := wrapText(item.declaration, width, 8, "F1")
		d.lines(x, start+38, 8, 10, "F1", ink, lines)
		if bottom := start + 38 + float64(len(lines))*10 + 86; bottom > d.y {
			d.y = bottom
		}
	}
	for i, item := range blocks {
		x := margin + float64(i)*(width+24)
		d.line(x, d.y-64, x+width, d.y-64, 0.6, rule)
		d.text(x, d.y-78, 8, "F1", ink, item.name)
		d.text(x, d.y-61, 7.5, "F1", muted, "Name")
	}
	for i := range blocks {
		x := margin + float64(i)*(width+24)
		d.line(x, d.y-40, x+width, d.y-40, 0.6, muted)
		d.text(x, d.y-36, 7.5, "F1", muted, "Unterschrift")
		d.line(x, d.y-12, x+width, d.y-12, 0.6, rule)
		d.text(x, d.y-8, 7.5, "F1", muted, "Ort, Datum")
	}
}

var terms = []struct{ title, explanation string }{
	{"LPC · Limitation of Power Consumption", "Vorgabe einer maximalen Bezugsleistung. Das Gerät soll den Steuerbefehl bestätigen und seinen Strombezug auf den Sollwert begrenzen."},
	{"LPP · Limitation of Power Production", "Vorgabe einer maximalen Einspeiseleistung für Erzeuger und Speicher. Geprüft werden Annahme und gemeldete Leistungsanpassung."},
	{"MPC · Monitoring of Power Consumption", "Leistungsmesskanal über EEBUS SPINE. Übermittelt Wirkleistungswerte des Geräts. Positive Werte stehen für Bezug, negative für Einspeisung. Keine unabhängige Messung."},
	{"Heartbeat · Lebenszeichen", "Regelmäßiges Lebenszeichen zwischen Steuerbox und Kundenanlage. Bleibt es aus, erkennt das Gerät den Signalausfall und wechselt in den Notbetrieb."},
	{"Failsafe · Notbetrieb", "Bei ausbleibenden Lebenszeichen begrenzt das Gerät seine Leistung selbstständig auf den eingestellten Notbetriebswert."},
	{"Mindestdauer im Notbetrieb", "Laut EEBUS LPC: 2–24 Std. Die Funktionsprüfung prüft den Eintritt und versucht, den Notbetrieb auf Befehl der Steuerbox vorzeitig zu beenden. Nur die Langzeitprüfung beobachtet den Ablauf der eingestellten Dauer."},
	{"SKI · Subject Key Identifier", "Kennung des elektronischen Gerätezertifikats (40 Hexadezimalzeichen). Die Partner vergleichen diese Kennung vor der Freigabe der Verbindung."},
	{"SPINE & SHIP", "SHIP (Smart Home IP) schützt den Transport im lokalen Netzwerk mit TLS. SPINE beschreibt die herstellerübergreifenden Befehle, Messwerte und Gerätedaten von EEBUS."},
}

func (d *document) appendixIntro(r inspection.Report) {
	texts := []string{"Schulungsübung · Messwerte simuliert; kein reales Gerät.", "Prüfaufbau: Steuerbox und Gegenstelle auf diesem Computer simuliert."}
	if r.Mode == inspection.ModeOnSite {
		texts = []string{"Gerät vor Ort · Messwerte über EEBUS empfangen; keine unabhängige Leistungsmessung.", "Prüfaufbau: Software-Simulation der Steuerbox über lokale IP-Netzwerkverbindung."}
	}
	for _, s := range texts {
		d.text(margin, d.y, 8.5, "F1", muted, s)
		d.y += 13
	}
	d.y += 8
}

func (d *document) glossary(r inspection.Report) {
	d.newPage()
	d.y += 12
	width := (bodyWidth - 36) / 2
	for i := 0; i < len(terms); i += 2 {
		height := 0.0
		for j := i; j < min(i+2, len(terms)); j++ {
			x := margin + 9 + float64(j-i)*(width+18)
			titles := wrapText(terms[j].title, width, 10, "F2")
			lines := wrapText(terms[j].explanation, width, 9, "F1")
			d.lines(x, d.y, 10, 13, "F2", ink, titles)
			d.lines(x, d.y+float64(len(titles))*13+7, 9, 13, "F1", muted, lines)
			height = max(height, float64(len(titles))*13+7+float64(len(lines))*13+25)
		}
		d.frame(margin, d.y-8, bodyWidth, height, rule)
		d.y += height + 8
	}
}

func (d *document) coverage(r inspection.Report) {
	var missing []string
	for _, c := range checkpoints(r) {
		for _, name := range c.steps {
			found := false
			for _, s := range r.Steps {
				if s.Name == name {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, inspection.StepName(name))
			}
		}
	}
	text := "Alle vorgesehenen Prüfschritte sind aufgezeichnet. Ergebnisse und Abweichungen: siehe Schritttabelle."
	if len(missing) > 0 {
		text = "Nicht geprüft / nicht aufgezeichnet: " + strings.Join(missing, "; ") + "."
	}
	lines := wrapText(text, bodyWidth, 8.5, "F1")
	// 038/038c: the notice (ADR 012) stays as a tiny plain line of this section,
	// footer size (6.5 pt), no box.
	notice := wrapText(strings.Join(noticeLines(r), " "), bodyWidth, 6.5, "F1")
	h := 23 + float64(len(lines))*11 + float64(len(notice))*8.5 + 14
	d.reserve(h)
	d.heading("Dokumentierter Prüfumfang")
	d.lines(margin, d.y, 8.5, 11, "F1", muted, lines)
	d.lines(margin, d.y+float64(len(lines))*11+6, 6.5, 8.5, "F1", muted, notice)
	d.y += h - 23
}

func noticeLines(r inspection.Report) []string {
	text := inspection.Notice
	custom := strings.TrimSpace(r.Notice) != "" && r.Notice != text
	for _, old := range inspection.LegacyNotices {
		custom = custom && r.Notice != old
	}
	if custom {
		text += " " + r.Notice
	}
	return wrapText(text, bodyWidth-24, 9, "F1")
}

// Optional assignment data is printed fully; long notes flow across pages.
func (d *document) site(r inspection.Report) {
	s := r.Config.Site
	if s.Customer == "" && s.Address == "" && s.Meter == "" && s.Note == "" {
		return
	}
	d.reserve(48)
	d.heading("Kunde / Anlage")
	for _, item := range []struct{ label, value string }{{"Kunde", s.Customer}, {"Anlagenadresse", s.Address}, {"Zählernummer / Anlagenkennung", s.Meter}, {"Notiz", s.Note}} {
		if item.value == "" {
			continue
		}
		for _, line := range wrapText(item.label+": "+item.value, bodyWidth, 9, "F1") {
			d.reserve(12)
			d.text(margin, d.y, 9, "F1", ink, line)
			d.y += 12
		}
		d.y += 6
	}
	d.y += 6
}

// 042 (Nutzer 2026-11-14): fallback values in plain words. EEBUS LPC/LPP call them failsafe power
// limit and failsafe duration minimum; page 1 avoids the technical terms (see glossary).
func failsafeValues(r inspection.Report) string {
	var parts []string
	for _, kind := range []string{"LPC", "LPP"} {
		v, ok := r.FailsafeOriginals[kind]
		if !ok {
			continue
		}
		direction := "Bezug"
		if kind == "LPP" {
			direction = "Einspeisung"
		}
		part := direction + " " + watts(v.Power)
		if v.Minimum > 0 {
			part += ", Mindestdauer " + humanDuration(v.Minimum)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

func failsafeExplanation(r inspection.Report) string {
	verb := "beziehen oder einspeisen"
	switch r.Config.Scope {
	case inspection.ScopeConsumption:
		verb = "beziehen"
	case inspection.ScopeProduction:
		verb = "einspeisen"
	}
	return "Notbetriebswerte legen fest, welche Leistung das Gerät bei Kommunikationsausfall höchstens " + verb + " darf und wie lange mindestens; die Prüfung ändert sie vorübergehend."
}

// FailsafeRestoreText is the value of the PDF field "Notbetriebswerte des Geräts".
func FailsafeRestoreText(r inspection.Report) string {
	values := failsafeValues(r)
	read := "Vor der Prüfung gelesen"
	if values != "" {
		read += " (" + values + ")"
	}
	switch r.FailsafeRestore() {
	case "simulated":
		return "Übung: nur simuliert, an keinem Gerät verändert."
	case "unchanged":
		return "Nicht verändert: Die Prüfung hat keine Notbetriebswerte am Gerät geschrieben."
	case "confirmed":
		return read + ", danach wiederhergestellt und vom Gerät bestätigt. Bei Kommunikationsausfall verhält sich das Gerät wieder wie vor der Prüfung."
	}
	if values == "" {
		return "Wiederherstellung nicht bestätigt."
	}
	return read + ". Wiederherstellung nicht bestätigt."
}

// FailsafeRestoreWarning is the action for an unconfirmed restore on a real device ("" otherwise).
func FailsafeRestoreWarning(r inspection.Report) string {
	if r.FailsafeRestore() != "unconfirmed" {
		return ""
	}
	target := "wieder wie vor der Prüfung"
	if values := failsafeValues(r); values != "" {
		target = "wieder auf " + values
	}
	return "Achtung: Notbetriebswerte am Gerät prüfen und " + target + " einstellen. Sonst gilt bei Kommunikationsausfall womöglich noch der Prüfwert " + watts(r.Config.Failsafe) + "."
}

// OriginalsRestoredText is the PDF field "Originalwerte wiederhergestellt" (236).
func OriginalsRestoredText(r inspection.Report) string {
	switch r.OriginalsRestored() {
	case "yes":
		return "Ja, vom Gerät zurückgelesen"
	case "unchanged":
		return "Nicht nötig, am Gerät nichts verändert"
	case "simulated":
		return "Übung, kein Gerät"
	case "unverified":
		return "Nur quittiert, nicht zurückgelesen"
	}
	text := "Nein"
	if r.RestorePending {
		text += ". Originalwerte gespeichert; der Simulator versucht es vor der nächsten Prüfung an diesem Gerät erneut"
	}
	return text
}

// ActiveLimitText is the PDF field "Aktive Begrenzung nach der Prüfung" (236).
func ActiveLimitText(r inspection.Report) string {
	switch {
	case r.ReleaseVerified != nil && *r.ReleaseVerified:
		return "Keine, vom Gerät zurückgelesen"
	case r.ReleaseVerified != nil:
		return "Nicht ausgeschlossen, am Gerät prüfen"
	case r.ReleaseStatus() == "passed":
		return "Aufhebung vom Gerät quittiert, nicht zurückgelesen"
	}
	return "Nicht bestätigt, am Gerät prüfen"
}

func (d *document) failsafeRestore(r inspection.Report) {
	if r.Mode == inspection.ModeOnSite && r.FailsafeRestore() != "unchanged" {
		d.gridRow([]field{{"Originalwerte wiederhergestellt", OriginalsRestoredText(r), false}, {"Aktive Begrenzung nach der Prüfung", ActiveLimitText(r), false}})
		d.y += 4
	}
	d.gridRow([]field{{"Notbetriebswerte des Geräts", FailsafeRestoreText(r), false}})
	d.y += 4
	lines := wrapText(failsafeExplanation(r), bodyWidth, 7.5, "F1")
	d.reserve(float64(len(lines)) * 10)
	d.lines(margin, d.y, 7.5, 10, "F1", muted, lines)
	d.y += float64(len(lines))*10 + 4
	if warning := FailsafeRestoreWarning(r); warning != "" {
		lines = wrapText(warning, bodyWidth, 9, "F2")
		d.reserve(float64(len(lines))*12 + 4)
		d.lines(margin, d.y+2, 9, 12, "F2", red, lines)
		d.y += float64(len(lines))*12 + 4
	}
	if r.Mode == inspection.ModeOnSite {
		// 236: quiet line, same as the web result.
		lines = wrapText("Kopplung am Gerät entfernen, wenn sie nicht mehr gebraucht wird.", bodyWidth, 7.5, "F1")
		d.reserve(float64(len(lines)) * 10)
		d.lines(margin, d.y, 7.5, 10, "F1", muted, lines)
		d.y += float64(len(lines))*10 + 4
	}
	d.y += 6
}
