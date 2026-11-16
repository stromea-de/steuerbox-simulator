package eebus

import (
	"os"
	"sync"
	"testing"

	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/spine"
)

type eventHandlerFunc func(spineapi.EventPayload)

func (f eventHandlerFunc) HandleEvent(p spineapi.EventPayload) { f(p) }

func TestMPCEventGuard(t *testing.T) {
	var received []spineapi.EventPayload
	guard := mpcEventGuard{next: eventHandlerFunc(func(p spineapi.EventPayload) { received = append(received, p) })}
	measurement := spineapi.EventPayload{Data: &model.MeasurementListDataType{}}
	description := spineapi.EventPayload{Data: &model.MeasurementDescriptionListDataType{}}
	connected := spineapi.EventPayload{EventType: spineapi.EventTypeDataChange}
	guard.HandleEvent(measurement)
	guard.HandleEvent(description)
	guard.HandleEvent(connected)
	if len(received) != 2 || received[0].Data != description.Data || received[1].EventType != connected.EventType {
		t.Fatalf("unexpected events forwarded: %#v", received)
	}
}

// Reproduces the upstream alias without networking. Full update stores the
// payload pointer; a subsequent partial update replaces its slice header.
// The guard prevents an asynchronous MPC handler from reading that header.
func TestMPCEventGuardWithAliasedUpstreamData(t *testing.T) {
	fd := spine.NewFunctionData[model.MeasurementListDataType](model.FunctionTypeMeasurementListData)
	initial := &model.MeasurementListDataType{MeasurementData: []model.MeasurementDataType{{}}}
	if _, err := fd.UpdateData(false, true, initial, nil, nil); err != nil {
		t.Fatal(err)
	}
	firstItem := &initial.MeasurementData[0]
	filter := &model.FilterType{}
	if _, err := fd.UpdateData(false, true, &model.MeasurementListDataType{}, filter, nil); err != nil {
		t.Fatal(err)
	}
	if len(initial.MeasurementData) != 1 {
		t.Fatal("unexpected update semantics")
	}
	if &initial.MeasurementData[0] == firstItem {
		t.Fatal("partial update did not replace the event's slice")
	}
	// The returned cache pointer is the original payload, not a snapshot.
	stored, err := fd.UpdateData(false, true, initial, nil, nil)
	if err != nil || stored != initial {
		t.Fatal("upstream alias assumption changed")
	}
	var wg sync.WaitGroup
	guard := mpcEventGuard{next: eventHandlerFunc(func(p spineapi.EventPayload) { _ = len(p.Data.(*model.MeasurementListDataType).MeasurementData) })}
	handler := spineapi.EventHandlerInterface(&guard)
	// Opt-in baseline demonstrates the original race and must fail under -race.
	if os.Getenv("EEBUS_REPRO_021_UNGUARDED") == "1" {
		handler = guard.next
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			handler.HandleEvent(spineapi.EventPayload{Data: initial})
		}
	}()
	for i := 0; i < 10000; i++ {
		if _, err := fd.UpdateData(false, true, &model.MeasurementListDataType{}, filter, nil); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}
