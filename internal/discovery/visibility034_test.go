package discovery

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLoopbackAnnouncementVisibility034(t *testing.T) {
	r, err := StartResponder(Device{Name: "Fixture", SKI: "0123456789abcdef0123456789abcdef01234567", Address: "127.0.0.1:4711"})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, visible := range []bool{true, false, true} {
		r.SetVisible(visible)
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		got, err := Search(ctx, r.Address())
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		if (len(got) == 1) != visible {
			t.Fatal(visible, got)
		}
	}
}
