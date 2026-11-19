package inspection

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// ReturnNotEvaluable marks return steps whose failsafe entry was not shown (046).
const ReturnNotEvaluable = "Rückkehr nicht bewertbar: Notbetrieb nicht nachgewiesen"

// ProtocolHeartbeatTimeout is the device's heartbeat timeout from LPC/LPP 1.0.0
// (LPC-005/006, -911/912: failsafe after 120 s without a received heartbeat).
// A device run never evaluates failsafe earlier, whatever timeout is configured
// (046, R046-01). Only Config.TestHeartbeatTimeout (test partners) replaces it.
const ProtocolHeartbeatTimeout = 120 * time.Second

// DeviceOutageTimeout is the outage timeout a device run actually uses.
func (c Config) DeviceOutageTimeout() time.Duration {
	minimum := ProtocolHeartbeatTimeout
	if c.TestHeartbeatTimeout > 0 {
		minimum = c.TestHeartbeatTimeout
	}
	return max(c.FailsafeTimeout, minimum)
}

// FailsafeStateReporter is implemented by a backend whose device reports whether
// it is in failsafe. LPC/LPP 1.0.0 define no SPINE data for this state, so the
// EEBUS session does not implement it; a reported state must then match (046).
type FailsafeStateReporter interface {
	FailsafeState(ctx context.Context, direction string) (bool, error)
}

type failsafeMeasurement struct {
	name, direction string
	sign            float64
	config          Config
	// due: the device must be in failsafe from here on (reference + heartbeat
	// timeout); deadline: due + observation reserve.
	due, deadline time.Time
	// reference: heartbeat stop (or later receipt); the limit applied until then.
	reference time.Time
	// receipt returns the latest positive heartbeat receipt; a receipt after the
	// reference moves due and deadline through shift (R046-03).
	receipt func() time.Time
	shift   func(receipt time.Time) (due, deadline time.Time)
}

// measureFailsafe evaluates the failsafe effect only after the device's heartbeat
// timeout (046, real run 2026-11-18). LPC/LPP 1.0.0 [LPC-005/006, -911/912]: the
// controllable system enters failsafe once no heartbeat was received for 120 s;
// before that the active limit still applies. BK6-22-300 Anlage 1 Nr. 4 requires
// prompt implementation but names no seconds; the reserve is an own decision (ADR 027).
// A reading at or below the failsafe value only proves failsafe if the device
// was clearly above it right before (transition) or reports failsafe itself.
// It returns whether failsafe was shown and whether the run may continue.
func measureFailsafe(ctx context.Context, m failsafeMeasurement, b Backend, r *Report, ms *measurements, add func(string, string, string, error, *float64) bool) (bool, bool) {
	c := m.config
	shiftNote := ""
	stopped := m.reference
	startStep(b, m.name, time.Until(m.deadline))
	ms.current = m.name
	// moved applies a positive receipt later than the reference: it proves a later
	// reception at the device and moves the window (R046-03/-09/-19). A later
	// receipt that may not move it (after stop + timeout, or after three moves)
	// leaves the reference unclear and ends the evidence as not evaluable (R046-22/-26).
	moves := 0
	unclear := false // a later receipt exists that cannot move the window (R046-22)
	moved := func() bool {
		if m.receipt == nil || m.shift == nil {
			return false
		}
		latest := m.receipt()
		if !latest.After(m.reference) {
			return false
		}
		if moves >= 3 || latest.After(stopped.Add(c.FailsafeTimeout)) {
			unclear = true
			return false
		}
		moves++
		m.due, m.deadline = m.shift(latest)
		m.reference = latest
		shiftNote = fmt.Sprintf("; Frist ab späterer Quittung %s neu: Bewertung ab %s", latest.Format(time.RFC3339Nano), m.due.Format(time.RFC3339Nano))
		startStep(b, m.name, time.Until(m.deadline))
		ms.current = m.name
		return true
	}
	reporter, reports := b.(FailsafeStateReporter)
	var last *float64
wait:
	for {
		wctx, cancel := context.WithDeadline(ctx, m.deadline)
		err := waitFailsafeDue(wctx, ms, m.due, c.StepTimeout)
		cancel()
		if err != nil {
			return false, add(m.name, "", "", err, nil)
		}
		if moved() {
			continue
		}
		if unclear {
			return false, add(m.name, "Notbetrieb nicht bewertbar: spätere Heartbeat-Quittung verschiebt den Bezugszeitpunkt über das zulässige Maß, Zeitbezug unklar", NotChecked, nil, nil)
		}
		fctx, cancel := context.WithDeadline(ctx, m.deadline)
		before, early := transition(r, m)
		distinguishable := before != nil && m.sign**before > c.Failsafe+c.Tolerance && early == nil
		contradicted := false
		for {
			priorDevice := ms.validDevice // R046-23: newest known device time before this read
			v, err := ms.power(fctx)
			if err == nil {
				err = fctx.Err()
			}
			valid := err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
			if valid {
				owned := v
				last = &owned
			}
			if err != nil {
				cancel()
				if contradicted && last != nil && ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
					err = fmt.Errorf("Gerät meldet keinen Notbetrieb; nach Ablauf der Ausfallfrist MPC %.1f W", *last)
				} else if last != nil {
					err = fmt.Errorf("Messwirkung nicht nachgewiesen, letzter Wert %.1f W: %w", *last, err)
				} else {
					err = fmt.Errorf("Messwirkung nicht nachgewiesen: %w", err)
				}
				return false, add(m.name, "", "", err, last)
			}
			isFresh, timed := ms.fresh(priorDevice)
			if valid && isFresh && m.sign*v <= c.Failsafe+c.Tolerance && m.sign*v >= -c.Tolerance {
				state, stateErr := false, errors.New("kein Zustand")
				if reports {
					state, stateErr = reporter.FailsafeState(fctx, m.direction)
					if fctx.Err() != nil {
						continue // late answer: the next read reports the deadline or cancel
					}
				}
				// A receipt that arrived during this read moves the window: discard the candidate.
				if moved() {
					cancel()
					continue wait
				}
				if unclear {
					cancel()
					return false, add(m.name, "Notbetrieb nicht bewertbar: spätere Heartbeat-Quittung verschiebt den Bezugszeitpunkt über das zulässige Maß, Zeitbezug unklar", NotChecked, nil, last)
				}
				limit := fmt.Sprintf("MPC %.1f W nach Ablauf der Ausfallfrist; Obergrenze %.0f W; Toleranz %.1f W%s", v, c.Failsafe, c.Tolerance, shiftNote)
				if !timed {
					limit += "; Gerät sendet keinen absoluten Messzeitpunkt, Aktualität nur über neue Antwort nach Fristablauf"
				}
				switch {
				case stateErr == nil && state:
					cancel()
					recordDecisive(ms, v)
					return true, add(m.name, limit+"; Gerät meldet Notbetrieb", "", nil, last)
				case stateErr == nil || contradicted:
					// An explicit "not in failsafe" is not overridden by power or a later read error.
					contradicted = true
				case distinguishable:
					cancel()
					recordDecisive(ms, v)
					return true, add(m.name, fmt.Sprintf("%s; beim Stopp %.1f W", limit, *before), "", nil, last)
				default:
					cancel()
					recordDecisive(ms, v)
					why := "beim Heartbeat-Stopp keine gültige Messung"
					switch {
					case early != nil:
						why = fmt.Sprintf("Leistung fiel schon %s nach dem Stopp auf %.1f W, früher als ein Notbetrieb möglich ist", early.At.Sub(m.reference).Round(time.Millisecond), *early.Watts)
					case before != nil:
						why = fmt.Sprintf("beim Heartbeat-Stopp %.1f W, nicht über Notbetriebswert + Toleranz", *before)
					}
					return false, add(m.name, fmt.Sprintf("Notbetrieb nicht unterscheidbar: %s; %s; kein Gerätezustand gemeldet", why, limit), NotChecked, nil, last)
				}
			}
			select {
			case <-fctx.Done():
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
}

// transition returns the reading at the heartbeat stop and the first reading at
// or below the failsafe value that came earlier than a failsafe can start.
// At the stop the device still followed the active limit. LPC/LPP 1.0.0 send a
// heartbeat at least every 60 s and enter failsafe 120 s after the last
// receipt, so failsafe cannot start before reference + timeout/2. A drop before
// that is a load change, not failsafe (R046-16); later drops may be an early,
// compliant entry and are not held against the device.
func transition(r *Report, m failsafeMeasurement) (*float64, *Sample) {
	c := m.config
	var before *float64
	for i := len(r.Samples) - 1; i >= 0; i-- {
		if s := r.Samples[i]; !s.At.After(m.reference) {
			if s.Watts != nil && m.reference.Sub(s.At) <= HeartbeatReserve {
				v := *s.Watts
				before = &v
			}
			break
		}
	}
	earliest := m.reference.Add(c.FailsafeTimeout / 2)
	for i := range r.Samples {
		s := r.Samples[i]
		if s.At.After(m.reference) && s.At.Before(earliest) && s.Watts != nil && m.sign**s.Watts <= c.Failsafe+c.Tolerance {
			return before, &s
		}
	}
	return before, nil
}

// recordDecisive keeps the deciding post-timeout reading in the curve even if
// the same value was recorded shortly before (R046-03).
func recordDecisive(ms *measurements, v float64) {
	ms.last = time.Time{}
	ms.record(v, nil)
}

// waitFailsafeDue waits for the device's heartbeat timeout and keeps sampling
// MPC for the curve; each read ends at the due time at the latest.
func waitFailsafeDue(ctx context.Context, ms *measurements, due time.Time, readTimeout time.Duration) error {
	if !time.Now().Before(due) {
		return ctx.Err()
	}
	wctx, cancel := context.WithDeadline(ctx, due)
	defer cancel()
	_ = ms.wait(wctx, time.Until(due), readTimeout)
	if err := ctx.Err(); err != nil {
		return err
	}
	if time.Now().Before(due) {
		return context.DeadlineExceeded // the step ended before the device had to be in failsafe
	}
	return nil
}
