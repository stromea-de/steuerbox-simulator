package webui

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

func connectedDevice040(t *testing.T) (*App, *session.Session, context.Context) {
	t.Helper()
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	if err = s.SelectDevice(s.Device, true); err != nil {
		t.Fatal(err)
	}
	a := New(s, Options{LoopbackOnly: true, LoopbackPartner: true})
	t.Cleanup(a.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	if w := request025(a, "POST", "/api/connect", `{}`); w.Code != http.StatusAccepted {
		t.Fatal(w)
	}
	if err = eebus.Poll(ctx, func() bool { return state025(t, a)["busy"] == false }); err != nil {
		t.Fatal(err)
	}
	if !s.LiveConnected() {
		t.Fatal("loopback device not connected")
	}
	return a, s, ctx
}

// slowPreparation040 replaces the device read with one that lasts until canceled or 400 ms passed.
func slowPreparation040(t *testing.T) {
	previous := prepareRead
	prepareRead = func(ctx context.Context, _ preparationReader, _ *inspection.Capabilities) preparation {
		select {
		case <-ctx.Done():
		case <-time.After(400 * time.Millisecond):
		}
		return preparation{Originals: map[string]inspection.FailsafeValues{}, Values: []preparationValue{}}
	}
	t.Cleanup(func() { prepareRead = previous })
}

// 040 F-04/F-07: "Abbrechen" while reading device values ends only the read. The device stays
// connected (the UI says „Lesen abgebrochen. Lade die Gerätewerte bei Bedarf erneut.“) and the
// operation text never claims that limits are being removed.
func TestCancelReadKeepsConnection040(t *testing.T) {
	a, s, ctx := connectedDevice040(t)
	slowPreparation040(t)
	finished := make(chan int, 1)
	go func() { finished <- request025(a, "POST", "/api/prepare", `{}`).Code }()
	if err := eebus.Poll(ctx, func() bool { return state025(t, a)["kind"] == "prepare" }); err != nil {
		t.Fatal(err)
	}
	if w := request025(a, "POST", "/api/cancel", `{"read":true}`); w.Code != http.StatusOK {
		t.Fatal(w)
	}
	<-finished
	if !s.LiveConnected() {
		t.Fatal("canceling the read disconnected the device")
	}
	v := state025(t, a)
	if v["connected"] != true || strings.Contains(v["operation"].(string), "Grenzen") || strings.Contains(v["operation"].(string), "Abbruch") {
		t.Fatalf("state after read cancel: %v", v)
	}
	// A late cancel for the read (reading already finished) must not disconnect either.
	if w := request025(a, "POST", "/api/cancel", `{"read":true}`); w.Code != http.StatusOK {
		t.Fatal(w)
	}
	if !s.LiveConnected() {
		t.Fatal("late read cancel disconnected the device")
	}
	// The explicit cancel without read scope keeps the 025 disconnect guarantee.
	if w := request025(a, "POST", "/api/cancel", `{}`); w.Code != http.StatusOK || s.LiveConnected() {
		t.Fatalf("plain idle cancel must still disconnect: %d %v", w.Code, s.LiveConnected())
	}
}

// 040 F-04 (UX-G12): reloading the page while reading must neither abort the read nor disconnect.
func TestReloadDuringReadKeepsConnection040(t *testing.T) {
	a, s, ctx := connectedDevice040(t)
	slowPreparation040(t)
	reqCtx, leave := context.WithCancel(context.Background())
	r := httptest.NewRequest("POST", "http://127.0.0.1:33287/api/prepare", bytes.NewBufferString(`{}`)).WithContext(reqCtx)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Stromea-Token", a.token)
	finished := make(chan struct{})
	go func() { a.ServeHTTP(httptest.NewRecorder(), r); close(finished) }()
	if err := eebus.Poll(ctx, func() bool { return state025(t, a)["kind"] == "prepare" }); err != nil {
		t.Fatal(err)
	}
	leave() // tab closed / reloaded
	<-finished
	if !s.LiveConnected() {
		t.Fatal("reload during read disconnected the device")
	}
	a.mu.Lock()
	stored := a.preparation != nil
	a.mu.Unlock()
	if !stored {
		t.Fatal("read result lost after reload")
	}
}
