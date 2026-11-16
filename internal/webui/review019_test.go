package webui

import (
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// The saved view must use recorded data and a server-computed decision, without
// fetching a later session or allowing user data to terminate the JSON script.
func TestSavedResultRecordedData019(t *testing.T) {
	a := New(nil)
	defer a.Close()
	a.result = &inspection.Report{Mode: inspection.ModeOnSite, Business: "</script><img src=x onerror=alert(1)>", Device: inspection.Device{Name: "Gespeichertes Gerät"}, Config: inspection.DefaultDeviceConfig()}
	w := httptest.NewRecorder()
	a.downloadPage(w, httptest.NewRequest("GET", "/protokoll", nil))
	html := w.Body.String()
	match := regexp.MustCompile(`<script id="saved-report" data-outcome="([^"]+)"[^>]* type="application/json">(.*?)</script>`).FindStringSubmatch(html)
	if len(match) != 3 {
		t.Fatal("missing saved data and explicit server outcome")
	}
	if match[1] != a.result.Outcome() {
		t.Fatalf("outcome %s", match[1])
	}
	var got inspection.Report
	if err := json.Unmarshal([]byte(match[2]), &got); err != nil {
		t.Fatal(err)
	}
	if got.Business != a.result.Business || got.Device.Name != a.result.Device.Name {
		t.Fatal("saved values changed")
	}
	if strings.Contains(match[2], "</script>") {
		t.Fatal("unsafe script delimiter")
	}
	for _, file := range []string{"result.js", "saved-report.js", "workspace.js"} {
		w := httptest.NewRecorder()
		a.handler.ServeHTTP(w, httptest.NewRequest("GET", "/"+file, nil))
		if w.Code != 200 {
			t.Fatalf("asset %s: %d", file, w.Code)
		}
	}
}
