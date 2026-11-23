package eebus

import (
	"encoding/json"
	"testing"

	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/util"
)

// R2-04: after a timed-out read its counter stays reserved; the late reply is
// swallowed and frees the counter, it is never a fresh value for a new waiter.
func TestLateLimitReplyIsNotFresh047(t *testing.T) {
	e := &Endpoint{}
	address := model.FeatureAddressType{Entity: []model.AddressEntityType{1}, Feature: util.Ptr(model.AddressFeatureType(2))}
	ref := model.MsgCounterType(9)
	req := limitsRequest{powerRequest: powerRequest{ski: "ski", source: address, destination: address}, result: make(chan limitsSample, 1)}
	e.limitsPending = map[model.MsgCounterType]limitsRequest{ref: req}
	e.abandonLimits(ref, req.result)
	if _, busy := e.limitsPending[ref]; !busy {
		t.Fatal("Nachrichtennummer muss reserviert bleiben, bis die Antwort kommt")
	}
	d := model.Datagram{Datagram: model.DatagramType{Header: model.HeaderType{MsgCounterReference: &ref, CmdClassifier: util.Ptr(model.CmdClassifierTypeReply), AddressSource: &address, AddressDestination: &address}, Payload: model.PayloadType{Cmd: []model.CmdType{{LoadControlLimitListData: &model.LoadControlLimitListDataType{}}}}}}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	e.captureLimits("ski", raw)
	if _, busy := e.limitsPending[ref]; busy {
		t.Fatal("verspätete Antwort muss die Reservierung lösen")
	}
	select {
	case v := <-req.result:
		t.Fatal("verspätete Antwort darf nicht zugestellt werden", v)
	default:
	}
}

type fakeDevice047 struct {
	spineapi.DeviceRemoteInterface
	id int
}

// R3-05: reservations of an earlier transport are dropped on the next read (their
// replies can never arrive), a waiter of that transport gets an error, and the
// bound is checked before a read is sent.
func TestLimitReservationsPerTransport047(t *testing.T) {
	e := &Endpoint{limitsPending: map[model.MsgCounterType]limitsRequest{}}
	old, current := &fakeDevice047{id: 1}, &fakeDevice047{id: 2}
	waiter := make(chan limitsSample, 1)
	for i := 0; i < maxLimitsPending; i++ {
		e.limitsPending[model.MsgCounterType(i)] = limitsRequest{powerRequest: powerRequest{ski: "ski"}, abandoned: i > 0, device: old, result: waiter}
	}
	if e.reserveLimitsLocked("other", current) {
		t.Fatal("Obergrenze muss vor dem Senden greifen")
	}
	if !e.reserveLimitsLocked("ski", current) || len(e.limitsPending) != 0 {
		t.Fatal("Reservierungen der alten Verbindung müssen entfallen", len(e.limitsPending))
	}
	select {
	case v := <-waiter:
		if v.err == nil {
			t.Fatal("wartende Leseanfrage braucht einen Fehler")
		}
	default:
		t.Fatal("wartende Leseanfrage der alten Verbindung muss beendet werden")
	}
	for i := 0; i < maxLimitsPending; i++ {
		e.limitsPending[model.MsgCounterType(i)] = limitsRequest{powerRequest: powerRequest{ski: "ski"}, abandoned: true, device: current}
	}
	if e.reserveLimitsLocked("ski", current) {
		t.Fatal("volle Reservierung derselben Verbindung blockiert weitere Leseanfragen")
	}
}

// R4-04: a late timeout of an old waiter never marks a newer read with the same counter.
func TestAbandonOnlyOwnWaiter047(t *testing.T) {
	e := &Endpoint{limitsPending: map[model.MsgCounterType]limitsRequest{}}
	old, cur := make(chan limitsSample, 1), make(chan limitsSample, 1)
	e.limitsPending[5] = limitsRequest{result: cur}
	e.abandonLimits(5, old)
	if e.limitsPending[5].abandoned {
		t.Fatal("neue Anfrage als aufgegeben markiert")
	}
	e.abandonLimits(5, cur)
	if !e.limitsPending[5].abandoned {
		t.Fatal("eigene Anfrage nicht markiert")
	}
}
