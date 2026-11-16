package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// These are synthetic layout fixtures, not evidence of an onsite or simulator run.
func TestReview006bArtifacts(t *testing.T) {
	version := "006b"
	if os.Getenv("STROMEA_007_PDF_ARTIFACTS") == "1" {
		version = "007"
	} else if os.Getenv("EEBUS_006C_PDF_ARTIFACTS") == "1" {
		version = "006c"
	} else if os.Getenv("EEBUS_006B_PDF_ARTIFACTS") != "1" {
		t.Skip("optional local layout fixtures")
	}
	for _, onsite := range []bool{false, true} {
		r := sampleReport(t)
		r.Device.Name = "DARSTELLUNGSMUSTER – Simulator"
		name := version + "-uebung-bestanden-ohne-unterschrift"
		r.Mode = inspection.ModeSimulator
		if onsite {
			name = version + "-vor-ort-teilweise-beide-unterschriften"
			r.Mode = inspection.ModeOnSite
			r.Config = inspection.DefaultDeviceConfig()
			r.Config.SignOperator, r.Config.SignInspector = true, true
			r.Config.Business, r.Config.Inspector = r.Business, r.Inspector
			r.Profile = "DARSTELLUNGSMUSTER mit erfundenen Daten; kein Gerätetest. Failsafe-Mindestdauer 2 h; Prüfzeiten siehe Schritte."
			r.Device.Name = "DARSTELLUNGSMUSTER – kein Gerätetest"
			r.Device.Model = "Beispielmodell"
			r.Device.Brand = "Beispielhersteller"
			r.Device.Serial = "MUSTER-" + version
			r.Device.Firmware = "Muster-Version"
			r.Device.Source = "Erfundene Daten für Layoutprüfung"
			r.Passed = false
			for i := range r.Steps {
				if strings.HasPrefix(r.Steps[i].Name, "LPP") {
					r.Steps[i].Status = inspection.NotChecked
					r.Steps[i].Passed = false
					r.Steps[i].Detail = "LPP im Darstellungsmuster nicht gemeldet"
					r.Steps[i].Watts = nil
				}
			}
			s := r.Steps[9]
			s.Name = "LPC Wiederkehr gemessene Wirkung"
			r.Steps = append(r.Steps, s)
		}
		b, err := PDF(r)
		if err != nil {
			t.Fatal(err)
		}
		first := reportPages(t, b)[0]
		want := "Bestanden"
		if onsite {
			want = "Unvollständig"
		}
		if !strings.Contains(first, want) || strings.Contains(first, "Unterschrift") != onsite {
			t.Fatal(first)
		}
		if err = os.MkdirAll("../../artifacts", 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join("../../artifacts", name+".pdf"), b, 0644); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join("../../artifacts", name+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
