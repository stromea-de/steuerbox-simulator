package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReview008bSummaryWithoutFloatingMark(t *testing.T) {
	for _, onsite := range []bool{false, true} {
		r := review008Report(t, onsite)
		d := &document{}
		d.newPage()
		d.summary(r)
		// The selected result has two border accents and two joined check strokes.
		if got := strings.Count(d.page.String(), " m "); got != 4 {
			t.Errorf("floating result mark: %d paths", got)
		}
	}
}

func TestReview008bPageOneAndMark(t *testing.T) {
	for _, onsite := range []bool{false, true} {
		r := review008Report(t, onsite)
		b, err := PDF(r)
		if err != nil {
			t.Fatal(err)
		}
		pages := reportPages(t, b)
		if onsite && strings.Count(pages[0], "Gerät vor Ort") != 1 {
			t.Error("onsite label must appear once on page 1")
		}
		// Original Stromea S curves, rounded caps, and monochrome sun dot.
		if !strings.Contains(string(b), "587 364 m 587 324 437 324 437 379 c") || !strings.Contains(string(b), "42 w 1 J 1 j") || !strings.Contains(string(b), "536 284 m") {
			t.Error("missing original Stromea S vector and dot")
		}
		if os.Getenv("STROMEA_008B_PDF_ARTIFACTS") == "1" {
			name := "008b-uebung"
			if onsite {
				name = "008b-vor-ort-teilweise-unterschriften"
			}
			if err := os.MkdirAll("../../artifacts", 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join("../../artifacts", name+".pdf"), b, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
