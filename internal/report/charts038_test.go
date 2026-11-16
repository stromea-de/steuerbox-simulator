package report

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// Phase arrows: thin horizontal measure lines above the plot (UX-S23).
var arrow038 = regexp.MustCompile(`RG 0\.50 w ([0-9.]+) ([0-9.]+) m ([0-9.]+) ([0-9.]+) l S Q`)

type span038 struct{ from, to, y float64 }

func arrows038(content string) []span038 {
	var out []span038
	for _, m := range arrow038.FindAllStringSubmatch(content, -1) {
		x1, _ := strconv.ParseFloat(m[1], 64)
		y1, _ := strconv.ParseFloat(m[2], 64)
		x2, _ := strconv.ParseFloat(m[3], 64)
		y2, _ := strconv.ParseFloat(m[4], 64)
		if y1 == y2 && x2 > x1 {
			out = append(out, span038{x1, x2, y1})
		}
	}
	return out
}

// 038 (UX-S23): Bezug and Einspeisung are separate charts, each with only its
// own limits; phases are measure arrows, no coloured areas; no step legend.
func Test038PDFSeparateChartsAndPhaseArrows(t *testing.T) {
	r := sampleReport(t)
	r.Config.Scope = inspection.ScopeBoth
	at := r.Started
	r.Samples = nil
	for i := 0; i < 20; i++ {
		step, v := "LPC gemessene Wirkung", 8000.0
		if i < 2 {
			step = "Ausgangsmessung MPC"
		}
		if i >= 10 {
			step, v = "LPP gemessene Wirkung", -6000
		}
		r.Samples = append(r.Samples, inspection.Sample{At: at.Add(time.Duration(i) * time.Second), Watts: w037c(v), Step: step})
	}
	r.Limits = []inspection.LimitChange{
		{At: at.Add(3 * time.Second), Direction: "LPC", Watts: w037c(4200)},
		{At: at.Add(13 * time.Second), Direction: "LPP", Watts: w037c(-3000)},
		{At: at.Add(18 * time.Second), Direction: "LPC"},
		{At: at.Add(18 * time.Second), Direction: "LPP"},
	}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := strings.Join(func() []string {
		var all []string
		for _, p := range d.pages {
			all = append(all, p.String())
		}
		return all
	}(), "\n")
	for _, title := range []string{"Messverlauf · Bezug (LPC)", "Messverlauf · Einspeisung (LPP)"} {
		if strings.Count(got, "("+literal(title)+")") != 1 {
			t.Errorf("chart %q must appear once", title)
		}
	}
	if strings.Contains(got, literal("Bezug / Einspeisung")) {
		t.Error("no combined chart")
	}
	if regexp.MustCompile(` 150\.00 re f`).MatchString(got) {
		t.Error("no coloured phase areas")
	}
	if len(arrows038(got)) < 3 {
		t.Errorf("phase arrows missing: %v", arrows038(got))
	}
	// LPC limit lasts until 18 s, but the Bezug chart ends where the LPP section
	// begins and the Einspeisung chart shows no LPC line.
	if n := len(limitPaths037c(got)); n != 2 {
		t.Errorf("each chart shows only its own direction's limit: %d limit paths", n)
	}
	if strings.Contains(got, "jeder Messwert gilt") || strings.Contains(got, "Treppenlinie") {
		t.Error("step legend removed")
	}
	if !strings.Contains(got, "Rot gestrichelt: Messausfall.") {
		t.Error("outage legend stays")
	}
}

// Consumption scope: one chart only.
func Test038PDFSingleDirectionSingleChart(t *testing.T) {
	r := sampleReport(t)
	r.Config.Scope = inspection.ScopeConsumption
	v := 8000.0
	r.Samples = []inspection.Sample{{At: r.Started, Watts: &v, Step: "Ausgangsmessung MPC"}, {At: r.Started.Add(time.Second), Watts: &v, Step: "LPC gemessene Wirkung"}}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	if !strings.Contains(got, "("+literal("Messverlauf · Bezug (LPC)")+")") || strings.Contains(got, literal("Einspeisung (LPP)")+")") {
		t.Fatal("consumption report has exactly the Bezug chart")
	}
	_ = math.Abs
}

var phaseName038 = regexp.MustCompile(`BT /F1 6\.00 Tf [0-9. ]+ rg 1 0 0 1 ([0-9.]+) ([0-9.]+) Tm \((Ausgangsleistung|Steuerung aktiv|Notbetrieb|Normalbetrieb)\) Tj`)

// Review 037d/038 P3: every phase arrow has a visible name, even when phases
// are short; names never overlap (extra label rows instead of dropping them).
func Test038PDFEveryPhaseArrowIsNamed(t *testing.T) {
	r := sampleReport(t)
	r.Config.Scope = inspection.ScopeConsumption
	at := r.Started
	r.Samples = nil
	for i := 0; i <= 20; i++ {
		r.Samples = append(r.Samples, inspection.Sample{At: at.Add(time.Duration(i) * 500 * time.Millisecond), Watts: w037c(8000), Step: "LPC gemessene Wirkung"})
	}
	r.Limits = []inspection.LimitChange{
		{At: at.Add(time.Second), Direction: "LPC", Watts: w037c(4200)},
		{At: at.Add(5 * time.Second), Direction: "LPC", Watts: w037c(500), Failsafe: true},
		{At: at.Add(5300 * time.Millisecond), Direction: "LPC", Watts: w037c(4200)},
		{At: at.Add(5600 * time.Millisecond), Direction: "LPC"},
	}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	arrows := arrows038(got)
	names := phaseName038.FindAllStringSubmatch(got, -1)
	if len(names) != len(arrows) {
		t.Fatalf("%d arrows but %d names", len(arrows), len(names))
	}
	type box struct{ x1, x2, y float64 }
	var boxes []box
	for _, m := range names {
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		boxes = append(boxes, box{x, x + textWidth(m[3], 6, "F1"), y})
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			if math.Abs(boxes[i].y-boxes[j].y) < 6 && boxes[i].x1 < boxes[j].x2 && boxes[j].x1 < boxes[i].x2 {
				t.Fatalf("phase names overlap: %v %v", boxes[i], boxes[j])
			}
		}
	}
}
