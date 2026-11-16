package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func TestOptionalSignatures(t *testing.T) {
	for _, operator := range []bool{false, true} {
		for _, inspector := range []bool{false, true} {
			r := sampleReport(t)
			r.Config.SignOperator, r.Config.SignInspector = operator, inspector
			b, err := PDF(r)
			if err != nil {
				t.Fatal(err)
			}
			pages := reportPages(t, b)
			first, all := strings.Join(pages, " "), strings.Join(pages, " ")
			if strings.Contains(first, "Empfangsbestätigung") != operator {
				t.Fatal("operator choice ignored", first)
			}
			if strings.Contains(first, "Bestätigung der Fachkraft") != inspector {
				t.Fatal("inspector choice ignored", first)
			}
			if strings.Contains(first, "Unterschrift") != (operator || inspector) {
				t.Fatal("unexpected signature block", first)
			}
			if operator && !strings.Contains(first, "Ich bestätige den Erhalt dieses Testberichts (Protokoll-ID:") {
				t.Fatal("missing report reference")
			}
			for _, forbidden := range []string{"wurden mir erläutert", "hält sie ein", "keine Garantie für den späteren Betrieb", "DIN VDE", "0100-600", "Kann dem Prüfprotokoll"} {
				if strings.Contains(all, forbidden) {
					t.Fatal("obsolete claim", forbidden)
				}
			}
			if !strings.Contains(all, inspection.Notice) {
				t.Fatal("missing certification notice")
			}
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), inspection.Notice) || strings.Contains(string(data), "0100-600") || strings.Contains(string(data), "DIN VDE") {
				t.Fatal("unexpected JSON notice", string(data))
			}
		}
	}
}
