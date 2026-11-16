package session

import (
	"context"
	"fmt"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
	"time"
)

func TestForeignLPPOnlyWireSignAndFailsafeRestore(t *testing.T) {
	s, err := NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, d := startForeign(t, s, "EEBUS_013_NO_LPC=1")
	if err = s.SelectDevice(d, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	caps, err := s.Capabilities(ctx)
	if err != nil || caps.LPC || !caps.LPP || len(caps.Scenarios["LPP"]) != 3 || len(caps.Scenarios["LPC"]) != 0 || len(caps.Scenarios["MPC"]) != 1 {
		t.Fatal(caps, err)
	}
	if _, err = s.Limit(ctx, "LPP", 3000.125, true); err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f.in, "lpp-value")
	if got := f.line(t, "LPPVALUE "); got != "-3000.125 <nil>" {
		t.Fatal("wrong encoded LPP", got)
	}
	r, err := s.Run(context.Background(), foreignConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range r.Steps {
		if !v.Passed && v.Status != inspection.NotChecked {
			t.Fatal(v)
		}
	}
	fmt.Fprintln(f.in, "failsafe")
	if got := f.line(t, "FAILSAFE "); !strings.HasPrefix(got, "900.000 2h0m0s <nil> <nil>") {
		t.Fatal("original failsafe not restored", got)
	}
}

func TestForeignFailsafeRestoresExactOriginalWirePrecision(t *testing.T) {
	s, err := NewController(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	f, d := startForeign(t, s, "EEBUS_013_NO_LPC=1", "EEBUS_013_PRECISE=1")
	if err = s.SelectDevice(d, true); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	original, err := s.ReadFailsafe(ctx, "LPP")
	if err != nil {
		t.Fatal(err)
	}
	if original.PowerNumber == nil || *original.PowerNumber != 900123456 || original.MinimumRaw != "PT180M" {
		t.Fatal(original)
	}
	if err = s.Heartbeat(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Limit(ctx, "LPP", 0, false); err != nil {
		t.Fatal(err)
	}
	for _, v := range []inspection.FailsafeValues{{Power: 1.001, Minimum: 2 * time.Hour}, original} {
		for _, field := range []string{"power", "minimum"} {
			if err = s.WriteFailsafe(ctx, "LPP", field, v); err != nil {
				t.Fatal(field, err)
			}
		}
	}
	fmt.Fprintln(f.in, "failsafe-raw")
	if got := f.line(t, "RAW "); got != "900123456 -6 PT180M" {
		t.Fatal("original representation changed", got)
	}
}
