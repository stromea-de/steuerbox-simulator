package inspection

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type preflight023 struct {
	onsiteFake
	prompts, writes int
	cancel          context.CancelFunc
}

func (f *preflight023) LowLoad(_ context.Context, v LowLoad) (bool, error) {
	f.prompts++
	if f.writes != 0 || len(v.Directions) != 2 {
		return false, errors.New("prompt must cover both directions before writes")
	}
	return false, nil
}
func (f *preflight023) Configure(context.Context, Config) error {
	f.writes++
	f.cancel()
	return context.Canceled
}
func TestCombinedLastDecisionBeforeWrites023(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &preflight023{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, LPP: true, MPC: true}, fake: fake{powers: []float64{0}}}, cancel: cancel}
	c := onsiteConfig()
	c.PauseLowLoad = true
	r, e := RunOnSite(ctx, c, Device{}, f)
	if e != nil || f.prompts != 1 || f.writes != 1 {
		t.Fatal(e, f.prompts, f.writes, r.Steps)
	}
}

type dead023 struct {
	onsiteFake
	writes int
}

func (f *dead023) Configure(context.Context, Config) error { f.writes++; return ErrControlUnresponsive }
func (f *dead023) Limit(context.Context, string, float64, bool) (string, error) {
	f.writes++
	return "", ErrControlUnresponsive
}
func (f *dead023) WriteFailsafe(context.Context, string, string, FailsafeValues) error {
	f.writes++
	return ErrControlUnresponsive
}
func TestNoTimeoutCascadeAfterLostAck023(t *testing.T) {
	f := &dead023{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, LPP: true}}}
	r, e := RunOnSite(context.Background(), onsiteConfig(), Device{}, f)
	if e != nil || f.writes != 1 || !r.ControlUnresponsive {
		t.Fatal(e, f.writes, r)
	}
	restored := 0
	for _, s := range r.Steps {
		if strings.Contains(s.Name, "zurückstellen") {
			restored++
			if s.Passed {
				t.Fatal(s)
			}
		}
	}
	if restored != 4 || r.Outcome() != "Unvollständig" || r.AbortReason != "timeout" {
		t.Fatal(r.Steps)
	}
}
