package eebus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/enbility/eebus-go/features/client"
	shipapi "github.com/enbility/ship-go/api"
	"github.com/enbility/ship-go/ship"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
)

type powerSample struct {
	watts float64
	err   error
}
type powerRequest struct {
	ski                 string
	source, destination model.FeatureAddressType
	samples             chan powerSample
	measurementID       *model.MeasurementIdType
	direction           float64
}

// Own the reply bytes across delegation to the mutable upstream cache.
// Upstream response callbacks run asynchronously and cannot own that data.
type measurementReader struct {
	endpoint   *Endpoint
	ski        string
	next       shipapi.ShipConnectionDataReaderInterface
	connection *ship.ShipConnection
}

func (r measurementReader) HandleShipPayloadMessage(b []byte) {
	owned := append([]byte(nil), b...)
	stackLifecycle.RLock()
	defer stackLifecycle.RUnlock()
	r.endpoint.spineMu.RLock()
	defer r.endpoint.spineMu.RUnlock()
	if r.connection != nil {
		r.endpoint.mu.Lock()
		current := r.endpoint.currentConnectionLocked(r.connection)
		r.endpoint.mu.Unlock()
		if !current {
			return
		}
	}
	// Sender.Request records its outstanding request after enqueueing bytes.
	// Wait for our RequestData call to finish before upstream clears that record.
	r.endpoint.powerMu.Lock()
	r.endpoint.powerMu.Unlock()
	r.next.HandleShipPayloadMessage(b)
	// Finish upstream bookkeeping before waking the next request/cleanup.
	r.endpoint.captureHeartbeatReceipt(r.ski, owned)
	r.endpoint.capturePower(r.ski, owned)
	r.endpoint.captureConfiguration(r.ski, owned)
	r.endpoint.captureDiscovery(owned)
}

func sameFeature(a *model.FeatureAddressType, b model.FeatureAddressType) bool {
	return a != nil && a.Feature != nil && b.Feature != nil && *a.Feature == *b.Feature && reflect.DeepEqual(a.Entity, b.Entity)
}
func (e *Endpoint) capturePower(ski string, b []byte) {
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
	req, ok := e.powerPending[*h.MsgCounterReference]
	if !ok || req.ski != ski || !sameFeature(h.AddressSource, req.source) || !sameFeature(h.AddressDestination, req.destination) {
		return
	}
	result := powerSample{err: errors.New("MPC-Antwort ohne eindeutigen Leistungswert")}
	cmd := d.Datagram.Payload.Cmd
	if len(cmd) == 1 && cmd[0].MeasurementListData != nil {
		data := cmd[0].MeasurementListData.MeasurementData
		if req.measurementID == nil {
			if len(data) == 1 && data[0].Value != nil && data[0].Value.Number != nil {
				result = powerSample{watts: data[0].Value.GetValue()}
			}
		} else {
			count := 0
			for _, item := range data {
				if item.MeasurementId != nil && *item.MeasurementId == *req.measurementID && item.Value != nil && item.Value.Number != nil {
					count++
					result = powerSample{watts: item.Value.GetValue() * req.direction}
				}
			}
			if count != 1 {
				result = powerSample{err: errors.New("MPC-Antwort ohne eindeutige Gesamtwirkleistung")}
			}
		}
	}
	select {
	case req.samples <- result:
	default:
	}
}

// ReadPower requires a reply from the paired peer to this exact fresh SPINE read.
// No cache value or subscription notification can satisfy it.
func (e *Endpoint) ReadPower(ctx context.Context) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r := e.RemoteEntity()
	if e.deviceMode {
		r = e.EntityForUseCase("MPC")
	}
	if r == nil {
		return 0, errors.New("keine Verbindung")
	}
	m, err := client.NewMeasurement(e.Entity, r)
	if err != nil {
		return 0, err
	}
	remote := r.FeatureOfTypeAndRole(model.FeatureTypeTypeMeasurement, model.RoleTypeServer)
	local := e.Entity.FeatureOfTypeAndRole(model.FeatureTypeTypeMeasurement, model.RoleTypeClient)
	if remote == nil || local == nil {
		return 0, errors.New("MPC-Feature fehlt")
	}
	req := powerRequest{ski: r.Device().Ski(), source: *remote.Address(), destination: *local.Address(), samples: make(chan powerSample, 1)}
	if e.deviceMode {
		id, direction, err := e.powerMetadata(r)
		if err != nil {
			return 0, err
		}
		req.measurementID = &id
		req.direction = direction
	}

	// Registration and request send share the lock with the synchronous receiver,
	// so even an immediate loopback reply cannot overtake waiter registration.
	e.powerMu.Lock()
	counter, err := m.RequestData(nil, nil)
	if err != nil || counter == nil {
		e.powerMu.Unlock()
		if err == nil {
			err = errors.New("MPC-Leseanforderung fehlt")
		}
		return 0, err
	}
	if e.powerPending == nil {
		e.powerPending = map[model.MsgCounterType]powerRequest{}
	}
	e.powerPending[*counter] = req
	e.powerMu.Unlock()
	defer func() { e.powerMu.Lock(); delete(e.powerPending, *counter); e.powerMu.Unlock() }()
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case sample := <-req.samples:
		return sample.watts, sample.err
	}
}

func (e *Endpoint) captureDiscovery(b []byte) {
	if !e.deviceMode {
		return
	}
	var d model.Datagram
	if json.Unmarshal(b, &d) != nil {
		return
	}
	h := d.Datagram.Header
	if h.CmdClassifier == nil || (*h.CmdClassifier != model.CmdClassifierTypeReply && *h.CmdClassifier != model.CmdClassifierTypeNotify) {
		return
	}
	for _, c := range d.Datagram.Payload.Cmd {
		if c.NodeManagementUseCaseData != nil {
			e.mu.Lock()
			e.useCasesReceived = true
			e.mu.Unlock()
		}
	}
}

func (e *Endpoint) powerMetadata(r spineapi.EntityRemoteInterface) (model.MeasurementIdType, float64, error) {
	m, err := client.NewMeasurement(e.Entity, r)
	if err != nil {
		return 0, 0, err
	}
	desc, err := m.GetDescriptionsForFilter(model.MeasurementDescriptionDataType{MeasurementType: util.Ptr(model.MeasurementTypeTypePower), CommodityType: util.Ptr(model.CommodityTypeTypeElectricity), ScopeType: util.Ptr(model.ScopeTypeTypeACPowerTotal), Unit: util.Ptr(model.UnitOfMeasurementTypeW)})
	if err != nil || len(desc) != 1 || desc[0].MeasurementId == nil {
		return 0, 0, errors.New("keine eindeutige MPC-Gesamtwirkleistung in W beschrieben")
	}
	id := *desc[0].MeasurementId
	ec, err := client.NewElectricalConnection(e.Entity, r)
	if err != nil {
		return 0, 0, err
	}
	d, err := ec.GetDescriptionForParameterDescriptionFilter(model.ElectricalConnectionParameterDescriptionDataType{MeasurementId: &id})
	if err != nil || d == nil || d.PositiveEnergyDirection == nil {
		return 0, 0, errors.New("MPC-Leistungsrichtung nicht beschrieben")
	}
	switch *d.PositiveEnergyDirection {
	case model.EnergyDirectionTypeConsume:
		return id, 1, nil
	case model.EnergyDirectionTypeProduce:
		return id, -1, nil
	}
	return 0, 0, errors.New("unbekannte MPC-Leistungsrichtung")
}
