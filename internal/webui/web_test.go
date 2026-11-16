package webui

import (
	"bytes"
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLocalUIAndMutationBoundary(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	app := New(s)
	defer app.Close()
	request := func(method, path, body, token, origin, host string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:33287"+path, bytes.NewBufferString(body))
		r.Host = host
		r.Header.Set("X-Stromea-Token", token)
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "/", "", "", "", "127.0.0.1:33287")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Stromea Steuerbox-Simulator") {
		t.Fatal(w)
	}
	cases := []struct {
		token, origin, host string
		want                int
	}{{"", "", "127.0.0.1:33287", 403}, {app.token, "http://evil.example", "127.0.0.1:33287", 403}, {app.token, "", "evil.example", 403}, {app.token, "", "127.0.0.1:33287", 400}}
	for _, c := range cases {
		if w := request(http.MethodPost, "/api/pair", `{"ski":"bad"}`, c.token, c.origin, c.host); w.Code != c.want {
			t.Fatalf("%d != %d: %s", w.Code, c.want, w.Body)
		}
	}
	if w := request("GET", "/api/report.pdf", "", "", "", "127.0.0.1:33287"); w.Code != 404 {
		t.Fatal(w)
	}
	if w := request("POST", "/api/pair", `{"ski":"`+s.Device.SKI+`"}`, app.token, "", "127.0.0.1:33287"); w.Code != 200 {
		t.Fatal(w)
	}
	if w := request("POST", "/api/run", `{"business":"","inspector":"","scope":"bad"}`, app.token, "", "127.0.0.1:33287"); w.Code != 400 {
		t.Fatal(w)
	}
}

func TestDownloadPage(t *testing.T) {
	app := New(nil)
	defer app.Close()
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://127.0.0.1:33287"+path, nil)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	w := get("/protokoll")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Noch kein Protokoll") || strings.Contains(w.Body.String(), `href="/api/report.pdf"`) {
		t.Fatalf("empty download page: %d %s", w.Code, w.Body)
	}
	app.result = &inspection.Report{Business: "<script>alert(1)</script>", Inspector: "Erika", Passed: false, Started: time.Now()}
	w = get("/protokoll")
	for _, want := range []string{"Stromea Steuerbox-Simulator", "Prüfergebnis", "PDF-Prüfbericht laden", "Unvollständig", "&lt;script&gt;", "/api/report.pdf", "/api/report.json", "ersetzt keine Konformitätsprüfung"} {
		if w.Code != 200 || !strings.Contains(w.Body.String(), want) {
			t.Fatalf("missing %q: %d %s", want, w.Code, w.Body)
		}
	}
	app.busy = true
	if w := get("/protokoll"); !strings.Contains(w.Body.String(), "Prüfung läuft") || strings.Contains(w.Body.String(), `href="/api/report.pdf"`) {
		t.Fatal("stale download during run", w.Body)
	}
	css := get("/style.css").Body.String()
	for _, color := range []string{"#7CB342", "#4C7A1E"} {
		if !strings.Contains(css, color) {
			t.Fatal("missing brand color", color)
		}
	}
}

func TestHTTPInspectionAndDownloads(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	app := New(s)
	defer app.Close()
	server := httptest.NewServer(app)
	defer server.Close()
	client := server.Client()
	client.Timeout = 3 * time.Second
	get := func(path string) []byte {
		t.Helper()
		r, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, err := io.ReadAll(r.Body)
		if err != nil || r.StatusCode != 200 {
			t.Fatalf("%s: %d %v", path, r.StatusCode, err)
		}
		return b
	}
	post := func(path, body string, want int) {
		t.Helper()
		r, _ := http.NewRequest("POST", server.URL+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Stromea-Token", app.token)
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != want {
			t.Fatalf("%s: %d %s", path, resp.StatusCode, b)
		}
	}
	var devices []discovery.Device
	if err = json.Unmarshal(get("/api/discover"), &devices); err != nil || len(devices) != 1 {
		t.Fatalf("discovery: %v %v", devices, err)
	}
	post("/api/pair", `{"ski":"`+devices[0].SKI+`"}`, 200)
	post("/api/run", `{"business":"Elektro Webtest","inspector":"Erika Prüfer","signOperator":true,"signInspector":true}`, 202)
	post("/api/run", `{"business":"Elektro Webtest","inspector":"Erika Prüfer"}`, 409)
	deadline := time.Now().Add(25 * time.Second)
	for {
		var state struct {
			Busy   bool               `json:"busy"`
			Report *inspection.Report `json:"report"`
		}
		if err = json.Unmarshal(get("/api/state"), &state); err != nil {
			t.Fatal(err)
		}
		if !state.Busy {
			if state.Report == nil || !state.Report.Passed || len(state.Report.Steps) != 16 {
				t.Fatalf("failed report: %+v", state.Report)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("HTTP inspection timed out")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !bytes.HasPrefix(get("/api/report.pdf"), []byte("%PDF-2.0")) {
		t.Fatal("missing PDF")
	}
	var r inspection.Report
	if err = json.Unmarshal(get("/api/report.json"), &r); err != nil || !r.Passed || !r.Config.SignOperator || !r.Config.SignInspector {
		t.Fatalf("JSON export: %v", err)
	}
}

func TestDeviceSelectionWithoutAuthorizationHTTP(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s)
	defer a.Close()
	post := func(path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1:33287"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Stromea-Token", a.token)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	d := s.Device
	d.Name = "Fremdgerät"
	data, _ := json.Marshal(map[string]any{"device": d, "confirmed": false})
	if w := post("/api/select", string(data)); w.Code != 400 {
		t.Fatal(w)
	}
	data, _ = json.Marshal(map[string]any{"device": d, "confirmed": true})
	if w := post("/api/select", string(data)); w.Code != 200 {
		t.Fatal(w)
	}
	// Without reported control functions no inspection can start.
	if w := post("/api/run", `{"business":"Test","inspector":"Test"}`); w.Code != 409 {
		t.Fatal(w)
	}
	a.mu.Lock()
	a.connected = true
	a.capabilities = &inspection.Capabilities{LPC: true}
	a.mu.Unlock()
	// Explicit loopback device selection is sufficient; no authorized field.
	if w := post("/api/run", `{"business":"Test","inspector":"Test","stepTimeout":1000000000,"effectTimeout":1000000000,"failsafeTimeout":1000000000,"recoveryTimeout":1000000000,"cleanupTimeout":1000000000}`); w.Code != 202 {
		t.Fatal(w)
	}
	if w := post("/api/cancel", `{}`); w.Code != 200 {
		t.Fatal(w)
	}
	a.wg.Wait()
	if w := post("/api/simulator", `{}`); w.Code != 200 || s.IsDeviceMode() {
		t.Fatal(w)
	}
}

func TestDownloadPartialResultAndSignatureChoices(t *testing.T) {
	app := New(nil)
	defer app.Close()
	for _, steps := range [][]inspection.Step{nil, {{Name: "LPP gemessene Wirkung", Status: inspection.NotChecked}}} {
		app.result = &inspection.Report{Mode: inspection.ModeOnSite, Passed: true, Steps: steps}
		w := httptest.NewRecorder()
		app.downloadPage(w, httptest.NewRequest("GET", "/protokoll", nil))
		body := w.Body.String()
		if !strings.Contains(body, "Unvollständig") || !strings.Contains(body, "Offene Nachweise") || strings.Contains(body, `class="pass"`) {
			t.Fatal(body)
		}
	}
	w := httptest.NewRecorder()
	app.index(w, httptest.NewRequest("GET", "/", nil))
	for _, id := range []string{"sign-operator", "sign-inspector"} {
		if !strings.Contains(w.Body.String(), `id="`+id+`" type="checkbox">`) {
			t.Fatal("missing or preselected signature choice", id)
		}
	}
}

func TestProtocolTextsWithoutStandardReferences(t *testing.T) {
	app := New(nil)
	defer app.Close()
	app.result = &inspection.Report{Mode: inspection.ModeSimulator, Notice: inspection.Notice}
	for _, path := range []string{"/", "/protokoll", "/api/report.json"} {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287"+path, nil))
		body := w.Body.String()
		// 036: the footer is concise; the existing product boundary stays in About.
		wantNotice := "Er ist keine Steuerbox-Hardware und ersetzt keine Konformitätsprüfung."
		if path == "/api/report.json" {
			wantNotice = inspection.Notice
		}
		if w.Code != 200 || !strings.Contains(body, wantNotice) {
			t.Fatalf("missing notice at %s: %d %s", path, w.Code, body)
		}
		for _, forbidden := range []string{"DIN VDE", "0100-600", "Kann dem Prüfprotokoll"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("obsolete reference %q at %s", forbidden, path)
			}
		}
	}
}
