package desktop

import (
	"strings"
	"testing"
)

func TestNativeExitMessage023(t *testing.T) {
	for _, busy := range []bool{false, true} {
		message := exitMessage(busy)
		for _, text := range []string{"Simulator beenden?", "Browserfensters", "Symbol im Infobereich"} {
			if !strings.Contains(message, text) {
				t.Fatal(message)
			}
		}
		if strings.Contains(message, "Prüfung oder Verbindung ist aktiv") != busy {
			t.Fatal(message)
		}
	}
}
