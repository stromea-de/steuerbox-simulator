package report

import (
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func TestCurveScale(t *testing.T) {
	for _, c := range []struct{ low, high float64 }{{-11000, 0}, {0, 11000}, {-8000, 11000}, {0, 0}} {
		lo, hi, tick := curveScale(c.low, c.high)
		if lo > c.low || hi < c.high || lo > 0 || hi < 0 || hi <= lo || tick <= 0 {
			t.Fatalf("bad scale %v %v %v", lo, hi, tick)
		}
		if lo/tick != float64(int(lo/tick)) {
			t.Fatal("zero must be a tick")
		}
	}
}

func TestProductionCurveHasZeroAndSparsePoint(t *testing.T) {
	at := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	watts := -3000.0
	r := inspection.Report{Config: inspection.DefaultDeviceConfig(), Samples: []inspection.Sample{{At: at, Watts: &watts, Step: "LPP gemessene Wirkung"}}}
	r.Config.Scope = inspection.ScopeProduction
	r.Config.LPC = 0
	r.Config.LPP = 3000
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	if !strings.Contains(got, "(0) Tj") || !strings.Contains(got, "2.00 2.00 re f") || strings.Contains(got, "14a") {
		t.Fatal(got)
	}
}
