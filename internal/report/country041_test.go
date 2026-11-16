package report

import (
	"strings"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// 041 (Nutzer 2026-11-14): Profil mit Feld „Land“ (Standard Deutschland). Das deutsche Protokoll nennt das
// Land nur, wenn es nicht Deutschland ist.
func TestProfileCountry041(t *testing.T) {
	for _, c := range []struct{ country, want, not string }{{"Deutschland", "79100 Freiburg", "Deutschland"}, {"Österreich", "\\326sterreich", ""}, {"", "79100 Freiburg", "Deutschland"}} {
		r := sampleReport(t)
		r.Config.Profile = inspection.InspectorProfile{Business: r.Business, Inspector: r.Inspector, Street: "Werkweg 2", PostalCode: "79100", City: "Freiburg", Country: c.country}
		if err := r.Config.Profile.Validate(); err != nil {
			t.Fatal(err)
		}
		b, err := PDF(r)
		if err != nil {
			t.Fatal(err)
		}
		text := pdfText(t, b)
		if !strings.Contains(text, c.want) || c.not != "" && strings.Contains(text, c.not) {
			t.Fatalf("country %q: want %q, not %q", c.country, c.want, c.not)
		}
	}
	long := inspection.InspectorProfile{Country: strings.Repeat("x", 201)}
	if long.Validate() == nil {
		t.Fatal("country longer than 200 characters accepted")
	}
}
