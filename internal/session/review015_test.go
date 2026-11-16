package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

func TestSampleProgressIsBoundedOwnedAndRaceSafe(t *testing.T) {
	s := &Session{}
	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				v := float64(i)
				s.PowerSample(inspection.Sample{At: time.Now(), Watts: &v, Step: "MPC"})
				p := s.Progress()
				if len(p.Samples) > 0 {
					*p.Samples[0].Watts = -123
				}
			}
		}()
	}
	wg.Wait()
	p := s.Progress()
	if len(p.Samples) != 600 || *p.Samples[0].Watts < 0 {
		t.Fatal(p)
	}
	s.ResetProgress()
	if len(s.Progress().Samples) != 0 {
		t.Fatal("old samples retained")
	}
}
func TestLowLoadDecisionCancellationAndStaleSubmission(t *testing.T) {
	s := &Session{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := s.LowLoad(ctx, inspection.LowLoad{Watts: 800, Required: 4250}); done <- err }()
	deadline := time.After(time.Second)
	for s.Progress().LowLoad == nil {
		select {
		case <-deadline:
			t.Fatal("prompt not published")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatal(err)
	}
	if s.Progress().LowLoad != nil || s.ResolveLowLoad(true) == nil {
		t.Fatal("stale decision accepted")
	}
}
