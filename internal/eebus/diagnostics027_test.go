package eebus

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
)

func TestDialPhaseTimeout027(t *testing.T) {
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"TCP-Verbindung", "TLS-Handshake", "WebSocket-Upgrade"} {
		t.Run(phase, func(t *testing.T) {
			trace := newConnectionTrace()
			d := &websocket.Dialer{HandshakeTimeout: 150 * time.Millisecond, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
			done := make(chan struct{})
			d.NetDialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
				if phase == "TCP-Verbindung" {
					defer close(done)
					<-ctx.Done()
					return nil, ctx.Err()
				}
				client, server := net.Pipe()
				go func() {
					defer close(done)
					defer server.Close()
					if phase == "TLS-Handshake" {
						_, _ = io.Copy(io.Discard, server)
						return
					}
					c := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{id.Certificate}})
					if c.Handshake() != nil {
						return
					}
					if _, err := http.ReadRequest(bufio.NewReader(c)); err != nil {
						return
					}
					_, _ = io.Copy(io.Discard, c)
				}()
				return client, nil
			}
			_, _, err := dialSHIPTraced(context.Background(), d, "wss://127.0.0.1:4711/ship/", trace)
			var detail *ConnectionError
			if !errors.As(err, &detail) || detail.Phase != phase || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("wrong phase/cause: %v", err)
			}
			rows := trace.snapshot()
			last := rows[len(rows)-1]
			if last.Phase != phase || last.Status != "failed" || last.DurationMS < 100 || !strings.Contains(err.Error(), phase) {
				t.Fatal(rows, err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("transport leaked")
			}
		})
	}
}

// Actual endpoint dial, deliberately slower than the old three-second budget.
func TestSlowTLSConnect027(t *testing.T) {
	if ConnectionSetupTimeout != 15*time.Second {
		t.Fatal(ConnectionSetupTimeout)
	}
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{Subprotocols: []string{"ship"}}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			if _, _, err = c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{id.Certificate}, GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) { time.Sleep(3200 * time.Millisecond); return nil, nil }}
	server.StartTLS()
	defer server.Close()
	own, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEndpoint(own, false)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err = e.Trust(id.SKI); err != nil {
		t.Fatal(err)
	}
	if err = e.Connect(context.Background(), server.Listener.Addr().String(), id.SKI); err != nil {
		t.Fatal(err)
	}
	rows := e.ConnectionTimings()
	if len(rows) != 4 || rows[1].Phase != "TLS-Handshake" || rows[1].DurationMS < 3200 || rows[2].Status != "complete" || rows[3].Phase != "SHIP-Handshake" {
		t.Fatal(rows)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = e.Ready(ctx)
	var detail *ConnectionError
	if !errors.As(err, &detail) || detail.Phase != "SHIP-Handshake" {
		t.Fatal(err)
	}
	if rows = e.ConnectionTimings(); rows[3].Status != "failed" || rows[3].DurationMS < 50 {
		t.Fatal(rows)
	}
}
