package report

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func curvePhase(name string) string {
	if strings.Contains(name, "Ausgangsmessung") || strings.Contains(name, "Last vor") || strings.Contains(name, "Lastentscheidung") || name == "Verbindung" || name == "Unterstützte Use Cases" || strings.Contains(name, "Ursprungswerte") || name == "Failsafe-Werte setzen" || name == "Heartbeat starten" {
		return "Ausgangsleistung"
	}
	if strings.Contains(name, "Failsafe gemessene") || strings.Contains(name, "Heartbeat-Ausfall") {
		return "Notbetrieb"
	}
	// "Wiederkehr" returns to the still active limit; only release steps end control.
	if strings.Contains(name, "Vorzeitige Rückkehr") || strings.Contains(name, "Freigabe") || strings.Contains(name, "freigeben") || strings.Contains(name, "aufheben") || strings.Contains(name, "zurückstellen") {
		return "Normalbetrieb"
	}
	return "Steuerung aktiv"
}

// limitPhase names the phase at t from the recorded limit timeline (UX-S21), so
// bands and labels share the time base of the limit lines.
func limitPhase(limits []inspection.LimitChange, t time.Time) string {
	failsafe, active, ever := false, false, false
	for _, direction := range []string{"LPC", "LPP"} {
		var current *inspection.LimitChange
		for i := range limits {
			c := &limits[i]
			if c.Direction != direction || c.At.After(t) {
				continue
			}
			current = c
			if c.Watts != nil {
				ever = true
			}
		}
		if current != nil && current.Watts != nil {
			if current.Failsafe {
				failsafe = true
			} else {
				active = true
			}
		}
	}
	switch {
	case failsafe:
		return "Notbetrieb"
	case active:
		return "Steuerung aktiv"
	case ever:
		return "Normalbetrieb"
	}
	return "Ausgangsleistung"
}

type phaseBand struct {
	from, to time.Time
	phase    string
}

// limitBands splits [first, last] at every limit change.
func limitBands(limits []inspection.LimitChange, first, last time.Time) []phaseBand {
	cuts := []time.Time{first}
	for _, c := range limits {
		if c.At.After(first) && c.At.Before(last) {
			cuts = append(cuts, c.At)
		}
	}
	cuts = append(cuts, last)
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].Before(cuts[j]) })
	var bands []phaseBand
	for i := 0; i+1 < len(cuts); i++ {
		if !cuts[i+1].After(cuts[i]) {
			continue
		}
		phase := limitPhase(limits, cuts[i])
		if n := len(bands); n > 0 && bands[n-1].phase == phase {
			bands[n-1].to = cuts[i+1]
			continue
		}
		bands = append(bands, phaseBand{cuts[i], cuts[i+1], phase})
	}
	return bands
}

// curveScale includes zero and uses round ticks for either direction.
func curveScale(low, high float64) (lo, hi, tick float64) {
	raw := math.Max(1, (high-low)/4)
	base := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, n := range []float64{1, 2, 2.5, 5, 10} {
		if n*base >= raw {
			tick = n * base
			break
		}
	}
	lo = math.Floor(math.Min(0, low)/tick) * tick
	hi = math.Max(lo+tick, math.Ceil(math.Max(0, high)/tick)*tick)
	return
}

// curveSection is one chart: Bezug (LPC) or Einspeisung (LPP). The section of a
// reading is the test step it was recorded in: from the first "LPP …" reading
// on, the readings belong to the feed-in test (UX-S23).
type curveSection struct {
	direction, title string
	samples          []inspection.Sample
}

func curveSections(r inspection.Report) []curveSection {
	split := len(r.Samples)
	if r.Config.Includes("LPC") && r.Config.Includes("LPP") {
		for i, s := range r.Samples {
			if strings.HasPrefix(s.Step, "LPP ") {
				split = i
				break
			}
		}
	}
	var out []curveSection
	add := func(direction, title string, samples []inspection.Sample) {
		for _, s := range samples {
			if s.Watts != nil && !s.At.IsZero() && !math.IsNaN(*s.Watts) && !math.IsInf(*s.Watts, 0) {
				out = append(out, curveSection{direction, title, samples})
				return
			}
		}
	}
	switch {
	case !r.Config.Includes("LPP"):
		add("LPC", "Messverlauf · Bezug (LPC)", r.Samples)
	case !r.Config.Includes("LPC"):
		add("LPP", "Messverlauf · Einspeisung (LPP)", r.Samples)
	default:
		add("LPC", "Messverlauf · Bezug (LPC)", r.Samples[:split])
		add("LPP", "Messverlauf · Einspeisung (LPP)", r.Samples[split:])
	}
	return out
}

// Native PDF paths preserve timestamp spacing, signs and observed gaps. No raster
// dependency or interpolation of unobserved values is needed by this writer.
func (d *document) powerCurve(r inspection.Report) {
	sections := curveSections(r)
	if len(sections) == 0 {
		d.emptyCurve()
		return
	}
	for i, section := range sections {
		d.powerChart(r, section, i == len(sections)-1)
	}
}

// phaseArrow marks a phase like a dimension line: ←→ with its name above.
func (d *document) phaseArrow(x1, x2, y float64) {
	d.line(x1, y, x2, y, .5, muted)
	for _, end := range []struct{ x, dir float64 }{{x1, 1}, {x2, -1}} {
		fmt.Fprintf(d.current(), "q %s rg %.2f %.2f m %.2f %.2f l %.2f %.2f l f Q\n", rgb(muted), end.x, pageHeight-y, end.x+end.dir*3.5, pageHeight-y-1.6, end.x+end.dir*3.5, pageHeight-y+1.6)
	}
}

func (d *document) powerChart(r inspection.Report, section curveSection, legend bool) {
	samples := section.samples
	var limits []inspection.LimitChange
	for _, c := range r.Limits {
		if c.Direction == section.direction {
			limits = append(limits, c)
		}
	}
	var first, last time.Time
	lo, hi := 0.0, 0.0
	if section.direction == "LPC" {
		hi = math.Max(r.Config.LPC, r.Config.Failsafe)
	} else {
		lo = -math.Max(r.Config.LPP, r.Config.Failsafe)
	}
	for _, s := range samples {
		if !s.At.IsZero() {
			if first.IsZero() || s.At.Before(first) {
				first = s.At
			}
			if last.IsZero() || s.At.After(last) {
				last = s.At
			}
		}
		if s.Watts == nil || math.IsNaN(*s.Watts) || math.IsInf(*s.Watts, 0) || s.At.IsZero() {
			continue
		}
		lo = math.Min(lo, *s.Watts)
		hi = math.Max(hi, *s.Watts)
	}
	for _, c := range limits {
		if c.Watts != nil {
			lo, hi = math.Min(lo, *c.Watts), math.Max(hi, *c.Watts)
		}
	}
	left, right := margin+35, pageWidth-margin-125
	span := math.Max(1, last.Sub(first).Seconds())
	x := func(t time.Time) float64 { return left + (right-left)*t.Sub(first).Seconds()/span }
	// UX-S23: phases as measure arrows above the plot, cut at this direction's
	// recorded limit changes (UX-S21); older reports use the recorded steps.
	var bands []phaseBand
	if len(limits) > 0 {
		bands = limitBands(limits, first, last)
	} else {
		// Contiguous readings extend the current phase; a new phase starts where
		// the previous one ends, and a failed read leaves a gap.
		contiguous := false
		for _, s := range samples {
			if s.Watts == nil || s.At.IsZero() || math.IsNaN(*s.Watts) || math.IsInf(*s.Watts, 0) {
				contiguous = false
				continue
			}
			phase := curvePhase(s.Step)
			n := len(bands)
			if contiguous && n > 0 {
				bands[n-1].to = s.At
				if bands[n-1].phase == phase {
					continue
				}
			}
			bands = append(bands, phaseBand{s.At, s.At, phase})
			contiguous = true
		}
	}
	// Review 037d/038: every arrow keeps a visible name. A name that does not fit
	// next to its neighbour moves to an extra row above instead of being dropped.
	type phaseLabel struct {
		x1, x2, labelX float64
		row            int
		name           string
	}
	var labels []phaseLabel
	var rowRight []float64
	for _, band := range bands {
		x1, x2 := x(band.from), x(band.to)
		if x2-x1 < .5 {
			continue
		}
		width := textWidth(band.phase, 6, "F1")
		labelX := math.Max(left, math.Min((x1+x2)/2-width/2, right-width))
		row := 0
		for row < len(rowRight) && labelX < rowRight[row]+6 {
			row++
		}
		if row == len(rowRight) {
			rowRight = append(rowRight, 0)
		}
		rowRight[row] = labelX + width
		labels = append(labels, phaseLabel{x1, x2, labelX, row, band.phase})
	}
	extra := 0.0
	if len(rowRight) > 1 {
		extra = 8 * float64(len(rowRight)-1)
	}
	height := 280.0 + extra
	if legend {
		height = 337 + extra
	}
	d.reserve(height)
	d.heading(section.title)
	top := d.y + 22 + extra
	bottom := top + 150
	lo, hi, tick := curveScale(lo, hi)
	y := func(v float64) float64 { return bottom - (v-lo)/(hi-lo)*150 }
	for _, l := range labels {
		d.phaseArrow(l.x1, l.x2, top-6)
		labelY := top - 17 - 8*float64(l.row)
		d.text(l.labelX, labelY, 6, "F1", muted, l.name)
	}
	for v := lo; v <= hi+tick/100; v += tick {
		d.line(left, y(v), right, y(v), .3, rule)
		d.text(margin, y(v)-4, 7, "F1", muted, germanNumber(v))
	}
	d.text(margin, top-18, 7, "F2", muted, "W")
	type ref struct {
		value float64
		label string
	}
	refs := []ref{{r.Config.LPC, "Bezug " + watts(r.Config.LPC) + " §14a"}, {r.Config.Failsafe, "Bezug " + watts(r.Config.Failsafe) + " Notbetrieb"}}
	if section.direction == "LPP" {
		refs = []ref{{-r.Config.LPP, "Einspeisung " + watts(r.Config.LPP)}, {-r.Config.Failsafe, "Einspeisung " + watts(r.Config.Failsafe) + " Notbetrieb"}}
	}
	// Keep close reference values readable without moving their actual grid lines.
	sort.SliceStable(refs, func(i, j int) bool { return y(refs[i].value) < y(refs[j].value) })
	labelYs := make([]float64, len(refs))
	for i, ref := range refs {
		labelYs[i] = math.Max(top, y(ref.value)-4)
		if i > 0 {
			labelYs[i] = math.Max(labelYs[i], labelYs[i-1]+14)
		}
	}
	for i := len(labelYs) - 1; i >= 0; i-- {
		labelYs[i] = math.Min(labelYs[i], bottom-8-float64(len(labelYs)-1-i)*14)
	}
	// UX-S21: the labels name the configured values at the right edge; the
	// limits themselves are drawn only while they applied.
	for i, ref := range refs {
		d.line(right, y(ref.value), right+5, labelYs[i]+4, .4, muted)
		d.text(right+7, labelYs[i], 7, "F1", muted, ref.label)
	}
	d.limitSteps(inspection.Report{Limits: limits}, first, last, x, y)

	// Step-after (UX-S19): a reading is valid until the next one, so it holds
	// horizontally and changes vertically. A failed read ends the hold at its
	// timestamp; no value between readings is interpolated.
	var previous *inspection.Sample
	var trace, dots [][2]float64
	flush := func() {
		d.polyline(trace, 1.8, ink)
		trace = nil
	}
	for i := range samples {
		s := &samples[i]
		if s.Watts == nil || s.At.IsZero() || math.IsNaN(*s.Watts) || math.IsInf(*s.Watts, 0) {
			if !s.At.IsZero() {
				if previous != nil && !s.At.Before(previous.At) {
					trace = append(trace, [2]float64{x(s.At), y(*previous.Watts)})
				}
				d.current().WriteString("q [4 3] 0 d\n")
				d.line(x(s.At), top, x(s.At), bottom, .8, red)
				d.current().WriteString("Q\n")
			}
			flush()
			previous = nil
			continue
		}
		if previous != nil && !s.At.Before(previous.At) {
			trace = append(trace, [2]float64{x(s.At), y(*previous.Watts)}, [2]float64{x(s.At), y(*s.Watts)})
		} else {
			flush()
			trace = [][2]float64{{x(s.At), y(*s.Watts)}}
		}
		dots = append(dots, [2]float64{x(s.At), y(*s.Watts)})
		previous = s
	}
	flush()
	for _, p := range dots {
		d.fillRect(p[0]-1, p[1]-1, 2, 2, ink)
	}
	d.text(left, bottom+7, 7, "F1", muted, "0 s")
	d.text(right-35, bottom+7, 7, "F1", muted, germanNumber(math.Round(span))+" s")
	// Connection band uses observed events; unobserved time remains neutral.
	bandY := bottom + 30
	// 038: label left of the bar, legend aligned with the bar start.
	d.text(left-2-textWidth("Verbindung", 6, "F1"), bandY+0.5, 6, "F1", muted, "Verbindung")
	d.fillRect(left, bandY, right-left, 7, "#EEEEEE")
	connected := true
	at := first
	for _, event := range r.Events {
		if event.At.Before(first) {
			continue
		}
		if event.At.After(last) {
			break
		}
		if connected {
			d.fillRect(x(at), bandY, max(0, x(event.At)-x(at)), 7, "#666666")
		}
		detail := strings.ToLower(event.Name + " " + event.Detail)
		if strings.Contains(detail, "getrennt") || strings.Contains(detail, "unterbrochen") {
			connected = false
		} else if strings.Contains(detail, "bereit") || strings.Contains(detail, "verbunden") {
			connected = true
		}
		at = event.At
	}
	if connected {
		d.fillRect(x(at), bandY, max(0, right-x(at)), 7, "#666666")
	}
	if !legend {
		d.y = bandY + 30
		return
	}
	d.text(left, bottom+47, 7.5, "F1", muted, "Verbindungsleiste: Grau = verbunden; Hell = getrennt oder nicht beobachtet.")
	note := "Rot gestrichelt: Messausfall."
	if r.Mode != inspection.ModeOnSite {
		note += " Übung: simulierte Werte."
	}
	d.text(left, bottom+63, 7.5, "F1", muted, note)
	limitsNote := "Gestrichelt: Grenze ab Quittung; gepunktet: Notbetrieb ab Ablauf der Ausfallfrist."
	if len(r.Limits) == 0 {
		limitsNote = "Kein Grenzverlauf aufgezeichnet; rechts die eingestellten Grenzwerte."
	}
	d.text(left, bottom+79, 7.5, "F1", muted, limitsNote)
	d.y = bottom + 101
	for _, c := range r.Limits {
		if c.Unconfirmed {
			d.text(left, bottom+95, 7.5, "F1", muted, "Hell gepunktet: Notbetrieb oder Rückkehr daraus nicht nachgewiesen.")
			d.y = bottom + 117
			break
		}
	}
}

// limitSteps draws each direction's recorded limit as a dashed step (failsafe
// dotted) from its change to the next change of the same direction; nothing
// where no limit was in effect. A change is a vertical jump from the previous
// level when the limit was in effect right before.
func (d *document) limitSteps(r inspection.Report, first, last time.Time, x func(time.Time) float64, y func(float64) float64) {
	for _, direction := range []string{"LPC", "LPP"} {
		var changes []inspection.LimitChange
		for _, c := range r.Limits {
			if c.Direction == direction {
				changes = append(changes, c)
			}
		}
		var previous *float64
		for i, c := range changes {
			end := last
			if i+1 < len(changes) {
				end = changes[i+1].At
			}
			if c.Watts == nil || math.IsNaN(*c.Watts) || math.IsInf(*c.Watts, 0) {
				previous = nil
				continue
			}
			level := y(*c.Watts)
			start := c.At
			clipped := start.Before(first)
			if clipped {
				start = first
			}
			if end.After(last) {
				end = last
			}
			if !end.After(start) {
				previous = &level
				continue
			}
			var points [][2]float64
			if previous != nil && !clipped {
				points = append(points, [2]float64{x(start), *previous})
			}
			points = append(points, [2]float64{x(start), level}, [2]float64{x(end), level})
			dash, color := "4 2", muted
			if c.Failsafe {
				dash = "1 2"
			}
			if c.Unconfirmed {
				dash, color = "1 4", "#9A9A9A"
			}
			d.dashedPolyline(points, 1, color, dash)
			previous = &level
		}
	}
}

func (d *document) dashedPolyline(points [][2]float64, width float64, color, dash string) {
	if len(points) < 2 {
		return
	}
	fmt.Fprintf(d.current(), "q [%s] 0 d %s RG %.2f w 1 J 1 j\n", dash, rgb(color), width)
	for i, p := range points {
		op := "l"
		if i == 0 {
			op = "m"
		}
		fmt.Fprintf(d.current(), "%.2f %.2f %s\n", p[0], pageHeight-p[1], op)
	}
	d.current().WriteString("S Q\n")
}

// polyline strokes one connected path with round joins, so step corners stay closed.
func (d *document) polyline(points [][2]float64, width float64, color string) {
	if len(points) < 2 {
		return
	}
	fmt.Fprintf(d.current(), "q %s RG %.2f w 1 J 1 j\n", rgb(color), width)
	for i, p := range points {
		op := "l"
		if i == 0 {
			op = "m"
		}
		fmt.Fprintf(d.current(), "%.2f %.2f %s\n", p[0], pageHeight-p[1], op)
	}
	d.current().WriteString("S Q\n")
}

func (d *document) fillRect(x, y, w, h float64, color string) {
	fmt.Fprintf(d.current(), "q %s rg %.2f %.2f %.2f %.2f re f Q\n", rgb(color), x, pageHeight-y-h, w, h)
}

func (d *document) emptyCurve() {
	d.reserve(58)
	d.heading("Messverlauf")
	d.lines(margin, d.y, 8.5, 11, "F1", muted, wrapText("Keine auswertbaren Messwerte: Das Gerät meldet keine Messwerte oder die Messung war nicht verfügbar.", bodyWidth, 8.5, "F1"))
	d.y += 32
}
