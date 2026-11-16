package session

import (
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
	"os"
	"path/filepath"
	"testing"
)

func TestOldPDFIdentitySurvivesSiteCorrection024(t *testing.T) {
	dir := t.TempDir()
	old := inspection.Report{ID: "1793440000000000023"}
	want := report.ProtocolIdentity(old)
	data, _ := json.Marshal(old)
	if e := os.WriteFile(filepath.Join(dir, old.ID+".json"), data, 0600); e != nil {
		t.Fatal(e)
	}
	loaded, e := LoadReport(dir, old.ID)
	if e != nil {
		t.Fatal(e)
	}
	loaded.Config.Site.Customer = "Nachtrag"
	if e = SaveReport(dir, &loaded); e != nil {
		t.Fatal(e)
	}
	again, e := LoadReport(dir, old.ID)
	if e != nil || again.ID != old.ID || report.ProtocolIdentity(again) != want {
		t.Fatal(e, again.ID, report.ProtocolIdentity(again), want)
	}
	freshID, e := inspection.NewReportID()
	if e != nil {
		t.Fatal(e)
	}
	fresh := inspection.Report{ID: freshID, SchemaVersion: 1}
	if e = SaveReport(dir, &fresh); e != nil {
		t.Fatal(e)
	}
	again, e = LoadReport(dir, freshID)
	if e != nil || again.ID != freshID || report.ProtocolIdentity(again) != freshID {
		t.Fatal(e, again)
	}
}
