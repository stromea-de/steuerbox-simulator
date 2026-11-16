package report

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func Test036ConnectionBandDoesNotCoverItsLabel(t *testing.T) {
	r := sampleReport(t)
	v := 10996.0
	r.Samples = []inspection.Sample{{At: r.Started, Watts: &v}, {At: r.Started.Add(time.Second), Watts: &v}}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	text := d.current().String()
	// 038: the label sits left of the bar; its real font size comes from the content.
	label := regexp.MustCompile(`/F1 ([0-9.]+) Tf [0-9. ]+ rg 1 0 0 1 ([0-9.]+) ([0-9.]+) Tm \(Verbindung\)`).FindStringSubmatch(text)
	if len(label) != 4 {
		t.Fatal("connection label missing")
	}
	num := func(s string) float64 {
		n, e := strconv.ParseFloat(s, 64)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	size, x, y := num(label[1]), num(label[2]), num(label[3])
	count := 0
	for _, rect := range regexp.MustCompile(`q [0-9. ]+ rg ([0-9.]+) ([0-9.]+) ([0-9.]+) ([0-9.]+) re f Q`).FindAllStringSubmatch(text, -1) {
		rx, ry, w, h := num(rect[1]), num(rect[2]), num(rect[3]), num(rect[4])
		if h != 7 {
			continue
		}
		count++
		if rx < x+textWidth("Verbindung", size, "F1") && rx+w > x && ry < y+size && ry+h > y {
			t.Fatal("connection band overlaps its label")
		}
	}
	if count < 2 {
		t.Fatal("connection band not rendered")
	}
}

func Test036SparseSamplesAreConnectedAndNullIsAnOutage(t *testing.T) {
	r := sampleReport(t)
	v := 10996.0
	r.Samples = []inspection.Sample{{At: r.Started, Watts: &v, Step: "Ausgangsmessung MPC"}, {At: r.Started.Add(10 * time.Second), Watts: &v, Step: "LPC gemessene Wirkung"}}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	text := d.current().String()
	if strings.Contains(text, "(kW)") || !strings.Contains(text, "(W)") {
		t.Error("curve axis must use W")
	}
	if !strings.Contains(text, "1.80 w") {
		t.Error("sparse valid samples are not connected with a legible line")
	}
	if strings.Contains(text, "Lücken über 3 s") || strings.Contains(text, "L\\374cken \\374ber 3 s") {
		t.Error("elapsed time alone is not a failed measurement")
	}
	r.Samples = append(r.Samples[:1], inspection.Sample{At: r.Started.Add(time.Second), Step: "Messausfall"}, r.Samples[1])
	d = &document{}
	d.newPage()
	d.powerCurve(r)
	text = d.current().String()
	// 037 (UX-S19): the first reading holds until the failed read, then the trace breaks.
	paths, gaps := tracePaths037(t, text), gapXs037(text)
	if len(gaps) != 1 || len(paths) != 1 || len(paths[0]) != 2 || paths[0][1][0] > gaps[0]+.011 || paths[0][0][1] != paths[0][1][1] {
		t.Errorf("failed read must break trace: %v %v", paths, gaps)
	}
	if !strings.Contains(text, "Messausfall") {
		t.Error("outage must be visibly identified")
	}
}

func Test036FailedReadExtendsTheObservedTimeAxis(t *testing.T) {
	r := sampleReport(t)
	v := 10996.0
	r.Samples = []inspection.Sample{{At: r.Started, Watts: &v}, {At: r.Started.Add(10 * time.Second)}}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	text := d.current().String()
	if !strings.Contains(text, "(10 s)") {
		t.Fatal("failed-read timestamp omitted from the axis")
	}
	if !strings.Contains(text, "0.80 w") {
		t.Fatal("outage marker missing")
	}
}
