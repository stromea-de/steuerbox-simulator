package report

import (
	"bytes"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"testing"
	"time"
)

func Test035cPracticePDFDoesNotPresentConfiguredWaitsAsElapsedTime(t *testing.T) {
	r := sampleReport(t)
	r.Mode = inspection.ModeSimulator
	r.Config.EffectTimeout = 0
	r.Config.FailsafeTimeout = 0
	r.Config.FailsafeMinimum = 0
	data, err := PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("Wirkung messen")) {
		t.Error("practice PDF shows configured wait row")
	}
	if r.Config.EffectTimeout != 0 || r.Config.FailsafeMinimum != 0 {
		t.Error("stored configuration changed")
	}
	r.Mode = inspection.ModeOnSite
	r.Config.EffectTimeout = 2 * time.Second
	data, err = PDF(r)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("Wirkung messen")) || !bytes.Contains(data, []byte("(2 s)")) {
		t.Error("on-site wait settings missing")
	}
}
