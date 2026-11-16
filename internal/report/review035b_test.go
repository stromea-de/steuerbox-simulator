package report

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func Test035bReadablePresentation(t *testing.T) {
	for _, v := range []time.Duration{0, time.Second, 500 * time.Millisecond} {
		if got := humanDuration(v); strings.Contains(got, "Std.") {
			t.Errorf("short duration %s: %s", v, got)
		}
	}
	for _, key := range []string{"Gerätekennung (SKI)", "Simulator-Kennung (SKI)", "Seriennummer"} {
		lines := identificationLabelLines(key)
		for _, line := range lines {
			if textWidth(line, 7, "F1") > 66 {
				t.Errorf("label overflows: %q", line)
			}
		}
		if key == "Gerätekennung (SKI)" && lines[0] != "Gerätekennung" {
			t.Fatalf("midword split: %q", lines)
		}
	}
	r := sampleReport(t)
	r.Mode = inspection.ModeSimulator
	r.Config.TestProfile = "quick"
	if s := r.ProfileSummary(); !strings.Contains(s, "simuliert") {
		t.Errorf("practice duration claimed as real: %s", s)
	}
	for _, raw := range []string{"TCP-Verbindung: 0 ms · complete", "nicht geprüft (Langtest): Wiederkehr nach Mindestdauer nicht abgewartet"} {
		v := inspection.UserDetail(raw)
		for _, bad := range []string{"complete", "Langtest", "Wiederkehr"} {
			if strings.Contains(v, bad) {
				t.Errorf("raw diagnostic in displayed detail: %s", v)
			}
		}
	}
}

func Test035bCloseFeedInLegendDoesNotOverlap(t *testing.T) {
	r := sampleReport(t)
	r.Config.LPC = 4200
	r.Config.LPP = 1000
	r.Config.Failsafe = 500
	r.Config.Scope = inspection.ScopeBoth
	v := -3400.0
	r.Samples = []inspection.Sample{{At: r.Started, Watts: &v, Step: "LPP Ausgangsmessung MPC"}, {At: r.Started.Add(time.Second), Watts: &v, Step: "LPP gemessene Wirkung"}}
	data, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`([0-9.]+) Tm \(Einspeisung (?:1\.000|500) W[^)]*\) Tj`).FindAllSubmatch(data, -1)
	if len(matches) != 2 {
		t.Fatalf("expected two feed-in reference labels, got %d", len(matches))
	}
	first, _ := strconv.ParseFloat(string(matches[0][1]), 64)
	second, _ := strconv.ParseFloat(string(matches[1][1]), 64)
	if math.Abs(first-second) < 13.9 {
		t.Fatalf("legend labels overlap: %v / %v", first, second)
	}
}
