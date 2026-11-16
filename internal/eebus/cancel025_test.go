package eebus

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/enbility/ship-go/ship"
	"github.com/enbility/ship-go/ws"
	"github.com/gorilla/websocket"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
)

// net.Pipe covers real blocking socket reads without a listener or network.
func TestDialCancellation025(t *testing.T) {
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"dial", "tls", "upgrade"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reached := make(chan struct{})
			peerDone := make(chan struct{})
			d := &websocket.Dialer{HandshakeTimeout: ConnectionSetupTimeout, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
			d.NetDialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
				if phase == "dial" {
					close(reached)
					<-ctx.Done()
					close(peerDone)
					return nil, ctx.Err()
				}
				client, server := net.Pipe()
				go func() {
					defer close(peerDone)
					defer server.Close()
					if phase == "tls" {
						close(reached)
						_, _ = io.Copy(io.Discard, server)
						return
					}
					conn := tls.Server(server, &tls.Config{Certificates: []tls.Certificate{id.Certificate}})
					if err := conn.Handshake(); err != nil {
						return
					}
					if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
						return
					}
					close(reached)
					_, _ = io.Copy(io.Discard, conn)
				}()
				return client, nil
			}
			finished := make(chan error, 1)
			trace := newConnectionTrace()
			go func() { _, _, err := dialSHIPTraced(ctx, d, "wss://127.0.0.1:4711/ship/", trace); finished <- err }()
			select {
			case <-reached:
			case <-time.After(time.Second):
				t.Fatal("phase not reached")
			}
			cancel()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) {
					t.Fatal("canceled cause lost", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancel stuck in", phase)
			}
			select {
			case <-peerDone:
			case <-time.After(time.Second):
				t.Fatal("peer socket leaked")
			}
			rows := trace.snapshot()
			last := rows[len(rows)-1]
			wantPhase := map[string]string{"dial": "TCP-Verbindung", "tls": "TLS-Handshake", "upgrade": "WebSocket-Upgrade"}[phase]
			if last.Phase != wantPhase || last.Status != "canceled" || last.DurationMS < 0 {
				t.Fatal("cancellation diagnosis lost", rows)
			}
		})
	}
}

func TestSHIPHandshakeCancellation025(t *testing.T) {
	e := &Endpoint{deviceMode: true, alive: true}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.Ready(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("handshake wait ignored cancel")
	}
	e.Disconnect()
	if e.Alive() || e.ConnectionTrust().Simulator || e.ConnectionState() != "Getrennt" {
		t.Fatal("aborted handshake retained state")
	}
}

// A successful upgrade must survive Gorilla canceling its internal handshake
// context. Ownership passes to the session after dialSHIP returns.
func TestSuccessfulDialKeepsTransport025(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		req, err := http.ReadRequest(bufio.NewReader(server))
		if err != nil {
			return
		}
		digest := sha1.Sum([]byte(req.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		_, err = fmt.Fprintf(server, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(digest[:]))
		if err == nil {
			_, _ = io.Copy(io.Discard, server)
		}
	}()
	d := &websocket.Dialer{HandshakeTimeout: time.Second, NetDialContext: func(context.Context, string, string) (net.Conn, error) { return client, nil }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _, err := dialSHIP(ctx, d, "ws://127.0.0.1:4711/ship/")
	if err != nil {
		t.Fatal(err)
	}
	cancel() // dial watcher is detached; only session cancellation owns this now
	_ = c.SetWriteDeadline(time.Now().Add(time.Second))
	if err := c.WriteMessage(websocket.BinaryMessage, []byte{0}); err != nil {
		t.Fatal("successful transport prematurely closed", err)
	}
	_ = c.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("socket leaked")
	}
}

// Exercise the actual SHIP state machine on a WebSocket backed by net.Pipe.
// The peer accepts the upgrade but never answers CMI/HELLO.
func TestSHIPTransportCancellation025(t *testing.T) {
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	defer server.Close()
	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		r, err := http.ReadRequest(bufio.NewReader(server))
		if err != nil {
			return
		}
		digest := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		if _, err = fmt.Fprintf(server, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", base64.StdEncoding.EncodeToString(digest[:])); err != nil {
			return
		}
		_, _ = io.Copy(io.Discard, server)
	}()
	d := &websocket.Dialer{HandshakeTimeout: time.Second, NetDialContext: func(context.Context, string, string) (net.Conn, error) { return client, nil }}
	c, _, err := dialSHIP(context.Background(), d, "ws://127.0.0.1:4711/ship/")
	if err != nil {
		t.Fatal(err)
	}
	e := &Endpoint{Identity: id, deviceMode: true, peer: id.SKI, alive: true, tlsVerified: true}
	callbacks := &handshakeCallbacks{Endpoint: e}
	e.mu.Lock()
	h := ship.NewConnectionHandler(callbacks, ws.NewWebsocketConnection(c, id.SKI), ship.ShipRoleClient, id.SKI, id.SKI, "")
	callbacks.connection = h
	e.connections = append(e.connections, h)
	e.mu.Unlock()
	h.Run()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.Ready(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("SHIP wait stuck")
	}
	e.Disconnect()
	if e.Alive() || e.ConnectionTrust().Device || e.ConnectionState() != "Getrennt" {
		t.Fatal("SHIP state retained")
	}
	select {
	case <-peerDone:
	case <-time.After(5 * time.Second): // 045: generous bound for loaded CI runners; the close itself is immediate
		t.Fatal("SHIP transport left open")
	}
}
