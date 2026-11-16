package eebus

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/enbility/ship-go/cert"
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
)

// peerCert040 builds a self-signed SHIP certificate with its own key and the given validity/SKI rule.
func peerCert040(t *testing.T, notBefore, notAfter time.Time, spkiDigest bool) ([]byte, string) {
	t.Helper()
	c, err := cert.CreateCertificate("test", "peer", "DE", "peer")
	if err != nil {
		t.Fatal(err)
	}
	template, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	template.NotBefore, template.NotAfter = notBefore, notAfter
	if spkiDigest {
		d := sha1.Sum(template.RawSubjectPublicKeyInfo)
		template.SubjectKeyId = d[:]
	}
	key := c.PrivateKey.(*ecdsa.PrivateKey)
	raw, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return raw, hex.EncodeToString(template.SubjectKeyId)
}

// 040 F-08: SHIP trusts by SKI pinning. A device clock without NTP (expired or not yet valid
// self-signed certificate) must not block the connection; it is only noted. A SKI derived from the
// whole SubjectPublicKeyInfo is still bound to the key and accepted; a copied SKI stays rejected.
func TestCertificateTimeAndSKIRule040(t *testing.T) {
	local, err := identity.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	eg, err := NewEndpoint(local, false)
	if err != nil {
		t.Fatal(err)
	}
	defer eg.Close()
	now := time.Now()
	for name, c := range map[string]struct {
		from, to time.Time
		spki     bool
	}{
		"expired":     {now.Add(-48 * time.Hour), now.Add(-24 * time.Hour), false},
		"not yet":     {now.Add(24 * time.Hour), now.Add(48 * time.Hour), false},
		"spki digest": {now.Add(-time.Hour), now.Add(time.Hour), true},
	} {
		raw, ski := peerCert040(t, c.from, c.to, c.spki)
		if err := eg.Trust(ski); err != nil {
			t.Fatal(err)
		}
		eg.mu.Lock()
		eg.connectionTrace = newConnectionTrace()
		eg.mu.Unlock()
		if err := eg.verifyTrusted([][]byte{raw}, nil); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		rows := eg.ConnectionTimings()
		noted := strings.Contains(rows[len(rows)-1].Detail, "Gültigkeitszeitraum")
		if noted != (name != "spki digest") {
			t.Fatalf("%s: validity note %v in %q", name, noted, rows[len(rows)-1].Detail)
		}
	}
}
