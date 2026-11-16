package eebus

import (
	"crypto/x509"
	"errors"
	"github.com/enbility/ship-go/cert"
	shipmodel "github.com/enbility/ship-go/model"
)

type TrustState struct {
	Device    bool `json:"device"`
	Simulator bool `json:"simulator"`
}

// ConnectionTrust exposes transport evidence, never the user's local allowlist.
func (e *Endpoint) ConnectionTrust() TrustState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return TrustState{Device: e.alive && e.tlsVerified, Simulator: e.alive && e.tlsVerified && e.handshakeState == shipmodel.SmeStateComplete}
}

func (e *Endpoint) verifyExpected(raw [][]byte, chains [][]*x509.Certificate, expected string) error {
	e.mu.Lock()
	e.tlsVerified = false
	e.mu.Unlock()
	if len(raw) == 0 {
		return errors.New("Peer-Zertifikat fehlt")
	}
	leaf, err := x509.ParseCertificate(raw[0])
	if err != nil {
		return err
	}
	got, err := cert.SkiFromCertificate(leaf)
	if err != nil {
		return err
	}
	if got != expected {
		return errors.New("Anderes Gerät antwortet – SKI prüfen")
	}
	if err = e.verifyTrusted(raw, chains); err != nil {
		return err
	}
	e.mu.Lock()
	e.tlsVerified = true
	e.mu.Unlock()
	return nil
}
