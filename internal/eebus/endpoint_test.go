package eebus

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"github.com/enbility/ship-go/cert"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"net"
	"testing"
	"time"
)

func TestRejectNonLoopback(t *testing.T) {
	for _, a := range []string{"example.com:4711", "192.0.2.1:4711", "0.0.0.0:4711", "[::]:4711"} {
		if err := LoopbackAddress(a); err == nil {
			t.Fatalf("accepted %s", a)
		}
	}
	for _, a := range []string{"127.0.0.1:4711", "[::1]:4711"} {
		if err := LoopbackAddress(a); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSHIPRequiresMutualTrust(t *testing.T) {
	a, _ := identity.LoadOrCreate(t.TempDir())
	b, _ := identity.LoadOrCreate(t.TempDir())
	eg, err := NewEndpoint(a, false)
	if err != nil {
		t.Fatal(err)
	}
	defer eg.Close()
	cs, err := NewEndpoint(b, true)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := eg.Connect(ctx, cs.Address(), b.SKI); err == nil {
		t.Fatal("untrusted local peer accepted")
	}
	eg.Trust(b.SKI)
	if err := eg.Connect(ctx, cs.Address(), b.SKI); err == nil {
		t.Fatal("peer missing reciprocal trust accepted")
	}
	cs.Trust(a.SKI)
	if err := eg.Connect(ctx, cs.Address(), b.SKI); err != nil {
		t.Fatal(err)
	}
	if err := eg.Ready(ctx); err != nil {
		t.Log("SHIP", eg.ConnectionState())
		r := eg.RemoteEntity()
		t.Log("entity", r)
		if r != nil {
			for _, f := range r.Features() {
				for fn := range f.Operations() {
					b, _ := json.Marshal(f.DataCopy(fn))
					t.Log(f.Type(), fn, string(b))
				}
			}
			v, e := eg.MPC.Power(r)
			t.Log("MPC", v, e, "scenarios", eg.LPC.AvailableScenariosForEntity(r), eg.LPP.AvailableScenariosForEntity(r), eg.MPC.AvailableScenariosForEntity(r))
		}
		t.Fatal(err)
	}
}

// A copied SubjectKeyId must not let an unrelated self-signed key impersonate a trusted peer.
func TestRejectCopiedCertificateSKI(t *testing.T) {
	trusted, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	eg, err := NewEndpoint(trusted, false)
	if err != nil {
		t.Fatal(err)
	}
	defer eg.Close()
	if err = eg.Trust(trusted.SKI); err != nil {
		t.Fatal(err)
	}
	rogue, err := cert.CreateCertificate("test", "rogue", "DE", "rogue")
	if err != nil {
		t.Fatal(err)
	}
	template, err := x509.ParseCertificate(rogue.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	template.SubjectKeyId, _ = hex.DecodeString(trusted.SKI)
	raw, err := x509.CreateCertificate(rand.Reader, template, template, template.PublicKey, rogue.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = eg.verifyTrusted([][]byte{raw}, nil); err == nil {
		t.Fatal("copied SKI accepted with unrelated public key")
	}
}

func TestDeviceAddressRequiresLiteralLocalUnicast(t *testing.T) {
	for _, addr := range []string{"device.local:4711", "8.8.8.8:4711", "0.0.0.0:4711", "224.0.0.251:4711", "169.254.1.1:0", "[::]:4711"} {
		if DeviceAddress(addr) == nil {
			t.Fatal("accepted", addr)
		}
	}
	// Synthetic private addresses are constructed from range prefixes, never lab hosts.
	privateV4 := net.IPv4(10, 0, 0, 1).String()
	privateV6 := net.IP{0xfd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}.String()
	for _, addr := range []string{"127.0.0.1:4711", "169.254.1.1:4711", net.JoinHostPort(privateV4, "4711"), net.JoinHostPort(privateV6, "4711")} {
		if err := DeviceAddress(addr); err != nil {
			t.Fatal(addr, err)
		}
	}
}
