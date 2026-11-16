package inspection

import "testing"

func TestMatrixKeepsFailuresAndScope022(t *testing.T) {
	r := Report{Config: Config{Scope: ScopeConsumption}, Steps: []Step{{Name: "LPC Grenze + Quittung", Passed: false}, {Name: "LPC Grenze + Quittung", Passed: true}, {Name: "LPC gemessene Wirkung", Status: NotChecked}}}
	rows := r.ResultMatrix()
	if len(rows) != 2 || rows[0].LPC != "failed" || rows[0].LPP != "excluded" || rows[1].LPC != "not_checked" {
		t.Fatal(rows)
	}
	r.Config.Scope = ScopeBoth
	if r.ResultMatrix()[0].LPP != "open" {
		t.Fatal("missing peer direction must stay open")
	}
}
func TestEvidenceGroups022(t *testing.T) {
	r := Report{Steps: []Step{{Name: "LPC gemessene Wirkung", Status: NotChecked, Detail: "MPC nicht vom Gerät gemeldet"}, {Name: "LPP gemessene Wirkung", Status: NotChecked, Detail: "MPC fehlt; zu wenig Leistung"}, {Name: "Verbindung", Passed: true}}}
	g := r.EvidenceGroups()
	if len(g) != 1 || len(g[0].Steps) != 2 {
		t.Fatal(g)
	}
}
