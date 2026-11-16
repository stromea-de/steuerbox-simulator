package eebus

import (
	"context"
	"sync"
	"testing"
	"time"

	shipapi "github.com/enbility/ship-go/api"
	shipmodel "github.com/enbility/ship-go/model"
	"github.com/enbility/ship-go/ship"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/spine"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
)

type readerFunc023b func([]byte)

func (f readerFunc023b) HandleShipPayloadMessage(b []byte) { f(b) }

func TestOldReaderAndSetupCannotTouchNewConnection023b(t *testing.T) {
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEndpoint(id, false)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	old, current := new(ship.ShipConnection), new(ship.ShipConnection)
	// These transport sentinels have no sockets and must not be closed as real SHIP handlers.
	e.mu.Lock()
	e.connections = []*ship.ShipConnection{old, current}
	e.peer = "peer"
	e.alive = true
	e.handshakeState = shipmodel.SmeStateComplete
	e.useCasesReceived = true
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.connections = nil; e.mu.Unlock() }()
	calls := 0
	r := measurementReader{endpoint: e, ski: "peer", connection: old, next: readerFunc023b(func([]byte) { calls++ })}
	previous := &handshakeCallbacks{Endpoint: e, connection: old}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.HandleShipPayloadMessage(nil)
			previous.SetupRemoteDevice("peer", shipapi.ShipConnectionDataWriterInterface(old))
			e.HandleConnectionClosed(old, false)
		}()
	}
	wg.Wait()
	if calls != 0 || !e.Alive() || !e.handshakeComplete() || !e.useCasesReceived || e.Service.LocalDevice().RemoteDeviceForSki("peer") != nil {
		t.Fatal("old transport changed new session", calls)
	}
}

func TestRemoteEventGuard023b(t *testing.T) {
	a, _ := identity.LoadOrCreate(t.TempDir())
	b, _ := identity.LoadOrCreate(t.TempDir())
	e, err := NewEndpoint(a, false)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	peer, err := NewEndpoint(b, true)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	e.Trust(b.SKI)
	peer.Trust(a.SKI)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = e.Connect(ctx, peer.Address(), b.SKI); err != nil {
		t.Fatal(err)
	}
	if err = e.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	old := e.RemoteEntity()
	if old == nil || !e.LPC.IsScenarioAvailableAtEntity(old, 1) {
		t.Fatal("initial use cases missing")
	}
	e.Disconnect()
	// Exercise the guarded event boundary after replacement without network discovery.
	stackLifecycle.Lock()
	e.spineMu.Lock()
	replacement := spine.NewDeviceRemote(e.Service.LocalDevice(), b.SKI, nil)
	e.Service.LocalDevice().AddRemoteDeviceForSki(b.SKI, replacement)
	e.spineMu.Unlock()
	stackLifecycle.Unlock()
	count := 0
	guard := remoteEvents{endpoint: e, next: eventHandlerFunc(func(spineapi.EventPayload) { count++ })}
	guard.HandleEvent(spineapi.EventPayload{Ski: b.SKI, Device: old.Device(), Entity: old, EventType: spineapi.EventTypeEntityChange, ChangeType: spineapi.ElementChangeAdd})
	guard.HandleEvent(spineapi.EventPayload{Ski: b.SKI, Device: replacement})
	if count != 1 {
		t.Fatal("stale event accepted or current event lost", count)
	}
}
