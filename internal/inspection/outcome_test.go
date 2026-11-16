package inspection

import "testing"

func TestOutcomeCoverageAndFailures(t *testing.T) {
	r := Report{Passed: true, Mode: ModeSimulator}
	if r.Outcome() != "Unvollständig" {
		t.Fatal("empty report passed")
	}
	for _, group := range r.RequiredStepGroups() {
		for _, name := range group {
			r.Steps = append(r.Steps, Step{Name: name, Passed: true})
		}
	}
	if r.Outcome() != "Bestanden" {
		t.Fatal(r.Outcome())
	}
	// A positive aggregate flag must not hide an explicitly untested step.
	r.Steps[2].Status = NotChecked
	if r.Outcome() != "Unvollständig" {
		t.Fatal("not_checked hidden")
	}
	r.Passed = false
	if r.Outcome() != "Unvollständig" {
		t.Fatal("negative aggregate hid not_checked status")
	}
	r.Passed = true
	r.Steps[2].Status = ""
	r.Steps = append(r.Steps, Step{Name: r.Steps[2].Name, Passed: false})
	if r.Outcome() != "Nicht bestanden" {
		t.Fatal("duplicate failure hidden")
	}
	r.Steps = r.Steps[:len(r.Steps)-1]
	r.Mode = ModeOnSite
	if r.Outcome() != "Unvollständig" {
		t.Fatal("onsite recovery coverage missing")
	}
}
