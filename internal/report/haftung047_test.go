package report

import (
	"strings"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func TestOriginalsRestoredText047(t *testing.T) {
	yes, no := true, false
	ok := inspection.Report{Mode: inspection.ModeOnSite, ReleaseVerified: &yes, FailsafeOriginals: map[string]inspection.FailsafeValues{"LPC": {Power: 777}},
		Steps: []inspection.Step{{Name: "Failsafe-Werte setzen", Passed: true}, {Name: "LPC Failsafe-Leistung zurückstellen", Passed: true}, {Name: "LPC Failsafe-Mindestdauer zurückstellen", Passed: true}, {Name: "Grenzen aufheben", Passed: true}}}
	if OriginalsRestoredText(ok) != "Ja, vom Gerät zurückgelesen" || ActiveLimitText(ok) != "Keine, vom Gerät zurückgelesen" {
		t.Fatal(OriginalsRestoredText(ok), ActiveLimitText(ok))
	}
	bad := ok
	bad.ReleaseVerified = &no
	bad.RestorePending = true
	bad.Steps = append([]inspection.Step(nil), ok.Steps...)
	bad.Steps[3] = inspection.Step{Name: "Grenzen aufheben", Detail: "LPC Begrenzung laut Gerät weiter aktiv"}
	if !strings.HasPrefix(OriginalsRestoredText(bad), "Nein") || !strings.Contains(OriginalsRestoredText(bad), "erneut") || !strings.Contains(ActiveLimitText(bad), "am Gerät prüfen") {
		t.Fatal(OriginalsRestoredText(bad), ActiveLimitText(bad))
	}
	pdf, err := PDF(bad)
	if err != nil || len(pdf) == 0 {
		t.Fatal(err)
	}
}
