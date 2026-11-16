package eebus

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/enbility/eebus-go/features/client"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
)

type configurationSample struct {
	data []model.DeviceConfigurationKeyValueDataType
	err  error
}
type configurationRequest struct {
	powerRequest
	result chan configurationSample
}

// ReadConfiguration captures only an owned reply to this read from this peer and
// feature. A stale upstream cache or notify cannot satisfy the request.
func (e *Endpoint) ReadConfiguration(ctx context.Context, r spineapi.EntityRemoteInterface) ([]model.DeviceConfigurationKeyValueDataType, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("keine SPINE-Entität")
	}
	dc, err := client.NewDeviceConfiguration(e.Entity, r)
	if err != nil {
		return nil, err
	}
	remote := r.FeatureOfTypeAndRole(model.FeatureTypeTypeDeviceConfiguration, model.RoleTypeServer)
	local := e.Entity.FeatureOfTypeAndRole(model.FeatureTypeTypeDeviceConfiguration, model.RoleTypeClient)
	if remote == nil || local == nil {
		return nil, errors.New("DeviceConfiguration fehlt")
	}
	req := configurationRequest{powerRequest: powerRequest{ski: r.Device().Ski(), source: *remote.Address(), destination: *local.Address()}, result: make(chan configurationSample, 1)}
	e.powerMu.Lock()
	counter, err := dc.RequestKeyValues(nil, nil)
	if err != nil || counter == nil {
		e.powerMu.Unlock()
		if err == nil {
			err = errors.New("Konfigurations-Leseanforderung fehlt")
		}
		return nil, err
	}
	if e.configurationPending == nil {
		e.configurationPending = map[model.MsgCounterType]configurationRequest{}
	}
	e.configurationPending[*counter] = req
	e.powerMu.Unlock()
	defer func() { e.powerMu.Lock(); delete(e.configurationPending, *counter); e.powerMu.Unlock() }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-req.result:
		return result.data, result.err
	}
}
func (e *Endpoint) captureConfiguration(ski string, b []byte) {
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
	req, ok := e.configurationPending[*h.MsgCounterReference]
	if !ok || req.ski != ski || !sameFeature(h.AddressSource, req.source) || !sameFeature(h.AddressDestination, req.destination) {
		return
	}
	result := configurationSample{err: errors.New("Konfigurations-Antwort ohne Werte")}
	cmds := d.Datagram.Payload.Cmd
	if len(cmds) == 1 && cmds[0].DeviceConfigurationKeyValueListData != nil {
		result = configurationSample{data: cmds[0].DeviceConfigurationKeyValueListData.DeviceConfigurationKeyValueData}
	}
	select {
	case req.result <- result:
	default:
	}
}

// WithControlRegistration prevents a fast reply from reaching upstream response
// handling before the newly queued write has registered its result callback.
// The receive boundary uses the same gate as owned read registration.
func (e *Endpoint) WithControlRegistration(register func() error) error {
	e.powerMu.Lock()
	defer e.powerMu.Unlock()
	return register()
}
