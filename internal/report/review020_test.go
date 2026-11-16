package report

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func TestProfileAndHeartbeatReferenceInExports020(t *testing.T) {
	r := sampleReport(t)
	r.Config.TestProfile = "full"
	at := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r.Heartbeats = []inspection.HeartbeatObservation{inspection.NewHeartbeatObservation("LPC", at, at.Add(-time.Second), 120*time.Second)}
	data, err := JSON(r)
	if err != nil {
		t.Fatal(err)
	}
	var got inspection.Report
	if err = json.Unmarshal(data, &got); err != nil || got.Config.TestProfile != "full" || !got.Heartbeats[0].Reference.Equal(at) {
		t.Fatal(string(data), err)
	}
	pdf, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Langzeitpr", "Lebenszeichen gestoppt", "Beobachtung bis", "Reserve 30 s"} {
		if !bytes.Contains(pdf, []byte(text)) {
			t.Fatal("PDF missing", text)
		}
	}
	if got.Heartbeats[0].Receipt == nil || !got.Heartbeats[0].Stopped.Equal(at) {
		t.Fatal("stop/receipt lost in JSON")
	}
}

func TestReviewArtifacts020b(t *testing.T) {
	if os.Getenv("EEBUS_020B_ARTIFACTS") != "1" {
		t.Skip("opt-in offline review artifacts")
	}
	for _, profile := range []string{"quick", "full"} {
		r := sampleReport(t)
		r.Mode = inspection.ModeOnSite
		r.Config = inspection.DefaultDeviceConfig()
		r.Config.Scope = inspection.ScopeConsumption
		r.Config.TestProfile = profile
		r.Config.NormalizeProfile()
		r.Profile = "DARSTELLUNGSMUSTER mit erfundenen Daten, kein Gerätetest. " + r.Config.ProfileLabel() + ". LPC: 120 s ab lokalem Stopp nach Sendesperre plus 30 s Beobachtungsreserve. Fehlende oder negative Quittungen sind nur Information; bewertet wird die gemessene Wirkung."
		if profile == "full" {
			r.Finished = r.Started.Add(140 * time.Minute)
		}
		r.Steps = nil
		for _, s := range sampleReport(t).Steps {
			if !strings.HasPrefix(s.Name, "LPP") {
				r.Steps = append(r.Steps, s)
			}
		}
		at := r.Started.Add(2 * time.Second)
		r.Heartbeats = []inspection.HeartbeatObservation{inspection.NewHeartbeatObservation("LPC", at, time.Time{}, 120*time.Second)}
		if profile == "full" {
			r.Heartbeats[0] = inspection.NewHeartbeatObservation("LPC", at, at.Add(-5*time.Second), 120*time.Second)
		}
		r.FailsafeOriginals = map[string]inspection.FailsafeValues{"LPC": {Power: 900, Minimum: 2 * time.Hour}}
		for ext, generate := range map[string]func(inspection.Report) ([]byte, error){"pdf": PDF, "json": JSON} {
			data, err := generate(r)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile("../../artifacts/020b-Protokoll-"+profile+"."+ext, data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
