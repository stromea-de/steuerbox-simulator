package eebus

import (
	"encoding/json"
	"testing"

	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
)

type changingReader struct{}

func (changingReader) HandleShipPayloadMessage(b []byte) {
	for i := range b {
		b[i] = ' '
	}
}

func TestPowerReplyOwnsValueBeforeDelegation(t *testing.T) {
	ref := model.MsgCounterType(42)
	address := model.FeatureAddressType{Entity: []model.AddressEntityType{1}, Feature: util.Ptr(model.AddressFeatureType(2))}
	ch := make(chan powerSample, 1)
	e := &Endpoint{powerPending: map[model.MsgCounterType]powerRequest{ref: {ski: "trusted", source: address, destination: address, samples: ch}}}
	d := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{
		MsgCounterReference: &ref, CmdClassifier: util.Ptr(model.CmdClassifierTypeReply), AddressSource: &address, AddressDestination: &address,
	}, Payload: model.PayloadType{Cmd: []model.CmdType{{MeasurementListData: &model.MeasurementListDataType{MeasurementData: []model.MeasurementDataType{{Value: model.NewScaledNumberType(4200)}}}}}}}}
	for _, classifier := range []model.CmdClassifierType{model.CmdClassifierTypeNotify, model.CmdClassifierTypeRead} {
		d.Datagram.Header.CmdClassifier = &classifier
		b, _ := json.Marshal(d)
		e.capturePower("trusted", b)
		if len(ch) != 0 {
			t.Fatal("non-reply satisfied fresh measurement")
		}
	}
	d.Datagram.Header.CmdClassifier = util.Ptr(model.CmdClassifierTypeReply)
	b, _ := json.Marshal(d)
	e.capturePower("other-peer", b)
	if len(ch) != 0 {
		t.Fatal("wrong peer satisfied measurement")
	}
	wrongRef := model.MsgCounterType(41)
	d.Datagram.Header.MsgCounterReference = &wrongRef
	b, _ = json.Marshal(d)
	e.capturePower("trusted", b)
	if len(ch) != 0 {
		t.Fatal("old reply satisfied new measurement")
	}
	d.Datagram.Header.MsgCounterReference = &ref
	wrongAddress := model.FeatureAddressType{Entity: []model.AddressEntityType{99}, Feature: address.Feature}
	d.Datagram.Header.AddressSource = &wrongAddress
	b, _ = json.Marshal(d)
	e.capturePower("trusted", b)
	if len(ch) != 0 {
		t.Fatal("wrong feature satisfied measurement")
	}
	d.Datagram.Header.AddressSource = &address
	b, _ = json.Marshal(d)
	reader := measurementReader{endpoint: e, ski: "trusted", next: changingReader{}}
	reader.HandleShipPayloadMessage(b)
	if len(ch) != 1 {
		t.Fatal("fresh reply not captured")
	}
	got := <-ch
	if got.err != nil || got.watts != 4200 {
		t.Fatalf("reply corrupted: %+v", got)
	}
	d.Datagram.Payload.Cmd[0].MeasurementListData.MeasurementData[0].Value.Number = nil
	b, _ = json.Marshal(d)
	e.capturePower("trusted", b)
	if got := <-ch; got.err == nil {
		t.Fatal("measurement without number accepted")
	}
}

func TestDevicePowerSelectsMeasurementAndDirection(t *testing.T) {
	ref := model.MsgCounterType(42)
	id := model.MeasurementIdType(7)
	address := model.FeatureAddressType{Entity: []model.AddressEntityType{1}, Feature: util.Ptr(model.AddressFeatureType(2))}
	ch := make(chan powerSample, 1)
	e := &Endpoint{powerPending: map[model.MsgCounterType]powerRequest{ref: {ski: "trusted", source: address, destination: address, samples: ch, measurementID: &id, direction: -1}}}
	d := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{MsgCounterReference: &ref, CmdClassifier: util.Ptr(model.CmdClassifierTypeReply), AddressSource: &address, AddressDestination: &address}, Payload: model.PayloadType{Cmd: []model.CmdType{{MeasurementListData: &model.MeasurementListDataType{MeasurementData: []model.MeasurementDataType{{MeasurementId: util.Ptr(model.MeasurementIdType(3)), Value: model.NewScaledNumberType(99999)}, {MeasurementId: &id, Value: model.NewScaledNumberType(3000)}}}}}}}}
	b, _ := json.Marshal(d)
	e.capturePower("trusted", b)
	v := <-ch
	if v.err != nil || v.watts != -3000 {
		t.Fatal(v)
	}
	data := d.Datagram.Payload.Cmd[0].MeasurementListData
	data.MeasurementData = append(data.MeasurementData, data.MeasurementData[1])
	b, _ = json.Marshal(d)
	e.capturePower("trusted", b)
	if (<-ch).err == nil {
		t.Fatal("duplicate total measurement accepted")
	}
}
