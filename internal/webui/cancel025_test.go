package webui

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stromea-de/steuerbox-simulator/internal/discovery"
	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

func request025(a *App, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:33287"+path, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Stromea-Token", a.token)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func state025(t *testing.T, a *App) map[string]any {
	t.Helper()
	w := request025(a, "GET", "/api/state", "")
	var v map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestCancelConnection025(t *testing.T) {
	for _, phase := range []string{"tls", "ship", "reconnect"} {
		t.Run(phase, func(t *testing.T) {
			s, err := session.NewController(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := New(s)
			defer a.Close()
			id, err := identity.LoadOrCreate(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			reached := make(chan struct{}, 1)
			var address string
			if phase == "tls" {
				l, e := net.Listen("tcp4", "127.0.0.1:0")
				if e != nil {
					t.Fatal(e)
				}
				defer l.Close()
				address = l.Addr().String()
				go func() {
					c, e := l.Accept()
					if e != nil {
						return
					}
					defer c.Close()
					reached <- struct{}{}
					b := make([]byte, 4096)
					for {
						if _, e = c.Read(b); e != nil {
							return
						}
					}
				}()
			} else {
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					c, e := (&websocket.Upgrader{Subprotocols: []string{"ship"}}).Upgrade(w, r, nil)
					if e != nil {
						return
					}
					defer c.Close()
					reached <- struct{}{}
					for {
						if _, _, e = c.ReadMessage(); e != nil {
							return
						}
					}
				}))
				server.TLS = &tls.Config{Certificates: []tls.Certificate{id.Certificate}, MinVersion: tls.VersionTLS12, ClientAuth: tls.RequireAnyClientCert}
				server.StartTLS()
				defer server.Close()
				address = server.Listener.Addr().String()
			}
			d := discovery.Device{Address: address, SKI: id.SKI}
			if e := s.SelectDevice(d, true); e != nil {
				t.Fatal(e)
			}
			path := "/api/connect"
			if phase == "reconnect" {
				path = "/api/reconnect"
			}
			if w := request025(a, "POST", path, `{"timeout":60000000000}`); w.Code != 202 {
				t.Fatal(w)
			}
			select {
			case <-reached:
			case <-time.After(2 * time.Second):
				t.Fatal("peer not reached")
			}
			v := state025(t, a)
			if v["busy"] != true {
				t.Fatal("expected active connection", v)
			}
			start := time.Now()
			if w := request025(a, "POST", "/api/cancel", `{}`); w.Code != 200 {
				t.Fatal(w)
			}
			if time.Since(start) > time.Second {
				t.Fatal("cancel did not interrupt transport promptly")
			}
			// Repeated GETs represent F5, menu return and window reopen. None may resurrect the job.
			for i := 0; i < 3; i++ {
				v = state025(t, a)
				if v["busy"] != false || v["connected"] != false || v["connectionState"] != "disconnected" || v["kind"] == "connect" || v["error"] != "" {
					t.Fatalf("aborted state restored: %+v", v)
				}
			}
			if w := request025(a, "POST", "/api/select", `{"device":{"address":"127.0.0.1:4711","ski":"`+id.SKI+`"},"confirmed":true}`); w.Code != 200 {
				t.Fatal("new selection blocked", w)
			}
		})
	}
}

func TestCancelCompletionBarrier025(t *testing.T) {
	for _, kind := range []string{"connect", "prepare", "search"} {
		t.Run(kind, func(t *testing.T) {
			a := New(nil)
			defer a.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finished := make(chan struct{})
			release := make(chan struct{})
			if kind == "search" {
				a.searchCancel = cancel
				a.searchDone = finished
				a.search.Active = true
			} else {
				a.kind = kind
				a.busy = true
				a.runCancel = cancel
				a.operationDone = finished
			}
			worker := make(chan struct{})
			go func() {
				defer close(worker)
				<-ctx.Done()
				<-release
				a.mu.Lock()
				a.busy = false
				a.kind = ""
				a.search.Active = false
				a.mu.Unlock()
				close(finished)
			}()
			response := make(chan *httptest.ResponseRecorder, 1)
			go func() { response <- request025(a, "POST", "/api/cancel", `{}`) }()
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("worker not canceled")
			}
			select {
			case <-response:
				t.Fatal("acknowledged before worker stopped")
			default:
			}
			close(release)
			select {
			case w := <-response:
				if w.Code != 200 {
					t.Fatal(w)
				}
			case <-time.After(time.Second):
				t.Fatal("cancel held mutex needed by worker")
			}
			<-worker
			if a.busy || a.kind != "" || a.search.Active {
				t.Fatal("stale operation after cancel acknowledgement")
			}
		})
	}
}

// Exercise the entire cancellation barrier after a real loopback peer has
// acknowledged a control command, including cleanup and the next selection.
func TestCancelRunCleanup025b(t *testing.T) {
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
	a := New(s)
	defer a.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	c := inspection.DefaultDeviceConfig()
	c.Business, c.Inspector = "Test", "Test"
	c.Scope = inspection.ScopeConsumption
	c.StepTimeout = 2*eebus.HeartbeatInterval + time.Second
	c.CleanupTimeout = c.StepTimeout
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if w := request025(a, "POST", "/api/run", string(body)); w.Code != http.StatusAccepted {
		t.Fatal(w)
	}
	if err = eebus.Poll(ctx, func() bool {
		for _, step := range s.Progress().Steps {
			if step.Name == "LPC Grenze + Quittung" && step.Passed {
				return true
			}
		}
		if state025(t, a)["busy"] == false {
			t.Fatalf("run ended before control acknowledgement: %+v", s.Progress())
		}
		return false
	}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if w := request025(a, "POST", "/api/cancel", `{}`); w.Code != http.StatusOK {
		t.Fatal(w)
	}
	elapsed := time.Since(started)
	if elapsed > c.CleanupTimeout {
		t.Fatalf("healthy cleanup exceeded its deadline: %s", elapsed)
	}
	for i := 0; i < 3; i++ {
		v := state025(t, a)
		if v["busy"] != false || v["connected"] != false || v["connectionState"] != "disconnected" || v["error"] != "" {
			t.Fatal("canceled run retained backend state", v)
		}
	}
	a.mu.Lock()
	r := *a.result
	a.mu.Unlock()
	confirmed := 0
	for _, step := range r.Steps {
		switch step.Name {
		case "Grenzen aufheben", "LPC Failsafe-Leistung zurückstellen", "LPC Failsafe-Mindestdauer zurückstellen":
			if !step.Passed {
				t.Fatal("cleanup was not confirmed", step)
			}
			confirmed++
		}
	}
	if confirmed != 3 || r.AbortReason != "user_cancel" || r.Passed {
		t.Fatalf("incomplete canceled report: %+v", r)
	}
	if p := s.Reconnection(); p.Active || p.Attempt != 0 {
		t.Fatal("unexpected retry after explicit cancel", p)
	}
	oldAddress := s.EG.Address()
	body, err = json.Marshal(map[string]any{"device": s.Device, "confirmed": true})
	if err != nil {
		t.Fatal(err)
	}
	if w := request025(a, "POST", "/api/select", string(body)); w.Code != http.StatusOK {
		t.Fatal("new selection blocked after canceled run", w)
	}
	if conn, err := net.DialTimeout("tcp4", oldAddress, time.Second); err == nil {
		conn.Close()
		t.Fatal("replaced session left its listener open")
	}
	t.Logf("cancel completed in %s; release and both restorations confirmed; next selection closed old listener", elapsed)
}
