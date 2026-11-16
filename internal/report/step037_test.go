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

var trace037 = regexp.MustCompile(`RG 1\.80 w 1 J 1 j((?:\s+-?[0-9.]+ -?[0-9.]+ [ml])+)\s+S Q`)
var vertex037 = regexp.MustCompile(`(-?[0-9.]+) (-?[0-9.]+) [ml]`)

var gap037 = regexp.MustCompile(`\[4 3\] 0 d\nq [^\n]*? RG 0\.80 w (-?[0-9.]+) `)

// gapXs037 returns the x positions of the dashed failed-read markers.
func gapXs037(content string) []float64 {
	var xs []float64
	for _, m := range gap037.FindAllStringSubmatch(content, -1) {
		x, _ := strconv.ParseFloat(m[1], 64)
		xs = append(xs, x)
	}
	return xs
}

func tracePaths037(t *testing.T, content string) [][][2]float64 {
	t.Helper()
	var paths [][][2]float64
	for _, m := range trace037.FindAllStringSubmatch(content, -1) {
		var path [][2]float64
		for _, v := range vertex037.FindAllStringSubmatch(m[1], -1) {
			x, _ := strconv.ParseFloat(v[1], 64)
			y, _ := strconv.ParseFloat(v[2], 64)
			path = append(path, [2]float64{x, y})
		}
		paths = append(paths, path)
	}
	return paths
}

// UX-S19: the measured power is drawn step-after. Each reading holds horizontally
// until the next reading, the change is vertical, and a failed read ends the hold.
func Test037StepCurveHoldsReadingsUntilNextSample(t *testing.T) {
	r := sampleReport(t)
	v := []float64{11000, 4100, 3900, 500, 9000}
	at := r.Started
	r.Samples = []inspection.Sample{
		{At: at, Watts: &v[0], Step: "Ausgangsmessung MPC"},
		{At: at.Add(4 * time.Second), Watts: &v[1], Step: "LPC gemessene Wirkung"},
		{At: at.Add(8 * time.Second), Watts: &v[2], Step: "LPC gemessene Wirkung"},
		{At: at.Add(10 * time.Second), Step: "Messausfall"},
		{At: at.Add(14 * time.Second), Watts: &v[3], Step: "LPC Failsafe gemessene Wirkung"},
		{At: at.Add(18 * time.Second), Watts: &v[4], Step: "LPC Wiederkehr"},
	}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	paths := tracePaths037(t, got)
	if len(paths) != 2 {
		t.Fatalf("trace must break only at the failed read: %d subpaths", len(paths))
	}
	// m, then per following reading a horizontal hold and a vertical jump; the
	// first subpath additionally holds its last value until the failed read.
	if len(paths[0]) != 6 || len(paths[1]) != 3 {
		t.Fatalf("unexpected step vertices: %v", paths)
	}
	for _, path := range paths {
		for i := 1; i < len(path); i++ {
			a, b := path[i-1], path[i]
			if math.Abs(a[0]-b[0]) > .011 && math.Abs(a[1]-b[1]) > .011 {
				t.Fatalf("diagonal segment %v -> %v", a, b)
			}
		}
		for i := 1; i+1 < len(path); i += 2 {
			if math.Abs(path[i][1]-path[i-1][1]) > .011 || math.Abs(path[i+1][0]-path[i][0]) > .011 {
				t.Fatalf("expected hold then jump at %d: %v", i, path)
			}
		}
	}
	// Hold ends exactly at the failed read's timestamp (the red marker).
	if gaps := gapXs037(got); len(gaps) != 1 || math.Abs(paths[0][5][0]-gaps[0]) > .011 {
		t.Fatalf("hold does not end at the failed read marker: %v %v", paths[0], gaps)
	}
	if strings.Contains(got, "linear verbunden") || strings.Contains(got, "jeder Messwert gilt") {
		t.Fatal("no interpolation or step legend text (038)")
	}
}
