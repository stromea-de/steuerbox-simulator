package eebus

import (
	"encoding/json"
	"testing"

	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
)

type heartbeatWriter020 struct{ data []byte }

func (w *heartbeatWriter020) WriteShipMessageWithPayload(b []byte) {
	w.data = append([]byte(nil), b...)
}
func TestHeartbeatReceiptBoundary020(t *testing.T) {
	e := &Endpoint{deviceMode: true}
	w := &heartbeatWriter020{}
	writer := heartbeatWriter{endpoint: e, ski: "peer", next: w}
	source := model.FeatureAddressType{Feature: util.Ptr(model.AddressFeatureType(1))}
	destination := model.FeatureAddressType{Feature: util.Ptr(model.AddressFeatureType(2))}
	message := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{AddressSource: &source, AddressDestination: &destination, MsgCounter: util.Ptr(model.MsgCounterType(7)), CmdClassifier: util.Ptr(model.CmdClassifierTypeNotify)}, Payload: model.PayloadType{Cmd: []model.CmdType{{DeviceDiagnosisHeartbeatData: &model.DeviceDiagnosisHeartbeatDataType{HeartbeatCounter: util.Ptr(uint64(1))}}}}}}
	b, _ := json.Marshal(message)
	writer.WriteShipMessageWithPayload(b)
	var sent model.Datagram
	_ = json.Unmarshal(w.data, &sent)
	if sent.Datagram.Header.AckRequest == nil || !*sent.Datagram.Header.AckRequest {
		t.Fatal("heartbeat receive acknowledgment missing")
	}
	reply := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{AddressSource: &destination, AddressDestination: &source, MsgCounterReference: util.Ptr(model.MsgCounterType(7)), CmdClassifier: util.Ptr(model.CmdClassifierTypeResult)}, Payload: model.PayloadType{Cmd: []model.CmdType{{ResultData: &model.ResultDataType{ErrorNumber: util.Ptr(model.ErrorNumberType(0))}}}}}}
	ack, _ := json.Marshal(reply)
	e.captureHeartbeatReceipt("stranger", ack)
	if !e.LastHeartbeatReceipt().IsZero() {
		t.Fatal("wrong peer accepted")
	}
	negative := reply
	negative.Datagram.Payload.Cmd = []model.CmdType{{ResultData: &model.ResultDataType{ErrorNumber: util.Ptr(model.ErrorNumberType(7))}}}
	rejected, _ := json.Marshal(negative)
	e.captureHeartbeatReceipt("peer", rejected)
	if !e.LastHeartbeatReceipt().IsZero() || e.heartbeatPending.err == nil {
		t.Fatal("negative result invented a positive receipt")
	}
	// Next actual send replaces the rejected receipt; only this newest send counts.
	writer.WriteShipMessageWithPayload(b)
	e.captureHeartbeatReceipt("peer", ack)
	if e.LastHeartbeatReceipt().IsZero() {
		t.Fatal("valid receipt not recorded")
	}
	e.heartbeatMu.Lock()
	e.heartbeatBlocked = true
	e.heartbeatMu.Unlock()
	w.data = nil
	writer.WriteShipMessageWithPayload(b)
	if w.data != nil {
		t.Fatal("late heartbeat escaped after stop")
	}
}

func TestMissingHeartbeatReceiptRemainsUnknown020(t *testing.T) {
	e := &Endpoint{deviceMode: true}
	if !e.LastHeartbeatReceipt().IsZero() {
		t.Fatal("missing send invented receipt")
	}
	e.heartbeatPending = &heartbeatReceipt{}
	if !e.LastHeartbeatReceipt().IsZero() {
		t.Fatal("unacknowledged heartbeat invented receipt")
	}
}
