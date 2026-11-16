package inspection

import (
	"strings"
	"testing"
)

// 043 (Nutzer 2026-11-14): Pflicht ist nur der Name der prüfenden Person; die Firma ist optional.
func TestOnlyInspectorRequired043(t *testing.T) {
	c := DefaultDeviceConfig()
	c.Inspector = "Lucas Spohn"
	c.Business = ""
	if err := c.ValidateDevice(); err != nil {
		t.Fatal("empty company must be accepted:", err)
	}
	c.Business = "  "
	if err := c.Validate(); err != nil {
		t.Fatal("blank company must be accepted:", err)
	}
	c.Inspector = " "
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "Prüfer") {
		t.Fatal("missing inspector must name the inspector:", err)
	}
	c.Inspector = "Lucas Spohn"
	c.Business = strings.Repeat("ü", 201)
	if c.Validate() == nil {
		t.Fatal("company longer than 200 characters accepted")
	}
}
