package webui

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// QA-06: the browser's icon request must not fail (404 + console error) on any page.
func TestFavicon039(t *testing.T) {
	a := New(nil)
	defer a.Close()
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287/favicon.ico", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(w.Body.String(), "M587 364C587 324") {
		t.Fatalf("favicon: HTTP %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	for _, path := range []string{"/", "/hilfe"} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287"+path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `<link rel="icon" href="data:image/svg+xml,`) {
			t.Fatalf("%s: HTTP %d without icon link", path, w.Code)
		}
	}
	for _, name := range []string{"index.html", "protokoll.html"} {
		b, _ := assets.ReadFile("assets/" + name)
		if !strings.Contains(string(b), `<link rel="icon" href="data:image/svg+xml,`) {
			t.Errorf("%s: no inline icon link", name)
		}
	}
}

// C-01: a saved report can be deleted with all its versions; other reports remain.
func TestDeleteReportSeries039(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	ids := make([]string, 3)
	for i := range ids {
		ids[i], _ = inspection.NewReportID()
	}
	root := inspection.Report{ID: ids[0], SchemaVersion: 1, Mode: inspection.ModeSimulator, Started: at}
	revised := inspection.Report{ID: ids[1], SchemaVersion: 1, Mode: inspection.ModeSimulator, Started: at, Revision: &inspection.ReportRevision{Number: 2, RootID: root.ID, PreviousID: root.ID}}
	other := inspection.Report{ID: ids[2], SchemaVersion: 1, Mode: inspection.ModeSimulator, Started: at.Add(time.Hour)}
	for _, r := range []*inspection.Report{&root, &revised, &other} {
		if e := session.SaveReport(dir, r); e != nil {
			t.Fatal(e)
		}
	}
	a := New(nil)
	defer a.Close()
	a.reportDir = dir
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "http://127.0.0.1:33287/api/reports/delete", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		a.handler.ServeHTTP(w, r)
		return w
	}
	if w := post(`{"id":"` + ids[1] + `"}`); w.Code != 200 || !strings.Contains(w.Body.String(), ids[0]) {
		t.Fatalf("delete: HTTP %d %s", w.Code, w.Body.String())
	}
	left, e := session.ListReports(dir)
	if e != nil || len(left) != 1 || left[0].ID != other.ID {
		t.Fatalf("remaining reports: %+v %v", left, e)
	}
	if w := post(`{"id":"` + ids[1] + `"}`); w.Code != 404 {
		t.Fatalf("second delete: HTTP %d", w.Code)
	}
	if w := post(`{"id":"../x"}`); w.Code != 404 {
		t.Fatalf("invalid id: HTTP %d", w.Code)
	}
}
