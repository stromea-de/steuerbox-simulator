package session

import (
	"context"
	"sync"
	"testing"
	"time"
)

// 040 F-10: Connect and the control path read deviceMode/limitDuration only under s.mu.
// Under `go test -race` (scripts/check.sh) concurrent locked writers must not report a data race.
func TestModeFieldsReadUnderLock040(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			s.mu.Lock()
			s.deviceMode = false
			s.limitDuration = 0
			s.mu.Unlock()
			time.Sleep(time.Millisecond)
		}
	}()
	for i := 0; i < 3; i++ {
		if err := s.Connect(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Limit(ctx, "LPC", 4200, true); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()
}
