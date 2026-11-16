package webui

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

func Test035cReportModeSurvivesSummaryAndFallback(t *testing.T) {
	a := New(nil)
	defer a.Close()
	a.reportDir = t.TempDir()
	modes := []string{inspection.ModeSimulator, inspection.ModeOnSite, ""}
	want := map[string]string{}
	for i, mode := range modes {
		r := inspection.Report{Mode: mode, Started: time.Now().Add(time.Duration(i) * time.Second), Config: inspection.Config{Site: inspection.Site{Customer: "{{unbekannt}}<script>"}}, Steps: []inspection.Step{{Name: "only stored, not listed"}}}
		if err := session.SaveReport(a.reportDir, &r); err != nil {
			t.Fatal(err)
		}
		want[r.ID] = mode
	}
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287"+path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
		return w
	}
	var entries []reportListEntry
	if err := json.Unmarshal(get("/api/reports").Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(modes) {
		t.Fatalf("got %d summaries", len(entries))
	}
	for _, entry := range entries {
		if entry.Report.Mode != want[entry.ID] {
			t.Errorf("mode for %s: %q", entry.ID, entry.Report.Mode)
		}
		if len(entry.Report.Steps) != 0 {
			t.Error("summary includes diagnostics")
		}
		if strings.Contains(entry.StatusLabel(), "(Übung)") != (entry.Report.Mode == inspection.ModeSimulator) {
			t.Error("practice status inferred from wrong mode")
		}
	}
	html := get("/protokolle").Body.String()
	if !strings.Contains(html, `href="/report-list.css"`) || strings.Contains(html, "<style>") {
		t.Error("No-JS library styling violates the existing self-only CSP")
	}
	if !strings.Contains(get("/report-list.css").Body.String(), "#practice-badge") {
		t.Error("No-JS stylesheet unavailable")
	}
	for _, text := range []string{`id="report-list-fallback"`, "Nicht bestanden (Übung)", "Gerät vor Ort", "Nicht gespeichert", "{{unbekannt}}&lt;script&gt;"} {
		if !strings.Contains(html, text) {
			t.Errorf("No-JS list missing %q", text)
		}
	}
	if strings.Contains(html, "{{unbekannt}}<script>") {
		t.Error("unescaped customer data")
	}
}
