package inspection

import (
	"errors"
	"time"
)

// HeartbeatReserve is an observation allowance, not a protocol requirement:
// 10 s transport jitter + 10 s fresh MPC read + 10 s electrical response.
// Source: research 018 Q7 (LPC 1.0.0), own decision ADR 027, 2026-10-30.
const HeartbeatReserve = 30 * time.Second

// NewHeartbeatObservation uses the later local reference. A receipt can only
// extend observation; its absence never removes the local stop reference.
func NewHeartbeatObservation(direction string, stopped, receipt time.Time, timeout time.Duration) HeartbeatObservation {
	reference := stopped
	if receipt.After(reference) {
		reference = receipt
	}
	h := HeartbeatObservation{Direction: direction, Stopped: stopped, Reference: reference,
		Basis:    "lokaler Stopp nach Sendesperre (spätere Quittung verlängert die Frist)",
		Deadline: HeartbeatDeadline(reference, timeout), Reserve: HeartbeatReserve}
	if !receipt.IsZero() {
		h.Receipt = &receipt
	}
	return h
}
func HeartbeatDeadline(reference time.Time, timeout time.Duration) time.Time {
	return reference.Add(timeout + HeartbeatReserve)
}

func (c Config) FullTest() bool { return c.TestProfile == "full" || c.TestProfile == "" && c.LongTest }
func (c Config) ProfileLabel() string {
	if c.FullTest() {
		return "Langzeitprüfung"
	}
	return "Funktionsprüfung"
}
func (c *Config) NormalizeProfile() {
	if c.FullTest() {
		c.TestProfile = "full"
	} else {
		c.TestProfile = "quick"
	}
	c.LongTest = c.FullTest()
}

// TimeBudget is the planning estimate after pairing; user preparation and pauses
// are additional. Keep the offline guide's budget in sync (cross-checked in 020).
func (c Config) TimeBudget() time.Duration {
	n := time.Duration(1)
	if c.Scope == ScopeBoth || c.Scope == "" {
		n = 2
	}
	perDirection := 3*c.EffectTimeout + c.DeviceOutageTimeout() + HeartbeatReserve + 10*c.StepTimeout + 3*c.CleanupTimeout
	if c.FullTest() {
		perDirection += c.FailsafeMinimum
	}
	return n * perDirection
}

// LowLoadDecision is the window of one load decision before the start (session.LowLoad).
const LowLoadDecision = 5 * time.Minute

// RunDeadline is the single hard deadline for a device run, used by WebUI and CLI (040 F-06). It covers
// the displayed TimeBudget, the wait for the outage deadline per direction, the recovery wait of the
// long-duration test and up to three load decisions before the start, plus five minutes reserve.
// The recovery timeout is ignored in the function test, which never waits for it.
func (c Config) RunDeadline() time.Duration {
	n := time.Duration(1)
	if c.Scope == ScopeBoth || c.Scope == "" {
		n = 2
	}
	total := c.TimeBudget() + n*(c.DeviceOutageTimeout()+HeartbeatReserve+c.StepTimeout)
	if c.FullTest() && c.RecoveryTimeout > c.FailsafeMinimum {
		total += n * (c.RecoveryTimeout - c.FailsafeMinimum)
	}
	if c.PauseLowLoad {
		total += 3 * (LowLoadDecision + c.StepTimeout)
	}
	return total + 5*time.Minute
}

// ValidateMinimums enforces the form's 1 s lower bound for every timeout the request uses (040 F-05).
// Run/RunOnSite deliberately keep accepting shorter values for synthetic tests.
func (c Config) ValidateMinimums(device bool) error {
	values := []time.Duration{c.StepTimeout}
	if device {
		values = append(values, c.EffectTimeout, c.FailsafeTimeout, c.CleanupTimeout)
		if c.FullTest() {
			values = append(values, c.RecoveryTimeout)
		}
	}
	for _, d := range values {
		if d < time.Second {
			return errors.New("Zeitlimits müssen mindestens 1 s betragen")
		}
	}
	return nil
}

type HeartbeatObservation struct {
	Stopped   time.Time     `json:"stopped"`
	Receipt   *time.Time    `json:"receipt,omitempty"`
	Direction string        `json:"direction"`
	Reference time.Time     `json:"reference"`
	Basis     string        `json:"basis"`
	Deadline  time.Time     `json:"deadline"`
	Reserve   time.Duration `json:"reserve"`
}

// ProtocolTimeBudget omits unobservable electrical effects and minimum-duration
// verification. The conservative outage deadline is still included in full.
func (c Config) ProtocolTimeBudget() time.Duration {
	n := time.Duration(1)
	if c.Scope == ScopeBoth || c.Scope == "" {
		n = 2
	}
	return n * (c.DeviceOutageTimeout() + HeartbeatReserve + 10*c.StepTimeout + 3*c.CleanupTimeout)
}
