package webui

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFunctionFallback034(t *testing.T) {
	watts := 8000.0
	r := inspection.Report{Mode: inspection.ModeOnSite, SchemaVersion: 1, Config: inspection.Config{Scope: inspection.ScopeConsumption, TestProfile: "quick", LPC: 4200, Tolerance: 50}}
	for _, name := range []string{"Ausgangsmessung MPC", "LPC Grenze + Quittung", "LPC gemessene Wirkung", "LPC Heartbeat-Ausfall", "LPC Failsafe gemessene Wirkung", "LPC Vorzeitige Freigabe", "LPC Vorzeitige Rückkehr gemessene Wirkung", "Grenzen aufheben"} {
		s := inspection.Step{Name: name, Passed: true}
		if name == "Ausgangsmessung MPC" {
			s.Watts = &watts
		}
		r.Steps = append(r.Steps, s)
	}
	a := New(nil)
	defer a.Close()
	a.result = &r
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287/protokoll", nil))
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "Funktionen: 4 bestanden, 0 fehlgeschlagen, 0 nicht geprüft, 0 ausstehend.") {
		t.Fatalf("fallback count mismatch: HTTP %d, projection %+v", w.Code, r.FunctionCounts())
	}
	if strings.Contains(body, ">passed<") || strings.Contains(body, ">excluded<") {
		t.Fatal("fallback exposes untranslated status codes")
	}
}
