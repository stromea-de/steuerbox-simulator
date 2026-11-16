package webui

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/eebus"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

// QA 039 Teil B: a practice run canceled right after its start must not leave
// the loopback session in a state where the next practice run times out. After
// the cancel's disconnect spine-go keeps the old remote device; Ready and the
// next write must not use it (reconnect would otherwise write into a closed
// transport and wait for a reply that never comes).
func TestPracticeRunAfterImmediateCancel039(t *testing.T) {
	for _, delay := range []time.Duration{0, 20 * time.Millisecond, 100 * time.Millisecond, 300 * time.Millisecond, 800 * time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) { practiceAfterCancel039(t, delay) })
	}
}

func practiceAfterCancel039(t *testing.T, delay time.Duration) {
	s, err := session.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	a := New(s)
	defer a.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	idle := func() {
		t.Helper()
		if err := eebus.Poll(ctx, func() bool { return state025(t, a)["busy"] == false }); err != nil {
			t.Fatal(err)
		}
	}
	body := `{"business":"Test","inspector":"Test","scope":"both","lpc":4200,"lpp":3000,"failsafe":500,"tolerance":50,"testProfile":"quick"}`
	if w := request025(a, "POST", "/api/simulator", `{}`); w.Code != http.StatusOK {
		t.Fatal(w)
	}
	if w := request025(a, "POST", "/api/run", body); w.Code != http.StatusAccepted {
		t.Fatal(w)
	}
	time.Sleep(delay)
	if w := request025(a, "POST", "/api/cancel", `{}`); w.Code != http.StatusOK {
		t.Fatal(w)
	}
	idle()
	a.mu.Lock()
	first := a.result
	a.mu.Unlock()
	if first == nil || first.AbortReason != "user_cancel" && !first.Passed {
		t.Fatalf("first run not recorded as user cancel: %+v", first)
	}
	if w := request025(a, "POST", "/api/simulator", `{}`); w.Code != http.StatusOK {
		t.Fatal(w)
	}
	if w := request025(a, "POST", "/api/pair", `{"ski":"`+s.Device.SKI+`"}`); w.Code != http.StatusOK {
		t.Fatal(w)
	}
	if w := request025(a, "POST", "/api/run", body); w.Code != http.StatusAccepted {
		t.Fatal(w)
	}
	idle()
	a.mu.Lock()
	second := a.result
	a.mu.Unlock()
	if second == nil || !second.Passed || second.Outcome() != "Bestanden" {
		t.Fatalf("practice run after cancel did not pass: %s %+v", second.AbortReason, second.Steps)
	}
}
