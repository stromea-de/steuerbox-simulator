package report

import (
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOnsiteReportDoesNotClaimSimulatorOrFailSkipped(t *testing.T) {
	r := inspection.Report{Mode: inspection.ModeOnSite, Started: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC), Profile: "Gerät vor Ort", Device: inspection.Device{Serial: "OPTA-123", Source: "SPINE DeviceClassification"}, Steps: []inspection.Step{{Name: "LPP gemessene Wirkung", Status: inspection.NotChecked, Detail: "LPP nicht gemeldet"}}}
	_, out := outcome(r)
	if out != "Unvollständig" {
		t.Fatal(out)
	}
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"Prüfung mit Simulator", "Loopback, keine physikalische Messung"} {
		if strings.Contains(string(b), literal(unwanted)) {
			t.Fatal("false mode", unwanted)
		}
	}
	if !strings.Contains(string(b), literal("Gerät vor Ort")) || !strings.Contains(string(b), "OPTA-123") {
		t.Fatal("missing real mode/serial")
	}
}

// Only writes local design fixtures; never connects to a device.
func TestOnsiteReviewArtifact(t *testing.T) {
	if os.Getenv("EEBUS_006_PDF_ARTIFACTS") != "1" {
		t.Skip("optional local PDF fixture")
	}
	r := sampleReport(t)
	r.Mode = inspection.ModeOnSite
	r.Profile = "Gerät vor Ort; DARSTELLUNGSMUSTER mit erfundenen Daten, kein Gerätetest. Failsafe-Mindestdauer 2h; Schritt 1m; Wirkung 2m."
	r.Device = inspection.Device{Name: "DARSTELLUNGSMUSTER – kein Gerätetest", Brand: "Beispielhersteller", Model: "Beispielmodell", Serial: "MUSTER-006", SKI: r.Device.SKI, Source: "Erfundene Daten für Layoutprüfung"}
	r.Config = inspection.DefaultDeviceConfig()
	r.Passed = false
	for i := range r.Steps {
		if strings.HasPrefix(r.Steps[i].Name, "LPP") {
			r.Steps[i].Passed = false
			r.Steps[i].Status = inspection.NotChecked
			r.Steps[i].Detail = "LPP nicht vom Gerät gemeldet"
			r.Steps[i].Watts = nil
		}
	}
	b, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll("../../artifacts", 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("../../artifacts/006-Darstellungsmuster.pdf", b, 0600); err != nil {
		t.Fatal(err)
	}
	text := pdfText(t, b)
	for _, want := range []string{"Gerät vor Ort", "Unvollständig", "Nicht geprüft", "MUSTER-006"} {
		if !strings.Contains(text, want) {
			t.Fatal("missing", want)
		}
	}
	data, _ := json.MarshalIndent(r, "", "  ")
	if err = os.WriteFile("../../artifacts/006-Darstellungsmuster.json", data, 0600); err != nil {
		t.Fatal(err)
	}
}
