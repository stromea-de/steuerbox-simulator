package report

import (
	"regexp"
	"strings"
	"testing"
)

func TestRenamedPDFHeaders(t *testing.T) {
	b, err := PDF(sampleReport(t))
	if err != nil {
		t.Fatal(err)
	}
	for i, page := range reportPages(t, b) {
		if regexp.MustCompile(`(?i)\b(konform|zertifiziert)(e|er|es|en|em)?\b`).MatchString(page) {
			t.Errorf("page %d contains a conformity/certification assertion", i+1)
		}
		for _, want := range []string{"Stromea Steuerbox-Simulator", "Prüfbericht · Kommunikationsprüfung", "stromea.de/simulator"} {
			if !strings.Contains(page, want) {
				t.Errorf("page %d: missing %q", i+1, want)
			}
		}
		for _, forbidden := range []string{"Lucas", "Spohn", "Stromea Prüftool", "Stromea EEBUS Prüftool", "0100-600", "lokale Steuerbox mit", "lokale Steuerbox und"} {
			if strings.Contains(page, forbidden) {
				t.Errorf("page %d: contains %q", i+1, forbidden)
			}
		}
	}
}
