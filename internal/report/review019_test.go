package report

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
	"time"
)

func TestCurveReferencesNameDirection019(t *testing.T) {
	watts := -500.0
	r := inspection.Report{Config: inspection.DefaultDeviceConfig(), Samples: []inspection.Sample{{At: time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC), Watts: &watts, Step: "LPP Failsafe gemessene Wirkung"}}}
	r.Config.LPP = 3000 // Explicit fixture; production has no default.
	r.Config.Scope = inspection.ScopeProduction
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	text := d.current().String()
	for _, label := range []string{"Einspeisung 3.000 W", "Einspeisung 500 W Notbetrieb"} {
		if !strings.Contains(text, label) {
			t.Errorf("missing direction label %q", label)
		}
	}
	if strings.Contains(text, "-500 W Notbetrieb") {
		t.Error("reference label must name the direction instead of mixing signs")
	}
}
