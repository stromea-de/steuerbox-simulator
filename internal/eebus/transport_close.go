package eebus

import (
	"context"
	"io"
	"sync"

	shipapi "github.com/enbility/ship-go/api"
	"github.com/enbility/ship-go/ship"
	"github.com/enbility/ship-go/ws"
	"github.com/gorilla/websocket"
)

// Retain the actual socket: the upstream closed flag is set before its socket
// cleanup and can also be set by a read error. Neither proves local TCP closure.
type connectionTransport struct {
	shipapi.WebsocketDataWriterInterface
	socket   io.Closer
	done     chan struct{}
	once     sync.Once
	endpoint *Endpoint
}

func (e *Endpoint) trackTransport(c *websocket.Conn, ski string) *connectionTransport {
	t := &connectionTransport{WebsocketDataWriterInterface: ws.NewWebsocketConnection(c, ski), socket: c, done: make(chan struct{}), endpoint: e}
	// Caller holds e.mu while constructing/assigning the SHIP handler.
	e.transports = append(e.transports, t)
	return t
}
func (t *connectionTransport) closeSocket() {
	t.once.Do(func() {
		if t.socket != nil {
			_ = t.socket.Close()
		}
		close(t.done)
	})
}
func (t *connectionTransport) CloseDataConnection(code int, reason string) {
	defer t.closeSocket()
	t.WebsocketDataWriterInterface.CloseDataConnection(code, reason)
}
func (t *connectionTransport) InitDataProcessing(reader shipapi.WebsocketDataReaderInterface) {
	t.WebsocketDataWriterInterface.InitDataProcessing(transportReader{next: reader, transport: t})
}

type transportReader struct {
	next      shipapi.WebsocketDataReaderInterface
	transport *connectionTransport
}

func (r transportReader) HandleIncomingWebsocketMessage(b []byte) {
	r.next.HandleIncomingWebsocketMessage(b)
}
func (r transportReader) ReportConnectionError(err error) {
	e := r.transport.endpoint
	e.mu.Lock()
	current := len(e.transports) > 0 && e.transports[len(e.transports)-1] == r.transport && e.alive
	trace := e.connectionTrace
	e.mu.Unlock()
	// Record an early 1006 before upstream's close callback invalidates readiness.
	if current && trace != nil {
		_ = trace.finish(err)
	}
	r.next.ReportConnectionError(err)
}

func (e *Endpoint) detachConnections() ([]*ship.ShipConnection, []*connectionTransport) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cs, transports := e.connections, e.transports
	e.connections = nil
	e.transports = nil
	e.alive = false
	e.tlsVerified = false
	e.useCasesReceived = false
	e.remoteSetup = false
	e.remoteConnection = nil
	e.status = "Getrennt"
	return cs, transports
}
func closeTransports(cs []*ship.ShipConnection, transports []*connectionTransport) {
	for _, c := range cs {
		c.CloseConnection(false, 0, "Wiederverbindung")
	}
	for _, t := range transports {
		t.closeSocket()
	}
}

// DisconnectAndWait joins local SHIP close callbacks and socket closure. A peer's
// application cleanup cannot be acknowledged here; ship-check adds its pause.
func (e *Endpoint) DisconnectAndWait(ctx context.Context) error {
	cs, transports := e.detachConnections()
	done := make(chan struct{})
	go func() { defer close(done); closeTransports(cs, transports) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		for _, t := range transports {
			t.closeSocket()
		}
		return ctx.Err()
	}
}
