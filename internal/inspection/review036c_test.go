package inspection

import "testing"

func Test036cOptionalMinimumIsNotMissingEvidence(t *testing.T) {
	r := Report{Mode: ModeOnSite, Config: DefaultDeviceConfig(), Steps: []Step{
		{Name: "LPC Wiederkehr", Status: NotChecked, Detail: "Langtest nicht durchgeführt"},
		{Name: "LPC gemessene Wirkung", Status: NotChecked, Detail: "zu wenig Leistung"},
	}}
	r.Config.Scope = ScopeConsumption
	if missing := r.OpenEvidence(); len(missing) != 1 || missing[0].Name != "LPC gemessene Wirkung" {
		t.Fatalf("optional minimum reported as required evidence: %+v", missing)
	}
	r.Config.TestProfile = "full"
	if len(r.OpenEvidence()) != 2 {
		t.Fatal("full test must retain missing minimum-duration evidence")
	}
	r.Mode = ModeSimulator
	r.Config.TestProfile = "quick"
	if len(r.OpenEvidence()) != 2 {
		t.Fatal("practice recovery evidence must not be hidden")
	}
}
