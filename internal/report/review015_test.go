package report

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
	"time"
)

func TestCurveUsesSignedMeasuredPointsAndVisibleGap(t *testing.T) {
	at := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	r := inspection.Report{Config: inspection.DefaultConfig()}
	for i, v := range []float64{11000, 4200, 500, -3000} {
		v := v
		r.Samples = append(r.Samples, inspection.Sample{At: at.Add(time.Duration(i) * time.Second), Watts: &v, Step: "LPC gemessene Wirkung"})
	}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	// 038: all readings belong to the LPC section, so only the Bezug chart is drawn.
	if !strings.Contains(got, "Messverlauf") || !strings.Contains(got, "4.200 W") || !strings.Contains(got, "Bezug") {
		t.Fatal(got)
	}
	r.Samples[2].Watts = nil
	d = &document{}
	d.newPage()
	d.powerCurve(r)
	// 037 (UX-S19): the step trace holds until the failed read and never spans it.
	gaps := gapXs037(d.current().String())
	if len(gaps) != 1 {
		t.Fatalf("failed read marker missing: %v", gaps)
	}
	for _, path := range tracePaths037(t, d.current().String()) {
		before, after := false, false
		for _, v := range path {
			before = before || v[0] < gaps[0]-.011
			after = after || v[0] > gaps[0]+.011
		}
		if before && after {
			t.Fatal("gap connected")
		}
	}
	r.Config.Scope = inspection.ScopeConsumption
	d = &document{}
	d.newPage()
	d.powerCurve(r)
	if strings.Contains(d.current().String(), "3.000 W Einspeisung") {
		t.Fatal("irrelevant LPP line")
	}
}
