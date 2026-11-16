package report

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"testing"
)

func TestPresentation035(t *testing.T) {
	for _, name := range []string{"Last vor Prüfbeginn prüfen", "Heartbeat starten", "Failsafe-Werte setzen", "Verbindung", "Unterstützte Use Cases", "LPC Failsafe Ursprungswerte"} {
		if got := curvePhase(name); got != "Ausgangsleistung" {
			t.Errorf("preparation %q: %q", name, got)
		}
	}
	if got := inspection.StepName("Heartbeat starten"); got != "Lebenszeichen senden (Heartbeat)" {
		t.Errorf("heartbeat is not a control signal: %q", got)
	}
	d := inspection.Device{Type: "Generic"}
	if got := d.TypeLabel(); got != "Allgemeines EEBUS-Gerät (Generic)" {
		t.Errorf("friendly generic type: %q", got)
	}
	if d.Type != "Generic" {
		t.Fatal("raw manufacturer type changed")
	}
}
