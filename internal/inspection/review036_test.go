package inspection

import (
	"context"
	"testing"
)

type directional036 struct {
	onsiteFake
	t       *testing.T
	want    string
	cancel  context.CancelFunc
	prompts int
}

func (f *directional036) LowLoad(_ context.Context, low LowLoad) (bool, error) {
	f.prompts++
	if len(low.Directions) != 1 || low.Directions[0].Direction != f.want {
		f.t.Fatalf("wrong insufficient direction for signed power: %+v", low)
	}
	return false, nil
}
func (f *directional036) Configure(context.Context, Config) error {
	f.cancel()
	return context.Canceled
}

func Test036LowLoadKeepsImportAndFeedInSeparate(t *testing.T) {
	for _, tc := range []struct {
		watts float64
		want  string
	}{{10996, "LPP"}, {-10996, "LPC"}} {
		ctx, cancel := context.WithCancel(context.Background())
		f := &directional036{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, LPP: true, MPC: true}, fake: fake{powers: []float64{tc.watts}}}, t: t, want: tc.want, cancel: cancel}
		c := onsiteConfig()
		c.LPP = 1000
		c.PauseLowLoad = true
		_, _ = RunOnSite(ctx, c, Device{}, f)
		cancel()
		if f.prompts != 1 {
			t.Fatalf("expected a single preflight decision: %d", f.prompts)
		}
	}
}
