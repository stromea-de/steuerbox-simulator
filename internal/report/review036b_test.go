package report

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
	"time"
)

func Test036bChartLegendsDistinguishConnectionAndMissingReadings(t *testing.T) {
	r := sampleReport(t)
	v := 10996.0
	r.Samples = []inspection.Sample{{At: r.Started, Watts: &v}, {At: r.Started.Add(time.Second), Watts: &v}}
	d := &document{}
	d.newPage()
	d.powerCurve(r)
	got := d.current().String()
	for _, want := range []string{"Rot gestrichelt: Messausfall.", "Verbindungsleiste: Grau = verbunden; Hell = getrennt oder nicht beobachtet."} {
		if !strings.Contains(got, want) {
			t.Errorf("missing unambiguous legend: %s", want)
		}
	}
	if strings.Contains(got, "Linie unterbrochen") {
		t.Error("ambiguous interrupted-line legend remains")
	}
}
