package session

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
	"time"
)

func TestReportPersistenceAndSite023(t *testing.T) {
	dir := t.TempDir()
	r := inspection.Report{Started: time.Now(), Config: inspection.Config{Site: inspection.Site{Customer: "Testkunde", Address: "Testweg 1", Meter: "Z-23", Note: "Nur lokal"}}}
	if e := SaveReport(dir, &r); e != nil {
		t.Fatal(e)
	}
	list, e := ListReports(dir)
	if e != nil || len(list) != 1 || list[0].Config.Site != r.Config.Site {
		t.Fatal(e, list)
	}
	r.Config.Site.Note = "Nachtrag"
	if e = SaveReport(dir, &r); e != nil {
		t.Fatal(e)
	}
	got, e := LoadReport(dir, r.ID)
	if e != nil || got.Config.Site.Note != "Nachtrag" {
		t.Fatal(e, got)
	}
	for _, id := range []string{"../secret", "/etc/passwd", "123.json", ""} {
		if _, e = LoadReport(dir, id); e == nil {
			t.Fatal("invalid ID", id)
		}
	}
	if e = (inspection.Site{Note: strings.Repeat("a", 1001)}).Validate(); e == nil {
		t.Fatal("unbounded metadata")
	}
}
