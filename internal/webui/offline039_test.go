package webui

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// R039-01: without a network, the server refuses every device-mode pairing path; the practice path stays open.
func TestOfflineRefusesPairing039(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, Options{LoopbackOnly: true}) // no test partner: this server has no network
	defer a.Close()
	request := func(path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1:33287"+path, bytes.NewBufferString(body))
		r.Header.Set("X-Stromea-Token", a.token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	if w := request("/api/discover-lan", `{"interface":"loopback"}`); w.Code != 409 {
		t.Fatalf("discover-lan offline: HTTP %d %s", w.Code, w.Body.String())
	}
	body, _ := json.Marshal(map[string]any{"device": discovery.Device{Address: s.CS.Address(), SKI: s.Device.SKI}, "confirmed": true})
	if w := request("/api/select", string(body)); w.Code != 409 {
		t.Fatalf("select offline: HTTP %d %s", w.Code, w.Body.String())
	}
	if s.IsDeviceMode() {
		t.Fatal("refused selection changed the session")
	}
	for _, path := range []string{"/api/connect", "/api/reconnect"} {
		if err := s.SelectDevice(discovery.Device{Address: s.CS.Address(), SKI: s.Device.SKI}, true); err != nil {
			t.Fatal(err)
		}
		if w := request(path, `{}`); w.Code != 409 {
			t.Fatalf("%s offline in device mode: HTTP %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := request("/api/simulator", `{}`); w.Code >= 400 {
		t.Fatalf("practice mode offline: HTTP %d %s", w.Code, w.Body.String())
	}
}
