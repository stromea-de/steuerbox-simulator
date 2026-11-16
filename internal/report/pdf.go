// Package report renders deterministic A4 protocols with PDF standard fonts.
package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"image"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// Version is settable at build time with -X github.com/stromea-de/steuerbox-simulator/internal/report.Version=...
// Untagged builds are development builds; release scripts inject the exact tag.
var Version = ""
var Commit = "dev"

func BuildVersion() string {
	tag := strings.TrimPrefix(Version, "v")
	dirty := strings.Contains(tag, "-dirty") || strings.Contains(Commit, "-dirty")
	tag = strings.TrimSuffix(tag, "-dirty")
	commit := strings.TrimSuffix(Commit, "-dirty")
	label := "Testversion"
	if regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:[-+].*)?$`).MatchString(tag) {
		label = "Version " + tag
	}
	if commit != "" && commit != "dev" {
		if dirty {
			commit += "-dirty"
		}
		return label + " (Softwarestand " + commit + ")"
	}
	if dirty {
		label += " (dirty)"
	}
	return label
}

const (
	pageWidth  = 595.28
	pageHeight = 841.89
	margin     = 42.0
	bodyWidth  = pageWidth - 2*margin
	bodyTop    = 103.0
	bodyBottom = 778.0
	ink        = "#222222"
	green      = "#245C3E"
	lime       = "#7CB342"
	red        = "#C62828"
	muted      = "#555555"
	rule       = "#BBBBBB"
)

type document struct {
	logo          image.Image
	page          *strings.Builder
	pages         []*strings.Builder
	y             float64
	err           error
	mode          string
	title         string
	profile       string
	id            string
	operatorPages int
	digest        string
	attachment    []byte
	metadata      string
	software      string
	stepPages     []int // page index of each rendered step row (inspectable layout)
}

// PDF preserves the JSON report as the source of the protocol identity. It never
// reads the clock, local timezone, random data or the network.
func PDF(r inspection.Report) ([]byte, error) {
	d, err := layout(r)
	if err != nil {
		return nil, err
	}
	return d.bytes(), nil
}

// layout renders all pages without serialising them.
func layout(r inspection.Report) (*document, error) {
	data, err := JSON(r)
	if err != nil {
		return nil, fmt.Errorf("PDF-Bericht: %w", err)
	}
	hash := sha256.Sum256(data)
	id := ProtocolIdentity(r)
	d := &document{mode: r.Mode, title: r.ReportTitle(), profile: r.Config.ProfileLabel(), id: id, digest: fmt.Sprintf("%x", hash), attachment: data, metadata: xmpMetadata(r, id), software: r.Software}
	if r.Config.Profile.Logo != "" {
		d.logo, err = r.Config.Profile.Image()
		if err != nil {
			return nil, err
		}
	}
	r = renderReport(r)
	d.newPage()
	d.operator(r)
	d.operatorPages = len(d.pages)
	d.newPage()
	d.appendixIntro(r)
	d.identification(r)
	d.evidenceGroups(r)
	d.conditions(r)
	d.powerCurve(r)
	d.steps(r)
	if len(r.Events) > 0 {
		d.reserve(50)
		d.heading("Verbindungsereignisse")
		for _, event := range r.Events {
			// 038: like the detail text of the step table (8 pt, muted).
			for _, line := range wrapText(event.At.Format("02.01.2006 15:04:05 -07:00")+" · "+event.Name+": "+readableDetail(inspection.UserDetail(event.Detail)), bodyWidth, 8, "F1") {
				d.reserve(10)
				d.text(margin, d.y, 8, "F1", muted, line)
				d.y += 10
			}
			d.y += 5
		}
	}
	d.coverage(r)
	d.glossary(r)
	if d.err != nil {
		return nil, d.err
	}
	for i := range d.pages {
		d.furniture(i, id)
	}
	return d, nil
}

func (d *document) current() *strings.Builder { return d.page }
func (d *document) newPage() {
	d.page = &strings.Builder{}
	d.pages = append(d.pages, d.page)
	d.y = bodyTop
}
func (d *document) reserve(height float64) {
	if height > bodyBottom-bodyTop {
		d.err = fmt.Errorf("PDF: unteilbarer Abschnitt zu hoch (%.0f pt, maximal %.0f pt)", height, bodyBottom-bodyTop)
		return
	}
	if d.y+height > bodyBottom {
		d.newPage()
	}
}
func rgb(hex string) string {
	v, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 24)
	return fmt.Sprintf("%.4f %.4f %.4f", float64(v>>16)/255, float64(v>>8&255)/255, float64(v&255)/255)
}
func (d *document) frame(x, y, w, h float64, color string) {
	fmt.Fprintf(d.current(), "q %s RG 0.60 w %.2f %.2f %.2f %.2f re S Q\n", rgb(color), x, pageHeight-y-h, w, h)
}
func (d *document) fill(x, y, w, h float64, color string) {
	fmt.Fprintf(d.current(), "q %s rg %.2f %.2f %.2f %.2f re f Q\n", rgb(color), x, pageHeight-y-h, w, h)
}

// Larger checks deliberately extend beyond the 12pt form box.
func (d *document) formMark(x, y float64, label, color string) {
	if label == "bestanden" {
		d.line(x-1, y+6, x+4, y+12, 2.2, color)
		d.line(x+4, y+12, x+15, y-3, 2.2, color)
	} else {
		d.line(x+2, y+2, x+10, y+10, 1.6, color)
		d.line(x+2, y+10, x+10, y+2, 1.6, color)
	}
}
func (d *document) line(x1, y1, x2, y2, width float64, color string) {
	fmt.Fprintf(d.current(), "q %s RG %.2f w %.2f %.2f m %.2f %.2f l S Q\n", rgb(color), width, x1, pageHeight-y1, x2, pageHeight-y2)
}
func (d *document) text(x, y, size float64, font, color, s string) {
	fmt.Fprintf(d.current(), "BT /%s %.2f Tf %s rg 1 0 0 1 %.2f %.2f Tm (%s) Tj ET\n", font, size, rgb(color), x, pageHeight-y-size, literal(s))
}
func (d *document) lines(x, y, size, leading float64, font, color string, lines []string) {
	for i, s := range lines {
		d.text(x, y+float64(i)*leading, size, font, color, s)
	}
}
func (d *document) heading(title string) {
	d.fill(margin, d.y, bodyWidth, 22, "#F4F4F6")
	d.fill(margin, d.y, 3, 22, ink)
	d.text(margin+9, d.y+4, 10, "F2", ink, title)
	d.y += 26
}
func (d *document) furniture(page int, id string) {
	// Append to the selected page without changing the layout cursor.
	saved := d.page
	d.page = d.pages[page]
	d.fill(0, 0, pageWidth, 6, green)
	d.text(margin, 23, 7.5, "F1", muted, "Stromea · Prüfbericht")
	title := d.title
	subtitle := "Ergebnisübersicht für den Anlagenbetreiber"
	if page >= d.operatorPages && d.operatorPages > 0 {
		title = "Technischer Anhang"
		subtitle = "Prüfaufbau und Einzelnachweise"
	}
	if page == len(d.pages)-1 {
		title = "Glossar"
		subtitle = "Begriffe zur Prüfung"
	}
	d.text(margin, 42, 16, "F2", ink, title)
	d.text(margin, 65, 8, "F1", muted, subtitle)
	mode := "Übungsmodus – keine Prüfung am Gerät"
	if d.mode == inspection.ModeOnSite {
		mode = "Gerät vor Ort"
	}
	if d.profile != "" {
		mode += " · " + d.profile
	}
	d.text(margin, 79, 8, "F1", ink, mode)
	const nrX = 365.0
	d.frame(nrX, 40, pageWidth-margin-nrX, 33, ink)
	d.text(nrX+9, 44, 7, "F1", muted, "Protokoll-Nr.")
	d.text(nrX+9, 56, 9, "F3", ink, id)
	if d.logo != nil && page == 0 {
		bounds := d.logo.Bounds()
		scale := min(54/float64(bounds.Dx()), 28/float64(bounds.Dy()))
		w, h := float64(bounds.Dx())*scale, float64(bounds.Dy())*scale
		fmt.Fprintf(d.current(), "q %.2f 0 0 %.2f %.2f %.2f cm /Logo Do Q\n", w, h, nrX-64, pageHeight-43-h)
	}
	d.line(margin, 94, pageWidth-margin, 94, 1.2, ink)
	d.line(margin, 789, pageWidth-margin, 789, 0.6, rule)
	// Original Stromea mark (Logo 07): S and sun dot, monochrome.
	// SVG coordinates use downward y; the local PDF transform preserves them.
	fmt.Fprintf(d.current(), "q 0.035 0 0 -0.035 %.2f %.2f cm 0 G 0 g 42 w 1 J 1 j\n", margin-392*0.035, pageHeight-799+248*0.035)
	d.current().WriteString("587 364 m 587 324 437 324 437 379 c 437 439 587 459 587 519 c 587 574 437 574 437 534 c S\n")
	// Circle at (512,284), radius 24; cubic approximation, same black ink.
	d.current().WriteString("536 284 m 536 297.255 525.255 308 512 308 c 498.745 308 488 297.255 488 284 c 488 270.745 498.745 260 512 260 c 525.255 260 536 270.745 536 284 c f Q\n")
	// 038: more air around the checksum line.
	d.text(margin+15, 795, 7.5, "F1", muted, "Erstellt mit dem Stromea Steuerbox-Simulator · stromea.de/simulator")
	d.text(margin+15, 808, 6.5, "F3", muted, "Prüfsumme der Protokolldaten (SHA-256) "+d.digest)
	d.text(margin, 822, 7, "F1", muted, fmt.Sprintf("Seite %d von %d", page+1, len(d.pages)))
	d.text(135, 822, 7, "F3", muted, "Protokoll-Nr. "+id)
	version := d.software
	if version == "" {
		version = BuildVersion()
	}
	software := "Software " + version
	versionSize := math.Min(7, (pageWidth-margin-320)/textWidth(software, 1, "F1"))
	d.text(320, 822, versionSize, "F1", muted, software)
	d.page = saved
}
func (d *document) summary(r inspection.Report) {
	_, label := outcome(r)
	d.reserve(100)
	d.heading("1 · Ergebnis")
	const gap = 8.0
	w := (bodyWidth - 2*gap) / 3
	for i, title := range []string{"Bestanden", "Nicht bestanden", "Unvollständig"} {
		x := margin + float64(i)*(w+gap)
		color := rule
		if label == title {
			color = ink
			if title == "Nicht bestanden" {
				color = red
			}
		}
		d.frame(x, d.y, w, 35, color)
		if label == title {
			d.line(x, d.y, x+w, d.y, 1.5, color)
			d.line(x, d.y+35, x+w, d.y+35, 1.5, color)
		}
		d.frame(x+10, d.y+10, 14, 14, color)
		if label == title {
			d.formMark(x+11, d.y+10, "bestanden", ink)
		}
		d.text(x+32, d.y+11, 10, "F2", ink, title)
	}
	d.y += 44
	if reason := displayAbort(r); reason != "" {
		for _, line := range wrapText("Grund: "+reason, bodyWidth, 9, "F1") {
			d.text(margin+3, d.y, 9, "F1", ink, line)
			d.y += 12
		}
	}
	d.y += 9
}

type field struct {
	key, value string
	mono       bool
}

func groupSKI(s string) string {
	// Only group compact SKIs. Preserve already separated or unusual identifiers.
	compact := strings.TrimSpace(s)
	if strings.ContainsAny(compact, " \t\n:-") {
		return compact
	}
	rs := []rune(compact)
	var groups []string
	for len(rs) > 4 {
		groups = append(groups, string(rs[:4]))
		rs = rs[4:]
	}
	groups = append(groups, string(rs))
	return strings.Join(groups, " ")
}
func fieldLines(f field, w float64) []string {
	font := "F1"
	if f.mono {
		font = "F3"
	}
	var out []string
	for _, line := range strings.Split(f.value, "\n") {
		out = append(out, wrapText(line, w, 9, font)...)
	}
	return out
}

// Labels wrap at semantic boundaries; never split Gerätekennung/Seriennummer.
func identificationLabelLines(key string) []string {
	if key == "Simulator-Kennung (SKI)" {
		return []string{"Simulator-", "Kennung (SKI)"}
	}
	return wrapText(key, 66, 7, "F1")
}
func (d *document) identification(r inspection.Report) {
	left := []field{{"Firma", r.Business, false}, {"Prüfer/in", r.Inspector, false}}
	// 038: Beginn, Ende, Simulator-Kennung and functions form their own section.
	test := []field{{"Beginn", r.Started.Format("02.01.2006 15:04:05 -07:00"), false}, {"Ende", r.Finished.Format("02.01.2006 15:04:05 -07:00"), false}}
	right := []field{{"Name", r.Device.Name, false}, {"Gerätetyp", r.Device.TypeLabel(), false}, {"Hersteller", r.Device.Brand, false}, {"Modell", fallback(r.Device.ModelLabel(), "nicht gemeldet"), false}, {"Gerätekennung (SKI)", groupSKI(r.Device.SKI), true}}
	if r.Mode == inspection.ModeOnSite {
		test = append(test, field{"Simulator-Kennung (SKI)", groupSKI(r.LocalSKI), true})
		right = append(right, field{"Seriennummer", r.Device.Serial, false}, field{"Firmware", fallback(r.Device.Firmware, "vom Gerät nicht gemeldet"), false}, field{"Gerätedaten", "vom Gerät gemeldet", false})
	}
	if r.Capabilities != nil || r.Mode == inspection.ModeOnSite {
		test = append(test, field{"Funktionen", functionSummary(r), false})
	}
	const gap = 18.0
	width := (bodyWidth - gap) / 2
	valueWidth := width - 86
	height := func(fields []field) float64 {
		h := 0.0
		for _, f := range fields {
			h += math.Max(22, math.Max(float64(len(fieldLines(f, valueWidth)))*12+9, float64(len(identificationLabelLines(f.key)))*10+9))
		}
		return h
	}
	h := math.Max(height(left), height(right))
	d.reserve(h + 29)
	for i, fields := range [][]field{left, right} {
		x := margin + float64(i)*(width+gap)
		title := "Firma & Prüfer"
		if i == 1 {
			title = "Gerät"
		}
		d.text(x, d.y, 11, "F2", ink, title)
		y := d.y + 23
		d.line(x, y, x+width, y, 0.6, rule)
		for _, f := range fields {
			ls := fieldLines(f, valueWidth)
			keyLines := identificationLabelLines(f.key)
			rowH := math.Max(22, math.Max(float64(len(ls))*12+9, float64(len(keyLines))*10+9))
			font := "F1"
			if f.mono {
				font = "F3"
			}
			d.lines(x+10, y+5, 7, 10, "F1", muted, keyLines)
			d.lines(x+78, y+5, 9, 12, font, ink, ls)
			y += rowH
		}
	}
	d.y += h + 29
	// Full-width section "Prüfung" below the two columns.
	wide := bodyWidth - 86
	th := 0.0
	for _, f := range test {
		th += math.Max(22, math.Max(float64(len(fieldLines(f, wide)))*12+9, float64(len(identificationLabelLines(f.key)))*10+9))
	}
	d.reserve(th + 29)
	d.text(margin, d.y, 11, "F2", ink, "Prüfung")
	y := d.y + 23
	d.line(margin, y, margin+bodyWidth, y, 0.6, rule)
	for _, f := range test {
		ls := fieldLines(f, wide)
		keyLines := identificationLabelLines(f.key)
		rowH := math.Max(22, math.Max(float64(len(ls))*12+9, float64(len(keyLines))*10+9))
		font := "F1"
		if f.mono {
			font = "F3"
		}
		d.lines(margin+10, y+5, 7, 10, "F1", muted, keyLines)
		d.lines(margin+78, y+5, 9, 12, font, ink, ls)
		y += rowH
	}
	d.y += th + 29
}
func (d *document) conditions(r inspection.Report) {
	d.reserve(230)
	d.heading("B1 · Vorgaben · Prüfbedingungen")
	source := "Leistungswerte vom Gerät über EEBUS"
	if r.Mode != inspection.ModeOnSite {
		source = "Übung mit simulierten Messwerten"
	}
	coverage := r.ProfileSummary()
	for _, f := range []field{{"Prüfart", r.Config.ProfileLabel(), false}, {"Messwerte", source, false}, {"Prüfumfang", coverage, false}} {
		d.gridRow([]field{f})
	}
	params := []field{}
	if r.Config.Includes("LPC") {
		params = append(params, field{"Bezug (LPC) · Grenze", watts(r.Config.LPC), false})
	}
	if r.Config.Includes("LPP") {
		params = append(params, field{"Einspeisung (LPP) · Grenze", watts(r.Config.LPP), false})
	}
	params = append(params, field{"Notbetrieb (Failsafe)", watts(r.Config.Failsafe), false}, field{"Toleranz", watts(r.Config.Tolerance), false})
	d.gridRow(params)
	if r.Mode == inspection.ModeOnSite {
		d.gridRow([]field{{"Je Schritt", humanDuration(r.Config.StepTimeout), false}, {"Wirkung messen", humanDuration(r.Config.EffectTimeout), false}, {"Ausfall erkennen", humanDuration(r.Config.FailsafeTimeout), false}, {"Mindestdauer", humanDuration(r.Config.FailsafeMinimum), false}})
	}
	for _, h := range r.Heartbeats {
		if r.Config.Includes(h.Direction) {
			d.gridRow([]field{{h.Direction + " · Lebenszeichen", "Lebenszeichen gestoppt " + h.Reference.Format("15:04:05") + " · Beobachtung bis " + h.Deadline.Format("15:04:05") + " · Reserve " + humanDuration(h.Reserve), false}})
		}
	}
	d.y += 15
}

// Table geometry: the first header includes the section gap (15), the heading
// (26) and the column header row (25); a continuation header has no gap.
const (
	tableFirstHeader = 15 + 26 + 25
	tableContHeader  = 26 + 25
	tableNote        = 25
)

// paginateRows lays out table rows of the given heights, starting with the
// cursor at y. It reports whether the table must start on a fresh page and,
// per row, whether a page break with repeated header precedes it.
// QA-17: the header is only set where at least two rows (or the whole shorter
// table) fit below it. QA-18: a final row never stands alone on a page; the
// last two rows then break together. Pure and deterministic.
func paginateRows(y float64, rows []float64) (fresh bool, breaks []bool) {
	breaks = make([]bool, len(rows))
	lead := float64(tableFirstHeader)
	for i := 0; i < len(rows) && i < 2; i++ {
		lead += rows[i]
	}
	if y+lead > bodyBottom {
		fresh = true
		y = bodyTop
	}
	y += tableFirstHeader
	start, prevStart := 0, 0
	for i, h := range rows {
		if i > start && y+h > bodyBottom {
			breaks[i] = true
			prevStart, start = start, i
			y = bodyTop + tableContHeader
		}
		y += h
	}
	if n := len(rows); n >= 3 && start == n-1 && n-2-prevStart >= 2 &&
		bodyTop+tableContHeader+rows[n-2]+rows[n-1] <= bodyBottom {
		breaks[n-1], breaks[n-2] = false, true
	}
	return fresh, breaks
}

var columnWidths = []float64{27, 207, 57, 71, 71, bodyWidth - 433}

func (d *document) tableHeader(continuation bool) {
	title := "Prüfschritte"
	if continuation {
		title += " · Fortsetzung"
	}
	d.heading(title)
	d.line(margin, d.y, pageWidth-margin, d.y, 0.6, rule)
	x := margin
	for i, label := range []string{"Nr.", "Prüfschritt", "Zeit¹", "Soll²", "Messwert", "Ergebnis"} {
		d.text(x+7, d.y+8, 8, "F2", ink, label)
		x += columnWidths[i]
	}
	d.y += 25
}

// The existing report has no structured per-step target. Derive targets only for
// the exact inspection.Run step names, never from free-form details or substrings.
func target(s inspection.Step, c inspection.Config, mode string) string {
	switch s.Name {
	case "Ausgangsmessung MPC":
		return "> " + watts(c.LPC+c.Tolerance)
	case "LPP Ausgangsmessung MPC":
		return "> " + watts(c.LPP+c.Tolerance) + " Einspeisung"
	case "LPC Grenze + Quittung", "LPC gemessene Wirkung", "LPC Wiederkehr":
		return watts(c.LPC)
	case "LPP Grenze + Quittung":
		return watts(c.LPP)
	case "LPP gemessene Wirkung", "LPP Wiederkehr":
		return watts(c.LPP) + " Einspeisung"
	case "LPC Failsafe gemessene Wirkung":
		return watts(c.Failsafe)
	case "LPP Failsafe gemessene Wirkung":
		return watts(c.Failsafe) + " Einspeisung"
	case "Grenzen aufheben":
		if mode == inspection.ModeOnSite {
			return "Freigabe quittieren"
		}
		if c.Scope == inspection.ScopeConsumption {
			return "> " + watts(c.LPC+c.Tolerance) + " Bezug"
		}
		return "> " + watts(c.LPP+c.Tolerance) + " Einspeisung"
	default:
		return "–"
	}
}
func (d *document) steps(r inspection.Report) {
	r = renderReport(r)
	if len(r.Steps) == 0 {
		d.reserve(65)
		d.tableHeader(false)
		d.text(margin+7, d.y+10, 9, "F1", muted, "Keine Prüfschritte aufgezeichnet.")
		d.y += 32
		return
	}

	type stepRow struct {
		nr                                 int
		s                                  inspection.Step
		names, details, expected, measured []string
		h                                  float64
	}
	var rows []stepRow
	for i, s := range r.Steps {
		if (strings.HasPrefix(s.Name, "LPC ") && !r.Config.Includes("LPC")) || (strings.HasPrefix(s.Name, "LPP ") && !r.Config.Includes("LPP")) || (s.Name == "Ausgangsmessung MPC" && !r.Config.Includes("LPC")) {
			continue
		}
		row := stepRow{nr: i + 1, s: s}
		row.names = wrapText(r.StepLabel(s), columnWidths[1]-14, 9, "F2")
		row.details = wrapText(presentStepDetail(r, s), columnWidths[1]-14, 8, "F1")
		row.expected = wrapText(target(s, r.Config, r.Mode), columnWidths[3]-14, 8, "F1")
		row.measured = []string{"–"}
		if s.Watts != nil {
			row.measured = wrapText(powerLabel(*s.Watts), columnWidths[4]-14, 8, "F2")
		}
		row.h = math.Max(32, float64(len(row.names))*12+float64(len(row.details))*10+8)
		row.h = math.Max(row.h, float64(max(len(row.expected), len(row.measured)))*11+16)
		if row.h+tableFirstHeader > bodyBottom-bodyTop {
			d.err = fmt.Errorf("PDF: Prüfschritt %d passt als ungeteilte Zeile nicht auf eine Seite", i+1)
			return
		}
		rows = append(rows, row)
	}
	heights := make([]float64, len(rows))
	for i, row := range rows {
		heights[i] = row.h
	}
	if n := len(heights); n > 0 && heights[n-1]+tableNote+tableContHeader <= bodyBottom-bodyTop {
		// Keep the table note with its final row, where both fit.
		heights[n-1] += tableNote
	}
	if len(rows) == 0 {
		d.reserve(25)
		d.text(margin+7, d.y+7, 7, "F1", muted, "¹ Uhrzeit mit Sekunden; Zeitzone siehe Beginn/Ende.  ² Leistungswerte gemäß Prüfprofil, Toleranz siehe oben.")
		d.y += 25
		return
	}
	fresh, breaks := paginateRows(d.y, heights)
	if fresh {
		d.newPage()
	}
	d.y += 15
	d.tableHeader(false)
	for i, row := range rows {
		if breaks[i] {
			d.newPage()
			d.tableHeader(true)
		}
		d.stepPages = append(d.stepPages, len(d.pages)-1)
		s := row.s
		x := margin
		d.text(x+7, d.y+9, 8, "F1", muted, fmt.Sprintf("%02d", row.nr))
		x += columnWidths[0]
		d.lines(x+7, d.y+7, 9, 12, "F2", ink, row.names)
		d.lines(x+7, d.y+7+float64(len(row.names))*12, 8, 10, "F1", muted, row.details)
		x += columnWidths[1]
		d.text(x+7, d.y+9, 8, "F3", ink, s.At.Format("15:04:05"))
		x += columnWidths[2]
		d.lines(x+7, d.y+9, 8, 11, "F1", ink, row.expected)
		x += columnWidths[3]
		d.lines(x+7, d.y+9, 8, 11, "F2", ink, row.measured)
		x += columnWidths[4]
		if s.Status == inspection.NotChecked {
			d.stepStatus(x+7, d.y+10, "Nicht geprüft")
		} else if !s.IsCheckpoint() {
			label := "Erledigt"
			if !s.Passed {
				label = "Fehlgeschlagen"
			}
			d.stepStatus(x+7, d.y+10, label)
		} else {
			label := "Bestanden"
			if !s.Passed {
				label = "Nicht bestanden"
			}
			d.stepStatus(x+7, d.y+10, label)
		}
		d.y += row.h
		d.line(margin, d.y, pageWidth-margin, d.y, 0.4, rule)
	}
	d.reserve(25)
	d.text(margin+7, d.y+7, 7, "F1", muted, "¹ Uhrzeit mit Sekunden; Zeitzone siehe Beginn/Ende.  ² Leistungswerte gemäß Prüfprofil, Toleranz siehe oben.")
	d.y += 25
}
func (d *document) stepStatus(x, y float64, label string) {
	color := ink
	if label == "Bestanden" {
		color = green
	}
	if label == "Nicht bestanden" || label == "Fehlgeschlagen" {
		color = red
	}
	if label == "Nicht geprüft" {
		color = muted
	}
	d.frame(x, y, 10, 10, rule)
	d.statusMark(x+1, y+1, label, color)
	d.text(x+15, y+1, 6.5, "F1", ink, label)
}
func (d *document) badge(x, y float64, ok bool) {
	label := "Bestanden"
	if !ok {
		label = "Nicht bestanden"
	}
	d.statusBadge(x, y, label)
}

func (d *document) statusBadge(x, y float64, label string) {
	color := ink
	if label == "Bestanden" {
		color = green
	}
	if label == "Nicht bestanden" || label == "Fehlgeschlagen" {
		color = red
	}
	if label == "Nicht geprüft" {
		color = muted
	}
	d.frame(x, y, 64, 26, color)
	d.statusMark(x+28, y+4, label, color)
	d.text(x+(64-textWidth(label, 7, "F1"))/2, y+16, 7, "F1", ink, label)
}

func (d *document) statusMark(x, y float64, label, color string) {
	switch strings.ToLower(label) {
	case "bestanden", "erledigt":
		d.line(x, y+4, x+3, y+7, 1.2, color)
		d.line(x+3, y+7, x+8, y+1, 1.2, color)
	case "nicht bestanden", "fehlgeschlagen":
		d.line(x+1, y+1, x+7, y+7, 1.2, color)
		d.line(x+1, y+7, x+7, y+1, 1.2, color)
	default:
		d.line(x+1, y+4, x+7, y+4, 1.2, color)
	}
}

// Preserve the numeric evidence and threshold direction while spelling out
// negative power as feed-in in the displayed table. JSON stays unchanged.
func powerLabel(v float64) string {
	if v < 0 {
		return watts(-v) + " Einspeisung"
	}
	return watts(v)
}

var acknowledgement = regexp.MustCompile(`(^|[\s(;:=])SPINE result ([0-9]+), Nachricht ([0-9]+)`)
var reportedPower = regexp.MustCompile(`(^|[\s(;:=])MPC (-?[0-9]+(?:\.[0-9]+)?) W\b`)

func stepDetail(s inspection.Step) string {
	text := acknowledgement.ReplaceAllStringFunc(inspection.UserDetail(s.Detail), func(match string) string {
		m := acknowledgement.FindStringSubmatch(match)
		if m[2] == "0" {
			return m[1] + "Steuerbefehl bestätigt (SPINE-Code 0, Nachricht " + m[3] + ")"
		}
		return m[1] + "Quittung (Code " + m[2] + ", Nachricht " + m[3] + ")"
	})
	text = reportedPower.ReplaceAllStringFunc(text, func(match string) string {
		m := reportedPower.FindStringSubmatch(match)
		value := m[2]
		if strings.HasPrefix(value, "-") {
			return m[1] + "Leistung " + strings.TrimPrefix(value, "-") + " W (Einspeisung)"
		}
		return m[1] + "Leistung " + value + " W"
	})
	text = strings.ReplaceAll(text, "negative Einspeisung", "Einspeisung")
	return readableDetail(detailNumbers(text))
}

// Localize only standalone decimal watt values. Identifiers, already German
// numbers, timestamps and unrelated free text remain intact; JSON is untouched.
var decimalWatts = regexp.MustCompile(`(^|[\s(;:=<>])(-?[0-9]+(?:\.[0-9]+)?) W\b`)

func detailNumbers(s string) string {
	return decimalWatts.ReplaceAllStringFunc(s, func(match string) string {
		number := strings.TrimSuffix(match, " W")
		start := 0
		for start < len(number) && !((number[start] >= '0' && number[start] <= '9') || number[start] == '-') {
			start++
		}
		raw := number[start:]
		// A dot followed by exactly three digits is ambiguous with a German
		// thousands separator. Known Run details use one decimal or an integer.
		if strings.Contains(raw, ".") && len(strings.SplitN(raw, ".", 2)[1]) == 3 {
			return match
		}
		// Format lexically so even free-text values beyond float64 precision
		// retain every significant digit of the original evidence.
		parts := strings.SplitN(raw, ".", 2)
		whole := parts[0]
		sign := ""
		if strings.HasPrefix(whole, "-") {
			sign, whole = "-", whole[1:]
		}
		for i := len(whole) - 3; i > 0; i -= 3 {
			whole = whole[:i] + "." + whole[i:]
		}
		if len(parts) == 2 {
			fraction := strings.TrimRight(parts[1], "0")
			if fraction != "" {
				whole += "," + fraction
			}
		}
		return number[:start] + sign + whole + " W"
	})
}

func germanNumber(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "–"
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	s = strings.TrimSuffix(s, ".0")
	parts := strings.SplitN(s, ".", 2)
	whole := parts[0]
	sign := ""
	if strings.HasPrefix(whole, "-") {
		sign = "-"
		whole = whole[1:]
	}
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "." + whole[i:]
	}
	if len(parts) == 2 {
		whole += "," + parts[1]
	}
	return sign + whole
}
func watts(v float64) string { return germanNumber(v) + " W" }

// GermanNumber formats like the PDF and the web result (4.200, 10,9; UX-G7, 040 F-12).
func GermanNumber(v float64) string { return germanNumber(v) }

// Conservative glyph advances for the standard Helvetica faces. These are upper
// bounds rather than copied font metrics; wrapping deliberately leaves a small
// safety margin. Courier has a fixed advance of 0.6 em.
func textWidth(s string, size float64, font string) float64 {
	w := 0.0
	for _, r := range s {
		if font == "F3" {
			w += 0.6
			continue
		}
		switch {
		case strings.ContainsRune(" ilI.,:;!'|", r):
			w += 0.30
		case strings.ContainsRune("fjt()[]{}r-", r):
			w += 0.40
		case strings.ContainsRune("mwMW@%&Ææ", r):
			w += 1.0
		case unicode.IsUpper(r):
			w += 0.78
		default:
			w += 0.62
		}
	}
	return w * size
}
func wrapText(s string, width, size float64, font string) []string {
	// Retain explicit line breaks, normalize tabs/control characters, and split
	// even long unbroken identifiers by measured width.
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	var out []string
	for _, paragraph := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			if line != "" && textWidth(line+" "+word, size, font) <= width {
				line += " " + word
				continue
			}
			if line != "" {
				out = append(out, line)
				line = ""
			}
			for _, r := range word {
				if line != "" && textWidth(line+string(r), size, font) > width {
					out = append(out, line)
					line = ""
				}
				line += string(r)
			}
		}
		out = append(out, line)
	}
	return out
}

func (d *document) bytes() []byte {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>"}
	imageResource := ""
	if d.logo != nil {
		objects = append(objects, imageObject(d.logo))
		imageResource = " /XObject << /Logo 6 0 R >>"
	}
	var kids []string
	for _, stream := range d.pages {
		pageID := len(objects) + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageID))
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >>%s >> /Contents %d 0 R /Annots [%d 0 R] >>", pageWidth, pageHeight, imageResource, pageID+1, pageID+2), fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", stream.Len(), stream.String()),
			fmt.Sprintf("<< /Type /Annot /Subtype /Link /Rect [%.2f %.2f %.2f %.2f] /Border [0 0 0] /A << /S /URI /URI (https://stromea.de/simulator) >> >>", margin, pageHeight-813, pageWidth-margin, pageHeight-798))
	}
	embeddedID := len(objects) + 1
	objects = append(objects, fmt.Sprintf("<< /Type /EmbeddedFile /Subtype /application#2Fjson /Length %d /Params << /Size %d >> >>\nstream\n%s\nendstream", len(d.attachment), len(d.attachment), d.attachment))
	specID := len(objects) + 1
	objects = append(objects, fmt.Sprintf("<< /Type /Filespec /F (stromea-pruefbericht.json) /UF (stromea-pruefbericht.json) /Desc (Stromea report JSON; schema: docs/schema/pruefbericht-v1.json) /EF << /F %d 0 R /UF %d 0 R >> /AFRelationship /Data >>", embeddedID, embeddedID))
	metadataID := len(objects) + 1
	objects = append(objects, fmt.Sprintf("<< /Type /Metadata /Subtype /XML /Length %d >>\nstream\n%s\nendstream", len(d.metadata), d.metadata))
	objects[0] = fmt.Sprintf("<< /Type /Catalog /Pages 2 0 R /Names << /EmbeddedFiles << /Names [(stromea-pruefbericht.json) %d 0 R] >> >> /AF [%d 0 R] /Metadata %d 0 R >>", specID, specID, metadataID)
	objects[1] = fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(d.pages), strings.Join(kids, " "))
	var b bytes.Buffer
	b.WriteString("%PDF-2.0\n%\xe2\xe3\xcf\xd3\n")
	offsets := []int{0}
	for i, o := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	start := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), start)
	return b.Bytes()
}

func literal(s string) string {
	var b strings.Builder
	for _, r := range s {
		var c byte
		switch {
		case r >= 32 && r <= 126:
			c = byte(r)
		case r >= 160 && r <= 255:
			c = byte(r)
		case r == '€':
			c = 128
		case r == '–':
			c = 150
		case r == '—':
			c = 151
		case r == '‘':
			c = 145
		case r == '’':
			c = 146
		case r == '“':
			c = 147
		case r == '”':
			c = 148
		case r == '•':
			c = 149
		case r == '…':
			c = 133
		case r == '→':
			c = '>'
		default:
			c = '?'
		}
		if c == '(' || c == ')' || c == '\\' {
			b.WriteByte('\\')
			b.WriteByte(c)
		} else if c >= 128 {
			fmt.Fprintf(&b, "\\%03o", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func xmpMetadata(r inspection.Report, id string) string {
	escape := func(s string) string { var b bytes.Buffer; _ = xml.EscapeText(&b, []byte(s)); return b.String() }
	software := r.Software
	if software == "" {
		software = BuildVersion()
	}
	return `<?xpacket begin="` + "\uFEFF" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"><rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmlns:stromea="https://stromea.de/simulator/schema/metadata/1/">
<dc:title><rdf:Alt><rdf:li xml:lang="x-default">Prüfbericht · ` + escape(r.ReportTitle()) + `</rdf:li></rdf:Alt></dc:title><dc:identifier>` + escape(id) + `</dc:identifier><xmp:CreateDate>` + r.Started.Format(time.RFC3339Nano) + `</xmp:CreateDate><xmp:CreatorTool>` + escape(software) + `</xmp:CreatorTool><stromea:protocolID>` + escape(id) + `</stromea:protocolID><stromea:schema>docs/schema/pruefbericht-v1.json</stromea:schema></rdf:Description></rdf:RDF></x:xmpmeta>
<?xpacket end="w"?>`
}

// ProtocolIdentity retains historical content IDs independently of library keys.
func ProtocolIdentity(r inspection.Report) string {
	if r.LegacyProtocolID != "" {
		return r.LegacyProtocolID
	}
	if inspection.ValidReportID(r.ID) || len(r.ID) == 12 && strings.Trim(r.ID, "0123456789abcdef") == "" {
		return r.ID
	}
	r.LegacyProtocolID = ""
	data, _ := JSON(r)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:6])
}
