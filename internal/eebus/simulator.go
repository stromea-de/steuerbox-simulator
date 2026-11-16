package eebus

import (
	"fmt"
	"math"
	"time"

	"github.com/enbility/eebus-go/api"
	"github.com/enbility/eebus-go/features/server"
	cslpc "github.com/enbility/eebus-go/usecases/cs/lpc"
	cslpp "github.com/enbility/eebus-go/usecases/cs/lpp"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
)

// Approved incoming values are copied before the final SPINE approval. The
// stack's LoadControl DataCopy is shallow; another use case can finish approval
// asynchronously and mutate that shared list after simEvent returns.
// The power loop reads only these owned scalar values, never that mutable list.
type simulatorLimit struct {
	active bool
	value  float64
}

func (e *Endpoint) initMeasurement() error {
	f := e.Entity.GetOrAddFeature(model.FeatureTypeTypeMeasurement, model.RoleTypeServer)
	for _, fn := range []model.FunctionType{model.FunctionTypeMeasurementDescriptionListData, model.FunctionTypeMeasurementListData, model.FunctionTypeMeasurementConstraintsListData} {
		f.AddFunctionType(fn, true, false)
	}
	m, err := server.NewMeasurement(e.Entity)
	if err != nil {
		return err
	}
	id := m.AddDescription(model.MeasurementDescriptionDataType{MeasurementType: util.Ptr(model.MeasurementTypeTypePower), CommodityType: util.Ptr(model.CommodityTypeTypeElectricity), ScopeType: util.Ptr(model.ScopeTypeTypeACPowerTotal), Unit: util.Ptr(model.UnitOfMeasurementTypeW)})
	if id == nil {
		return fmt.Errorf("MPC-Beschreibung konnte nicht erzeugt werden")
	}
	f = e.Entity.GetOrAddFeature(model.FeatureTypeTypeElectricalConnection, model.RoleTypeServer)
	f.AddFunctionType(model.FunctionTypeElectricalConnectionDescriptionListData, true, false)
	f.AddFunctionType(model.FunctionTypeElectricalConnectionParameterDescriptionListData, true, false)
	ec, err := server.NewElectricalConnection(e.Entity)
	if err != nil {
		return err
	}
	if err = ec.AddDescription(model.ElectricalConnectionDescriptionDataType{ElectricalConnectionId: util.Ptr(model.ElectricalConnectionIdType(0)), PositiveEnergyDirection: util.Ptr(model.EnergyDirectionTypeConsume)}); err != nil {
		return err
	}
	if ec.AddParameterDescription(model.ElectricalConnectionParameterDescriptionDataType{ElectricalConnectionId: util.Ptr(model.ElectricalConnectionIdType(0)), MeasurementId: id}) == nil {
		return fmt.Errorf("MPC-Parameter konnte nicht erzeugt werden")
	}
	e.Entity.AddUseCaseSupport(model.UseCaseActorTypeMonitoredUnit, model.UseCaseNameTypeMonitoringOfPowerConsumption, "1.0.0", "release", true, []model.UseCaseScenarioSupportType{1})
	return m.UpdateDataForId(model.MeasurementDataType{ValueType: util.Ptr(model.MeasurementValueTypeTypeValue), Value: model.NewScaledNumberType(11000)}, nil, *id)
}
func (e *Endpoint) simEvent(_ string, _ spineapi.DeviceRemoteInterface, _ spineapi.EntityRemoteInterface, event api.EventType) {
	switch event {
	case cslpc.WriteApprovalRequired:
		e.simLimitsMu.Lock()
		for id, limit := range e.CSLPC.PendingConsumptionLimits() {
			e.simConsumptionLimit = simulatorLimit{active: limit.IsActive, value: limit.Value}
			e.CSLPC.ApproveOrDenyConsumptionLimit(id, true, "")
		}
		e.simLimitsMu.Unlock()
		e.mu.Lock()
		e.production = false
		e.effectReadyAt = time.Now().Add(e.options.EffectDelay)
		e.mu.Unlock()
	case cslpp.WriteApprovalRequired:
		e.simLimitsMu.Lock()
		for id, limit := range e.CSLPP.PendingProductionLimits() {
			e.simProductionLimit = simulatorLimit{active: limit.IsActive, value: limit.Value}
			e.CSLPP.ApproveOrDenyProductionLimit(id, true, "")
		}
		e.simLimitsMu.Unlock()
		e.mu.Lock()
		e.production = true
		e.effectReadyAt = time.Now().Add(e.options.EffectDelay)
		e.mu.Unlock()
	case cslpc.DataUpdateHeartbeat, cslpp.DataUpdateHeartbeat:
		e.mu.Lock()
		e.lastHeartbeat = time.Now()
		e.mu.Unlock()
	}
}
func (e *Endpoint) simulate() {
	speed := max(1, e.practiceSpeed.Load())
	t := time.NewTicker(100 * time.Millisecond / time.Duration(speed))
	defer t.Stop()
	measured := 11000.0
	if e.options.InitialPower > 0 {
		measured = e.options.InitialPower
	}
	tick := 0
	for {
		select {
		case <-e.closed:
			return
		case <-t.C:
			if next := max(1, e.practiceSpeed.Load()); next != speed {
				speed = next
				t.Reset(100 * time.Millisecond / time.Duration(speed))
			}
			e.mu.Lock()
			last, production, effectReady := e.lastHeartbeat, e.production, e.effectReadyAt
			e.mu.Unlock()
			power := 11000.0
			if e.options.InitialPower > 0 {
				power = e.options.InitialPower
			}
			if production {
				power = 8000
			}
			e.simLimitsMu.Lock()
			if last.IsZero() || time.Since(last) > FailsafeAfter/time.Duration(speed) {
				if production {
					v, _, err := e.CSLPP.FailsafeProductionActivePowerLimit()
					if err == nil {
						power = math.Min(power, v)
					}
				} else {
					v, _, err := e.CSLPC.FailsafeConsumptionActivePowerLimit()
					if err == nil {
						power = math.Min(power, v)
					}
				}
			} else if time.Now().Before(effectReady) {
				// Deliberately slow foreign-device fixture.
			} else if production {
				v := e.simProductionLimit
				if v.active {
					power = math.Min(power, -v.value)
				}
			} else {
				v := e.simConsumptionLimit
				if v.active {
					power = math.Min(power, v.value)
				}
			}
			e.simLimitsMu.Unlock()
			if production {
				power = -power
			}
			measured += (power - measured) * 0.35
			tick++
			power = math.Round(measured + math.Sin(float64(tick)*0.7)*math.Min(4, math.Abs(power)*0.002))
			m, err := server.NewMeasurement(e.Entity)
			if err == nil {
				stackLifecycle.RLock()
				_ = m.UpdateDataForId(model.MeasurementDataType{ValueType: util.Ptr(model.MeasurementValueTypeTypeValue), Value: &model.ScaledNumberType{Number: util.Ptr(model.NumberType(math.Round(power * 1000))), Scale: util.Ptr(model.ScaleType(-3))}, Timestamp: model.NewAbsoluteOrRelativeTimeTypeFromTime(time.Now())}, nil, 0)
				stackLifecycle.RUnlock()
			}
		}
	}
}

// SetPracticeSpeed changes only the internal simulated device clock. A controller
// selected for a real device always has the normal constructor configuration.
func (e *Endpoint) SetPracticeSpeed(speed int) {
	if (e.CSLPC != nil || e.CSLPP != nil) && (speed == 1 || speed == 10) {
		e.practiceSpeed.Store(int32(speed))
	}
}

// FailsafeDelay is the simulated device's heartbeat timeout on its current
// clock: it enters failsafe this long after the last received heartbeat.
func (e *Endpoint) FailsafeDelay() time.Duration {
	return FailsafeAfter / time.Duration(max(1, e.practiceSpeed.Load()))
}
