package webui

import (
	"context"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
	"testing"
	"time"
)

func TestStopDiscovery028PreservesConnectedRun(t *testing.T) {
	s, err := session.NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s)
	defer a.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	a.searchCancel = cancel
	a.searchDone = done
	a.search.Active = true
	a.connected = true
	a.busy = true
	a.kind = "run"
	go func() {
		<-ctx.Done()
		a.mu.Lock()
		a.search.Active = false
		a.searchCancel = nil
		a.mu.Unlock()
		close(done)
	}()
	result := make(chan int, 1)
	go func() { result <- request025(a, "POST", "/api/discover-stop", "{}").Code }()
	select {
	case code := <-result:
		if code != 200 {
			t.Fatal(code)
		}
	case <-time.After(time.Second):
		t.Fatal("search did not stop")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.connected || !a.busy || a.kind != "run" || a.search.Active {
		t.Fatal("search stop changed inspection/connection")
	}
}

func TestSelection028KeepsDiscoveryRoute(t *testing.T) {
	s, err := session.NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s)
	defer a.Close()
	response := request025(a, "POST", "/api/select", `{"device":{"address":"127.0.0.1:4711","ski":"0123456789abcdef0123456789abcdef01234567","interface":"loop-test","localAddress":"127.0.0.1"},"confirmed":true}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	device := state025(t, a)["device"].(map[string]any)
	if device["interface"] != "loop-test" || device["localAddress"] != "127.0.0.1" {
		t.Fatal(device)
	}
}
