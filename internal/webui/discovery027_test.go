package webui

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// No mDNS/LAN: stand in for a pending search worker at its cancellation barrier.
func TestSelectDuringSearch027(t *testing.T) {
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
	go func() {
		<-ctx.Done()
		a.mu.Lock()
		a.search.Active = false
		a.searchCancel = nil
		a.mu.Unlock()
		close(done)
	}()
	response := make(chan int, 1)
	go func() {
		response <- request025(a, "POST", "/api/select", `{"device":{"address":"127.0.0.1:4711","ski":"0123456789abcdef0123456789abcdef01234567"},"confirmed":true}`).Code
	}()
	select {
	case code := <-response:
		if code != http.StatusOK {
			t.Fatalf("selection blocked during search: %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("selection waited for search deadline")
	}
	select {
	case <-done:
	default:
		t.Fatal("selection did not join old search")
	}
	if !s.Paired() {
		t.Fatal("selection missing")
	}
}

func TestConnectionTimeoutKeepsPhase027(t *testing.T) {
	s, err := session.NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s)
	defer a.Close()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		_, _ = io.Copy(io.Discard, c)
	}()
	body := fmt.Sprintf(`{"device":{"address":%q,"ski":"0123456789abcdef0123456789abcdef01234567"},"confirmed":true}`, l.Addr().String())
	if w := request025(a, "POST", "/api/select", body); w.Code != 200 {
		t.Fatal(w)
	}
	if w := request025(a, "POST", "/api/connect", `{"timeout":150000000}`); w.Code != 202 {
		t.Fatal(w)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for {
		v := state025(t, a)
		if v["busy"] == false {
			if !strings.Contains(v["error"].(string), "TLS-Handshake") || v["reconnection"].(map[string]any)["timedOut"] != true {
				t.Fatal(v)
			}
			rows := v["connectionTimings"].([]any)
			if len(rows) != 2 || rows[1].(map[string]any)["status"] != "failed" {
				t.Fatal(rows)
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("timeout worker stuck")
		case <-time.After(10 * time.Millisecond):
		}
	}
	select {
	case <-peerDone:
	case <-ctx.Done():
		t.Fatal("socket leaked")
	}
}

func TestCancelSearchDisconnects027(t *testing.T) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	a := New(s)
	defer a.Close()
	searchCtx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan struct{})
	a.searchCancel = stop
	a.searchDone = done
	a.search.Active = true
	go func() {
		<-searchCtx.Done()
		a.mu.Lock()
		a.search.Active = false
		a.searchCancel = nil
		a.mu.Unlock()
		close(done)
	}()
	if w := request025(a, "POST", "/api/cancel", `{}`); w.Code != 200 {
		t.Fatal(w)
	}
	if s.LiveConnected() {
		t.Fatal("explicit cancel retained old connection")
	}
}
