package report

import (
	"bytes"
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMatrixScenarioAndEvidenceExports022(t *testing.T) {
	r := sampleReport(t)
	r.Mode = inspection.ModeOnSite
	r.Config = inspection.DefaultDeviceConfig()
	r.Config.Scope = inspection.ScopeBoth
	r.Config.NormalizeProfile()
	r.Device.Brand = "Stromea"
	r.Device.Model = "Opta"
	r.Device.Type = "EnergyManagementSystem"
	r.Capabilities = &inspection.Capabilities{LPC: true, LPP: true, Scenarios: map[string][]uint{"LPC": {1, 2, 3}, "LPP": {1, 2, 3}, "MPC": {}}}
	r.Steps = nil
	r.Samples = nil
	r.Passed = false
	r.Profile = "DARSTELLUNGSMUSTER: erfundene Daten, kein Gerätetest. Protokolltest ohne MPC."
	for _, name := range []string{"Verbindung", "Unterstützte Use Cases", "Heartbeat starten", "Failsafe-Werte setzen"} {
		r.Steps = append(r.Steps, inspection.Step{Name: name, Passed: true, At: r.Started})
	}
	for _, k := range []string{"LPC", "LPP"} {
		for _, name := range []string{"Failsafe Ursprungswerte", "Grenze + Quittung", "Heartbeat-Ausfall", "Ausfallfrist abwarten", "Heartbeat wieder starten", "Wiederkehr Protokoll", "Vorzeitige Freigabe", "Grenze freigeben", "Failsafe-Leistung zurückstellen", "Failsafe-Mindestdauer zurückstellen"} {
			r.Steps = append(r.Steps, inspection.Step{Name: k + " " + name, Passed: true, At: r.Started})
		}
		for _, name := range []string{"Ausgangsmessung MPC", "gemessene Wirkung", "Failsafe gemessene Wirkung", "Wiederkehr", "Wiederkehr gemessene Wirkung", "Vorzeitige Rückkehr gemessene Wirkung"} {
			if k == "LPC" && name == "Ausgangsmessung MPC" {
				r.Steps = append(r.Steps, inspection.Step{Name: name, Status: inspection.NotChecked, Detail: "MPC nicht vom Gerät gemeldet", At: r.Started})
				continue
			}
			r.Steps = append(r.Steps, inspection.Step{Name: k + " " + name, Status: inspection.NotChecked, Detail: "MPC nicht vom Gerät gemeldet", At: r.Started})
		}
		r.Heartbeats = append(r.Heartbeats, inspection.NewHeartbeatObservation(k, r.Started.Add(time.Second), time.Time{}, 120*time.Second))
	}
	r.Steps = append(r.Steps, inspection.Step{Name: "Grenzen aufheben", Passed: true, At: r.Started.Add(310 * time.Second)})
	r.FailsafeOriginals = map[string]inspection.FailsafeValues{"LPC": {Power: 900, Minimum: 2 * time.Hour}, "LPP": {Power: 900, Minimum: 2 * time.Hour}}
	r.Finished = r.Started.Add(310 * time.Second)
	data, e := JSON(r)
	if e != nil {
		t.Fatal(e)
	}
	var restored inspection.Report
	if e = json.Unmarshal(data, &restored); e != nil || len(restored.Capabilities.Scenarios["LPP"]) != 3 {
		t.Fatal(string(data), e)
	}
	pdf, e := PDF(r)
	if e != nil {
		t.Fatal(e)
	}
	text := pdfText(t, pdf)
	for _, want := range []string{"Prüfschritt", "Bezug", "Einspeisung", "Offene Nachweise · nach Grund", "12 Nachweise", "Keine auswertbaren Messwerte", "Grenze, Notbetriebswerte, Lebenszeichen", "Energiemanagementsystem", "Unvollständig", "Software " + BuildVersion()} {
		if !strings.Contains(strings.Join(strings.Fields(text), " "), want) {
			t.Error("missing", want)
		}
	}
	if len(r.EvidenceGroups()) != 1 {
		t.Fatal(r.EvidenceGroups())
	}
	if os.Getenv("STROMEA_022_ARTIFACTS") == "1" {
		for ext, b := range map[string][]byte{"pdf": pdf, "json": data} {
			if e = os.WriteFile("../../artifacts/022-Protokoll-ohne-mpc."+ext, b, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestVersionCommitOnEveryPDFFooter022(t *testing.T) {
	previous := Commit
	Commit = "abc1234"
	defer func() { Commit = previous }()
	b, e := PDF(sampleReport(t))
	if e != nil {
		t.Fatal(e)
	}
	if got, want := bytes.Count(b, []byte(literal("Software "+BuildVersion()))), bytes.Count(b, []byte("/Type /Page ")); got != want {
		t.Fatal(got, want)
	}
}
