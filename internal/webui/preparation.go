package webui

import (
	"context"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"math"
	"net/http"
	"time"
)

type preparationValue struct {
	Key    string `json:"key"`
	Value  any    `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}
type preparation struct {
	Values []preparationValue `json:"values"`

	Sample    *inspection.Sample                   `json:"sample,omitempty"`
	Originals map[string]inspection.FailsafeValues `json:"originals"`
}
type preparationReader interface {
	Power(context.Context) (float64, error)
	ReadFailsafe(context.Context, string) (inspection.FailsafeValues, error)
}

// Read-only previews are advisory. Inspection always freshly reads originals
// again before any configuration change. Failed reads never become zero values.
func readPreparation(ctx context.Context, b preparationReader, caps *inspection.Capabilities) preparation {
	ctx, cancelAll := context.WithTimeout(ctx, 15*time.Second)
	defer cancelAll()
	p := preparation{Originals: map[string]inspection.FailsafeValues{}, Values: []preparationValue{}}
	reason := func(err error) string {
		if err != nil {
			if ctx.Err() != nil || err == context.DeadlineExceeded {
				return "Zeitlimit erreicht"
			}
			return "Gerät meldet keinen Wert"
		}
		return "Gerät meldet keinen Wert"
	}

	if caps == nil {
		p.Values = append(p.Values, preparationValue{Key: "MPC", Reason: "Gerätefunktionen nicht verfügbar"})
		return p
	}
	if caps.MPC {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		v, err := b.Power(c)
		cancel()
		if err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
			p.Sample = &inspection.Sample{At: time.Now(), Watts: &v}
			p.Values = append(p.Values, preparationValue{Key: "MPC", Value: v})
		} else {
			p.Values = append(p.Values, preparationValue{Key: "MPC", Reason: reason(err)})
		}
	}
	if !caps.MPC {
		p.Values = append(p.Values, preparationValue{Key: "MPC", Reason: "MPC nicht vom Gerät gemeldet"})
	}
	for _, k := range []string{"LPC", "LPP"} {
		if k == "LPC" && !caps.LPC || k == "LPP" && !caps.LPP {
			continue
		}
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		v, err := b.ReadFailsafe(c, k)
		cancel()
		if err == nil {
			p.Originals[k] = v
		}
		for _, field := range []string{"power", "minimum"} {
			item := preparationValue{Key: k + "." + field}
			if err != nil && !(field == "power" && v.PowerNumber != nil || field == "minimum" && v.MinimumRaw != "" && v.Minimum > 0) {
				item.Reason = reason(err)
			} else if field == "power" && !math.IsNaN(v.Power) && !math.IsInf(v.Power, 0) && v.Power >= 0 {
				item.Value = v.Power
			} else if field == "minimum" && v.Minimum > 0 {
				item.Value = v.Minimum.Seconds()
			} else {
				item.Reason = "Gerät meldet keinen Wert"
			}
			p.Values = append(p.Values, item)
		}
	}
	return p
}

// prepareRead is replaced in tests to simulate a slow device read.
var prepareRead = readPreparation

// prepare reads advisory device values. 040 F-04: the read uses the application context, so closing or
// reloading the tab does not abort it (UX-G12), and canceling it ends only the read, never the connection.
func (a *App) prepare(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct{}
	if !decode(w, r, &in) {
		return
	}
	a.mu.Lock()
	if a.blockedLocked() || !a.s.LiveConnected() || !a.s.IsDeviceMode() {
		a.mu.Unlock()
		http.Error(w, "Gerät muss verbunden und unbeschäftigt sein", 409)
		return
	}
	a.busy = true
	previousKind, previousOperation := a.kind, a.operation
	a.kind = "prepare"
	caps := a.capabilities
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	done := make(chan struct{})
	a.runCancel, a.operationDone = cancel, done
	a.wg.Add(1)
	defer a.wg.Done()
	defer close(done)
	a.mu.Unlock()
	p := prepareRead(ctx, a.s, caps)
	canceled := ctx.Err() != nil
	a.mu.Lock()
	a.runCancel = nil
	if !canceled {
		a.preparation = &p
	}
	a.busy = false
	a.kind, a.operation = previousKind, previousOperation // 040 F-07: no stale "Abbruch angefordert" text
	a.mu.Unlock()
	if canceled {
		http.Error(w, "Lesen abgebrochen", http.StatusConflict)
		return
	}
	send(w, p)
}
