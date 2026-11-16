package inspection

import "testing"

// B-03: a run the user aborts is incomplete, not failed; the interrupted check is "not checked".
func TestUserCancelIsNotAFailure039(t *testing.T) {
	r := Report{Mode: ModeSimulator, SchemaVersion: 1, Config: Config{Scope: ScopeConsumption, TestProfile: "quick", LPC: 4200, Tolerance: 50}, AbortReason: "user_cancel"}
	r.Steps = []Step{
		{Name: "LPC Grenze + Quittung", Passed: true, Detail: "Bestätigt"},
		{Name: "LPC Heartbeat-Ausfall", Passed: false, Detail: ReasonText("user_cancel")},
	}
	r.finishEvaluation()
	if got := r.Steps[1].Status; got != NotChecked {
		t.Fatalf("interrupted step status %q, want %q", got, NotChecked)
	}
	if c := r.CheckCounts(); c.Failed != 0 {
		t.Fatalf("aborted run counts a failure: %+v", c)
	}
	if got := r.Outcome(); got != "Unvollständig" {
		t.Fatalf("outcome %q, want Unvollständig", got)
	}
	// A genuine failure stays a failure.
	f := Report{Mode: ModeSimulator, SchemaVersion: 1, Config: r.Config, Steps: []Step{{Name: "LPC Grenze + Quittung", Passed: false, Detail: "Keine Quittung"}}}
	f.finishEvaluation()
	if f.Outcome() != "Nicht bestanden" {
		t.Fatalf("real failure: %q", f.Outcome())
	}
}
