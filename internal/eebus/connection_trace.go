package eebus

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectionSetupTimeout covers TCP, TLS and the HTTP/WebSocket upgrade together.
// Embedded peers may need several seconds for software ECDHE/ECDSA. User cancel
// still closes the dial socket immediately, independently of this upper bound.
const ConnectionSetupTimeout = 15 * time.Second

// ConnectionTiming is diagnostic evidence, never an inspection verdict. Phases
// not reached are omitted; a live snapshot includes elapsed time for the active phase.
type ConnectionTiming struct {
	Phase      string    `json:"phase"`
	Started    time.Time `json:"started"`
	DurationMS int64     `json:"durationMs"`
	Status     string    `json:"status"` // active, complete, failed, canceled
	Detail     string    `json:"detail,omitempty"`
}

// ConnectionError retains the phase and actual elapsed time as well as the
// underlying cause for errors.Is (in particular timeout versus explicit cancel).
type ConnectionError struct {
	Phase    string
	Duration time.Duration
	Err      error
}

func (e *ConnectionError) Error() string {
	duration := fmt.Sprintf("%.0f s", e.Duration.Seconds())
	if e.Duration < time.Second {
		duration = fmt.Sprintf("%d ms", e.Duration.Milliseconds())
	}
	return fmt.Sprintf("%s nach %s nicht abgeschlossen: %v", e.Phase, duration, e.Err)
}
func (e *ConnectionError) Unwrap() error { return e.Err }

type connectionTrace struct {
	mu   sync.Mutex
	rows []ConnectionTiming
}

func newConnectionTrace() *connectionTrace {
	t := &connectionTrace{}
	t.begin("TCP-Verbindung")
	return t
}
func (t *connectionTrace) begin(phase string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rows = append(t.rows, ConnectionTiming{Phase: phase, Started: time.Now(), Status: "active"})
}
func (t *connectionTrace) finish(err error) error {
	if t == nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.rows) == 0 {
		return err
	}
	row := &t.rows[len(t.rows)-1]
	elapsed := time.Since(row.Started)
	if row.Status != "active" {
		elapsed = time.Duration(row.DurationMS) * time.Millisecond
	}
	if row.Status == "active" {
		row.DurationMS = elapsed.Milliseconds()
		row.Status = "complete"
		if err != nil {
			row.Status = "failed"
			if row.Detail != "" {
				row.Detail += " · "
			}
			row.Detail += err.Error()
		}
		if errors.Is(err, context.Canceled) {
			row.Status = "canceled"
		}
		// Desktop.Run directs the standard logger into its local diagnostic file.
		// Quote protocol errors to keep untrusted text within one log entry; no site data.
		log.Printf("EEBUS Verbindung: phase=%q duration_ms=%d status=%s detail=%q", row.Phase, row.DurationMS, row.Status, row.Detail)
	}
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	return &ConnectionError{Phase: row.Phase, Duration: elapsed, Err: err}
}

func (t *connectionTrace) snapshot() []ConnectionTiming {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	rows := append([]ConnectionTiming(nil), t.rows...)
	for i := range rows {
		if rows[i].Status == "active" {
			rows[i].DurationMS = time.Since(rows[i].Started).Milliseconds()
		}
	}
	return rows
}
func (e *Endpoint) ConnectionTimings() []ConnectionTiming {
	e.mu.Lock()
	trace := e.connectionTrace
	e.mu.Unlock()
	return trace.snapshot()
}

// dialSHIP also interrupts an HTTP upgrade that never answers. The dialer is
// owned by this call; tests can supply an in-memory TCP transport.
func dialSHIP(ctx context.Context, d *websocket.Dialer, url string) (*websocket.Conn, *http.Response, error) {
	return dialSHIPTraced(ctx, d, url, nil)
}
func dialSHIPTraced(ctx context.Context, d *websocket.Dialer, url string, trace *connectionTrace) (*websocket.Conn, *http.Response, error) {
	// Own the same overall deadline as Gorilla so socket timeouts remain identifiable
	// via errors.Is even when a blocked upgrade reports a net.Error instead.
	if d.HandshakeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.HandshakeTimeout)
		defer cancel()
	}
	dial := d.NetDialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	var stopDial func() bool
	d.NetDialContext = func(dialCtx context.Context, network, address string) (net.Conn, error) {
		c, err := dial(dialCtx, network, address)
		if err == nil {
			stopDial = context.AfterFunc(ctx, func() { _ = c.Close() })
			trace.finish(nil)
			if len(url) >= 6 && url[:6] == "wss://" {
				trace.begin("TLS-Handshake")
			} else {
				trace.begin("WebSocket-Upgrade")
			}
		}
		return c, err
	}
	defer func() {
		if stopDial != nil {
			stopDial()
		}
	}()
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
		if err == nil {
			trace.finish(nil)
			trace.begin("WebSocket-Upgrade")
		}
	}})
	c, resp, err := d.DialContext(ctx, url, nil)
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		} else {
			var timeout net.Error
			if errors.As(err, &timeout) && timeout.Timeout() {
				err = fmt.Errorf("%w: %v", context.DeadlineExceeded, err)
			}
		}
	}
	// Our SHIP endpoint requires the subprotocol; count a missing one as an upgrade failure.
	if err == nil && len(d.Subprotocols) == 1 && d.Subprotocols[0] == "ship" && c.Subprotocol() != "ship" {
		_ = c.Close()
		c = nil
		err = errors.New("SHIP-Unterprotokoll fehlt")
	}
	err = trace.finish(err)
	return c, resp, err
}

func (t *connectionTrace) note(detail string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.rows) == 0 {
		return
	}
	r := &t.rows[len(t.rows)-1]
	if r.Detail != "" {
		r.Detail += " · "
	}
	r.Detail += detail
}
