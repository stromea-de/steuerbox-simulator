package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func reportPages(t *testing.T, rBytes []byte) []string {
	t.Helper()
	text := pdfText(t, rBytes)
	pages := strings.Split(strings.TrimSuffix(text, "\f"), "\f")
	for i := range pages {
		pages[i] = strings.Join(strings.Fields(pages[i]), " ")
	}
	return pages
}

func TestPDFOperatorPageAndAppendix(t *testing.T) {
	r := sampleReport(t)
	r.Config.SignOperator, r.Config.SignInspector = true, true
	r.Steps[4].Detail = "MPC 11000.0 W; Soll 4200 W; Toleranz 50.25 W; Kennung ABC11000.0 bleibt"
	r.Steps[4].Watts = ptr(11000)
	original, _ := json.Marshal(r)
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	pages := reportPages(t, b)
	if len(pages) < 2 {
		t.Fatal("technical appendix needs a separate page")
	}
	first, appendix := pages[0], strings.Join(pages[1:], " ")
	for _, forbidden := range []string{"MPC", "SPINE", "Heartbeat", "Failsafe", "SKI", "Inbetriebnahmeprotokoll", "sicheren Zustand"} {
		if strings.Contains(first, forbidden) {
			t.Errorf("operator page contains %q", forbidden)
		}
	}
	for _, want := range []string{
		"Kommunikationsprüfung", "Bestanden", "Übungsmodus – keine Prüfung am Gerät",
		"Gerät", "24.10.2026", "4.200 W", "3.000 W",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("operator page missing %q: %s", want, first)
		}
	}
	for _, want := range []string{"Technischer Anhang", "Prüfbedingungen", "Prüfschritte", "Glossar", "Limitation of Power Consumption", "Limitation of Power Production", "Monitoring of Power Consumption", "Heartbeat · Lebenszeichen", "Failsafe · Notbetrieb", "SKI · Subject Key Identifier", "11.000 W", "50,25 W", "ABC11000.0"} {
		if !strings.Contains(appendix, want) {
			t.Errorf("appendix missing %q", want)
		}
	}
	if strings.Contains(appendix, "11000.0 W") {
		t.Error("detail numbers not localized")
	}
	after, _ := json.Marshal(r)
	if !bytes.Equal(original, after) {
		t.Fatal("rendering changed original JSON evidence")
	}
}

func TestPDFOperatorCoverage(t *testing.T) {
	for _, variant := range []string{"empty", "partial", "failure", "duplicate-failure", "unknown-profile"} {
		t.Run(variant, func(t *testing.T) {
			r := sampleReport(t)
			r.Config.SignOperator, r.Config.SignInspector = true, true
			switch variant {
			case "empty":
				r.Steps = nil
			case "partial":
				r.Steps = r.Steps[:6]
			case "failure":
				r.Steps[6].Passed = false
			case "duplicate-failure":
				s := r.Steps[6]
				s.Passed = false
				r.Steps = append(r.Steps, s)
			case "unknown-profile":
				r.Profile = "Anderes Profil, keine Zeitangaben"
			}
			b, err := PDF(r)
			if err != nil {
				t.Fatal(err)
			}
			first := reportPages(t, b)[0]
			want := "Unvollständig"
			if variant == "failure" || variant == "duplicate-failure" {
				want = "Nicht bestanden"
			}
			if variant == "unknown-profile" {
				want = "Bestanden"
			}
			if !strings.Contains(first, want) {
				t.Errorf("missing outcome %q", want)
			}
			if variant == "empty" || variant == "partial" {
				if !strings.Contains(first, "Nicht geprüft") {
					t.Error("missing coverage status")
				}
			}
			if variant == "unknown-profile" && strings.Contains(first, "2 h") {
				t.Error("invented duration for unknown profile")
			}
		})
	}
}

func TestPDFOperatorLongValuesRemainOnOnePage(t *testing.T) {
	r := sampleReport(t)
	r.Config.SignOperator, r.Config.SignInspector = true, true
	r.Device.Name = strings.Repeat("Sehr langer Gerätename ", 8) + "ENDEGERAET"
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	pages := reportPages(t, b)
	if !strings.Contains(strings.Join(pages, " "), "Bestätigung der Fachkraft") || !strings.Contains(strings.Join(pages[1:], " "), "Technischer Anhang") {
		t.Fatal("operator page spilled into appendix")
	}
	if !strings.Contains(strings.Join(pages[1:], " "), "ENDEGERAET") {
		t.Error("full device name lost from appendix")
	}
}

func TestDetailNumbersPreserveEvidence(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"MPC 11000.0 W; Soll -3000 W; Toleranz 50.25 W", "MPC 11.000 W; Soll -3.000 W; Toleranz 50,25 W"},
		{"Soll 4.200 W; Ist 4.200,5 W; 12:34:56; Kennung ABC11000.0 W", "Soll 4.200 W; Ist 4.200,5 W; 12:34:56; Kennung ABC11000.0 W"},
		{"Wert 9007199254740993.25 W", "Wert 9.007.199.254.740.993,25 W"},
		{"MPC -0.5000 W; Messung (4200.25 W)", "MPC -0,5 W; Messung (4.200,25 W)"},
	} {
		if got := detailNumbers(tc.input); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPDFOperatorExactDeclarationsAndMaximumValues(t *testing.T) {
	r := sampleReport(t)
	r.Config.SignOperator, r.Config.SignInspector = true, true
	r.Device.Name = strings.Repeat("Gerätebezeichnung ", 12)
	r.Config.LPC, r.Config.LPP = 9999999.1234567, 8888888.1234567
	r.Config.Failsafe, r.Config.Tolerance = 1111111.1234567, 2222222.1234567
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.Join(reportPages(t, b), " ")
	data, _ := json.Marshal(r)
	hash := sha256.Sum256(data)
	for _, want := range []string{fmt.Sprintf(operatorDeclaration, fmt.Sprintf("%x", hash[:6])), inspectorDeclaration} {
		if !strings.Contains(first, want) {
			t.Error("declaration missing or split onto another page")
		}
	}
}
