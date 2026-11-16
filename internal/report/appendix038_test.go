package report

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func appendix038(t *testing.T) (inspection.Report, string) {
	t.Helper()
	r := sampleReport(t)
	r.Config.SignOperator, r.Config.SignInspector = true, true
	r.Capabilities = &inspection.Capabilities{LPC: true, LPP: true, MPC: true, Scenarios: map[string][]uint{"LPC": {1, 2, 3}, "MPC": {1}}}
	r.Events = []inspection.Step{{Name: "TCP-Verbindung", Detail: "0 ms · complete", At: r.Started, Passed: true}}
	v := 8000.0
	r.Samples = []inspection.Sample{{At: r.Started, Watts: &v, Step: "Ausgangsmessung MPC"}, {At: r.Started.Add(time.Second), Watts: &v, Step: "LPC gemessene Wirkung"}}
	data, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	return r, string(data)
}

func textAt038(content, text string) (x, y, size float64, color string, ok bool) {
	pattern := regexp.QuoteMeta(literal(text))
	if strings.HasSuffix(text, ")") {
		pattern = regexp.QuoteMeta(literal(strings.TrimSuffix(text, ")"))) + `\) Tj`
	}
	m := regexp.MustCompile(`BT /F\d ([0-9.]+) Tf ([0-9. ]+) rg 1 0 0 1 ([0-9.]+) ([0-9.]+) Tm \(` + pattern).FindStringSubmatch(content)
	if m == nil {
		return 0, 0, 0, "", false
	}
	size, _ = strconv.ParseFloat(m[1], 64)
	x, _ = strconv.ParseFloat(m[3], 64)
	y, _ = strconv.ParseFloat(m[4], 64)
	return x, y, size, m[2], true
}

// 038 (UX-S24) items 7, 9-14.
func Test038PDFAppendixLayout(t *testing.T) {
	r, got := appendix038(t)
	// 7: "Verbindung" left of the bar; its legend starts where the bar starts.
	if x, _, size, _, ok := textAt038(got, "Verbindung)"); !ok || x+textWidth("Verbindung", size, "F1") > margin+35-1.9 || x < margin-5 {
		t.Errorf("connection label must end left of the bar start %v: x=%v size=%v", margin+35, x, size)
	}
	if x, _, _, _, ok := textAt038(got, "Verbindungsleiste:"); !ok || x != margin+35 {
		t.Errorf("connection legend at x=%v, want bar start %v", x, margin+35)
	}
	// 9: one line per function, clear wording.
	if strings.Contains(got, ": gemeldet") || !strings.Contains(got, literal("Bezug begrenzen (LPC): vom Gerät angeboten")) || !strings.Contains(got, "("+literal("Leistung messen (MPC): vom Gerät angeboten")) {
		t.Error("functions: one line each, \"vom Gerät angeboten\"")
	}
	// 10: Beginn/Ende/Funktionen in their own section "Prüfung", after "Firma & Prüfer".
	firm, test, begin := strings.Index(got, "("+literal("Firma & Prüfer")+")"), strings.Index(got, "("+literal("Prüfung")+")"), strings.Index(got, "(Beginn)")
	if firm < 0 || test < 0 || begin < test || begin < firm {
		t.Errorf("section order firm=%d test=%d begin=%d", firm, test, begin)
	}
	// 11: connection events like step details (8 pt, muted).
	if _, _, size, color, ok := textAt038(got, r.Started.Format("02.01.2006 15:04:05 -07:00")+" · TCP-Verbindung"); !ok || size != 8 || color != rgb(muted) {
		t.Errorf("event line %v %q, want 8 pt muted", size, color)
	}
	// 12: no disclaimer box; the sentence stays as plain text (ADR 012).
	// 038c: new wording (PO 11.11.2026), very small like the footer, no box.
	if _, _, size, color, ok := textAt038(got, "Dokumentierter Funktionstest. Ersetzt keine Konformitätsprüfung."); !ok || size != 6.5 || color != rgb(muted) {
		t.Errorf("notice as tiny muted line: %v %q", size, color)
	}
	if strings.Contains(got, "Keine Zertifizierung") {
		t.Error("old notice wording remains")
	}
	// 13: footer lines with more air around the checksum.
	_, created, _, _, ok1 := textAt038(got, "Erstellt mit dem Stromea")
	_, digest, _, _, ok2 := textAt038(got, "Prüfsumme der Protokolldaten")
	_, page, _, _, ok3 := textAt038(got, "Seite 1 von")
	if !ok1 || !ok2 || !ok3 || created-digest < 12 || digest-page < 12 {
		t.Errorf("footer spacing created=%v digest=%v page=%v", created, digest, page)
	}
	// 14: signatures under a section heading like "Dokumentierter Prüfumfang".
	if !regexp.MustCompile(`BT /F2 10\.00 Tf [0-9. ]+ rg 1 0 0 1 [0-9.]+ [0-9.]+ Tm \(Unterschriften\) Tj`).MatchString(got) {
		t.Error("signature section heading missing")
	}
}

// 038c: stored older reports carry the previous wording; the PDF shows the
// current notice once, never both.
func Test038cLegacyNoticeNotRepeated(t *testing.T) {
	r := sampleReport(t)
	r.Notice = "Keine Zertifizierung. Dieses Protokoll ersetzt keine Konformitätsprüfung."
	// Only the drawn page content; the embedded original JSON keeps its wording.
	d := &document{}
	d.newPage()
	d.coverage(r)
	got := d.current().String()
	if strings.Contains(got, "Keine Zertifizierung") || strings.Count(got, literal("Dokumentierter Funktionstest.")) != 1 {
		t.Fatal("legacy notice must be replaced by the current one, shown once")
	}
}
