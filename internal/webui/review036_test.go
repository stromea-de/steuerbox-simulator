package webui

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

func Test036CustomerCorrectionCreatesImmutableRevision(t *testing.T) {
	a := New(nil)
	defer a.Close()
	a.reportDir = t.TempDir()
	id, err := inspection.NewReportID()
	if err != nil {
		t.Fatal(err)
	}
	watts := 10996.0
	r := inspection.Report{ID: id, SchemaVersion: 1, Mode: inspection.ModeSimulator, Started: time.Now(), Finished: time.Now(), Config: inspection.DefaultConfig(), Samples: []inspection.Sample{{At: time.Now(), Watts: &watts, Step: "Ausgangsmessung MPC"}}}
	r.Config.Site.Customer = "Vorher"
	if err := session.SaveReport(a.reportDir, &r); err != nil {
		t.Fatal(err)
	}
	a.result = &r
	path := filepath.Join(a.reportDir, r.ID+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://127.0.0.1:33287/api/report/site?id="+r.ID, bytes.NewBufferString(`{"customer":"Korrigiert"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stromea-Token", a.token)
	a.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("correction status %d", w.Code)
	}
	var updated inspection.Report
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.ID == r.ID || updated.ID == "" {
		t.Fatal("correction overwrites the original report instead of creating a new ID")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("original report bytes changed")
	}
	oldSamples, _ := json.Marshal(r.Samples)
	newSamples, _ := json.Marshal(updated.Samples)
	if !bytes.Equal(oldSamples, newSamples) || !updated.Started.Equal(r.Started) || !updated.Finished.Equal(r.Finished) {
		t.Fatal("measurement data or test timestamps changed")
	}
	var data map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &data)
	var revision struct {
		Number     int    `json:"number"`
		PreviousID string `json:"previousId"`
	}
	if err := json.Unmarshal(data["revision"], &revision); err != nil || revision.Number != 2 || revision.PreviousID != r.ID {
		t.Fatal("missing revision 2 / previous report relation")
	}
	oldJSON, _ := report.JSON(r)
	newJSON, _ := report.JSON(updated)
	if bytes.Equal(oldJSON, newJSON) {
		t.Fatal("JSON/digest input unchanged")
	}
	pdf, err := report.PDF(updated)
	if err != nil || !bytes.Contains(pdf, []byte("Version 2")) || !bytes.Contains(pdf, []byte("ersetzt Version 1")) {
		t.Fatal("PDF lacks the correction version and predecessor")
	}
	if a.result.ID != updated.ID {
		t.Fatal("current result does not follow the new revision")
	}
	list, err := session.ListReports(a.reportDir)
	if err != nil || len(list) != 2 {
		t.Fatal("old and new revision not both in archive")
	}
	for _, saved := range list {
		for _, endpoint := range []string{"/api/report.json?id=", "/api/report.pdf?id=", "/protokoll?id="} {
			rr := httptest.NewRecorder()
			a.ServeHTTP(rr, httptest.NewRequest("GET", "http://127.0.0.1:33287"+endpoint+saved.ID, nil))
			if rr.Code != 200 {
				t.Fatalf("saved revision not available: %s %d", endpoint, rr.Code)
			}
		}
	}
	// 041 (Nutzer 2026-11-14): a historical version can no longer be corrected (HTTP 409); correcting the
	// newest version advances the same series instead of producing a second "Version 2".
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "http://127.0.0.1:33287/api/report/site?id="+r.ID, bytes.NewBufferString(`{"customer":"Weitere Korrektur"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stromea-Token", a.token)
	a.ServeHTTP(w, req)
	if w.Code != 409 {
		t.Fatalf("historical correction must be refused: %d", w.Code)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "http://127.0.0.1:33287/api/report/site?id="+updated.ID, bytes.NewBufferString(`{"customer":"Weitere Korrektur"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stromea-Token", a.token)
	a.ServeHTTP(w, req)
	var third inspection.Report
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &third) != nil || third.Revision == nil || third.Revision.Number != 3 || third.Revision.PreviousID != updated.ID {
		t.Fatal("correction of the newest version does not advance the revision series")
	}
}
