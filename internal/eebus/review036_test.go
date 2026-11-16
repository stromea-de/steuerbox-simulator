package eebus

import (
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/identity"
)

func Test036NoAcceleratedRealDeviceController(t *testing.T) {
	if HeartbeatInterval != time.Second || FailsafeAfter != 2500*time.Millisecond {
		t.Fatal("normal protocol times changed")
	}
	// Must fail before opening a listener or using the identity.
	if e, err := NewEndpointWithOptions(identity.Identity{}, false, EndpointOptions{DeviceMode: true, PracticeSpeed: 10}); err == nil {
		e.Close()
		t.Fatal("real device controller accepts the practice clock")
	}
}
