package report

import (
	"crypto/sha256"
	"fmt"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
)

func TestPDFFooterContainsExactExportDigestOnEveryPage(t *testing.T) {
	r := inspection.Report{Business: "Test"}
	data, err := JSON(r)
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	pages := strings.Count(string(pdf), "/Type /Page ")
	if pages < 2 || strings.Count(string(pdf), literal(fmt.Sprintf("Prüfsumme der Protokolldaten (SHA-256) %x", sum))) != pages {
		t.Fatal("export hash missing")
	}
}
