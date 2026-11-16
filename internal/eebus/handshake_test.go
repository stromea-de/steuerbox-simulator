package eebus

import (
	"context"
	shipmodel "github.com/enbility/ship-go/model"
	"github.com/enbility/ship-go/ship"
	"github.com/enbility/spine-go/model"
	"github.com/enbility/spine-go/spine"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"sync"
	"testing"
	"time"
)

func TestHandshakeCallbackReadiness(t *testing.T) {
	e := &Endpoint{deviceMode: true, peer: "peer", alive: true}
	e.HandleShipHandshakeStateUpdate("other", shipmodel.ShipState{State: shipmodel.SmeStateComplete})
	if e.handshakeComplete() {
		t.Fatal("foreign callback accepted")
	}
	e.HandleShipHandshakeStateUpdate("peer", shipmodel.ShipState{State: shipmodel.SmeStateComplete})
	if !e.handshakeComplete() {
		t.Fatal("completion missing")
	}
	e.Disconnect()
	if e.handshakeComplete() {
		t.Fatal("disconnect kept readiness")
	}
	e.HandleShipHandshakeStateUpdate("peer", shipmodel.ShipState{State: shipmodel.SmeStateComplete})
	if e.handshakeComplete() {
		t.Fatal("late callback revived closed connection")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if e.Ready(ctx) == nil {
		t.Fatal("closed endpoint ready")
	}
}
func TestHandshakeCallbackConcurrentRead(t *testing.T) {
	e := &Endpoint{peer: "peer", alive: true}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			e.HandleShipHandshakeStateUpdate("peer", shipmodel.ShipState{State: shipmodel.SmeStateComplete})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			e.handshakeComplete()
			e.ConnectionState()
		}
	}()
	wg.Wait()
}

func TestEndpointClassificationConfiguration(t *testing.T) {
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, simulator := range []bool{false, true} {
		cfg, _, err := endpointConfiguration(id, simulator, 4711)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.DeviceBrand() != "Stromea" || cfg.DeviceSerialNumber() != id.SKI[:8] {
			t.Fatal("wrong manufacturer/serial", cfg)
		}
		// Exercise the upstream classification data used by the TCP foreign process,
		// without starting a listener or registering any global event handlers.
		local := spine.NewDeviceLocal(cfg.DeviceBrand(), cfg.DeviceModel(), cfg.DeviceSerialNumber(), cfg.Identifier(), "fixture", cfg.DeviceType(), cfg.FeatureSet())
		entity := local.EntityForType(model.EntityTypeTypeDeviceInformation)
		f := entity.FeatureOfTypeAndRole(model.FeatureTypeTypeDeviceClassification, model.RoleTypeServer)
		data := f.DataCopy(model.FunctionTypeDeviceClassificationManufacturerData).(*model.DeviceClassificationManufacturerDataType)
		if data.VendorName == nil || string(*data.VendorName) != "Stromea" || data.SerialNumber == nil || string(*data.SerialNumber) != id.SKI[:8] || data.SoftwareRevision != nil {
			t.Fatalf("unexpected SPINE metadata: %+v", data)
		}
	}
}

func TestOldConnectionCallbackCannotOverwriteReconnect(t *testing.T) {
	old, current := new(ship.ShipConnection), new(ship.ShipConnection)
	e := &Endpoint{peer: "peer", alive: true, connections: []*ship.ShipConnection{old, current}}
	previous := &handshakeCallbacks{Endpoint: e, connection: old}
	latest := &handshakeCallbacks{Endpoint: e, connection: current}
	previous.HandleShipHandshakeStateUpdate("peer", shipmodel.ShipState{State: shipmodel.SmeStateComplete})
	if e.handshakeComplete() {
		t.Fatal("old connection completed new handshake")
	}
	latest.HandleShipHandshakeStateUpdate("peer", shipmodel.ShipState{State: shipmodel.SmeStateComplete})
	previous.HandleShipHandshakeStateUpdate("peer", shipmodel.ShipState{State: shipmodel.SmeStateError})
	if !e.handshakeComplete() {
		t.Fatal("late old callback invalidated new connection")
	}
}
