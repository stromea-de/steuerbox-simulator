package inspection

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type lowBaseline024 struct{ onsiteFake }

func (f *lowBaseline024) Limit(ctx context.Context, k string, v float64, on bool) (string, error) {
	if on {
		return "", errors.New("synthetische Ablehnung nach Ausgangsmessung")
	}
	return "OK", nil
}
func TestZeroBaselineNotGreen024(t *testing.T) {
	f := &lowBaseline024{onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{0}}}}
	c := onsiteConfig()
	c.Scope = ScopeConsumption
	r, e := RunOnSite(context.Background(), c, Device{}, f)
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range r.Steps {
		if s.Name == "Ausgangsmessung MPC" {
			if s.Passed || s.Status != NotChecked || s.Watts == nil || *s.Watts != 0 {
				t.Fatalf("0 W baseline falsely green: %+v", s)
			}
			return
		}
	}
	t.Fatal("baseline absent")
}
func TestCancellationNeverExposesContext024(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e := RunOnSite(ctx, onsiteConfig(), Device{}, &onsiteFake{})
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range r.Steps {
		if strings.Contains(s.Detail, "context canceled") || strings.Contains(s.Detail, "context deadline exceeded") {
			t.Fatal(s.Detail)
		}
	}
}
func TestCheckpointCountsExcludeWorkflow024(t *testing.T) {
	r := Report{SchemaVersion: 1, Passed: true, Mode: ModeOnSite, Config: Config{Scope: ScopeConsumption, TestProfile: "quick"}}
	for _, g := range r.RequiredStepGroups() {
		for _, name := range g {
			r.Steps = append(r.Steps, Step{Name: name, Passed: true})
		}
	}
	r.Steps = append(r.Steps, Step{Name: "Heartbeat starten", Passed: true})
	for _, s := range r.Steps {
		s = r.classify(s)
		if s.Category == Workflow && s.Status != "done" {
			t.Fatal(s)
		}
	}
	c := r.CheckCounts()
	if c.Passed == 0 || c.Passed >= len(r.Steps) || c.Failed != 0 || c.NotChecked != 0 {
		t.Fatal(c)
	}
	for i := range r.Steps {
		if r.Steps[i].Name == "LPC gemessene Wirkung" {
			r.Steps[i].Status = NotChecked
		}
	}
	if r.Outcome() == "Bestanden" || r.CheckCounts().NotChecked != 1 {
		t.Fatal(r.Outcome(), r.CheckCounts())
	}
}
func TestRandomReportIDs024(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id, e := NewReportID()
		if e != nil || !ValidReportID(id) || seen[id] {
			t.Fatal(id, e)
		}
		seen[id] = true
	}
}
