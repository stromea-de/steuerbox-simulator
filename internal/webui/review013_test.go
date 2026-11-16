package webui

import (
	"bytes"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExportFailurePreservesReportAndStepRecording(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s)
	defer a.Close()
	a.result = &inspection.Report{Config: inspection.Config{Profile: inspection.InspectorProfile{Logo: "ungültiges Logo"}}}
	before := a.result
	w := httptest.NewRecorder()
	a.pdf(w, httptest.NewRequest("GET", "/api/report.pdf", nil))
	if w.Code != 500 || a.result != before {
		t.Fatal(w.Code, "report lost")
	}
	s.StepFinished(inspection.Step{Name: "Nach Exportfehler", Passed: true})
	if len(s.Progress().Steps) != 1 {
		t.Fatal("step recording stopped")
	}
	w = httptest.NewRecorder()
	a.jsonReport(w, httptest.NewRequest("GET", "/api/report.json", nil))
	want, _ := report.JSON(*a.result)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), want) {
		t.Fatal("JSON no longer available after PDF failure")
	}
}
func TestIdentityResetHTTPRequiresConfirmationAndIdle(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s)
	defer a.Close()
	before := s.EG.Identity.SKI
	for _, tc := range []struct {
		confirmed, busy bool
		want            int
	}{{false, false, 400}, {true, true, 409}, {true, false, 200}} {
		a.busy = tc.busy
		body := `{"confirmed":false}`
		if tc.confirmed {
			body = `{"confirmed":true}`
		}
		r := httptest.NewRequest("POST", "http://127.0.0.1:33287/api/identity/reset", strings.NewReader(body))
		r.Header.Set("X-Stromea-Token", a.token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.want != 200 && s.EG.Identity.SKI != before {
			t.Fatal("identity changed without authorization")
		}
	}
	if s.EG.Identity.SKI == before {
		t.Fatal("identity did not rotate")
	}
}
