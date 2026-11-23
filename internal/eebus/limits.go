package eebus

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/enbility/eebus-go/features/client"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
)

type limitsSample struct {
	data []model.LoadControlLimitDataType
	err  error
}
type limitsRequest struct {
	powerRequest
	result    chan limitsSample
	abandoned bool // R2-04: timed out; the late reply only frees the counter
	// device is the remote device (transport) the read was sent to (R3-05).
	device spineapi.DeviceRemoteInterface
}

// maxLimitsPending bounds counters kept reserved for replies that never came.
const maxLimitsPending = 32

// abandonLimits keeps the counter of a timed-out read reserved until its late reply.
// R4-04: only the waiter's own entry (same result channel) is marked; a newer read
// that reuses the counter on a new transport keeps its reply.
func (e *Endpoint) abandonLimits(counter model.MsgCounterType, result chan limitsSample) {
	e.powerMu.Lock()
	defer e.powerMu.Unlock()
	if req, ok := e.limitsPending[counter]; ok && req.result == result {
		req.abandoned = true
		e.limitsPending[counter] = req
	}
}

// reserveLimitsLocked drops reservations of an earlier transport of the same peer
// and reports whether another read may be sent (R3-05). Their replies can never
// arrive any more; a waiter still blocked on one gets an error. The bound is
// checked before anything is sent, so no sent read is ever left unreserved.
// Caller holds powerMu.
func (e *Endpoint) reserveLimitsLocked(ski string, device spineapi.DeviceRemoteInterface) bool {
	n := 0
	for counter, req := range e.limitsPending {
		if req.ski == ski && req.device != device {
			delete(e.limitsPending, counter)
			if !req.abandoned {
				select {
				case req.result <- limitsSample{err: errors.New("Verbindung neu aufgebaut")}:
				default:
				}
			}
			continue
		}
		n++
	}
	return n < maxLimitsPending
}

// ReadLimits captures only an owned reply to this LoadControl read from this peer
// and feature (236). A cached value or notify cannot prove a released limit.
func (e *Endpoint) ReadLimits(ctx context.Context, r spineapi.EntityRemoteInterface) ([]model.LoadControlLimitDataType, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("keine SPINE-Entität")
	}
	lc, err := client.NewLoadControl(e.Entity, r)
	if err != nil {
		return nil, err
	}
	remote := r.FeatureOfTypeAndRole(model.FeatureTypeTypeLoadControl, model.RoleTypeServer)
	local := e.Entity.FeatureOfTypeAndRole(model.FeatureTypeTypeLoadControl, model.RoleTypeClient)
	if remote == nil || local == nil {
		return nil, errors.New("LoadControl fehlt")
	}
	req := limitsRequest{powerRequest: powerRequest{ski: r.Device().Ski(), source: *remote.Address(), destination: *local.Address()}, result: make(chan limitsSample, 1), device: r.Device()}
	e.powerMu.Lock()
	if !e.reserveLimitsLocked(req.ski, req.device) {
		e.powerMu.Unlock()
		return nil, errors.New("zu viele unbeantwortete Grenzwert-Leseanfragen")
	}
	counter, err := lc.RequestLimitData(nil, nil)
	if err != nil || counter == nil {
		e.powerMu.Unlock()
		if err == nil {
			err = errors.New("Grenzwert-Leseanforderung fehlt")
		}
		return nil, err
	}
	if e.limitsPending == nil {
		e.limitsPending = map[model.MsgCounterType]limitsRequest{}
	}
	// R236-05: spine-go may hand out the counter of an identical outstanding read;
	// never share or replace another waiter's reply.
	if _, busy := e.limitsPending[*counter]; busy {
		e.powerMu.Unlock()
		return nil, errors.New("Grenzwert-Leseanfrage läuft noch")
	}
	e.limitsPending[*counter] = req
	e.powerMu.Unlock()
	select {
	case <-ctx.Done():
		e.abandonLimits(*counter, req.result)
		return nil, ctx.Err()
	case result := <-req.result:
		return result.data, result.err
	}
}

func (e *Endpoint) captureLimits(ski string, b []byte) {
	var d model.Datagram
	if json.Unmarshal(b, &d) != nil {
		return
	}
	h := d.Datagram.Header
	if h.MsgCounterReference == nil || h.CmdClassifier == nil || *h.CmdClassifier != model.CmdClassifierTypeReply {
		return
	}
	e.powerMu.Lock()
	defer e.powerMu.Unlock()
	req, ok := e.limitsPending[*h.MsgCounterReference]
	if !ok || req.ski != ski || !sameFeature(h.AddressSource, req.source) || !sameFeature(h.AddressDestination, req.destination) {
		return
	}
	delete(e.limitsPending, *h.MsgCounterReference) // one reply per read
	if req.abandoned {
		return
	}
	result := limitsSample{err: errors.New("Grenzwert-Antwort ohne Werte")}
	cmds := d.Datagram.Payload.Cmd
	if len(cmds) == 1 && cmds[0].LoadControlLimitListData != nil {
		result = limitsSample{data: cmds[0].LoadControlLimitListData.LoadControlLimitData}
	}
	select {
	case req.result <- result:
	default:
	}
}
