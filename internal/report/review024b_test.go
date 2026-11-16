package report

import (
	"bytes"
	"encoding/base64"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPruefformular024b(t *testing.T) {
	for _, scope := range []string{inspection.ScopeConsumption, inspection.ScopeProduction, inspection.ScopeBoth} {
		r := sampleReport(t)
		r.Config.Scope = scope
		r.Mode = inspection.ModeOnSite
		r.Config.Site = inspection.Site{Customer: "Ada Betreiber", Address: "Anlagenweg 1", Meter: "AB-123"}
		b, err := PDF(r)
		if err != nil {
			t.Fatal(err)
		}
		text := pdfText(t, b)
		for _, want := range []string{"Anlagenbetreiber", "Anlage", "Prüfer", "Zähler- oder Anlagennummer", "ergebnisrelevant", "informativ", "Vorgaben", "Glossar"} {
			if !strings.Contains(text, want) {
				t.Errorf("missing %s", want)
			}
		}
		for _, bad := range []string{"Übung: simulierte Werte", "§14a-PRÜFUNG", "Summe von", "zählen fürs Ergebnis"} {
			if strings.Contains(text, bad) {
				t.Errorf("unexpected %s", bad)
			}
		}
		if scope == inspection.ScopeConsumption && strings.Contains(text, "Einspeisung (LPP) · Grenze") {
			t.Fatal("unused direction")
		}
		if scope == inspection.ScopeProduction && strings.Contains(text, "Bezug (LPC) · Grenze") {
			t.Fatal("unused direction")
		}
		if os.Getenv("STROMEA_024B_ARTIFACTS") == "1" {
			if err = os.WriteFile("../../artifacts/024b-"+scope+".pdf", b, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestReadableEvidence024b(t *testing.T) {
	s := inspection.Step{Name: "LPC Failsafe Ursprungswerte", Passed: true, Detail: "500 W; Mindestdauer 2h0m0s; changeable=true; kein Cache"}
	text := stepDetail(s)
	for _, bad := range []string{"2h0m0s", "true", "kein Cache"} {
		if strings.Contains(text, bad) {
			t.Error(text)
		}
	}
}

func TestReviewArtifacts024b(t *testing.T) {
	if os.Getenv("STROMEA_024B_ARTIFACTS") != "1" && os.Getenv("STROMEA_024C_ARTIFACTS") != "1" {
		t.Skip("local fixtures only")
	}
	prefix := "024b"
	if os.Getenv("STROMEA_024C_ARTIFACTS") == "1" {
		prefix = "024c"
	}
	for _, variant := range []string{"bestanden", "nicht-bestanden", "nur-bezug", "nur-einspeisung", "abbruch"} {
		r := sampleReport(t)
		r.ID = "K7Q2-9XMB-4T1R-HZ6P"
		r.SchemaVersion = 1
		r.Mode = inspection.ModeSimulator
		r.Config.Scope = inspection.ScopeBoth
		r.Config.FailsafeMinimum = 2 * time.Hour
		r.Config.Site = inspection.Site{Customer: "Ada Betreiber", CustomerAddress: "Kundenweg 7\n79100 Freiburg", Address: "Anlagenweg 12\n79100 Freiburg", Meter: "DE-024B-123", Note: "Darstellungsmuster mit simulierten Werten."}
		r.Config.Profile = inspection.InspectorProfile{Business: r.Business, Inspector: r.Inspector, Street: "Werkweg 2", PostalCode: "79100", City: "Freiburg"}
		r.Config.SignOperator = true
		r.Config.SignInspector = true
		if variant == "nur-bezug" {
			r.Config.Scope = inspection.ScopeConsumption
		}
		if variant == "nur-einspeisung" {
			r.Config.Scope = inspection.ScopeProduction
		}
		// Keep selected directions only, preserving exact names and categories.
		var selected []inspection.Step
		for _, s := range r.Steps {
			if strings.HasPrefix(s.Name, "LPC ") && !r.Config.Includes("LPC") || strings.HasPrefix(s.Name, "LPP ") && !r.Config.Includes("LPP") || s.Name == "Ausgangsmessung MPC" && !r.Config.Includes("LPC") {
				continue
			}
			s.Category = inspection.StepCategory(s.Name)
			selected = append(selected, s)
		}
		r.Steps = selected
		for _, g := range r.RequiredStepGroups() {
			for _, name := range g {
				found := false
				for _, s := range r.Steps {
					if s.Name == name {
						found = true
					}
				}
				if !found {
					r.Steps = append(r.Steps, inspection.Step{Name: name, Category: inspection.StepCategory(name), Passed: true, At: r.Started, Detail: "Bestätigt"})
				}
			}
		}
		for i := 0; i < 9; i++ {
			v := []float64{6500, 6500, 4200, 4200, 500, 500, 6500, 6500, 6500}[i]
			if variant == "nur-einspeisung" {
				v = -v
			}
			r.Samples = append(r.Samples, inspection.Sample{At: r.Started.Add(time.Duration(i) * time.Second), Watts: &v, Step: []string{"Ausgangsmessung MPC", "Ausgangsmessung MPC", "LPC gemessene Wirkung", "LPC gemessene Wirkung", "LPC Failsafe gemessene Wirkung", "LPC Failsafe gemessene Wirkung", "LPC Wiederkehr", "LPC Wiederkehr", "LPC Wiederkehr"}[i]})
		}
		if variant == "nicht-bestanden" {
			r.Steps[5].Passed = false
			r.Steps[5].Detail = "deviceConfiguration: resultData error 7"
			r.Passed = false
		}
		if variant == "abbruch" {
			r = syntheticAbortReport024c(t)
		}
		pdf, e := PDF(r)
		if e != nil {
			t.Fatal(variant, e)
		}
		text := pdfText(t, pdf)
		if variant == "bestanden" && (r.Outcome() != "Bestanden" || strings.Contains(text, "Nächste Schritte")) {
			t.Fatal("passed form has next steps", r.Outcome())
		}
		if variant == "nicht-bestanden" && (!strings.Contains(text, "Nächste Schritte") || strings.Contains(text, "deviceConfiguration:")) {
			t.Fatal("failed form has raw errors or no next steps")
		}
		if e = os.WriteFile("../../artifacts/"+prefix+"-"+variant+".pdf", pdf, 0600); e != nil {
			t.Fatal(e)
		}
		data, e := JSON(r)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile("../../artifacts/024b-"+variant+".json", data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func TestLongSiteFieldsRemainVisible024b(t *testing.T) {
	r := sampleReport(t)
	r.Config.Site = inspection.Site{Customer: strings.Repeat("Name ", 195) + "ENDE-NAME", CustomerAddress: strings.Repeat("Weg ", 245) + "ENDE-KUNDE", Address: strings.Repeat("Ort ", 245) + "ENDE-ANLAGE"}
	b, e := PDF(r)
	if e != nil {
		t.Fatal(e)
	}
	text := pdfText(t, b)
	for _, v := range []string{"ENDE-NAME", "ENDE-KUNDE", "ENDE-ANLAGE"} {
		if !strings.Contains(text, v) {
			t.Fatal("lost optional field", v)
		}
	}
}
func TestBrowserLogoEmbedsAndDoesNotChangeAttachment024b(t *testing.T) {
	if os.Getenv("STROMEA_024B_ARTIFACTS") != "1" && os.Getenv("STROMEA_024C_ARTIFACTS") != "1" {
		t.Skip("browser-generated local fixture")
	}
	b, e := os.ReadFile("../../artifacts/024b-profil-logo.png")
	if e != nil {
		t.Fatal(e)
	}
	r := sampleReport(t)
	r.Config.Profile.Logo = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
	if e = r.Config.Profile.Validate(); e != nil {
		t.Fatal(e)
	}
	data, e := JSON(r)
	if e != nil {
		t.Fatal(e)
	}
	pdf, e := PDF(r)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(pdf, []byte("/Subtype /Image")) || !bytes.Contains(pdf, data) {
		t.Fatal("missing logo or exact report attachment")
	}
	if e = os.WriteFile("../../artifacts/"+func() string {
		if os.Getenv("STROMEA_024C_ARTIFACTS") == "1" {
			return "024c"
		}
		return "024b"
	}()+"-mit-logo.pdf", pdf, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestNextStepsBulletEncoding024b(t *testing.T) {
	if literal("•") != "\\225" {
		t.Fatal("bullet is not a PDF WinAnsi bullet")
	}
}

func TestHistoricalZeroBaselineIsNeverGreen024b(t *testing.T) {
	r := sampleReport(t)
	r.Steps = []inspection.Step{{Name: "Ausgangsmessung MPC", Passed: true, Detail: "MPC 0.0 W", At: r.Started}}
	before, _ := JSON(r)
	d := &document{}
	d.newPage()
	d.steps(r)
	if strings.Contains(d.current().String(), literal("Bestanden")) {
		t.Fatal("historical 0 W baseline shown as passed")
	}
	if !strings.Contains(d.current().String(), literal("0 W")) || !strings.Contains(d.current().String(), literal("Nicht geprüft")) {
		t.Fatal("missing value or unchecked result")
	}
	after, _ := JSON(r)
	if !bytes.Equal(before, after) {
		t.Fatal("presentation changed source report")
	}
}
