package webui

import (
	"bytes"
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLibraryAndCustomerUpdates023(t *testing.T) {
	a := New(nil)
	defer a.Close()
	a.reportDir = t.TempDir()
	r := inspection.Report{Started: time.Now(), Config: inspection.Config{Site: inspection.Site{Customer: "Kunde 023", Address: "Anlage 23", Meter: "Z-023", Note: "Notiz"}}, Device: inspection.Device{Model: "Simulator 023"}}
	if e := session.SaveReport(a.reportDir, &r); e != nil {
		t.Fatal(e)
	}
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://127.0.0.1:33287"+path, bytes.NewBufferString(body))
		req.Header.Set("X-Stromea-Token", token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"/protokolle", "/hilfe"} {
		if w := request("GET", path, "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Hauptmenü") {
			t.Fatal(w)
		}
	}
	w := request("GET", "/api/reports", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Kunde 023") {
		t.Fatal(w)
	}
	body := `{"customer":"Nachtrag <script>","address":"Neu 23","meter":"Z-23","note":"Nur lokal"}`
	if w = request("POST", "/api/report/site?id="+r.ID, body, ""); w.Code != 403 {
		t.Fatal("missing CSRF", w)
	}
	if w = request("POST", "/api/report/site?id="+r.ID, body, a.token); w.Code != 200 {
		t.Fatal(w)
	}
	var updated inspection.Report
	if e := json.Unmarshal(w.Body.Bytes(), &updated); e != nil || updated.Config.Site.Note != "Nur lokal" {
		t.Fatal(e, w)
	}
	for _, path := range []string{"/api/report.json?id=" + r.ID, "/api/report.pdf?id=" + r.ID, "/protokoll?id=" + r.ID} {
		w = request("GET", path, "", "")
		if w.Code != 200 {
			t.Fatal(path, w)
		}
		if strings.HasPrefix(path, "/protokoll") && strings.Contains(w.Body.String(), "Nachtrag <script>") {
			t.Fatal("unescaped customer data")
		}
	}
	if w = request("GET", "/api/report.json?id=../other", "", ""); w.Code != 404 {
		t.Fatal(w)
	}
	// A second App reads the saved metadata without relying on in-memory results.
	b := New(nil)
	defer b.Close()
	b.reportDir = a.reportDir
	req := httptest.NewRequest("GET", "http://127.0.0.1:33287/api/reports", nil)
	w = httptest.NewRecorder()
	b.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "Nachtrag") || !strings.Contains(w.Body.String(), "Z-23") {
		t.Fatal(w)
	}
}

func TestPDFFailureIsReadablePage023(t *testing.T) {
	a := New(nil)
	defer a.Close()
	a.result = &inspection.Report{Config: inspection.Config{LPC: math.NaN()}}
	req := httptest.NewRequest("GET", "http://127.0.0.1:33287/api/report.pdf", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, req)
	if w.Code != 500 || !strings.Contains(w.Header().Get("Content-Type"), "text/html") || !strings.Contains(w.Body.String(), "PDF konnte nicht erstellt werden") || !strings.Contains(w.Body.String(), "Messdaten sichern (JSON)") {
		t.Fatal(w)
	}
}
