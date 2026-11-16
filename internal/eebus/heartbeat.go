package eebus

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	shipapi "github.com/enbility/ship-go/api"
	"github.com/enbility/spine-go/model"
)

type heartbeatReceipt struct {
	ski    string
	header model.HeaderType
	at     time.Time
	err    error
}
type heartbeatWriter struct {
	endpoint *Endpoint
	ski      string
	next     shipapi.ShipConnectionDataWriterInterface
}

// Request a SPINE result for heartbeat notifications. A result received locally
// is optional diagnostic information and never gates or shortens the outage test.
// It bounds the peer's receive time from above; never substitute the peer's clock
// or its heartbeat in the opposite direction. No payload values are changed.
func (w heartbeatWriter) WriteShipMessageWithPayload(b []byte) {
	var d model.Datagram
	if !w.endpoint.deviceMode || json.Unmarshal(b, &d) != nil {
		w.next.WriteShipMessageWithPayload(b)
		return
	}
	heartbeat := false
	for _, c := range d.Datagram.Payload.Cmd {
		if c.DeviceDiagnosisHeartbeatData != nil {
			heartbeat = true
		}
	}
	h := d.Datagram.Header
	if !heartbeat || h.CmdClassifier == nil || *h.CmdClassifier != model.CmdClassifierTypeNotify || h.MsgCounter == nil {
		w.next.WriteShipMessageWithPayload(b)
		return
	}
	e := w.endpoint
	e.heartbeatMu.Lock()
	defer e.heartbeatMu.Unlock()
	// Stop closes this gate before stopping the upstream timer, so an already
	// scheduled timer tick cannot reset the receive timer during the outage.
	if e.heartbeatBlocked {
		return
	}
	ack := true
	d.Datagram.Header.AckRequest = &ack
	owned, err := json.Marshal(d)
	if err != nil {
		return
	}
	e.heartbeatPending = &heartbeatReceipt{ski: w.ski, header: h}
	w.next.WriteShipMessageWithPayload(owned)
}
func (e *Endpoint) captureHeartbeatReceipt(ski string, b []byte) {
	var d model.Datagram
	if json.Unmarshal(b, &d) != nil {
		return
	}
	h := d.Datagram.Header
	if h.CmdClassifier == nil || *h.CmdClassifier != model.CmdClassifierTypeResult || h.MsgCounterReference == nil {
		return
	}
	e.heartbeatMu.Lock()
	defer e.heartbeatMu.Unlock()
	p := e.heartbeatPending
	if p == nil || !p.at.IsZero() || p.err != nil || p.ski != ski || *h.MsgCounterReference != *p.header.MsgCounter || p.header.AddressDestination == nil || p.header.AddressSource == nil || !sameFeature(h.AddressSource, *p.header.AddressDestination) || !sameFeature(h.AddressDestination, *p.header.AddressSource) {
		return
	}
	cmds := d.Datagram.Payload.Cmd
	if len(cmds) != 1 || cmds[0].ResultData == nil || cmds[0].ResultData.ErrorNumber == nil {
		return
	}
	if *cmds[0].ResultData.ErrorNumber != 0 {
		p.err = errors.New("Heartbeat-Empfang negativ quittiert")
	} else {
		p.at = time.Now()
	}
}
func (e *Endpoint) LastHeartbeatReceipt() time.Time {
	e.heartbeatMu.Lock()
	defer e.heartbeatMu.Unlock()
	if e.heartbeatPending == nil {
		return time.Time{}
	}
	return e.heartbeatPending.at
}

// StopHeartbeat closes the send gate before stopping the upstream timer.
// Do not wait for a result: Notify acknowledgments are optional for this test,
// and waiting would consume observation time without proving electrical effect.
func (e *Endpoint) StopHeartbeat(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.heartbeatMu.Lock()
	e.heartbeatBlocked = true
	e.heartbeatMu.Unlock()
	e.Entity.HeartbeatManager().StopHeartbeat()
	return nil
}
func (e *Endpoint) AllowHeartbeats() {
	e.heartbeatMu.Lock()
	e.heartbeatBlocked = false
	e.heartbeatMu.Unlock()
}
