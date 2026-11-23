package webui

import (
	"context"
	"strings"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// restoreOutcome reports the automatic restore after a restart (236).
type restoreOutcome struct {
	Device string            `json:"device"`
	OK     bool              `json:"ok"`
	Error  string            `json:"error,omitempty"`
	Steps  []inspection.Step `json:"steps,omitempty"`
	At     time.Time         `json:"at"`
}

type pendingRestoreView struct {
	Device string    `json:"device"`
	SKI    string    `json:"ski"`
	Saved  time.Time `json:"saved"`
}

func (a *App) pendingRestoresLocked() []pendingRestoreView {
	if a.s == nil {
		return nil
	}
	var out []pendingRestoreView
	for _, p := range a.s.PendingRestores() {
		out = append(out, pendingRestoreView{Device: p.Device.Name, SKI: p.Device.SKI, Saved: p.Saved})
	}
	// R4-05: unreadable records are open as well and are named next to the others.
	for _, name := range a.s.BrokenRestores() {
		out = append(out, pendingRestoreView{Device: name})
	}
	return out
}

// startPendingRestore puts devices of an interrupted test back automatically after
// a restart (236): no dialog, the result appears as a line in the app. Records stay
// until the device confirmed every value; the next run on that device retries first.
func (a *App) startPendingRestore() {
	if a.s == nil || a.loopbackOnly {
		return
	}
	a.s.RecoverAllClaims()
	pending := a.s.PendingRestores()
	broken := a.s.BrokenRestores()
	if len(pending) == 0 && len(broken) == 0 {
		return
	}
	a.mu.Lock()
	a.busy = true
	a.kind = "restore"
	a.operation = "Originalwerte werden wiederhergestellt"
	done := make(chan struct{})
	a.operationDone = done
	ctx, cancel := context.WithCancel(a.ctx)
	a.runCancel = cancel // shutdown waits for the restore like for a run
	a.mu.Unlock()
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer close(done)
		defer cancel()
		var last *restoreOutcome
		var failed []string
		for _, p := range pending {
			if !a.networkAvailable() {
				last = &restoreOutcome{Device: p.Device.Name, Error: "Kein Netzwerk", At: time.Now()}
				failed = append(failed, p.Device.Name)
				continue
			}
			steps, ok, err := a.s.RestorePending(ctx, p.Device.SKI)
			outcome := &restoreOutcome{Device: p.Device.Name, OK: ok, Steps: steps, At: time.Now()}
			if err != nil {
				outcome.Error = inspection.UserError(err)
			}
			if !ok {
				failed = append(failed, p.Device.Name)
			}
			// R236-06: try every device; a failure stays the visible outcome.
			if last == nil || last.OK || !ok {
				last = outcome
			}
		}
		// R2-06/R3-06: unreadable records stay on disk and stay visible next to any
		// other failure: the line names every device that is not restored.
		if len(broken) > 0 && (last == nil || last.OK) {
			last = &restoreOutcome{Error: "Gespeicherte Originalwerte nicht lesbar; Datensatz bleibt erhalten, Werte am Gerät prüfen", At: time.Now()}
		}
		if failed = append(failed, broken...); len(failed) > 0 && last != nil && !last.OK {
			last.Device = strings.Join(failed, ", ")
		}
		a.mu.Lock()
		a.runCancel = nil
		a.busy = false
		a.kind = ""
		a.operation = ""
		a.restoreResult = last
		a.mu.Unlock()
	}()
}
