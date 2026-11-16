package eebus

import (
	"encoding/json"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
	"testing"
)

func TestConfigurationSnapshotRequiresFreshOwnedPeerReply(t *testing.T) {
	ref := model.MsgCounterType(52)
	address := model.FeatureAddressType{Entity: []model.AddressEntityType{1}, Feature: util.Ptr(model.AddressFeatureType(3))}
	ch := make(chan configurationSample, 1)
	e := &Endpoint{configurationPending: map[model.MsgCounterType]configurationRequest{ref: {powerRequest: powerRequest{ski: "trusted", source: address, destination: address}, result: ch}}}
	d := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{MsgCounterReference: &ref, CmdClassifier: util.Ptr(model.CmdClassifierTypeReply), AddressSource: &address, AddressDestination: &address}, Payload: model.PayloadType{Cmd: []model.CmdType{{DeviceConfigurationKeyValueListData: &model.DeviceConfigurationKeyValueListDataType{DeviceConfigurationKeyValueData: []model.DeviceConfigurationKeyValueDataType{{KeyId: util.Ptr(model.DeviceConfigurationKeyIdType(1)), Value: &model.DeviceConfigurationKeyValueValueType{ScaledNumber: model.NewScaledNumberType(900)}}}}}}}}}
	for _, change := range []string{"notify", "peer", "counter", "source", "destination"} {
		before, _ := json.Marshal(d)
		var wrong model.Datagram
		_ = json.Unmarshal(before, &wrong)
		ski := "trusted"
		switch change {
		case "notify":
			wrong.Datagram.Header.CmdClassifier = util.Ptr(model.CmdClassifierTypeNotify)
		case "peer":
			ski = "other"
		case "counter":
			wrong.Datagram.Header.MsgCounterReference = util.Ptr(model.MsgCounterType(51))
		case "source":
			wrong.Datagram.Header.AddressSource.Feature = util.Ptr(model.AddressFeatureType(99))
		case "destination":
			wrong.Datagram.Header.AddressDestination.Entity = []model.AddressEntityType{99}
		}
		raw, _ := json.Marshal(wrong)
		e.captureConfiguration(ski, raw)
		if len(ch) > 0 {
			t.Fatal(change, "satisfied snapshot")
		}
	}
	raw, _ := json.Marshal(d)
	reader := measurementReader{endpoint: e, ski: "trusted", next: changingReader{}}
	reader.HandleShipPayloadMessage(raw)
	if len(ch) != 1 {
		t.Fatal("valid reply lost")
	}
	got := <-ch
	if got.err != nil || len(got.data) != 1 || got.data[0].Value.ScaledNumber.GetValue() != 900 {
		t.Fatal(got)
	}
}
