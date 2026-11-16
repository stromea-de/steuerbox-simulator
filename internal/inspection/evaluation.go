package inspection

import (
	"context"
	"errors"
	"strings"
)

const (
	Checkpoint = "checkpoint"
	Workflow   = "workflow"
)

var ErrConnectionLost = errors.New("Verbindung zum Gerät unterbrochen")

// ErrNoLoadDecision marks an expired power decision (043, R043-04): it is a timeout, but not of a measurement.
var ErrNoLoadDecision = errors.New("Keine Entscheidung zur Leistung")

// Category is also the conservative migration rule for reports predating v1.
func StepCategory(name string) string {
	for _, s := range []string{"Ausgangsmessung MPC", "Grenze + Quittung", "gemessene Wirkung", "Wiederkehr", "Wiederkehr Protokoll", "Vorzeitige Freigabe"} {
		if name == s || name == "LPC "+s || name == "LPP "+s || strings.HasSuffix(name, " "+s) {
			return Checkpoint
		}
	}
	return Workflow
}
func (s Step) IsCheckpoint() bool {
	if s.Category != "" {
		return s.Category == Checkpoint
	}
	return StepCategory(s.Name) == Checkpoint
}
func (r Report) classify(s Step) Step {
	s.Category = StepCategory(s.Name)
	// 039 (B-03): a check interrupted by the user's cancel was not evaluated; it is no device failure.
	if !s.Passed && s.Status != NotChecked && s.Detail == ReasonText("user_cancel") {
		s.Status = NotChecked
		s.ReasonCode = "user_cancel"
	}
	if s.Category == Workflow && s.Status != NotChecked {
		s.Status = "done"
		if !s.Passed {
			s.Status = "failed"
		}
	}
	if s.Status == NotChecked && strings.Contains(s.Detail, "zu wenig Leistung") {
		s.ReasonCode = "insufficient_load"
	}
	required := s.Category == Checkpoint && !r.optionalStep(s.Name)
	s.Required = &required
	return s
}

type Counts struct {
	Passed     int `json:"passed"`
	Failed     int `json:"failed"`
	NotChecked int `json:"notChecked"`
}

// CheckCounts counts unique checks (worst recorded attempt wins), including
// missing required checks after an early exit. Workflow actions never add green.
func (r Report) CheckCounts() Counts {
	states := map[string]int{}
	for _, s := range r.Steps {
		if !s.IsCheckpoint() {
			continue
		}
		rank := 1
		if s.Status == NotChecked {
			rank = 2
		} else if !s.Passed {
			rank = 3
		}
		if rank > states[s.Name] {
			states[s.Name] = rank
		}
	}
	for _, group := range r.RequiredStepGroups() {
		for _, name := range group {
			if StepCategory(name) == Checkpoint && states[name] == 0 {
				states[name] = 2
			}
		}
	}
	var c Counts
	for _, rank := range states {
		switch rank {
		case 1:
			c.Passed++
		case 2:
			c.NotChecked++
		case 3:
			c.Failed++
		}
	}
	return c
}
func AbortReason(err error) string {
	switch {
	case errors.Is(err, ErrConnectionLost):
		return "connection_lost"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrControlUnresponsive):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "user_cancel"
	}
	return ""
}
func ReasonText(reason string) string {
	switch reason {
	case "connection_lost":
		return "Verbindung zum Gerät unterbrochen"
	case "insufficient_load":
		return "Nicht geprüft: zu wenig Leistung"
	case "user_cancel":
		return "Prüfung abgebrochen"
	case "timeout":
		return "Zeitlimit erreicht"
	}
	return ""
}

// UserError keeps diagnostic causes out of visible reports and API responses.
func UserError(err error) string {
	if err == nil {
		return ""
	}
	reason := AbortReason(err)
	if reason != "" {
		return ReasonText(reason)
	}
	return err.Error()
}
func (r *Report) finishEvaluation() {
	for i, s := range r.Steps {
		r.Steps[i] = r.classify(s)
	}
	if r.AbortReason != "" {
		r.AbortText = ReasonText(r.AbortReason)
	}
	r.Passed = r.Outcome() == "Bestanden"
}

// UserDetail sanitizes historical diagnostics without changing stored evidence.
func UserDetail(detail string) string {
	if strings.Contains(detail, "nicht geprüft (Langtest)") {
		return "Nicht geprüft: Nur die Langzeitprüfung wartet die Mindestdauer ab."
	}
	detail = strings.NewReplacer("frisch", "erneut", "quittiert", "bestätigt", "complete", "abgeschlossen", "Wiederkehr", "Rückkehr", "Langtest", "Langzeitprüfung").Replace(detail)
	return strings.NewReplacer("context canceled", ReasonText("user_cancel"), "context deadline exceeded", ReasonText("timeout")).Replace(detail)
}
