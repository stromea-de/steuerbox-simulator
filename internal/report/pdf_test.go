package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func sampleReport(t *testing.T) inspection.Report {
	t.Helper()
	at := time.Date(2026, 10, 24, 10, 0, 0, 0, time.FixedZone("CEST", 7200))
	r := inspection.Report{Business: "Elektro Muster GmbH", Inspector: "Erika Prüfer", Started: at, Finished: at.Add(8 * time.Second), Device: inspection.Device{Name: "Stromea EEBUS Loopback-Simulator", Brand: "Stromea", Model: "Simulator CS/MU", SKI: "0123456789abcdef0123456789abcdef01234567"}, Config: inspection.DefaultConfig(), Profile: inspection.Profile, Notice: inspection.Notice, Passed: true}
	names := []string{"Verbindung", "Unterstützte Use Cases", "Failsafe-Werte setzen", "Heartbeat starten", "Ausgangsmessung MPC", "LPC Grenze + Quittung", "LPC gemessene Wirkung", "LPC Heartbeat-Ausfall", "LPC Failsafe gemessene Wirkung", "LPC Wiederkehr", "LPP Grenze + Quittung", "LPP gemessene Wirkung", "LPP Heartbeat-Ausfall", "LPP Failsafe gemessene Wirkung", "LPP Wiederkehr", "Grenzen aufheben"}
	for i, name := range names {
		s := inspection.Step{Name: name, Passed: true, At: at.Add(time.Duration(i) * 500 * time.Millisecond), Detail: "Bestätigt"}
		if strings.Contains(name, "Wirkung") || strings.Contains(name, "Wiederkehr") {
			v := 4200.0
			if strings.HasPrefix(name, "LPP") {
				v = -3000
			}
			if strings.Contains(name, "Failsafe") {
				v = 500
				if strings.HasPrefix(name, "LPP") {
					v = -500
				}
			}
			s.Watts = &v
			s.Detail = fmt.Sprintf("MPC %.1f W; Soll bestätigt; Toleranz 50.0 W", v)
		}
		r.Steps = append(r.Steps, s)
	}
	return r
}

func pdfText(t *testing.T, b []byte) string {
	t.Helper()
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext nicht installiert")
	}
	file := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(file, b, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("pdftotext", "-enc", "UTF-8", file, "-").CombinedOutput()
	if err != nil {
		t.Fatalf("pdftotext: %v: %s", err, out)
	}
	return string(out)
}

func TestPDFIsCompleteAndHasValidXref(t *testing.T) {
	r := sampleReport(t)
	r.Business = "Müller (Elektro) \\ GmbH"
	r.Inspector = "Groß €"
	r.Passed = false
	base := r.Steps[0]
	r.Steps = nil
	for i := 0; i < 120; i++ {
		s := base
		s.Name = fmt.Sprintf("Schritt %d", i+1)
		s.Detail = "Messung bestätigt"
		s.Passed = i != 3
		r.Steps = append(r.Steps, s)
	}
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-2.0")) || !bytes.Contains(b, []byte("keine Konformit")) || !bytes.Contains(b, []byte("Nicht bestanden")) || !bytes.HasSuffix(b, []byte("%%EOF\n")) {
		t.Fatal("incomplete PDF")
	}
	if !bytes.Contains(b, []byte(`M\374ller \(Elektro\) \\ GmbH`)) || !bytes.Contains(b, []byte(`Gro\337 \200`)) {
		t.Fatal("WinAnsi/escaping failed")
	}
	re := regexp.MustCompile(`(?m)^(\d{10}) 00000 n `)
	entries := re.FindAllSubmatch(b, -1)
	if len(entries) == 0 {
		t.Fatal("xref missing")
	}
	for i, m := range entries {
		offset, _ := strconv.Atoi(string(m[1]))
		if offset >= len(b) || !bytes.HasPrefix(b[offset:], []byte(fmt.Sprintf("%d 0 obj", i+1))) {
			t.Fatalf("xref %d", i)
		}
	}
	pages := bytes.Count(b, []byte("/Type /Page "))
	if pages < 3 {
		t.Fatal("pagination missing")
	}
	streams := regexp.MustCompile(`(?s)stream\n(.*?)endstream`).FindAllSubmatch(b, -1)
	for i, stream := range streams[:pages] {
		if !bytes.Contains(stream[1], []byte(`(Stromea \267 Pr\374fbericht)`)) || !bytes.Contains(stream[1], []byte(fmt.Sprintf("Seite %d von %d", i+1, pages))) {
			t.Fatalf("page %d: header/footer missing", i+1)
		}
		if bytes.Contains(stream[1], []byte("Schritt ")) && !bytes.Contains(stream[1], []byte(`(Pr\374fschritt)`)) {
			t.Fatalf("page %d: table header missing", i+1)
		}
	}
	// Workflow steps appear once in the informative technical table.
	for i := 1; i <= 120; i++ {
		if bytes.Count(b, []byte(fmt.Sprintf("(Schritt %d)", i))) != 1 {
			t.Fatalf("step %d missing/duplicated", i)
		}
	}
}

func TestPDFDeterministicAndIdentity(t *testing.T) {
	r := sampleReport(t)
	a, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := PDF(r)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("same report must produce identical bytes")
	}
	r.Business += " Neu"
	c, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	id := regexp.MustCompile(`Protokoll-Nr. ([0-9a-f]{12})`)
	ai, ci := id.FindSubmatch(a), id.FindSubmatch(c)
	if len(ai) == 0 || len(ci) == 0 || bytes.Equal(ai[1], ci[1]) {
		t.Fatal("report identity missing or not content-derived")
	}
}

func TestPDFExtractedTextAndOrder(t *testing.T) {
	r := sampleReport(t)
	r.Business = "Müller & Söhne (Elektro) \\ Groß GmbH €"
	r.Steps[6].Watts = ptr(4200.5)
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	text := pdfText(t, b)
	normalized := strings.Join(strings.Fields(text), " ")
	for _, want := range []string{"Müller & Söhne (Elektro) \\ Groß GmbH €", "4.200 W", "4.200,5 W", "3.000 W", "Heartbeat", inspection.Notice, "Messwerte simuliert", "Prüfer/in", "Software " + BuildVersion()} {
		if !strings.Contains(normalized, want) {
			t.Errorf("missing extracted text %q in:\n%s", want, text)
		}
	}
	pos := 0
	for _, want := range []string{"Kommunikationsprüfung", "Bestanden", "Technischer Anhang", "Betrieb & Prüfer", "Prüfbedingungen", "Prüfschritte", "Datenverbindung prüfen", "Testgrenzen aufheben", inspection.Notice, "Glossar"} {
		i := strings.Index(normalized[pos:], want)
		if i < 0 {
			t.Fatalf("missing/out of order %q", want)
		}
		pos += i + len(want)
	}
	pages := strings.Split(strings.TrimSuffix(text, "\f"), "\f")
	for i, page := range pages {
		if strings.Contains(page, "gemessene Wirkung") && (!strings.Contains(page, "Prüfschritt") || !strings.Contains(page, "Ergebnis")) {
			t.Errorf("page %d table header not repeated", i+1)
		}
	}
}

func ptr(v float64) *float64 { return &v }

func TestPDFFailedStepOverridesInconsistentSummary(t *testing.T) {
	r := sampleReport(t)
	r.Steps[2].Passed = false
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("Nicht bestanden")) || !bytes.Contains(b, []byte("N"+literal("ächste Schritte"))) {
		t.Fatal("failed step must be visible in overall result")
	}
}

// Regenerate the four review examples without accessing any device/network.
func TestPDFReviewArtifacts(t *testing.T) {
	if os.Getenv("EEBUS_PDF_ARTIFACTS") != "1" {
		t.Skip("set EEBUS_PDF_ARTIFACTS=1 to regenerate review artifacts")
	}
	r := sampleReport(t)
	if data, err := os.ReadFile("../../artifacts/002-Protokoll.json"); err == nil {
		if err := json.Unmarshal(data, &r); err != nil {
			t.Fatal(err)
		}
	}
	output := "../../artifacts"
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"bestanden", "nicht-bestanden", "lang", "teilweise"} {
		report := r
		report.Steps = append([]inspection.Step(nil), r.Steps...)
		switch variant {
		case "nicht-bestanden":
			report.Passed = false
			report.Steps[6].Passed = false
			report.Steps[6].Detail = "Messwirkung nicht nachgewiesen: Soll 4.200 W, letzter Messwert 7.600 W; Zeitlimit überschritten."
			report.Steps[6].Watts = ptr(7600)
		case "teilweise":
			report.Steps = report.Steps[:6]
		case "lang":
			report.Business = "Müller & Söhne Elektrotechnik, Energiemanagement und Gebäudeautomation für Wohnquartiere und Gewerbeanlagen im Großraum Freiburg im Breisgau (Meisterbetrieb) GmbH & Co. KG"
			report.Device.Name += " mit besonders ausführlicher Gerätebezeichnung zur Prüfung des Zeilenumbruchs"
			report.Device.SKI = strings.Repeat("0123456789abcdef", 5)
			report.Inspector = "Dr. Erika Groß-Prüfer (Elektrofachkraft für Energiemanagement und Gebäudeautomation)"
			for len(report.Steps) < 48 {
				report.Steps = append(report.Steps, r.Steps...)
			}
			report.Steps[20].Detail = strings.Repeat("Ausführlicher Prüfhinweis: Rückmeldung und Messwirkung plausibel; ", 8)
		}
		b, err := PDF(report)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, "005-Protokoll-"+variant+".pdf"), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPDFLongValuesStayCompleteAndRowsStayTogether(t *testing.T) {
	r := sampleReport(t)
	r.Business = strings.Repeat("Gebäudeautomation Groß GmbH ", 7)
	r.Inspector = strings.Repeat("Prüfername", 15) // unbroken text must wrap too
	r.Device.SKI = strings.Repeat("0123456789abcdef", 8)
	r.Config.LPC = 4200.25
	r.Steps = nil
	for i := 0; i < 40; i++ {
		r.Steps = append(r.Steps, inspection.Step{Name: fmt.Sprintf("Messung-%02d", i), Detail: strings.Repeat("Langer Prüfhinweis mit Umlauten äöüß. ", 5) + fmt.Sprintf("Ende-%02d", i), At: r.Started, Passed: true})
	}
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	text := pdfText(t, b)
	normalized := strings.Join(strings.Fields(text), " ")
	if !strings.Contains(normalized, strings.TrimSpace(r.Business)) {
		t.Fatal("long business name incomplete")
	}
	if !strings.Contains(normalized, "4.200,25 W") {
		t.Fatal("target precision lost")
	}
	// Remove wrapping whitespace when checking identifiers.
	joined := strings.Join(strings.Fields(text), "")
	if !strings.Contains(joined, r.Inspector) || !strings.Contains(joined, r.Device.SKI) {
		t.Fatal("long identifier incomplete")
	}
	for i := 0; i < 40; i++ {
		found := false
		for _, page := range strings.Split(text, "\f") {
			if strings.Contains(page, "Nr.") && strings.Contains(page, "Ergebnis") && strings.Contains(page, fmt.Sprintf("Messung-%02d", i)) {
				found = true
				if !strings.Contains(page, fmt.Sprintf("Ende-%02d", i)) {
					t.Fatalf("row %d split over pages", i)
				}
				if !strings.Contains(page, "Prüfschritt") || !strings.Contains(page, "Ergebnis") {
					t.Fatalf("row %d table header missing", i)
				}
			}
		}
		if !found {
			t.Fatalf("row %d lost", i)
		}
	}
}

func TestPDFRejectsOversizedRowInsteadOfClipping(t *testing.T) {
	r := sampleReport(t)
	r.Steps[0].Detail = strings.Repeat("Dieser Detailtext ist länger als eine ganze Seite. ", 500)
	b, err := PDF(r)
	if err == nil || b != nil || !strings.Contains(err.Error(), "ungeteilte Zeile") {
		t.Fatalf("expected explicit oversized row error, got %v", err)
	}
}

func TestPDFLastRowAndNotesStayTogether(t *testing.T) {
	r := sampleReport(t)
	base := r.Steps[6]
	r.Steps = nil
	for i := 0; i < 47; i++ {
		s := base
		s.Name = fmt.Sprintf("Messung %d", i)
		s.Detail = strings.Repeat("Messwirkung bestätigt. ", 3)
		r.Steps = append(r.Steps, s)
	}
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range strings.Split(pdfText(t, b), "\f") {
		if strings.Contains(page, "¹ Uhrzeit") && !strings.Contains(page, "Messung 46") {
			t.Fatal("table note orphaned from last row")
		}
	}
}
