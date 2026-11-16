package session

import (
	"context"
	"testing"
	"time"
)

// Connect must not finish between discovery and arrival of load-control values.
// A caller may issue a command immediately, without an arbitrary startup sleep.
func Test036PracticeConnectWaitsForLimitData(t *testing.T) {
	for i := 0; i < 30; i++ {
		s, err := New(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		err = s.Pair(s.Device.SKI)
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		if err == nil {
			err = s.Connect(ctx)
		}
		if err == nil {
			remote := s.EG.RemoteEntity()
			_, err = s.EG.LPC.ConsumptionLimit(remote)
			if err == nil {
				_, err = s.EG.LPP.ProductionLimit(remote)
			}
		}
		cancel()
		s.Close()
		if err != nil {
			t.Fatalf("connection %d returned before both limit values were ready: %v", i, err)
		}
	}
}
