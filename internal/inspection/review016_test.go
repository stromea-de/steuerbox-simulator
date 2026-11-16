package inspection

import (
	"context"
	"strings"
	"testing"
)

func TestProductionOnlyScope016(t *testing.T) {
	c := onsiteConfig()
	c.Scope = "production"
	c.LPC = 0
	f := &preservingDevice{onsiteFake: onsiteFake{caps: Capabilities{LPP: true, MPC: true}, fake: fake{powers: []float64{-11000, -3000, -500, -11000}}}}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range r.Steps {
		if strings.HasPrefix(s.Name, "LPC") {
			t.Fatal("consumption step for inverter", s)
		}
	}
	for _, call := range f.calls {
		if call == "LPC" {
			t.Fatal("consumption write", f.calls)
		}
	}
	if r.Outcome() != "Bestanden" {
		t.Fatal(r.Outcome(), r.Steps)
	}
	c.LPP = 0
	if c.Validate() == nil {
		t.Fatal("zero production limit accepted")
	}
}

func TestReasonKeepsDecisionWithoutRawDiagnostics016(t *testing.T) {
	cases := map[string]string{
		"LPP nicht vom Gerät gemeldet":                                           "vom Gerät nicht gemeldet",
		"Wirkung nicht nachweisbar (zu wenig Leistung)":                          "Die Leistung reicht für diesen Nachweis nicht aus.",
		"MPC nicht vom Gerät gemeldet":                                           "Das Gerät meldet keine Leistungsmessung.",
		"Vorzeitige Rückkehr nicht nachgewiesen; Notbetrieb kann bis 2h bleiben": "Die Rückkehr wurde nicht bestätigt. Notbetrieb am Gerät prüfen.",
	}
	for raw, want := range cases {
		s := Step{Status: NotChecked, Detail: raw}
		if s.Reason() != want {
			t.Fatal(raw, s.Reason())
		}
		if s.Detail != raw {
			t.Fatal("recorded diagnostics modified")
		}
	}
}

func TestProductionOnlySimulatorScope016(t *testing.T) {
	c := testConfig()
	c.Scope = ScopeProduction
	c.LPC = 0
	f := &fake{powers: []float64{-11000, -3000, -500, -3000, -11000}}
	r, err := Run(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outcome() != "Bestanden" {
		t.Fatal(r.Outcome(), r.Steps)
	}
	for _, call := range f.calls {
		if call == "LPC" {
			t.Fatal("consumer command", f.calls)
		}
	}
}
