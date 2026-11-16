package inspection

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFunctionProjection034(t *testing.T) {
	r := Report{Mode: ModeOnSite, SchemaVersion: 1, Config: Config{Scope: ScopeConsumption, TestProfile: "quick", LPC: 4200, Tolerance: 50}, Steps: []Step{{Name: "LPC Grenze + Quittung", Passed: false}, {Name: "LPC Grenze + Quittung", Passed: true}}}
	c := r.FunctionCounts()
	if c.Failed != 1 || c.Open != 3 || c.Passed != 0 {
		t.Fatal(c)
	}
	if len(r.FunctionMatrix()) != 4 || r.FunctionMatrix()[0].LPP != "excluded" {
		t.Fatal(r.FunctionMatrix())
	}
	r.Config.TestProfile = "full"
	if r.FunctionCounts().Open != 4 {
		t.Fatal(r.FunctionCounts())
	}
	r.Config.TestProfile = "quick"
	r.Config.Scope = ScopeBoth
	if r.FunctionCounts().Failed != 1 || r.FunctionCounts().Open != 3 {
		t.Fatal(r.FunctionCounts())
	}
	if len(r.Steps) != 2 || r.Steps[0].Passed {
		t.Fatal("projection mutated evidence")
	}
}

func TestFunctionProjectionMatchesBrowser034(t *testing.T) {
	data, err := os.ReadFile("../../scripts/fixtures/functions034.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name   string
		Report Report
		Counts struct{ Passed, Failed, NotChecked, Open int }
		Matrix []struct{ Title, LPC, LPP string }
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			before, _ := json.Marshal(f.Report)
			got := f.Report.FunctionCounts()
			if got.Passed != f.Counts.Passed || got.Failed != f.Counts.Failed || got.NotChecked != f.Counts.NotChecked || got.Open != f.Counts.Open {
				t.Fatal(got, f.Counts)
			}
			rows := f.Report.FunctionMatrix()
			if len(rows) != len(f.Matrix) {
				t.Fatal(rows)
			}
			for i, row := range rows {
				want := f.Matrix[i]
				if row.Title != want.Title || row.LPC != want.LPC || row.LPP != want.LPP {
					t.Fatal(row, want)
				}
			}
			after, _ := json.Marshal(f.Report)
			if string(before) != string(after) {
				t.Fatal("evidence changed")
			}
		})
	}
}
