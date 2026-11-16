package inspection

import (
	"fmt"
	"regexp"
	"strings"
)

// TypeLabel uses SPINE DeviceTypeType raw values (spine-go v0.7.0 model/commondatatypes.go).
// Unknown values are preserved in JSON and the technical report.
func (d Device) TypeLabel() string {
	switch d.Type {
	// HeatPump is not an enum constant in v0.7.0; preserve a peer-provided extension.
	case "HeatPump":
		return "Wärmepumpe"
	case "HeatGenerationSystem":
		return "Wärmeerzeuger"
	case "Inverter":
		return "Wechselrichter"
	case "ChargingStation":
		return "Wallbox"
	case "EnergyManagementSystem":
		return "Energiemanagementsystem"
	case "Generic":
		return "Allgemeines EEBUS-Gerät (Generic)"
	case "":
		return "Gerätetyp nicht gemeldet"
	default:
		return "Nicht zugeordnet (" + d.Type + ")"
	}
}

// FunctionSummary names absent declarations explicitly, independently of the selected scope.
func (r Report) FunctionSummary() string {
	if r.Capabilities == nil {
		return "Gerätefunktionen nicht gespeichert"
	}
	result := ""
	for _, item := range []struct {
		name string
		yes  bool
	}{{"LPC", r.Capabilities.LPC}, {"LPP", r.Capabilities.LPP}, {"MPC", r.Capabilities.MPC}} {
		if result != "" {
			result += "; "
		}
		result += map[string]string{"LPC": "Bezug begrenzen (§14a, LPC)", "LPP": "Einspeisung begrenzen (LPP)", "MPC": "Leistung messen (MPC)"}[item.name] + ": "
		if item.yes {
			result += "gemeldet"
		} else {
			result += "vom Gerät nicht gemeldet"
		}
		if r.Capabilities.Scenarios != nil {
			result += fmt.Sprintf("; gemeldete Szenarien: %s", strings.Trim(strings.ReplaceAll(fmt.Sprint(r.Capabilities.Scenarios[item.name]), " ", ", "), "[]"))
		}
	}
	return result
}

// ModelLabel excludes certificate-derived instance identifiers from model names.
func (d Device) ModelLabel() string {
	model := strings.TrimSpace(d.Model)
	if model == d.SKI && model != "" || model == d.Serial && model != "" || regexp.MustCompile(`[0-9a-fA-F]{16,}`).MatchString(model) {
		return ""
	}
	return model
}
