package report

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// onsite042 is an invented consumption-only device report (heat pump); no device, no network.
func onsite042(t *testing.T, restored bool) inspection.Report {
	t.Helper()
	r := sampleReport(t)
	r.Mode = inspection.ModeOnSite
	r.Config = inspection.DefaultDeviceConfig()
	r.Config.Scope = inspection.ScopeConsumption
	r.Config.NormalizeProfile()
	r.Device = inspection.Device{Name: "Beispiel-Wärmepumpe (fiktiv)", Type: "HeatPump"}
	r.FailsafeOriginals = map[string]inspection.FailsafeValues{"LPC": {Power: 4200, Minimum: 2 * time.Hour}}
	var steps []inspection.Step
	for _, s := range r.Steps {
		if !strings.HasPrefix(s.Name, "LPP") {
			steps = append(steps, s)
		}
	}
	at := r.Started.Add(7 * time.Second)
	steps = append(steps[:len(steps)-1], inspection.Step{Name: "LPC Failsafe-Leistung zurückstellen", Passed: true, At: at}, inspection.Step{Name: "LPC Failsafe-Mindestdauer zurückstellen", Passed: restored, At: at}, steps[len(steps)-1])
	r.Steps = steps
	return r
}

func TestPDFTitleNamesScope042(t *testing.T) {
	r := onsite042(t, true)
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	pages := pages042(t, b)
	if !strings.Contains(pages[0], "LPC-Prüfung am Gerät") {
		t.Fatalf("page 1 title: %s", pages[0][:200])
	}
	if strings.Contains(pages[0], "LPC/LPP-Prüfung") || strings.Contains(pages[0], "LPP-Prüfung") {
		t.Fatal("consumption-only report names LPP")
	}
	if !strings.Contains(string(b), "Prüfbericht · LPC-Prüfung am Gerät") {
		t.Fatal("document title (XMP) differs from the page title")
	}
	r.Mode = inspection.ModeSimulator
	r.Config.Scope = inspection.ScopeBoth
	b, err = PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if first := pages042(t, b)[0]; !strings.Contains(first, "LPC/LPP-Prüfung im Simulator") || !strings.Contains(first, "Übungsmodus – keine Prüfung am Gerät") {
		t.Fatalf("simulator title: %s", first[:200])
	}
}

// 042 (Nutzer 2026-11-14): the failsafe row explains in one sentence why it matters; a missing
// confirmation is a clear warning with an action and the values to restore.
func TestPDFFailsafeRestoreExplained042(t *testing.T) {
	b, err := PDF(onsite042(t, true))
	if err != nil {
		t.Fatal(err)
	}
	first := pages042(t, b)[0]
	for _, want := range []string{
		"Notbetriebswerte des Geräts",
		"Vor der Prüfung gelesen (Bezug 4.200 W, Mindestdauer 2 Std.), danach wiederhergestellt und vom Gerät bestätigt. Bei Kommunikationsausfall verhält sich das Gerät wieder wie vor der Prüfung.",
		"Notbetriebswerte legen fest, welche Leistung das Gerät bei Kommunikationsausfall höchstens beziehen darf und wie lange mindestens; die Prüfung ändert sie vorübergehend.",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("page 1 missing %q: %s", want, first)
		}
	}
	for _, old := range []string{"zurückgestellt", "ausgelesen und geprüft", "Achtung"} {
		if strings.Contains(first, old) {
			t.Errorf("page 1 still contains %q", old)
		}
	}
	b, err = PDF(onsite042(t, false))
	if err != nil {
		t.Fatal(err)
	}
	first = pages042(t, b)[0]
	for _, want := range []string{
		"Vor der Prüfung gelesen (Bezug 4.200 W, Mindestdauer 2 Std.). Wiederherstellung nicht bestätigt.",
		"Achtung: Notbetriebswerte am Gerät prüfen und wieder auf Bezug 4.200 W, Mindestdauer 2 Std. einstellen. Sonst gilt bei Kommunikationsausfall womöglich noch der Prüfwert 500 W.",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("unconfirmed page 1 missing %q: %s", want, first)
		}
	}
	// R042-01: cancelled before any fallback value was written: no warning, no action.
	early := onsite042(t, true)
	early.Steps = early.Steps[:2]
	early.Passed = false
	b, err = PDF(early)
	if err != nil {
		t.Fatal(err)
	}
	first = pages042(t, b)[0]
	if !strings.Contains(first, "Nicht verändert: Die Prüfung hat keine Notbetriebswerte am Gerät geschrieben.") || strings.Contains(first, "Achtung") || strings.Contains(first, "Notbetriebswerte am Gerät prüfen") {
		t.Fatalf("early abort failsafe row: %s", first)
	}
	r := sampleReport(t)
	r.Config.Scope = inspection.ScopeBoth
	b, err = PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	first = pages042(t, b)[0]
	if !strings.Contains(first, "Übung: nur simuliert, an keinem Gerät verändert.") || !strings.Contains(first, "höchstens beziehen oder einspeisen darf") || strings.Contains(first, "Achtung") {
		t.Fatalf("practice failsafe row: %s", first)
	}
}

// pages042 uses pdftotext when installed; otherwise it decodes the uncompressed text operators of
// each content stream (this PDF writer emits WinAnsi literals), so the test never silently skips.
func pages042(t *testing.T, b []byte) []string {
	t.Helper()
	if _, err := exec.LookPath("pdftotext"); err == nil {
		return reportPages(t, b)
	}
	win := map[byte]rune{128: '€', 145: '‘', 146: '’', 147: '“', 148: '”', 149: '•', 150: '–', 151: '—', 133: '…'}
	var pages []string
	for _, chunk := range strings.Split(string(b), "endstream") {
		if !strings.Contains(chunk, ") Tj") {
			continue
		}
		var words []string
		for _, m := range tj042.FindAllStringSubmatch(chunk, -1) {
			var s []rune
			raw := m[1]
			for i := 0; i < len(raw); i++ {
				c := raw[i]
				if c == '\\' && i+1 < len(raw) {
					i++
					c = raw[i]
					if c >= '0' && c <= '7' {
						n := 0
						for j := 0; j < 3 && i < len(raw) && raw[i] >= '0' && raw[i] <= '7'; j++ {
							n = n*8 + int(raw[i]-'0')
							i++
						}
						i--
						c = byte(n)
					}
				}
				if r, ok := win[c]; ok {
					s = append(s, r)
				} else {
					s = append(s, rune(c))
				}
			}
			words = append(words, string(s))
		}
		pages = append(pages, strings.Join(strings.Fields(strings.Join(words, " ")), " "))
	}
	return pages
}

var tj042 = regexp.MustCompile(`\(((?:\\.|[^\\)])*)\) Tj`)
