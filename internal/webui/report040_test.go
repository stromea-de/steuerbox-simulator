package webui

import (
	"context"
	"crypto/sha256"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// 040 F-01: a finished, saved report is immutable. A later reconnect must not append
// events to it or rewrite its file (the handed-out PDF/JSON checksum must stay valid).
func TestReconnectKeepsSavedReport040(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	a := New(s)
	defer a.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	idle := func() {
		t.Helper()
		if err := eebus.Poll(ctx, func() bool { return state025(t, a)["busy"] == false }); err != nil {
			t.Fatal(err)
		}
	}
	body := `{"business":"Test","inspector":"Test","scope":"both","lpc":4200,"lpp":3000,"failsafe":500,"tolerance":50,"testProfile":"quick"}`
	if w := request025(a, "POST", "/api/run", body); w.Code != http.StatusAccepted {
		t.Fatal(w)
	}
	idle()
	a.mu.Lock()
	id, events := a.result.ID, len(a.result.Events)
	a.mu.Unlock()
	path := filepath.Join(s.ReportDir(), id+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	jsonBefore := request025(a, "GET", "/api/report.json", "").Body.Bytes()
	if w := request025(a, "POST", "/api/reconnect", `{}`); w.Code != http.StatusAccepted {
		t.Fatal(w)
	}
	idle()
	if state025(t, a)["connected"] != true {
		t.Fatal("reconnect failed")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("saved report file changed after reconnect")
	}
	jsonAfter := request025(a, "GET", "/api/report.json", "").Body.Bytes()
	if sha256.Sum256(jsonBefore) != sha256.Sum256(jsonAfter) {
		t.Fatal("JSON export changed after reconnect")
	}
	a.mu.Lock()
	n := len(a.result.Events)
	a.mu.Unlock()
	if n != events {
		t.Fatalf("result events changed %d -> %d", events, n)
	}
}

// 040 F-02: a foreign file in the report folder must not break list, delete and correction.
func TestForeignReportFile040(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	a := New(s)
	defer a.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	body := `{"business":"Test","inspector":"Test","scope":"both","lpc":4200,"lpp":3000,"failsafe":500,"tolerance":50,"testProfile":"quick"}`
	if w := request025(a, "POST", "/api/run", body); w.Code != http.StatusAccepted {
		t.Fatal(w)
	}
	if err := eebus.Poll(ctx, func() bool { return state025(t, a)["busy"] == false }); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	id := a.result.ID
	a.mu.Unlock()
	if err := os.WriteFile(filepath.Join(s.ReportDir(), "junk.json"), []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	w := request025(a, "GET", "/api/reports", "")
	if w.Code != 200 || w.Header().Get("X-Stromea-Skipped-Reports") != "1" {
		t.Fatalf("list: %d %q %s", w.Code, w.Header().Get("X-Stromea-Skipped-Reports"), w.Body)
	}
	if w := request025(a, "POST", "/api/report/site?id="+id, `{"customer":"Neu"}`); w.Code != 200 {
		t.Fatalf("correction: %d %s", w.Code, w.Body)
	}
	if w := request025(a, "POST", "/api/reports/delete", `{"id":"`+id+`"}`); w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}
}
