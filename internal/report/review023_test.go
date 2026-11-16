package report

import (
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"os"
	"strings"
	"testing"
)

func TestHardware23PDFPaginates(t *testing.T) {
	data, err := os.ReadFile("testdata/hil23.json")
	if os.IsNotExist(err) {
		t.Skip("Private hardware fixture is excluded from the source export")
	}
	if err != nil {
		t.Fatal(err)
	}
	var r inspection.Report
	if err = json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	pdf, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("STROMEA_023_ARTIFACTS") == "1" {
		if err = os.WriteFile("../../artifacts/023-HIL23-original.pdf", pdf, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.HasPrefix(string(pdf), "%PDF") {
		t.Fatal("no PDF")
	}
}

func TestSiteAndConnectionEventsInPDF023(t *testing.T) {
	r := sampleReport(t)
	r.Config.Site = inspection.Site{Customer: "Kundin 023", Address: "Musterweg 23", Meter: "Z-023", Note: strings.Repeat("Notiz 023 ", 100) + "ENDE-NOTIZ"}
	r.Events = []inspection.Step{{Name: "Verbindungsabbruch", At: r.Started, Detail: "SHIP getrennt"}, {Name: "Verbindung neu aufgebaut", At: r.Finished, Detail: "Erkennung wiederholt"}}
	b, e := PDF(r)
	if e != nil {
		t.Fatal(e)
	}
	if os.Getenv("STROMEA_023_ARTIFACTS") == "1" {
		if e = os.WriteFile("../../artifacts/023-Kundendaten.pdf", b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	text := pdfText(t, b)
	for _, want := range []string{"Kundin 023", "Musterweg 23", "Z-023", "ENDE-NOTIZ", "Verbindungsabbruch", "Verbindung neu aufgebaut", "Erkennung wiederholt"} {
		if !strings.Contains(text, want) {
			t.Fatal("missing", want)
		}
	}
}
