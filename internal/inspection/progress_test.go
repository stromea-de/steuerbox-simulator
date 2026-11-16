package inspection

import (
	"context"
	"reflect"
	"testing"
)

type observedBackend struct {
	fake
	started   []string
	completed []Step
}

func (b *observedBackend) StepStarted(name string) { b.started = append(b.started, name) }
func (b *observedBackend) StepFinished(s Step)     { b.completed = append(b.completed, s) }
func TestSimulatorProgressIncludesCleanup(t *testing.T) {
	b := &observedBackend{fake: fake{powers: []float64{11000, 4200, 500, 4200, -3000, -500, -3000, -8000}}}
	r, err := Run(context.Background(), testConfig(), Device{}, b)
	if err != nil || !r.Passed || !reflect.DeepEqual(r.Steps, b.completed) || len(b.started) != len(r.Steps) {
		t.Fatal("missing actual progress", err, b.started, b.completed)
	}
	for i, s := range r.Steps {
		if b.started[i] != s.Name {
			t.Fatalf("start/completion mismatch at %d", i)
		}
	}
}

type observedOnsite struct {
	onsiteFake
	completed []Step
}

func (b *observedOnsite) StepFinished(s Step) { b.completed = append(b.completed, s) }
func TestOnsiteProgressIncludesSkippedStepsAndCleanup(t *testing.T) {
	b := &observedOnsite{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}, fake: fake{powers: []float64{100}}}}
	r, err := RunOnSite(context.Background(), onsiteConfig(), Device{}, b)
	if err != nil || !reflect.DeepEqual(r.Steps, b.completed) {
		t.Fatal("incomplete progress", err, r.Steps, b.completed)
	}
	if b.completed[len(b.completed)-1].Name != "Grenzen aufheben" {
		t.Fatal("cleanup absent")
	}
	skipped := false
	for _, s := range b.completed {
		skipped = skipped || s.Status == NotChecked
	}
	if !skipped {
		t.Fatal("skipped steps not observed")
	}
}
