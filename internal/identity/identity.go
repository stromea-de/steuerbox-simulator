// Package identity stores locally generated SHIP credentials; private keys never enter logs.
package identity

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/enbility/ship-go/cert"
)

type Identity struct {
	Certificate tls.Certificate
	SKI         string
}

func NormalizeSKI(s string) (string, error) {
	s = strings.ToLower(strings.Join(strings.Fields(strings.NewReplacer(":", "", "-", "").Replace(s)), ""))
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 20 {
		return "", errors.New("SKI muss aus 40 Hex-Zeichen bestehen")
	}
	return s, nil
}

func LoadOrCreate(dir string) (Identity, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return Identity{}, err
	}
	p := filepath.Join(dir, "identity.pem")
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		c, e := cert.CreateCertificate("Stromea", "Steuerbox-Simulator", "DE", "Loopback")
		if e != nil {
			return Identity{}, e
		}
		key, e := x509.MarshalECPrivateKey(c.PrivateKey.(*ecdsa.PrivateKey))
		if e != nil {
			return Identity{}, e
		}
		data = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate[0]}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: key})...)
		f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return Identity{}, e
		}
		_, e = f.Write(data)
		closeErr := f.Close()
		if e != nil {
			return Identity{}, e
		}
		if closeErr != nil {
			return Identity{}, closeErr
		}
	} else if err != nil {
		return Identity{}, err
	}
	return Parse(data)
}

// Parse reads a PEM identity (certificate and private key) without storing it (041: import).
func Parse(data []byte) (Identity, error) {
	c, err := tls.X509KeyPair(data, data)
	if err != nil {
		return Identity{}, fmt.Errorf("lokale Identität beschädigt: %w", err)
	}
	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		return Identity{}, err
	}
	ski, err := cert.SkiFromCertificate(leaf)
	return Identity{c, ski}, err
}
