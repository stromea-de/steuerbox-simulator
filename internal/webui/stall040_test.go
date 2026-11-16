package webui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// 040 F-03: a local client that announces a body and never sends it must not freeze the UI.
// /api/run reads its body before taking App.mu, so /api/state keeps answering.
func TestStalledRunBodyDoesNotBlockState040(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s)
	defer a.Close()
	body, writer := io.Pipe()
	defer writer.Close()
	r := httptest.NewRequest("POST", "http://127.0.0.1:33287/api/run", body)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Stromea-Token", a.token)
	go a.ServeHTTP(httptest.NewRecorder(), r)
	time.Sleep(100 * time.Millisecond)
	done := make(chan int, 1)
	go func() { done <- request025(a, "GET", "/api/state", "").Code }()
	select {
	case code := <-done:
		if code != http.StatusOK {
			t.Fatalf("state: %d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("/api/state blocked by a stalled /api/run body")
	}
}

// 040 F-05: the API rejects sub-second timeouts (1 ns used to be accepted).
func TestRunRejectsTinyTimeout040(t *testing.T) {
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
	w := request025(a, "POST", "/api/run", `{"business":"T","inspector":"T","scope":"both","lpc":4200,"lpp":3000,"failsafe":500,"tolerance":50,"stepTimeout":1}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("1 ns step timeout: HTTP %d %s", w.Code, w.Body)
	}
}
