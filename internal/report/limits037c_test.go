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

var limitPath037c = regexp.MustCompile(`q \[([0-9 ]+)\] 0 d [^\n]* RG 1\.00 w 1 J 1 j((?:\s+-?[0-9.]+ -?[0-9.]+ [ml])+)\s+S Q`)

// A full-width reference line from the plot's left (77.00) to its right edge (428.28).
var constantLine037c = regexp.MustCompile(`RG 0\.60 w 77\.00 (-?[0-9.]+) m 428\.28 (-?[0-9.]+) l S Q`)

type limitSegment037c struct {
	dash   string
	points [][2]float64
}

func limitPaths037c(content string) []limitSegment037c {
	var out []limitSegment037c
	for _, m := range limitPath037c.FindAllStringSubmatch(content, -1) {
		seg := limitSegment037c{dash: m[1]}
		for _, v := range vertex037.FindAllStringSubmatch(m[2], -1) {
			x, _ := strconv.ParseFloat(v[1], 64)
			y, _ := strconv.ParseFloat(v[2], 64)
			seg.points = append(seg.points, [2]float64{x, y})
		}
		out = append(out, seg)
	}
	return out
}

func w037c(v float64) *float64 { return &v }

// UX-S21: limits are dashed steps only while they applied; failsafe dotted.
func Test037cPDFLimitsOnlyWhileValid(t *testing.T) {
	r := sampleReport(t)
	r.Config.Scope = inspection.ScopeConsumption
	at := r.Started
	var samples []inspection.Sample
	for i := 0; i <= 20; i++ {
		samples = append(samples, inspection.Sample{At: at.Add(time.Duration(i) * time.Second), Watts: w037c(8000), Step: "LPC gemessene Wirkung"})
	}
	r.Samples = samples
	r.Limits = []inspection.LimitChange{
		{At: at.Add(5 * time.Second), Direction: "LPC", Watts: w037c(4200)},
		{At: at.Add(10 * time.Second), Direction: "LPC", Watts: w037c(500), Failsafe: true},
		{At: at.Add(15 * time.Second), Direction: "LPC"},
	}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	segs := limitPaths037c(got)
	if len(segs) != 2 {
		t.Fatalf("expected limit and failsafe segment, got %d", len(segs))
	}
	left, right := margin+35, pageWidth-margin-125
	span := 20.0
	x := func(s float64) float64 { return left + (right-left)*s/span }
	limit, failsafe := segs[0], segs[1]
	if limit.dash == failsafe.dash {
		t.Fatal("failsafe must be distinguishable from the normal limit")
	}
	if math.Abs(limit.points[0][0]-x(5)) > .02 || math.Abs(limit.points[len(limit.points)-1][0]-x(10)) > .02 {
		t.Fatalf("limit must start at its acknowledgement and end at failsafe: %v", limit.points)
	}
	if math.Abs(failsafe.points[0][0]-x(10)) > .02 || math.Abs(failsafe.points[len(failsafe.points)-1][0]-x(15)) > .02 {
		t.Fatalf("failsafe must end at its end: %v", failsafe.points)
	}
	// Vertical jump at the change: failsafe starts at the previous level.
	if math.Abs(failsafe.points[0][1]-limit.points[len(limit.points)-1][1]) > .02 || failsafe.points[1][0] != failsafe.points[0][0] {
		t.Fatalf("change must be a vertical jump: %v -> %v", limit.points, failsafe.points)
	}
	for _, s := range segs {
		for i := 1; i < len(s.points); i++ {
			if math.Abs(s.points[i][0]-s.points[i-1][0]) > .011 && math.Abs(s.points[i][1]-s.points[i-1][1]) > .011 {
				t.Fatal("limit segments are axis-parallel")
			}
		}
	}
	// No full-width constant reference line any more.
	if constantLine037c.MatchString(got) {
		t.Fatal("constant full-width reference line remains")
	}
	if !strings.Contains(got, "Gestrichelt: Grenze ab Quittung; gepunktet: Notbetrieb ab Ablauf der Ausfallfrist.") {
		t.Fatal("legend must explain the limit steps")
	}
}

// Reports without recorded timeline draw no invented limit lines.
func Test037cPDFLegacyReportDrawsNoLimitLines(t *testing.T) {
	r := sampleReport(t)
	v := 8000.0
	r.Samples = []inspection.Sample{{At: r.Started, Watts: &v}, {At: r.Started.Add(time.Second), Watts: &v}}
	r.Limits = nil
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	if len(limitPaths037c(got)) != 0 || constantLine037c.MatchString(got) {
		t.Fatal("no limit line without recorded timeline")
	}
	if !strings.Contains(got, "Kein Grenzverlauf aufgezeichnet") {
		t.Fatal("legacy note missing")
	}
}

// 037d (Review 037c P2): phase bands and their labels use the limit timeline,
// the same time base as the limit lines; misleading step names do not matter.
func Test037dPDFPhasesFollowLimitTimeline(t *testing.T) {
	r := sampleReport(t)
	r.Config.Scope = inspection.ScopeConsumption
	at := r.Started
	r.Samples = nil
	for i := 0; i <= 20; i++ {
		r.Samples = append(r.Samples, inspection.Sample{At: at.Add(time.Duration(i) * time.Second), Watts: w037c(8000), Step: "LPC Wiederkehr"})
	}
	r.Limits = []inspection.LimitChange{
		{At: at.Add(5 * time.Second), Direction: "LPC", Watts: w037c(4200)},
		{At: at.Add(10 * time.Second), Direction: "LPC", Watts: w037c(500), Failsafe: true},
		{At: at.Add(15 * time.Second), Direction: "LPC"},
	}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	left, right := margin+35, pageWidth-margin-125
	x := func(s float64) float64 { return left + (right-left)*s/20 }
	// 038: phases are measure arrows (no coloured areas), cut at the same limit changes.
	spans := arrows038(got)
	want := [][2]float64{{x(0), x(5)}, {x(5), x(10)}, {x(10), x(15)}, {x(15), x(20)}}
	if len(spans) != len(want) {
		t.Fatalf("phase arrows %v, want %v", spans, want)
	}
	for i, w := range want {
		if math.Abs(spans[i].from-w[0]) > .02 || math.Abs(spans[i].to-w[1]) > .02 {
			t.Errorf("arrow %d %v, want %v", i, spans[i], w)
		}
	}
	for _, name := range []string{"Ausgangsleistung", "Steuerung aktiv", "Notbetrieb", "Normalbetrieb"} {
		if !strings.Contains(got, "("+literal(name)+")") {
			t.Errorf("phase name %q missing", name)
		}
	}
	if regexp.MustCompile(` 150\.00 re f`).MatchString(got) {
		t.Error("no coloured phase areas")
	}
}

// Unconfirmed return is drawn differently and explained.
func Test037dPDFUnconfirmedFailsafe(t *testing.T) {
	r := sampleReport(t)
	r.Config.Scope = inspection.ScopeConsumption
	at := r.Started
	r.Samples = []inspection.Sample{{At: at, Watts: w037c(8000)}, {At: at.Add(20 * time.Second), Watts: w037c(500)}}
	r.Limits = []inspection.LimitChange{
		{At: at.Add(5 * time.Second), Direction: "LPC", Watts: w037c(500), Failsafe: true},
		{At: at.Add(10 * time.Second), Direction: "LPC", Watts: w037c(500), Failsafe: true, Unconfirmed: true},
	}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	segs := limitPaths037c(got)
	if len(segs) != 2 || segs[0].dash == segs[1].dash {
		t.Fatalf("unconfirmed failsafe must look different: %v", segs)
	}
	// 046: the light dotted line also marks an unproven failsafe entry.
	if !strings.Contains(got, "Notbetrieb oder R") || !strings.Contains(got, "daraus nicht nachgewiesen") {
		t.Fatal("legend must explain the unconfirmed failsafe or return")
	}
}
