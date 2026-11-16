package inspection

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type fake struct {
	reject bool
	calls  []string
	powers []float64
}

func (f *fake) Connect(context.Context) error           { return nil }
func (f *fake) Supports(context.Context) error          { return nil }
func (f *fake) Configure(context.Context, Config) error { return nil }
func (f *fake) Heartbeat(_ context.Context, on bool) error {
	f.calls = append(f.calls, "heartbeat")
	return nil
}
func (f *fake) Limit(_ context.Context, kind string, _ float64, active bool) (string, error) {
	f.calls = append(f.calls, kind)
	if f.reject && active {
		return "", errors.New("Quittung abgelehnt: 7")
	}
	return "SPINE result 0", nil
}
func (f *fake) Power(context.Context) (float64, error) {
	if len(f.powers) == 0 {
		return 0, errors.New("keine Messdaten")
	}
	v := f.powers[0]
	f.powers = f.powers[1:]
	return v, nil
}
func testConfig() Config {
	c := DefaultConfig()
	c.Business = "Elektro Beispiel"
	c.Inspector = "Erika Muster"
	c.StepTimeout = 30 * time.Millisecond
	return c
}
func TestCompleteRun(t *testing.T) {
	f := &fake{powers: []float64{11000, 4200, 500, 4200, -3000, -500, -3000, -8000}}
	r, err := Run(context.Background(), testConfig(), Device{SKI: "abc"}, f)
	if err != nil || !r.Passed || len(r.Steps) != 16 {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestRejectedWriteFailsAndCleansUp(t *testing.T) {
	f := &fake{reject: true, powers: []float64{11000, -8000}}
	r, err := Run(context.Background(), testConfig(), Device{}, f)
	if err != nil || r.Passed {
		t.Fatalf("%v %v", r, err)
	}
	if f.calls[len(f.calls)-1] != "LPP" {
		t.Fatalf("cleanup missing: %v", f.calls)
	}
	if r.Steps[len(r.Steps)-1].Name != "Grenzen aufheben" {
		t.Fatal(r.Steps)
	}
}
func TestMissingMeasurementCannotPass(t *testing.T) {
	f := &fake{}
	r, err := Run(context.Background(), testConfig(), Device{}, f)
	if err != nil || r.Passed {
		t.Fatalf("%v %v", r, err)
	}
}
func TestRejectInvalidConfig(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), -1, 0} {
		c := testConfig()
		c.LPC = v
		if _, err := Run(context.Background(), c, Device{}, &fake{}); err == nil {
			t.Fatalf("accepted %v", v)
		}
	}
}
