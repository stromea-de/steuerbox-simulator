package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// 040 F-12 (UX-G7): the /protokoll fallback shows test values in German format (4.200 W) and the
// unit survives the client-side number localisation (only the number carries data-number).
func TestProtocolNumbersGrouped040(t *testing.T) {
	app := New(nil)
	defer app.Close()
	c := inspection.DefaultConfig()
	c.LPC, c.LPP, c.Failsafe, c.Tolerance = 4200, 10900.5, 500, 50
	app.result = &inspection.Report{Business: "B", Inspector: "P", Started: time.Now(), Config: c}
	w := httptest.NewRecorder()
	app.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287/protokoll", nil))
	body := w.Body.String()
	for _, want := range []string{`<span data-number="4200">4.200</span> W`, `<span data-number="10900.5">10.900,5</span> W`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in %s", want, body[strings.Index(body, "Prüfwerte"):][:600])
		}
	}
}

// 040 F-13: correction and deletion use the same lock rule as pairing (no changes during a LAN search).
func TestReportChangesBlockedDuringSearch040(t *testing.T) {
	app := New(nil)
	defer app.Close()
	app.search.Active = true
	for path, body := range map[string]string{"/api/reports/delete": `{"id":"123"}`, "/api/report/site?id=123": `{"customer":"X"}`} {
		if w := request025(app, "POST", path, body); w.Code != 409 {
			t.Fatalf("%s during search: HTTP %d %s", path, w.Code, w.Body)
		}
	}
}
