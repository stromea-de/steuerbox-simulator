package session

import (
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// 046 (R046-12): a confirmation amends the live change with the same time and
// direction instead of appending a contradicting second entry.
func TestLimitChangedConfirmationReplaces046(t *testing.T) {
	s := &Session{}
	at := time.Now()
	w := 500.0
	s.LimitChanged(inspection.LimitChange{At: at, Direction: "LPC", Watts: &w, Failsafe: true, Unconfirmed: true})
	s.LimitChanged(inspection.LimitChange{At: at, Direction: "LPP", Watts: &w, Failsafe: true, Unconfirmed: true})
	s.LimitChanged(inspection.LimitChange{At: at, Direction: "LPC", Watts: &w, Failsafe: true})
	if len(s.limits) != 2 || s.limits[0].Unconfirmed || !s.limits[1].Unconfirmed {
		t.Fatalf("%+v", s.limits)
	}
	s.LimitChanged(inspection.LimitChange{At: at.Add(time.Second), Direction: "LPC"})
	if len(s.limits) != 3 {
		t.Fatalf("%+v", s.limits)
	}
}
