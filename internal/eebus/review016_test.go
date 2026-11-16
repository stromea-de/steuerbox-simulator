package eebus

import (
	shipmodel "github.com/enbility/ship-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"strings"
	"testing"
)

func TestExpectedCertificateAndSeparateTrust016(t *testing.T) {
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &Endpoint{trusted: map[string]bool{id.SKI: true}, alive: true, peer: id.SKI}
	if got := e.ConnectionTrust(); got.Device || got.Simulator {
		t.Fatal(got)
	}
	if err = e.verifyExpected(id.Certificate.Certificate, nil, strings.Repeat("0", 40)); err == nil || !strings.Contains(err.Error(), "Anderes Gerät") {
		t.Fatal(err)
	}
	if e.ConnectionTrust().Device {
		t.Fatal("mismatched certificate trusted")
	}
	if err = e.verifyExpected(id.Certificate.Certificate, nil, id.SKI); err != nil {
		t.Fatal(err)
	}
	if got := e.ConnectionTrust(); !got.Device || got.Simulator {
		t.Fatal(got)
	}
	e.HandleShipHandshakeStateUpdate(id.SKI, shipmodel.ShipState{State: shipmodel.SmeStateComplete})
	if got := e.ConnectionTrust(); !got.Device || !got.Simulator {
		t.Fatal(got)
	}
	e.alive = false
	if got := e.ConnectionTrust(); got.Device || got.Simulator {
		t.Fatal("stale trust", got)
	}
}

func TestFailedExpectedCertificateClearsEvidence016(t *testing.T) {
	id, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &Endpoint{trusted: map[string]bool{id.SKI: true}, alive: true}
	if err = e.verifyExpected(id.Certificate.Certificate, nil, id.SKI); err != nil {
		t.Fatal(err)
	}
	if err = e.verifyExpected(id.Certificate.Certificate, nil, strings.Repeat("0", 40)); err == nil {
		t.Fatal("mismatch accepted")
	}
	if e.ConnectionTrust().Device {
		t.Fatal("old TLS evidence kept after failed verification")
	}
}
