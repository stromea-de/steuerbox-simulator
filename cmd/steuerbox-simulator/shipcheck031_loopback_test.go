package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
)

// Uses the existing full SHIP/SPINE CS endpoint, with literal loopback addresses
// only. These tests deliberately fail (not skip) if the sandbox forbids sockets.
func TestShipCheck031Loopback(t *testing.T) {
	state := t.TempDir()
	local, err := identity.LoadOrCreate(filepath.Join(state, "energyguard"))
	if err != nil {
		t.Fatal(err)
	}
	remote, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	peer, err := eebus.NewEndpoint(remote, true)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err = peer.Trust(local.SKI); err != nil {
		t.Fatal(err)
	}
	t.Run("repeated mutual SHIP", func(t *testing.T) {
		var out, errs bytes.Buffer
		args := []string{"ship-check", "--state", state, "--address", peer.Address(), "--ski", remote.SKI, "--hold", "20ms", "--repeat", "2", "--pause", "0s", "--connect-timeout", "8s"}
		code := execute(context.Background(), args, &out, &errs)
		r := readCheck031(t, &out)
		if code != 0 || !r.Passed || r.LocalSKI != local.SKI || len(r.Attempts) != 2 {
			t.Fatal(code, r, errs.String())
		}
		for _, a := range r.Attempts {
			if len(a.Phases) != 4 || a.Phases[3].Phase != "SHIP-Handshake" || a.Phases[3].Status != "complete" || !a.Hold.Passed {
				t.Fatal(a)
			}
		}
	})
	t.Run("wrong pinned SKI", func(t *testing.T) {
		var out, errs bytes.Buffer
		code := execute(context.Background(), []string{"ship-check", "--state", state, "--address", peer.Address(), "--ski", local.SKI, "--hold", "20ms", "--connect-timeout", "2s"}, &out, &errs)
		r := readCheck031(t, &out)
		if code != 1 || r.Passed || len(r.Attempts) != 1 || len(r.Attempts[0].Phases) != 2 || r.Attempts[0].Phases[1].Phase != "TLS-Handshake" || r.Attempts[0].Phases[1].Status != "failed" {
			t.Fatal(code, r, errs.String())
		}
	})
}
