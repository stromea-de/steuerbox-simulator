package inspection

import "testing"

func TestInstanceIsNotModel024c(t *testing.T) {
	for _, d := range []Device{{Model: "Stromea-0123456789abcdef0123456789abcdef01234567"}, {Model: "serial-123", Serial: "serial-123"}} {
		if d.ModelLabel() != "" {
			t.Fatal(d)
		}
	}
	if got := (Device{Model: "Opta"}).ModelLabel(); got != "Opta" {
		t.Fatal(got)
	}
}
