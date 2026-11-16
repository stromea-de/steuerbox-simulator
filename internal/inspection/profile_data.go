package inspection

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// Profile is a local report snapshot. Customer data is never part of the profile.
type InspectorProfile struct {
	Business   string `json:"business,omitempty"`
	Street     string `json:"street,omitempty"`
	PostalCode string `json:"postalCode,omitempty"`
	City       string `json:"city,omitempty"`
	Country    string `json:"country,omitempty"` // 041: Standard „Deutschland“
	Inspector  string `json:"inspector,omitempty"`
	Logo       string `json:"logo,omitempty"`
}

func (p InspectorProfile) Validate() error {
	for _, s := range []string{p.Business, p.Street, p.PostalCode, p.City, p.Country, p.Inspector} {
		if len([]rune(s)) > 200 {
			return errors.New("Profil: höchstens 200 Zeichen je Feld")
		}
	}
	if p.Logo != "" {
		_, err := p.Image()
		return err
	}
	return nil
}
func (p InspectorProfile) Image() (image.Image, error) {
	fail := errors.New("Logo: PNG/JPG bis 640 × 640 Pixel und 750 KB erforderlich")
	if len(p.Logo) > 1000000 {
		return nil, fail
	}
	var raw string
	for _, prefix := range []string{"data:image/png;base64,", "data:image/jpeg;base64,"} {
		if strings.HasPrefix(p.Logo, prefix) {
			raw = strings.TrimPrefix(p.Logo, prefix)
		}
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(b) == 0 || len(b) > 750000 {
		return nil, fail
	}
	c, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || c.Width < 1 || c.Height < 1 || c.Width > 640 || c.Height > 640 {
		return nil, fail
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fail
	}
	return im, nil
}

// An absent optional profile must not alter legacy report hashes/identities.
// A pointer in the wire wrapper avoids encoding/json's non-empty struct rule.
func (c Config) MarshalJSON() ([]byte, error) {
	type wireConfig Config
	wire := struct {
		wireConfig
		Profile *InspectorProfile `json:"profile,omitempty"`
	}{wireConfig: wireConfig(c)}
	if c.Profile != (InspectorProfile{}) {
		wire.Profile = &c.Profile
	}
	return json.Marshal(wire)
}
