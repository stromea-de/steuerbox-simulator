package webui

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// 041 (Nutzer 2026-11-14): Export/Import, „Simulator beenden“, Versionen „v2 · v1“, eine Zeile je Prüfung.

func practiceReport041(t *testing.T, a *App) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	body := `{"business":"Test","inspector":"Test","scope":"both","lpc":4200,"lpp":3000,"failsafe":500,"tolerance":50,"testProfile":"quick"}`
	if w := request025(a, "POST", "/api/run", body); w.Code != http.StatusAccepted {
		t.Fatal(w.Body.String())
	}
	if err := eebus.Poll(ctx, func() bool { return state025(t, a)["busy"] == false }); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.result.ID
}

func newApp041(t *testing.T) (*App, *session.Session, string) {
	t.Helper()
	state := t.TempDir()
	s, err := session.New(state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	a := New(s)
	t.Cleanup(a.Close)
	return a, s, state
}

func rawRequest041(a *App, method, path, contentType string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:33287"+path, bytes.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	r.Header.Set("X-Stromea-Token", a.token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}

func TestExportImportAPI041(t *testing.T) {
	a, _, _ := newApp041(t)
	id := practiceReport041(t, a)
	jsonBefore := request025(a, "GET", "/api/report.json?id="+id, "").Body.Bytes()
	profile := `{"profile":{"business":"Elektro Muster","country":"Deutschland","inspector":"Erika"}}`
	w := request025(a, "POST", "/api/export", profile)
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("export: %d %v", w.Code, w.Header())
	}
	archive := w.Body.Bytes()
	z, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range z.File {
		found[f.Name] = true
	}
	for _, name := range []string{"manifest.json", "profile.json", "identity/energyguard.pem", "reports/" + id + ".json"} {
		if !found[name] {
			t.Fatalf("missing %s in %v", name, found)
		}
	}
	if w := rawRequest041(a, "POST", "/api/export", "application/json", []byte(`{"profile":{"logo":"javascript:x"}}`)); w.Code != 400 {
		t.Fatalf("invalid profile must be rejected: %d", w.Code)
	}
	// Import into a second installation: reports appear, checksums stay valid, profile is returned.
	b, s2, _ := newApp041(t)
	_, skiBefore, _, _, _ := s2.Snapshot()
	w = rawRequest041(b, "POST", "/api/import?reports=1&profile=1&identity=1", "application/zip", archive)
	if w.Code != 200 {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}
	var res struct {
		Imported, Skipped int
		Profile           map[string]string
		Identity          bool
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if res.Imported != 1 || res.Profile["business"] != "Elektro Muster" || !res.Identity {
		t.Fatalf("import result %s", w.Body.String())
	}
	if after := request025(b, "GET", "/api/report.json?id="+id, "").Body.Bytes(); !bytes.Equal(after, jsonBefore) {
		t.Fatal("imported report JSON (and its checksum) changed")
	}
	_, skiAfter, _, _, _ := s2.Snapshot()
	_, skiA, _, _, _ := a.s.Snapshot()
	if skiAfter == skiBefore || skiAfter != skiA {
		t.Fatal("simulator identity not imported")
	}
	// Second import: duplicates are skipped.
	w = rawRequest041(b, "POST", "/api/import?reports=1", "application/zip", archive)
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if w.Code != 200 || res.Imported != 0 || res.Skipped != 1 {
		t.Fatalf("duplicate import: %s", w.Body.String())
	}
	// Junk and wrong content type are rejected, nothing selected is rejected.
	if w := rawRequest041(b, "POST", "/api/import?reports=1", "application/zip", []byte("junk")); w.Code != 400 {
		t.Fatalf("junk: %d", w.Code)
	}
	if w := rawRequest041(b, "POST", "/api/import?reports=1", "text/plain", archive); w.Code != 415 {
		t.Fatalf("content type: %d", w.Code)
	}
	if w := rawRequest041(b, "POST", "/api/import", "application/zip", archive); w.Code != 400 {
		t.Fatalf("no selection: %d", w.Code)
	}
	// Token check applies like every POST.
	r := httptest.NewRequest("POST", "http://127.0.0.1:33287/api/import?reports=1", bytes.NewReader(archive))
	r.Header.Set("Content-Type", "application/zip")
	rec := httptest.NewRecorder()
	b.ServeHTTP(rec, r)
	if rec.Code != 403 {
		t.Fatalf("import without token: %d", rec.Code)
	}
}

func TestImportRefusedWhileBusy041(t *testing.T) {
	a, _, _ := newApp041(t)
	a.mu.Lock()
	a.busy = true
	a.mu.Unlock()
	if w := rawRequest041(a, "POST", "/api/import?reports=1", "application/zip", []byte("PK")); w.Code != 409 {
		t.Fatalf("import during a run: %d", w.Code)
	}
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()
}

func TestShutdown041(t *testing.T) {
	var called atomic.Int32
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, Options{Shutdown: func() { called.Add(1) }})
	defer a.Close()
	if w := request025(a, "GET", "/api/shutdown", ""); w.Code != 405 {
		t.Fatalf("GET must not stop the simulator: %d", w.Code)
	}
	w := request025(a, "POST", "/api/shutdown", `{}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("shutdown: %d %s", w.Code, w.Body.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := eebus.Poll(ctx, func() bool { return called.Load() == 1 }); err != nil {
		t.Fatal("shutdown hook not called")
	}
	// Review R041-04: once shutting down, no new operation or change may start.
	for path, body := range map[string]string{"/api/run": `{}`, "/api/simulator": `{}`, "/api/select": `{"device":{"ski":"0123456789abcdef0123456789abcdef01234567","address":"127.0.0.1:1"},"confirmed":true}`, "/api/connect": `{}`, "/api/prepare": `{}`, "/api/discover-lan": `{"interface":"loopback"}`, "/api/report/site?id=X": `{}`, "/api/reports/delete": `{"id":"X"}`, "/api/identity/reset": `{"confirmed":true}`} {
		if w := request025(a, "POST", path, body); w.Code != http.StatusConflict {
			t.Errorf("%s after shutdown: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := rawRequest041(a, "POST", "/api/import?reports=1", "application/zip", []byte("PK")); w.Code != http.StatusConflict {
		t.Errorf("import after shutdown: %d", w.Code)
	}
	// Without an owner-provided hook (tests, embedded use) the endpoint refuses instead of exiting.
	b := New(s)
	defer b.Close()
	if w := request025(b, "POST", "/api/shutdown", `{}`); w.Code != http.StatusNotImplemented {
		t.Fatalf("shutdown without hook: %d", w.Code)
	}
}

// A running inspection is aborted first (limits lifted by the run's own cleanup), then the hook runs.
func TestShutdownAbortsRun041(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	type seen struct {
		busy     bool
		released bool
		steps    []string
	}
	result := make(chan seen, 1)
	var a *App
	// The hook records what the shutdown waited for: the run has ended and lifted its limits (Review R041-08).
	a = New(s, Options{Shutdown: func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		v := seen{busy: a.busy}
		if a.result != nil {
			for _, step := range a.result.Steps {
				v.steps = append(v.steps, step.Name)
				if step.Name == "Grenzen aufheben" && step.Passed {
					v.released = true
				}
			}
		}
		result <- v
	}})
	defer a.Close()
	body := `{"business":"Test","inspector":"Test","scope":"both","lpc":4200,"lpp":3000,"failsafe":500,"tolerance":50,"testProfile":"quick"}`
	if w := request025(a, "POST", "/api/run", body); w.Code != http.StatusAccepted {
		t.Fatal(w.Body.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Wait until a test limit is actually active before quitting.
	if err := eebus.Poll(ctx, func() bool {
		for _, l := range s.Progress().Limits {
			if l.Watts != nil {
				return true
			}
		}
		return false
	}); err != nil {
		t.Fatal("no active limit before shutdown")
	}
	w := request025(a, "POST", "/api/shutdown", `{}`)
	var v map[string]bool
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if w.Code != http.StatusAccepted || !v["aborting"] {
		t.Fatalf("shutdown during run: %d %s", w.Code, w.Body.String())
	}
	select {
	case got := <-result:
		if got.busy || !got.released {
			t.Fatalf("hook ran before the run lifted its limits: %+v", got)
		}
	case <-ctx.Done():
		t.Fatal("shutdown hook not called after the abort")
	}
}

// Review R041-03: while an import is reserved, state stays responsive and changes are refused.
func TestImportReservationBlocksChanges041(t *testing.T) {
	a, _, _ := newApp041(t)
	a.mu.Lock()
	a.importing = true
	a.mu.Unlock()
	if w := request025(a, "GET", "/api/state", ""); w.Code != 200 {
		t.Fatal("state blocked during import")
	}
	for path, body := range map[string]string{"/api/run": `{}`, "/api/identity/reset": `{"confirmed":true}`, "/api/reports/delete": `{"id":"X"}`} {
		if w := request025(a, "POST", path, body); w.Code != http.StatusConflict {
			t.Errorf("%s during import: %d", path, w.Code)
		}
	}
	a.mu.Lock()
	a.importing = false
	a.mu.Unlock()
}

func TestVersionsAndLatestCorrection041(t *testing.T) {
	a, s, _ := newApp041(t)
	v1 := practiceReport041(t, a)
	w := request025(a, "POST", "/api/report/site?id="+v1, `{"customer":"Kunde A"}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var v2 struct{ ID string }
	_ = json.Unmarshal(w.Body.Bytes(), &v2)
	// Only the newest version may be corrected.
	if w := request025(a, "POST", "/api/report/site?id="+v1, `{"customer":"Kunde B"}`); w.Code != http.StatusConflict {
		t.Fatalf("correction of an older version: %d", w.Code)
	}
	for _, id := range []string{v1, v2.ID} {
		w := request025(a, "GET", "/api/report/versions?id="+id, "")
		var versions []struct {
			ID     string
			Number int
		}
		_ = json.Unmarshal(w.Body.Bytes(), &versions)
		if w.Code != 200 || len(versions) != 2 || versions[0].ID != v2.ID || versions[0].Number != 2 || versions[1].ID != v1 || versions[1].Number != 1 {
			t.Fatalf("versions of %s: %s", id, w.Body.String())
		}
	}
	// The list shows one row per inspection: the newest version.
	w = request025(a, "GET", "/api/reports", "")
	var list []struct {
		ID       string
		Versions int
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != v2.ID || list[0].Versions != 2 {
		t.Fatalf("list: %s", w.Body.String())
	}
	page := request025(a, "GET", "/protokolle", "").Body.String()
	if strings.Count(page, `class="report-row"`) != 1 || strings.Contains(page, "Frühere Version") || strings.Contains(page, "Berichtsversion") {
		t.Fatal("server-rendered list must show one row without version links")
	}
	saved := request025(a, "GET", "/protokoll?id="+v1, "").Body.String()
	if !strings.Contains(saved, `href="/protokoll?id=`+v2.ID+`">v2</a>`) || !strings.Contains(saved, `<strong aria-current="page">v1</strong>`) || strings.Contains(saved, "Frühere Version") {
		t.Fatal("saved report without JavaScript shows v2 · v1 switcher")
	}
	_ = os.Remove(filepath.Join(s.ReportDir(), v2.ID+".json"))
	if w := request025(a, "POST", "/api/report/site?id="+v1, `{"customer":"Kunde C"}`); w.Code != 200 {
		t.Fatalf("after deleting v2, v1 is the newest again: %d", w.Code)
	}
}

// Review R041-10: the shutdown waits for a running import (commit or rollback) before the service ends.
func TestShutdownWaitsForImport041(t *testing.T) {
	var called atomic.Int32
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, Options{Shutdown: func() { called.Add(1) }})
	defer a.Close()
	done := make(chan struct{})
	a.mu.Lock()
	a.importing, a.importDone = true, done
	a.mu.Unlock()
	if w := request025(a, "POST", "/api/shutdown", `{}`); w.Code != http.StatusAccepted {
		t.Fatal(w.Code)
	}
	time.Sleep(300 * time.Millisecond)
	if called.Load() != 0 {
		t.Fatal("shutdown hook ran during the import")
	}
	close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := eebus.Poll(ctx, func() bool { return called.Load() == 1 }); err != nil {
		t.Fatal("shutdown hook not called after the import")
	}
}
