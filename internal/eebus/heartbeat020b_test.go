package eebus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

type stopManager020b struct {
	spineapi.HeartbeatManagerInterface
	stop func()
}

func (m stopManager020b) StopHeartbeat() { m.stop() }

type stopEntity020b struct {
	spineapi.EntityLocalInterface
	manager spineapi.HeartbeatManagerInterface
}

func (e stopEntity020b) HeartbeatManager() spineapi.HeartbeatManagerInterface { return e.manager }
func TestStopNeverWaitsForOptionalReceipt020b(t *testing.T) {
	for _, state := range []string{"no-send", "missing", "negative", "positive"} {
		t.Run(state, func(t *testing.T) {
			e := &Endpoint{deviceMode: true}
			if state != "no-send" {
				e.heartbeatPending = &heartbeatReceipt{}
			}
			switch state {
			case "negative":
				e.heartbeatPending.err = errors.New("negative")
			case "positive":
				e.heartbeatPending.at = time.Now()
			}
			stopped := false
			e.Entity = stopEntity020b{manager: stopManager020b{stop: func() {
				e.heartbeatMu.Lock()
				defer e.heartbeatMu.Unlock()
				if !e.heartbeatBlocked {
					t.Error("timer stopped before send gate closed")
				}
				stopped = true
			}}}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			before := time.Now()
			if err := e.StopHeartbeat(ctx); err != nil {
				t.Fatal("optional receipt failed stop:", err)
			}
			if !stopped || time.Since(before) >= 50*time.Millisecond {
				t.Fatal("stop waited for receipt or did not stop")
			}
		})
	}
}

// Connect the actual send/receipt/stop boundary to the inspection flow without
// opening a socket. Only device measurements/configuration are simulated.
type outageDevice020b struct {
	inspection.DeviceBackend
	endpoint   *Endpoint
	powerCalls int
	failEffect bool
	// 046: the device answers by time; failsafe only after its (scaled) timeout.
	stopped time.Time
	timeout time.Duration
	shown   bool
}

func (*outageDevice020b) Connect(context.Context) error { return nil }
func (*outageDevice020b) DeviceInfo() inspection.Device { return inspection.Device{} }
func (*outageDevice020b) Capabilities(context.Context) (inspection.Capabilities, error) {
	return inspection.Capabilities{LPC: true, MPC: true}, nil
}
func (*outageDevice020b) Configure(context.Context, inspection.Config) error { return nil }
func (*outageDevice020b) Recover(context.Context) error                      { return nil }
func (*outageDevice020b) ReadFailsafe(context.Context, string) (inspection.FailsafeValues, error) {
	return inspection.FailsafeValues{Power: 900, Minimum: 2 * time.Hour}, nil
}
func (*outageDevice020b) WriteFailsafe(context.Context, string, string, inspection.FailsafeValues) error {
	return nil
}
func (*outageDevice020b) Limit(context.Context, string, float64, bool) (string, error) {
	return "SPINE result 0", nil
}
func (d *outageDevice020b) Heartbeat(ctx context.Context, on bool) error {
	if on {
		d.endpoint.AllowHeartbeats()
		return nil
	}
	d.stopped = time.Now()
	return d.endpoint.StopHeartbeat(ctx)
}
func (d *outageDevice020b) LastHeartbeatReceipt() (time.Time, string) {
	return d.endpoint.LastHeartbeatReceipt(), "optional result"
}
func (d *outageDevice020b) Power(context.Context) (float64, error) {
	d.powerCalls++
	switch {
	case d.powerCalls == 1:
		return 11000, nil
	case d.stopped.IsZero() || time.Since(d.stopped) < d.timeout:
		return 4200, nil // limit still applies before the device's timeout
	case d.failEffect:
		return 4200, context.DeadlineExceeded
	case !d.shown:
		d.shown = true
		return 500, nil
	default:
		return 11000, nil // return after the release
	}
}
func TestUnansweredAndNegativeNotifyOnlyEffectDecides020b(t *testing.T) {
	for _, reply := range []string{"missing", "negative"} {
		for _, fail := range []bool{false, true} {
			t.Run(reply+"/effect-fails="+fmt.Sprint(fail), func(t *testing.T) {
				e := &Endpoint{deviceMode: true, Entity: stopEntity020b{manager: stopManager020b{stop: func() {}}}}
				source := model.FeatureAddressType{Feature: util.Ptr(model.AddressFeatureType(1))}
				destination := model.FeatureAddressType{Feature: util.Ptr(model.AddressFeatureType(2))}
				message := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{AddressSource: &source, AddressDestination: &destination, MsgCounter: util.Ptr(model.MsgCounterType(7)), CmdClassifier: util.Ptr(model.CmdClassifierTypeNotify)}, Payload: model.PayloadType{Cmd: []model.CmdType{{DeviceDiagnosisHeartbeatData: &model.DeviceDiagnosisHeartbeatDataType{HeartbeatCounter: util.Ptr(uint64(1))}}}}}}
				payload, _ := json.Marshal(message)
				heartbeatWriter{endpoint: e, ski: "peer", next: &heartbeatWriter020{}}.WriteShipMessageWithPayload(payload)
				if reply == "negative" {
					rejection := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{AddressSource: &destination, AddressDestination: &source, MsgCounterReference: util.Ptr(model.MsgCounterType(7)), CmdClassifier: util.Ptr(model.CmdClassifierTypeResult)}, Payload: model.PayloadType{Cmd: []model.CmdType{{ResultData: &model.ResultDataType{ErrorNumber: util.Ptr(model.ErrorNumberType(7))}}}}}}
					raw, _ := json.Marshal(rejection)
					e.captureHeartbeatReceipt("peer", raw)
					if e.heartbeatPending.err == nil {
						t.Fatal("negative result not captured")
					}
				}
				// 046: protocol timeout scaled to 500 ms instead of 120 s real time.
				d := &outageDevice020b{endpoint: e, failEffect: fail, timeout: 500 * time.Millisecond}
				c := inspection.DefaultDeviceConfig()
				c.FailsafeTimeout = 500 * time.Millisecond
				c.TestHeartbeatTimeout = c.FailsafeTimeout
				c.Business = "Test"
				c.Inspector = "Test"
				c.Scope = inspection.ScopeConsumption
				c.StepTimeout = 100 * time.Millisecond
				c.EffectTimeout = 100 * time.Millisecond
				r, err := inspection.RunOnSite(context.Background(), c, inspection.Device{}, d)
				if err != nil || r.Passed == fail || len(r.Heartbeats) != 1 || !e.LastHeartbeatReceipt().IsZero() {
					t.Fatalf("report=%+v err=%v", r, err)
				}
				seen := false
				for _, step := range r.Steps {
					if step.Name == "LPC Failsafe gemessene Wirkung" {
						seen = true
						if step.Passed == fail {
							t.Fatal(step)
						}
					}
				}
				if !seen {
					t.Fatal("outage effect not evaluated")
				}
			})
		}
	}
}
