package eebus

import (
	"context"
	"errors"
	shipapi "github.com/enbility/ship-go/api"
	"github.com/enbility/ship-go/ship"
	"github.com/gorilla/websocket"
	"sync"
	"testing"
	"time"
)

type closingSocket032 struct {
	once    sync.Once
	closed  chan struct{}
	release chan struct{}
}

func (c *closingSocket032) Close() error {
	c.once.Do(func() {
		close(c.closed)
		if c.release != nil {
			close(c.release)
		}
	})
	return nil
}

type blockedWriter032 struct{ entered, release chan struct{} }

func (w *blockedWriter032) InitDataProcessing(shipapi.WebsocketDataReaderInterface) {}
func (w *blockedWriter032) WriteMessageToWebsocketConnection([]byte) error          { return nil }
func (w *blockedWriter032) IsDataConnectionClosed() (bool, error)                   { return false, nil }
func (w *blockedWriter032) CloseDataConnection(int, string)                         { close(w.entered); <-w.release }
func TestTransport032WaitsForSocketAndCloseCallbacks(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "wait", true: "cancel forces socket close"}[cancelled], func(t *testing.T) {
			e := &Endpoint{alive: true}
			w := &blockedWriter032{make(chan struct{}), make(chan struct{})}
			socket := &closingSocket032{closed: make(chan struct{})}
			if cancelled {
				socket.release = w.release
			}
			transport := &connectionTransport{WebsocketDataWriterInterface: w, socket: socket, done: make(chan struct{}), endpoint: e}
			c := ship.NewConnectionHandler(e, transport, ship.ShipRoleClient, "local", "remote", "")
			e.connections = []*ship.ShipConnection{c}
			e.transports = []*connectionTransport{transport}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- e.DisconnectAndWait(ctx) }()
			<-w.entered
			select {
			case <-result:
				t.Fatal("returned before close callback ended")
			default:
			}
			if cancelled {
				cancel()
			} else {
				close(w.release)
			}
			select {
			case err := <-result:
				if cancelled && !errors.Is(err, context.Canceled) || !cancelled && err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("close did not finish")
			}
			select {
			case <-socket.closed:
			default:
				t.Fatal("socket still open")
			}
		})
	}
}

type errorReader032 struct{ err error }

func (r *errorReader032) HandleIncomingWebsocketMessage([]byte) {}
func (r *errorReader032) ReportConnectionError(err error)       { r.err = err }
func TestTransport032PreservesEarlyCloseBeforeCallback(t *testing.T) {
	trace := newConnectionTrace()
	_ = trace.finish(nil)
	trace.begin("TLS-Handshake")
	_ = trace.finish(nil)
	trace.begin("WebSocket-Upgrade")
	_ = trace.finish(nil)
	trace.begin("SHIP-Handshake")
	e := &Endpoint{alive: true, connectionTrace: trace}
	transport := &connectionTransport{endpoint: e}
	e.transports = []*connectionTransport{transport}
	next := new(errorReader032)
	(transportReader{next: next, transport: transport}).ReportConnectionError(&websocket.CloseError{Code: 1006, Text: "unexpected EOF"})
	rows := trace.snapshot()
	if rows[3].Status != "failed" || next.err == nil {
		t.Fatal(rows, next.err)
	}
}
