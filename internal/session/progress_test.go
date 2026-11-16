package session

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"sync"
	"testing"
)

func TestProgressSnapshotIsOwnedAndSynchronized(t *testing.T) {
	s := &Session{}
	var wg sync.WaitGroup
	for n := 0; n < 10; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.StepStarted("Messung")
			s.StepFinished(inspection.Step{Name: "Messung", Passed: true})
			_ = s.Progress()
		}()
	}
	wg.Wait()
	p := s.Progress()
	if len(p.Steps) != 10 {
		t.Fatal(p)
	}
	p.Steps[0].Name = "modified"
	if s.Progress().Steps[0].Name != "Messung" {
		t.Fatal("snapshot aliases mutable slice")
	}
}
