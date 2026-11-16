package webui

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

func TestLowLoadHTTPDecisionRequiresSessionToken(t *testing.T) {
	s := &session.Session{}
	a := New(s)
	defer a.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan bool, 1)
	go func() { retry, _ := s.LowLoad(ctx, inspection.LowLoad{Watts: 800, Required: 4250}); done <- retry }()
	deadline := time.After(time.Second)
	for s.Progress().LowLoad == nil {
		select {
		case <-deadline:
			t.Fatal("no prompt")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	post := func(token string) int {
		r := httptest.NewRequest("POST", "http://127.0.0.1:33287/api/low-load", strings.NewReader(`{"retry":true}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Stromea-Token", token)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w.Code
	}
	if code := post(""); code != 403 {
		t.Fatal(code)
	}
	if code := post(a.token); code != 200 {
		t.Fatal(code)
	}
	select {
	case retry := <-done:
		if !retry {
			t.Fatal("retry lost")
		}
	case <-time.After(time.Second):
		t.Fatal("decision did not reach run")
	}
	if code := post(a.token); code != 409 {
		t.Fatal("stale decision accepted", code)
	}
}
