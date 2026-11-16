package webui

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

func TestLoopback032BoundaryAndRealDiscovery(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, Options{LoopbackOnly: true, LoopbackPartner: true})
	defer a.Close()
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://127.0.0.1:33287"+path, bytes.NewBufferString(body))
		r.Header.Set("X-Stromea-Token", a.token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/interfaces", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"loopbackOnly":true`) || strings.Contains(w.Body.String(), `"en0"`) {
		t.Fatal(w)
	}
	if w := request("POST", "/api/discover-lan", `{"interface":"en0"}`); w.Code != 400 {
		t.Fatal(w)
	}
	w := request("POST", "/api/discover-lan", `{"interface":"loopback"}`)
	var devices []discovery.Device
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &devices) != nil || len(devices) != 1 || devices[0].SKI != s.Device.SKI || devices[0].Address != s.CS.Address() {
		t.Fatal(w)
	}
	for _, addr := range []string{"192.0.2.1:4711", "localhost:4711", "[::ffff:192.0.2.1]:4711"} {
		body, _ := json.Marshal(map[string]any{"device": discovery.Device{Address: addr, SKI: s.Device.SKI}, "confirmed": true})
		if w := request("POST", "/api/select", string(body)); w.Code != 400 {
			t.Fatal(w)
		}
	}
	if s.IsDeviceMode() {
		t.Fatal("Rejected selection changed session")
	}
}
