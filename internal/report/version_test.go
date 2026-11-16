package report

import (
	"strings"
	"testing"
)

func TestDevelopmentVersionInReport(t *testing.T) {
	if Version != "" {
		t.Fatalf("unexpected default version: %s", Version)
	}
	b, err := PDF(sampleReport(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Software "+BuildVersion()) {
		t.Fatal("PDF missing software version")
	}
}
