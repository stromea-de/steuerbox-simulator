package report

import (
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
)

func TestDeviceTypeAndMissingDeclarationsInExports016(t *testing.T) {
	r := sampleReport(t)
	r.Device.Type = "Inverter"
	r.Capabilities = &inspection.Capabilities{LPP: true, MPC: false}
	r.Config.Scope = inspection.ScopeProduction
	r.Config.LPC = 0
	data, err := JSON(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded inspection.Report
	if err = json.Unmarshal(data, &decoded); err != nil || decoded.Device.Type != "Inverter" {
		t.Fatal(err, decoded.Device)
	}
	pdf, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pdf), "Wechselrichter") || !strings.Contains(string(pdf), "vom Ger") {
		t.Fatal("type or absent declaration missing")
	}
	for _, check := range checkpoints(r) {
		if strings.Contains(check.text, "Verbrauchsgrenze") {
			t.Fatal("unused consumer check", check)
		}
	}
	if got := (inspection.Device{}).TypeLabel(); got != "Gerätetyp nicht gemeldet" {
		t.Fatal(got)
	}
}
