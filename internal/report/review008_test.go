package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// Synthetic documents exercise layout only; no device or network is contacted.
func review008Report(t *testing.T, onsite bool) inspection.Report {
	r := sampleReport(t)
	r.Mode = inspection.ModeSimulator
	r.Steps[4].Watts = ptr(11000)
	r.Steps[4].Detail = "MPC 11000.0 W; ausreichender Bezug für LPC; Toleranz 50.0 W"
	r.Steps[6].Detail = "MPC 4200.0 W; LPC Soll; Toleranz 50.0 W"
	r.Steps[8].Detail = "MPC 500.0 W; Failsafe Soll, positiver Bezug; Toleranz 50.0 W"
	r.Steps[9].Detail = "MPC 4200.0 W; LPC nach Wiederkehr; Toleranz 50.0 W"
	r.Steps[11].Detail = "MPC -3000.0 W; LPP Soll, negative Einspeisung; Toleranz 50.0 W"
	r.Steps[13].Detail = "MPC -500.0 W; Failsafe Soll, negative Einspeisung; Toleranz 50.0 W"
	r.Steps[14].Detail = "MPC -3000.0 W; LPP nach Wiederkehr; Toleranz 50.0 W"
	r.Steps[5].Detail = "SPINE result 0, Nachricht 38; Soll 4200 W"
	r.Steps[10].Detail = "SPINE result 0, Nachricht 71; Soll 3000 W"
	r.Steps[15].Watts = ptr(-8000)
	r.Steps[15].Detail = "MPC -8000.0 W; Einspeisung nach Freigabe; Toleranz 50.0 W"
	if onsite {
		r.Mode = inspection.ModeOnSite
		r.Config = inspection.DefaultDeviceConfig()
		r.Config.SignOperator, r.Config.SignInspector = true, true
		r.Profile = "DARSTELLUNGSMUSTER mit erfundenen Daten; kein Gerätetest. Failsafe-Mindestdauer 2 h; Prüfzeiten siehe Schritte."
		r.Device.Name = "DARSTELLUNGSMUSTER – kein Gerätetest"
		r.Device.Brand, r.Device.Model = "Beispielhersteller", "Beispielmodell"
		r.Device.Serial, r.Device.Firmware = "MUSTER-008", "Muster-Version"
		r.Device.Source = "Erfundene Daten für Layoutprüfung"
		r.LocalSKI = "abcdef0123456789abcdef0123456789abcdef0123"
		r.Passed = false
		// Onsite release is evidenced by acknowledgement, not by forcing feed-in.
		r.Steps[15].Watts = nil
		r.Steps[15].Detail = "Alle unterstützten Grenzen per SPINE-Quittung aufgehoben"
		for i := range r.Steps {
			if strings.HasPrefix(r.Steps[i].Name, "LPP") {
				r.Steps[i].Status, r.Steps[i].Passed = inspection.NotChecked, false
				r.Steps[i].Detail, r.Steps[i].Watts = "LPP im Darstellungsmuster nicht gemeldet", nil
			}
		}
		s := r.Steps[9]
		s.Name = "LPC Wiederkehr gemessene Wirkung"
		r.Steps = append(r.Steps, s)
	}
	return r
}

func TestReview008PrintLayout(t *testing.T) {
	for _, onsite := range []bool{false, true} {
		r := review008Report(t, onsite)
		original, _ := json.Marshal(r)
		b, err := PDF(r)
		if err != nil {
			t.Fatal(err)
		}
		pages := reportPages(t, b)
		expectedPages := 4
		if onsite {
			expectedPages = 6
		} // Grouped evidence and the empty measurement explanation add appendix space.
		if len(pages) != expectedPages {
			t.Errorf("onsite=%v: want %d pages, got %d", onsite, expectedPages, len(pages))
		}
		all := strings.Join(pages, " ")
		if strings.Contains(pages[0], "Einhaltung über 2 h") || strings.Contains(pages[0], "SPINE") {
			t.Error("technical details on operator page")
		}
		if !onsite && !strings.Contains(pages[0], "Übungsmodus – keine Prüfung am Gerät") {
			t.Error("missing practice warning")
		}
		if onsite && (!strings.Contains(pages[0], "Unvollständig") || !strings.Contains(all, inspectorDeclaration)) {
			t.Error("partial result or signature lost")
		}
		for _, term := range terms {
			if strings.Count(all, term.title) != 1 {
				t.Errorf("duplicated/missing term %q", term.title)
			}
		}
		if !strings.Contains(all, "Messwert") || strings.Contains(all, "Ist (W)") || strings.Contains(all, "SPINE result") {
			t.Error("obsolete table text")
		}
		for i, page := range pages {
			for _, want := range []string{"Prüfbericht · Kommunikationsprüfung", "Erstellt mit dem Stromea Steuerbox-Simulator · stromea.de/simulator"} {
				if !strings.Contains(page, want) {
					t.Errorf("page %d missing %q", i+1, want)
				}
			}
		}
		// Concept 024c uses pale section bands and dark neutral text.
		textColors := regexp.MustCompile(`Tf ([0-9.]+) ([0-9.]+) ([0-9.]+) rg`).FindAllSubmatch(b, -1)
		for _, c := range textColors {
			if !bytes.Equal(c[1], c[2]) || !bytes.Equal(c[2], c[3]) {
				t.Error("colored text is not print neutral")
				break
			}
		}
		if strings.Count(string(b), "/Subtype /Link") != len(pages) || strings.Count(string(b), "/Annots [") != len(pages) || strings.Count(string(b), "/S /URI /URI (https://stromea.de/simulator)") != len(pages) {
			t.Error("missing clickable footer on each page")
		}
		after, _ := json.Marshal(r)
		if !bytes.Equal(original, after) {
			t.Fatal("PDF changed source evidence")
		}
		if os.Getenv("STROMEA_008_PDF_ARTIFACTS") == "1" {
			name := "008-uebung"
			if onsite {
				name = "008-vor-ort-teilweise-unterschriften"
			}
			if err := os.MkdirAll("../../artifacts", 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join("../../artifacts", name+".pdf"), b, 0644); err != nil {
				t.Fatal(err)
			}
			data, _ := json.MarshalIndent(r, "", "  ")
			if err := os.WriteFile(filepath.Join("../../artifacts", name+".json"), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestReview008EvidencePresentation(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"SPINE result 0, Nachricht 38; Soll 4200 W", "Steuerbefehl bestätigt (SPINE-Code 0, Nachricht 38); Soll 4.200 W"},
		{"Fehler: SPINE result 7, Nachricht 39; Zeitlimit", "Fehler: Quittung (Code 7, Nachricht 39); Zeitlimit"},
		{"MPC -3000.0 W; LPP Soll, negative Einspeisung; Toleranz 50.0 W", "Leistung 3.000 W (Einspeisung); LPP Soll, Einspeisung; Toleranz 50 W"},
		{"MPC 9007199254740993.25 W", "Leistung 9.007.199.254.740.993,25 W"},
		{"Kennung ABCMPC -3000.0 WXYZ; ABCSPINE result 0, Nachricht 38", "Kennung ABCMPC -3000.0 WXYZ; ABCSPINE result 0, Nachricht 38"},
		{"Freigabe nicht bestätigt; Grenzen können am Gerät weiter gelten", "Freigabe nicht bestätigt; Grenzen können am Gerät weiter gelten"},
	} {
		if got := stepDetail(inspection.Step{Detail: tc.input}); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestReview008FailureAndLegacyMode(t *testing.T) {
	r := sampleReport(t) // Empty legacy Mode must still be visibly practice mode.
	r.Steps[6].Passed = false
	r.Steps[6].Detail = "Freigabe nicht bestätigt; Grenzen können am Gerät weiter gelten"
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	first := reportPages(t, b)[0]
	if !strings.Contains(first, "Übungsmodus – keine Prüfung am Gerät") || !strings.Contains(first, "Nicht bestanden") {
		t.Fatal("failure/legacy mode lost")
	}
	if !strings.Contains(strings.Join(reportPages(t, b), " "), "Nächste Schritte") {
		t.Fatal("missing explanation without signatures")
	}
	if !strings.Contains(strings.Join(reportPages(t, b), " "), r.Steps[6].Reason()) {
		t.Fatal("failure evidence lost")
	}
	r.Steps[6].Passed, r.Steps[6].Status = true, inspection.NotChecked
	b, err = PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if r.CheckCounts().Passed != 8 || r.CheckCounts().NotChecked != 1 || !strings.Contains(reportPages(t, b)[0], "Nicht geprüft") {
		t.Fatal("unchecked step counted as passed")
	}
}

func TestReview008HeaderAndLinkReferences(t *testing.T) {
	r := review008Report(t, true)
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	streams := regexp.MustCompile(`(?s)stream\n(.*?)endstream`).FindAllSubmatch(b, -1)
	positionedText := regexp.MustCompile(`1 0 0 1 [0-9.]+ ([0-9.]+) Tm \((.*)\) Tj`)
	for _, stream := range streams {
		for _, match := range positionedText.FindAllSubmatch(stream[1], -1) {
			y, _ := strconv.ParseFloat(string(match[1]), 64)
			if y > pageHeight-95 && (bytes.Contains(match[2], []byte("Simulator")) || bytes.Contains(match[2], []byte("Software-Simulation")) || bytes.Contains(match[2], []byte("LPC"))) {
				t.Error("product/developer branding in real report header")
			}
		}
	}
	refs := regexp.MustCompile(`/Annots \[(\d+) 0 R\]`).FindAllSubmatch(b, -1)
	for _, ref := range refs {
		prefix := string(ref[1]) + " 0 obj\n<< /Type /Annot /Subtype /Link"
		if !strings.Contains(string(b), prefix) {
			t.Error("page references missing link object")
		}
	}
	if got := target(inspection.Step{Name: "Grenzen aufheben"}, r.Config, r.Mode); got != "Freigabe quittieren" {
		t.Fatal("onsite release must not imply measured feed-in", got)
	}
}

func TestReview008NoticeStaysWithGlossary(t *testing.T) {
	// Vary the table length around page boundaries, including long reports.
	for _, count := range []int{1, 8, 14, 18, 23, 30, 45} {
		r := sampleReport(t)
		base := r.Steps[6]
		r.Steps = nil
		for i := 0; i < count; i++ {
			r.Steps = append(r.Steps, base)
		}
		b, err := PDF(r)
		if err != nil {
			t.Fatal(err)
		}
		pages := reportPages(t, b)
		last := pages[len(pages)-1]
		if !strings.Contains(last, "SPINE & SHIP") || !strings.Contains(strings.Join(pages, " "), inspection.Notice) {
			t.Errorf("count=%d: orphaned closing notice", count)
		}
	}
}

func TestReview008LongCustomNoticeCanUseOwnPage(t *testing.T) {
	r := sampleReport(t)
	for float64(len(noticeLines(r)))*12+20 < bodyBottom-bodyTop-45 {
		r.Notice += "Zusatzhinweis. "
	}
	b, err := PDF(r)
	if err != nil {
		t.Fatal("individually fitting notice rejected by keep-with rule", err)
	}
	pages := reportPages(t, b)
	if !strings.Contains(strings.Join(pages, " "), strings.TrimSpace(r.Notice)) {
		t.Fatal("custom notice truncated")
	}
}
