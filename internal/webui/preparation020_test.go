package webui

import (
	"context"
	"errors"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"testing"
)

type preparation020 struct{}

func (preparation020) ReadFailsafe(_ context.Context, k string) (inspection.FailsafeValues, error) {
	if k == "LPP" {
		return inspection.FailsafeValues{}, errors.New("unreadable")
	}
	return inspection.FailsafeValues{Power: 900}, nil
}
func (preparation020) Power(context.Context) (float64, error) { return -4000, nil }
func TestReadOnlyPreparation020(t *testing.T) {
	p := readPreparation(context.Background(), preparation020{}, &inspection.Capabilities{LPC: true, LPP: true, MPC: true})
	if p.Sample == nil || *p.Sample.Watts != -4000 || p.Sample.At.IsZero() || len(p.Originals) != 1 || p.Originals["LPC"].Power != 900 {
		t.Fatal(p)
	}
}
