package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// Re-Review 1, R039-01: after a previous device connection with cached capabilities, losing the network must
// also block the inspection start and every internal connection setup (central gate in the session).
func TestOfflineBlocksRunAfterEarlierConnection039(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	if err = s.SelectDevice(s.Device, true); err != nil {
		t.Fatal(err)
	}
	a := New(s, Options{LoopbackOnly: true, LoopbackPartner: true})
	defer a.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if w := request025(a, "POST", "/api/connect", `{}`); w.Code != http.StatusAccepted {
		t.Fatal(w)
	}
	if err = eebus.Poll(ctx, func() bool { return state025(t, a)["busy"] == false }); err != nil {
		t.Fatal(err)
	}
	if v := state025(t, a); v["connected"] != true {
		t.Fatal("loopback connection failed", v)
	}
	// The network disappears (same effect as the availability rule turning false) and the link drops.
	a.loopbackPartner.Store(false)
	s.Disconnect()
	c := inspection.DefaultDeviceConfig()
	c.Business, c.Inspector = "Test", "Test"
	c.Scope = inspection.ScopeConsumption
	body, _ := json.Marshal(c)
	if w := request025(a, "POST", "/api/run", string(body)); w.Code != http.StatusConflict {
		t.Fatalf("run offline: HTTP %d %s", w.Code, w.Body.String())
	}
	// Central gate: no internal path may dial the device while offline.
	err = s.Connect(ctx)
	if err == nil || !strings.Contains(err.Error(), "Keine Netzwerkverbindung") {
		t.Fatalf("session connect offline: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if s.LiveConnected() {
		t.Fatal("device connection was rebuilt while offline")
	}
}
