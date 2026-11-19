package inspection

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrControlUnresponsive = errors.New("Gerät antwortet nicht auf Steuerbefehle; nicht bestätigt. Verbindung neu aufbauen und Freigabe sowie Notbetriebs-Werte am Gerät prüfen")

const (
	ModeOnSite    = "onsite"
	ModeSimulator = "simulator"
	NotChecked    = "not_checked"
)

func (r Report) ModeLabel() string {
	if r.Mode == ModeOnSite {
		return "Gerät vor Ort"
	}
	return "Übungsmodus (Simulator)"
}

// RequiredStepGroups defines the coverage shared by PDF and web results.
// Additional failed steps also count, even if they are not in these groups.
func (r Report) RequiredStepGroups() [][]string {
	groups := [][]string{
		{"Verbindung", "Unterstützte Use Cases"},
		{"Ausgangsmessung MPC", "LPC Grenze + Quittung", "LPC gemessene Wirkung"},
		{"LPP Grenze + Quittung", "LPP gemessene Wirkung"},
		{"Failsafe-Werte setzen", "Heartbeat starten", "LPC Heartbeat-Ausfall", "LPC Failsafe gemessene Wirkung", "LPP Heartbeat-Ausfall", "LPP Failsafe gemessene Wirkung"},
		{"LPC Wiederkehr", "LPP Wiederkehr", "Grenzen aufheben"},
	}
	if r.Config.Scope == ScopeProduction {
		groups[1] = nil
		groups[2] = append([]string{"LPP Ausgangsmessung MPC"}, groups[2]...)
	}
	if r.SchemaVersion >= 1 && r.Mode == ModeOnSite && r.Config.Scope != ScopeConsumption && r.Config.Scope != ScopeProduction {
		groups[2] = append([]string{"LPP Ausgangsmessung MPC"}, groups[2]...)
	}
	if r.Mode == ModeOnSite {
		groups[4] = append(groups[4], "LPC Wiederkehr gemessene Wirkung", "LPP Wiederkehr gemessene Wirkung")
	}
	if r.Config.Scope != "" {
		for i, group := range groups {
			var filtered []string
			for _, name := range group {
				if !r.Config.Includes(strings.Split(name, " ")[0]) {
					continue
				}
				if r.optionalStep(name) {
					continue
				}
				filtered = append(filtered, name)
			}
			groups[i] = filtered
		}
	}
	if r.Mode == ModeOnSite && r.Config.Scope != "" {
		for _, kind := range []string{"LPC", "LPP"} {
			if !r.Config.Includes(kind) {
				continue
			}
			for _, suffix := range []string{" Failsafe Ursprungswerte", " Failsafe-Leistung zurückstellen", " Failsafe-Mindestdauer zurückstellen"} {
				groups[4] = append(groups[4], kind+suffix)
			}
			if !r.Config.FullTest() {
				groups[4] = append(groups[4], kind+" Vorzeitige Freigabe", kind+" Vorzeitige Rückkehr gemessene Wirkung")
			}
		}
	}
	return groups
}

func (r Report) Outcome() string {
	if r.SchemaVersion >= 1 {
		counts := r.CheckCounts()
		if counts.Failed > 0 {
			return "Nicht bestanden"
		}
		for _, s := range r.Steps {
			if s.IsCheckpoint() && s.Status == NotChecked && !r.optionalStep(s.Name) {
				return "Unvollständig"
			}
		}
		for _, g := range r.RequiredStepGroups() {
			for _, name := range g {
				if StepCategory(name) != Checkpoint {
					continue
				}
				found := false
				for _, s := range r.Steps {
					if s.Name == name {
						found = true
					}
				}
				if !found {
					return "Unvollständig"
				}
			}
		}
		if r.AbortReason != "" || !r.Passed {
			return "Unvollständig"
		}
		return "Bestanden"
	}

	for _, s := range r.Steps {
		if !s.Passed && s.Status != NotChecked {
			return "Nicht bestanden"
		}
	}
	for _, s := range r.Steps {
		if s.Status == NotChecked && !r.optionalStep(s.Name) {
			return "Unvollständig"
		}
	}
	for _, group := range r.RequiredStepGroups() {
		for _, name := range group {
			found := false
			for _, s := range r.Steps {
				if s.Name == name {
					found = true
					break
				}
			}
			if !found {
				return "Unvollständig"
			}
		}
	}
	// Preserve a negative overall result even for imported reports with all steps.
	if !r.Passed {
		if r.Mode == ModeOnSite {
			return "Unvollständig"
		}
		return "Nicht bestanden"
	}
	return "Bestanden"
}

const PartialExplanation = "Unvollständig: Für einzelne Funktionen fehlt ein Prüfnachweis. Einzelheiten stehen im technischen Anhang."

func (c Config) LimitLease() time.Duration {
	return c.EffectTimeout + c.DeviceOutageTimeout() + HeartbeatReserve + 2*c.StepTimeout + time.Minute
}

func DefaultDeviceConfig() Config {
	c := DefaultConfig()
	c.LPP = 1000 // Simulator suggestion only; electrical effect requires feed-in above this value.
	c.StepTimeout = 60 * time.Second
	c.EffectTimeout = 120 * time.Second
	c.FailsafeTimeout = 120 * time.Second
	c.FailsafeMinimum = 2 * time.Hour
	c.RecoveryTimeout = 2*time.Hour + 120*time.Second
	c.CleanupTimeout = 30 * time.Second
	return c
}
func (c Config) ValidateDevice() error {
	if err := c.Validate(); err != nil {
		return err
	}
	for _, d := range []time.Duration{c.EffectTimeout, c.FailsafeTimeout, c.RecoveryTimeout, c.CleanupTimeout} {
		if d <= 0 || d > 25*time.Hour {
			return errors.New("Zeitlimits müssen größer 0 und höchstens 25 h sein")
		}
	}
	if c.EffectTimeout > time.Hour || c.CleanupTimeout > 10*time.Minute {
		return errors.New("Wirkungs-Zeitlimit höchstens 1 h, Aufräumen höchstens 10 min")
	}
	if c.FailsafeTimeout > 120*time.Second {
		return errors.New("Ausfall-Erkennung darf höchstens 120 s dauern")
	}
	if c.FullTest() && c.RecoveryTimeout < c.FailsafeMinimum+c.StepTimeout {
		return errors.New("Wiederkehr-Zeitlimit muss Mindestdauer und Quittierung abdecken")
	}
	if c.FailsafeMinimum < 2*time.Hour || c.FailsafeMinimum > 24*time.Hour {
		return errors.New("Failsafe-Mindestdauer muss zwischen 2 h und 24 h liegen")
	}
	return nil
}

type Capabilities struct {
	LPC       bool              `json:"lpc"`
	LPP       bool              `json:"lpp"`
	MPC       bool              `json:"mpc"`
	Scenarios map[string][]uint `json:"scenarios,omitempty"`
}
type FailsafeValues struct {
	Source      string        `json:"source,omitempty"`
	ReadAt      time.Time     `json:"readAt,omitempty"`
	Power       float64       `json:"power"`
	Minimum     time.Duration `json:"minimum"`
	PowerNumber *int64        `json:"powerNumber,omitempty"`
	PowerScale  *int8         `json:"powerScale,omitempty"`
	MinimumRaw  string        `json:"minimumRaw,omitempty"`
}

type DeviceBackend interface {
	Backend
	Capabilities(context.Context) (Capabilities, error)
	DeviceInfo() Device
	LastHeartbeatReceipt() (time.Time, string)
	Recover(context.Context) error
	ReadFailsafe(context.Context, string) (FailsafeValues, error)
	WriteFailsafe(context.Context, string, string, FailsafeValues) error
}

// RunOnSite never assumes load direction can be changed by a control command.
// A physical disconnection cannot guarantee release: cleanup reconnects and records
// a missing acknowledgment as failure. Each active write also carries a finite lease.
func RunOnSite(ctx context.Context, c Config, d Device, b DeviceBackend) (Report, error) {
	if err := c.ValidateDevice(); err != nil {
		return Report{}, err
	}
	c.NormalizeProfile()
	raised := ""
	if t := c.DeviceOutageTimeout(); t != c.FailsafeTimeout {
		// 046 (R046-01): the device must not be in failsafe before its 120 s timeout.
		raised = fmt.Sprintf(" Ausfallfrist von %s auf %s angehoben (LPC/LPP 1.0.0).", c.FailsafeTimeout, t)
		c.FailsafeTimeout = t
	}
	id, err := NewReportID()
	if err != nil {
		return Report{}, err
	}
	r := Report{ID: id, SchemaVersion: 1, Business: c.Business, Inspector: c.Inspector, Started: time.Now(), Device: d, Config: c, Mode: ModeOnSite, Notice: Notice, Passed: true,
		Profile: fmt.Sprintf("%s; Gerät vor Ort; SPINE-Messwerte des Geräts, keine unabhängige Leistungsmessung. Schritt %s; Wirkung %s; Heartbeat-Ausfall %s ab lokalem Stopp nach Sendesperre (spätere positive Quittung verlängert) + 30 s Beobachtungsreserve; Wiederkehr-Zeitlimit nur Langtest %s; Failsafe-Mindestdauer %s; Freigabe %s; Gültigkeit aktiver Grenzen %s.%s", c.ProfileLabel(), c.StepTimeout, c.EffectTimeout, c.FailsafeTimeout, c.RecoveryTimeout, c.FailsafeMinimum, c.CleanupTimeout, c.LimitLease(), raised)}
	add := func(name, detail string, status string, err error, w *float64) bool {
		reasonCode := ""
		if err != nil {
			if errors.Is(err, ErrControlUnresponsive) {
				r.ControlUnresponsive = true
			}
			cause := err
			if errors.Is(err, context.Canceled) && context.Cause(ctx) != nil {
				cause = context.Cause(ctx)
			}
			reasonCode = AbortReason(cause)
			if (reasonCode == "user_cancel" || reasonCode == "connection_lost") && StepCategory(name) == Checkpoint {
				status = NotChecked
			}
			detail = UserError(cause)
			if reason := AbortReason(cause); reason != "" && (r.AbortReason == "" || r.AbortReason == "insufficient_load") {
				r.AbortReason = reason
			}
			r.Passed = false
		}
		if status == NotChecked && !r.optionalStep(name) {
			r.Passed = false
		}
		if w == nil {
			for i := len(r.Samples) - 1; i >= 0; i-- {
				if r.Samples[i].Step == name {
					w = r.Samples[i].Watts
					break
				}
			}
		}
		if strings.Contains(detail, "zu wenig Leistung") {
			if r.AbortReason == "" {
				r.AbortReason = "insufficient_load"
			}
		}
		ok := err == nil && status != NotChecked
		r.Steps = append(r.Steps, Step{ReasonCode: reasonCode, Name: name, Detail: detail, Status: status, Passed: ok, At: time.Now(), Watts: w})
		r.Steps[len(r.Steps)-1] = r.classify(r.Steps[len(r.Steps)-1])
		stepFinished(b, r.Steps[len(r.Steps)-1])
		return err == nil
	}
	measurements := newMeasurements(&r, b)
	step := func(name string, timeout time.Duration, fn func(context.Context) (string, error)) bool {
		startStep(b, name, timeout)
		measurements.current = name
		sctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := sctx.Err(); err != nil {
			return add(name, "", "", err, nil)
		}
		v, err := fn(sctx)
		status := ""
		if err == nil && strings.Contains(name, "Ausgangsmessung MPC") {
			for i := len(r.Samples) - 1; i >= 0; i-- {
				sample := r.Samples[i]
				if sample.Step != name || sample.Watts == nil {
					continue
				}
				value, sign := c.LPC, 1.0
				if strings.HasPrefix(name, "LPP ") {
					value, sign = c.LPP, -1
				}
				if sign**sample.Watts <= value+c.Tolerance {
					status = NotChecked
					v += "; nicht geprüft (zu wenig Leistung)"
				}
				break
			}
		}
		return add(name, v, status, err, nil)
	}
	action := func(fn func(context.Context) error) func(context.Context) (string, error) {
		return func(ctx context.Context) (string, error) { return "Bestätigt", fn(ctx) }
	}
	skip := func(name, why string) { add(name, why, NotChecked, nil, nil) }
	var caps Capabilities
	touched := map[string]bool{}
	// UX-S21: limit timeline from acknowledged writes and heartbeat changes; a real
	// device leaves failsafe only with the next acknowledged write.
	limits := newLimitTimeline(&r, c.Failsafe, func(k string) bool { return touched[k] }, false)
	writeLimit := func(ctx context.Context, k string, v float64, on bool) (string, error) {
		ack, err := b.Limit(ctx, k, v, on)
		if err == nil {
			limits.Limit(b, k, v, on)
		}
		return ack, err
	}
	// Restart only; the stop is recorded with its observation deadline below.
	heartbeat := func(ctx context.Context, on bool) error {
		err := b.Heartbeat(ctx, on)
		if err == nil && on {
			limits.HeartbeatStarted(b)
		}
		return err
	}
	originals := map[string]FailsafeValues{}
	configured := false
	// Finish always releases every supported limit, including writes with lost ACKs.
	flow := func() {
		if !step("Verbindung", c.StepTimeout, action(b.Connect)) {
			return
		}
		r.Device = b.DeviceInfo()
		r.Device.SKI = d.SKI
		if r.Device.Name == "" {
			r.Device.Name = d.Name
		}
		if !step("Unterstützte Use Cases", c.StepTimeout, func(ctx context.Context) (string, error) {
			var err error
			caps, err = b.Capabilities(ctx)
			r.Capabilities = &caps
			return fmt.Sprintf("LPC: %t; LPP: %t; MPC: %t", caps.LPC, caps.LPP, caps.MPC), err
		}) {
			return
		}
		for k, yes := range map[string]bool{"LPC": caps.LPC, "LPP": caps.LPP} {
			if yes && c.Includes(k) {
				touched[k] = true
			}
		}
		if len(touched) == 0 {
			if caps.MPC {
				step("Ausgangsmessung MPC", c.StepTimeout, func(ctx context.Context) (string, error) {
					v, err := measurements.power(ctx)
					if math.IsNaN(v) || math.IsInf(v, 0) {
						err = errors.New("ungültiger Leistungswert")
					}
					return fmt.Sprintf("MPC %.1f W", v), err
				})
			} else {
				skip("Ausgangsmessung MPC", "MPC nicht vom Gerät gemeldet")
			}
			for _, k := range []string{"LPC", "LPP"} {
				for _, n := range []string{" Grenze + Quittung", " gemessene Wirkung", " Heartbeat-Ausfall", " Failsafe gemessene Wirkung", " Wiederkehr"} {
					skip(k+n, k+" nicht vom Gerät gemeldet")
				}
			}
			skip("Failsafe-Werte setzen", "LPC/LPP nicht vom Gerät gemeldet")
			return
		}
		if c.PauseLowLoad && caps.MPC {
			for {
				var baseline float64
				if !step("Last vor Start prüfen", c.StepTimeout, func(ctx context.Context) (string, error) {
					v, e := measurements.power(ctx)
					baseline = v
					if math.IsNaN(v) || math.IsInf(v, 0) {
						e = errors.New("ungültiger Leistungswert")
					}
					return fmt.Sprintf("MPC %.1f W; vor allen Steuerbefehlen", v), e
				}) {
					return
				}
				var low []LowLoad
				for _, k := range []string{"LPC", "LPP"} {
					if !touched[k] {
						continue
					}
					value, sign := c.LPC, 1.0
					if k == "LPP" {
						value, sign = c.LPP, -1
					}
					if sign*baseline <= value+c.Tolerance {
						low = append(low, LowLoad{Direction: k, Watts: baseline, Required: value + c.Tolerance})
					}
				}
				if len(low) == 0 {
					break
				}
				observer, ok := b.(interface {
					LowLoad(context.Context, LowLoad) (bool, error)
				})
				if !ok {
					break
				}
				prompt := low[0]
				prompt.Directions = low
				retry, e := observer.LowLoad(ctx, prompt)
				if e != nil {
					name := "Last vor Start prüfen"
					if errors.Is(e, ErrNoLoadDecision) {
						name = "Lastentscheidung vor Start" // R043-04: the decision expired, not a measurement
					}
					add(name, "", "", e, nil)
					return
				}
				if !retry {
					break
				}
				measurements.last = time.Time{}
			}
		}
		// Capture every supported direction before the first configuration write. The
		// minimum can be shared by LPC/LPP on one entity.
		readable := true
		for _, k := range []string{"LPC", "LPP"} {
			if !touched[k] {
				continue
			}
			ok := step(k+" Failsafe Ursprungswerte", c.StepTimeout, func(ctx context.Context) (string, error) {
				v, err := b.ReadFailsafe(ctx, k)
				if err != nil {
					return "", fmt.Errorf("Notbetriebs-Werte am Gerät prüfen: %s Ursprungswerte nicht lesbar: %w", k, err)
				}
				if math.IsNaN(v.Power) || math.IsInf(v.Power, 0) || v.Power < 0 || v.Minimum <= 0 {
					return "", fmt.Errorf("Notbetriebs-Werte am Gerät prüfen: %s Ursprungswerte ungültig", k)
				}
				originals[k] = v
				r.FailsafeOriginals = originals
				return fmt.Sprintf("Ursprung %g W; Mindestdauer %s; Quelle: %s", v.Power, v.Minimum, v.Source), nil
			})
			readable = readable && ok
		}
		if !readable {
			skip("Failsafe-Werte setzen", "Keine Änderung ohne lesbare Ursprungswerte. Notbetriebs-Werte am Gerät prüfen")
			return
		}
		if !step("Heartbeat starten", c.StepTimeout, action(func(ctx context.Context) error { return heartbeat(ctx, true) })) {
			return
		}
		configured = true // A lost acknowledgement can still mean a successful write.
		if !step("Failsafe-Werte setzen", c.StepTimeout, action(func(ctx context.Context) error { return b.Configure(ctx, c) })) {
			return
		}
		for _, k := range []string{"LPC", "LPP"} {
			if !c.Includes(k) {
				continue
			}
			supported := caps.LPC
			if k == "LPP" {
				supported = caps.LPP
			}
			names := []string{k + " Grenze + Quittung", k + " gemessene Wirkung", k + " Heartbeat-Ausfall", k + " Failsafe gemessene Wirkung", k + " Wiederkehr"}
			if !supported {
				for _, n := range names {
					skip(n, k+" nicht vom Gerät gemeldet")
				}
				continue
			}
			value := c.LPC
			sign := 1.0
			if k == "LPP" {
				value = c.LPP
				sign = -1
			}
			var baseline float64
			measured := false
			// 046: a return from failsafe can only be evaluated after its entry was shown.
			failsafeShown := false
			baselineName := "Ausgangsmessung MPC"
			if k == "LPP" {
				baselineName = "LPP Ausgangsmessung MPC"
			}
			if caps.MPC {
				for {
					if !step(baselineName, c.StepTimeout, func(ctx context.Context) (string, error) {
						v, err := measurements.power(ctx)
						baseline = v
						measured = err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
						if err == nil && !measured {
							err = errors.New("ungültiger Leistungswert")
						}
						return fmt.Sprintf("MPC %.1f W", v), err
					}) {
						return
					}
					break // Lastentscheidung erfolgte gemeinsam vor dem ersten Schreibzugriff.
				}
			} else {
				skip(baselineName, "MPC nicht vom Gerät gemeldet")
			}
			if !step(names[0], c.StepTimeout, func(ctx context.Context) (string, error) { return writeLimit(ctx, k, value, true) }) {
				return
			}
			adequate := measured && sign*baseline > value+c.Tolerance
			effect := func(name string, target float64, timeout time.Duration, recovery bool) bool {
				if !caps.MPC {
					skip(name, "MPC nicht vom Gerät gemeldet")
					return true
				}
				if !adequate {
					skip(name, "Wirkung nicht nachweisbar (zu wenig Leistung)")
					return true
				}
				if recovery && !failsafeShown {
					skip(name, ReturnNotEvaluable)
					return true
				}
				var last *float64
				prior := measurements.validDevice // 046: only readings measured after this count
				ok := step(name, timeout, func(ctx context.Context) (string, error) {
					for {
						if measurements.validDevice.After(prior) {
							prior = measurements.validDevice // R046-23: newest known device time before this read
						}
						v, err := measurements.power(ctx)
						if err == nil {
							err = ctx.Err()
						}
						if err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
							owned := v
							last = &owned
						}
						if err != nil {
							return "", err
						}
						fresh, known := measurements.fresh(prior)
						if fresh && !math.IsNaN(v) && !math.IsInf(v, 0) && sign*v <= target+c.Tolerance && sign*v >= -c.Tolerance && (!recovery || sign*v > c.Failsafe+c.Tolerance) {
							measurements.finish(v)
							detail := fmt.Sprintf("MPC %.1f W; Obergrenze %.0f W; Toleranz %.1f W", v, target, c.Tolerance)
							if recovery && !known {
								detail += "; Gerät sendet keinen absoluten Messzeitpunkt, Aktualität nur über neue Antwort"
							}
							return detail, nil
						}
						select {
						case <-ctx.Done():
							return "", fmt.Errorf("Messwirkung nicht nachgewiesen, letzter Wert %.1f W: %w", v, ctx.Err())
						case <-time.After(250 * time.Millisecond):
						}
					}
				})
				if len(r.Steps) > 0 {
					r.Steps[len(r.Steps)-1].Watts = last
				}
				if ok && recovery {
					limits.ReturnMeasured(b, k)
				}
				return ok
			}
			if !effect(names[1], value, c.EffectTimeout, false) {
				return
			}
			// Even without electrical evidence, exercise the complete outage protocol.
			{
				var observation HeartbeatObservation
				if !step(names[2], c.StepTimeout, func(ctx context.Context) (string, error) {
					if err := heartbeat(ctx, false); err != nil {
						return "", err
					}
					stopped := time.Now() // send gate and timer have stopped
					receipt, _ := b.LastHeartbeatReceipt()
					observation = NewHeartbeatObservation(k, stopped, receipt, c.FailsafeTimeout)
					limits.HeartbeatStopped(observation.Reference.Add(c.FailsafeTimeout))
					r.Heartbeats = append(r.Heartbeats, observation)
					detail := fmt.Sprintf("Bezugszeitpunkt: %s; %s; Ausfallfrist %s + Reserve %s, Beobachtung bis %s", observation.Reference.Format(time.RFC3339Nano), observation.Basis, c.FailsafeTimeout, HeartbeatReserve, observation.Deadline.Format(time.RFC3339Nano))
					if observation.Receipt != nil {
						detail += "; letzter Heartbeat am Gerät spätestens " + observation.Receipt.Format(time.RFC3339Nano) + " (positive Quittung lokal empfangen; nur Information)"
					} else {
						detail += "; keine positive Heartbeat-Quittung (fehlend oder negativ; nur Information)"
					}
					return detail, nil
				}) {
					return
				}

				if !adequate {
					if !step(k+" Ausfallfrist abwarten", time.Until(observation.Deadline)+c.StepTimeout, func(ctx context.Context) (string, error) {
						if observer, ok := b.(interface{ StepWaiting(string, time.Time) }); ok {
							observer.StepWaiting(k+" Ausfallfrist abwarten", observation.Deadline)
						}
						// No MPC polling: absent/low readings cannot establish a physical effect.
						if err := waitProtocolDeadline(ctx, observation.Deadline); err != nil {
							return "", err
						}
						return "Ausfallfrist einschließlich Beobachtungsreserve abgewartet; nur Protokolltest", nil
					}) {
						return
					}
				}
				if !caps.MPC || !adequate {
					effect(names[3], c.Failsafe, 0, false) // records why it is not checked
				} else {
					index := len(r.Heartbeats) - 1
					receipt := func() time.Time { t, _ := b.LastHeartbeatReceipt(); return t }
					// A positive receipt after the reference proves a later reception at the device.
					shift := func(latest time.Time) (time.Time, time.Time) {
						h := NewHeartbeatObservation(k, observation.Stopped, latest, c.FailsafeTimeout)
						h.Basis += "; Quittung nach dem Stopp empfangen, Frist ab Quittung"
						observation = h
						r.Heartbeats[index] = h
						limits.HeartbeatStopped(h.Reference.Add(c.FailsafeTimeout))
						return h.Reference.Add(c.FailsafeTimeout), h.Deadline
					}
					shown, ok := measureFailsafe(ctx, failsafeMeasurement{name: names[3], direction: k, sign: sign, config: c,
						due: observation.Reference.Add(c.FailsafeTimeout), deadline: observation.Deadline, reference: observation.Reference, receipt: receipt, shift: shift}, b, &r, measurements, add)
					if shown {
						failsafeShown = true
						limits.FailsafeMeasured(b, k)
					}
					if !ok {
						return
					}
				}
				if !step(k+" Heartbeat wieder starten", c.StepTimeout, action(func(ctx context.Context) error { return heartbeat(ctx, true) })) {
					return
				}
				if !adequate {
					if !step(k+" Wiederkehr Protokoll", c.StepTimeout, func(ctx context.Context) (string, error) {
						if err := b.Recover(ctx); err != nil {
							return "", err
						}
						ack, err := writeLimit(ctx, k, value, true)
						if err != nil {
							return ack, err
						}
						// 046 (R046-02): protocol evidence only, no electrical return.
						return "Protokollnachweis (Quittung), keine Wirkungsmessung: " + ack, b.Recover(ctx)
					}) {
						return
					}
					why := "Wirkung nicht nachweisbar (zu wenig Leistung); Mindestdauer nicht geprüft"
					if !caps.MPC {
						why = "MPC nicht vom Gerät gemeldet; Mindestdauer nicht geprüft"
					}
					skip(names[4], why)
					effect(k+" Wiederkehr gemessene Wirkung", value, c.EffectTimeout, true)
					if !c.FullTest() {
						if !step(k+" Vorzeitige Freigabe", c.StepTimeout, func(ctx context.Context) (string, error) { return writeLimit(ctx, k, 0, false) }) {
							return
						}
						effect(k+" Vorzeitige Rückkehr gemessene Wirkung", value, c.EffectTimeout, true)
					}
				} else if !c.FullTest() {
					skip(names[4], "nicht geprüft (Langtest): Wiederkehr nach Mindestdauer nicht abgewartet")
					skip(k+" Wiederkehr gemessene Wirkung", "nicht geprüft (Langtest)")
					// A fresh heartbeat followed by deactivation permits a return to
					// controlled operation. An ACK alone does not prove the transition.
					if !step(k+" Vorzeitige Freigabe", c.StepTimeout, func(ctx context.Context) (string, error) { return writeLimit(ctx, k, 0, false) }) {
						return
					}
					if !failsafeShown {
						skip(k+" Vorzeitige Rückkehr gemessene Wirkung", ReturnNotEvaluable)
					} else {
						measurements.current = k + " Vorzeitige Rückkehr gemessene Wirkung"
						startStep(b, measurements.current, c.EffectTimeout)
						mctx, cancel := context.WithTimeout(ctx, c.EffectTimeout)
						var last float64
						var readErr error
						recovered, timedReturn := false, false
						prior := measurements.validDevice // 046 (R046-10): a repeated pre-release value is no return
						for mctx.Err() == nil {
							if measurements.validDevice.After(prior) {
								prior = measurements.validDevice // R046-23
							}
							v, err := measurements.power(mctx)
							last = v
							if err == nil {
								err = mctx.Err() // a late answer after the deadline or cancel does not count
							}
							if err != nil {
								readErr = err
								break
							}
							if fresh, known := measurements.fresh(prior); fresh && !math.IsNaN(v) && !math.IsInf(v, 0) && sign*v > c.Failsafe+c.Tolerance {
								recovered = true
								timedReturn = known
								break
							}
							select {
							case <-mctx.Done():
							case <-time.After(250 * time.Millisecond):
							}
						}
						cancel()
						if !recovered && (ctx.Err() != nil || readErr != nil && !errors.Is(readErr, context.DeadlineExceeded)) {
							// 046 (R046-04): cancel or connection loss keep their cause.
							cause := readErr
							if ctx.Err() != nil {
								cause = ctx.Err()
							}
							add(k+" Vorzeitige Rückkehr gemessene Wirkung", "", "", cause, nil)
							return
						} else if recovered {
							limits.ReturnMeasured(b, k)
							measurements.finish(last)
							detail := fmt.Sprintf("Neuer Heartbeat + Aufheben: Rückkehr vor Mindestdauer beobachtet; MPC %.1f W", last)
							if !timedReturn {
								detail += "; Gerät sendet keinen absoluten Messzeitpunkt, Aktualität nur über neue Antwort nach Freigabe"
							}
							add(k+" Vorzeitige Rückkehr gemessene Wirkung", detail, "", nil, &last)
						} else {
							skip(k+" Vorzeitige Rückkehr gemessene Wirkung", fmt.Sprintf("Vorzeitige Rückkehr nicht nachgewiesen; Notbetrieb kann bis %s bleiben. Notbetriebs-Werte am Gerät prüfen", c.FailsafeMinimum))
						}
					}
				} else if !failsafeShown {
					// 046 (R046-02): no minimum-duration evidence without a shown failsafe.
					skip(names[4], ReturnNotEvaluable)
					skip(k+" Wiederkehr gemessene Wirkung", ReturnNotEvaluable)
				} else {
					// Wait the configured minimum before claiming return to normal operation.
					if !step(names[4], c.RecoveryTimeout, func(ctx context.Context) (string, error) {
						if err := measurements.wait(ctx, c.FailsafeMinimum, c.StepTimeout); err != nil {
							return "", err
						}
						_, err := writeLimit(ctx, k, value, true)
						if err != nil {
							return "", err
						}
						return "Mindestdauer abgewartet; Grenze erneut quittiert", nil
					}) {
						return
					}
					if !effect(k+" Wiederkehr gemessene Wirkung", value, c.EffectTimeout, true) {
						return
					}
				}
			}
			if !step(k+" Grenze freigeben", c.StepTimeout, func(ctx context.Context) (string, error) { return writeLimit(ctx, k, 0, false) }) {
				return
			}
		}
	}
	flow()
	if len(touched) > 0 && configured {
		var errs []error
		// Each release gets its own deadline, independent of the canceled test context.
		for _, k := range []string{"LPC", "LPP"} {
			if !touched[k] {
				continue
			}
			startStep(b, k+" Grenzen aufheben", c.CleanupTimeout)
			cc, cancel := context.WithTimeout(context.Background(), c.CleanupTimeout)
			if err := controlReady(cc, b, r.ControlUnresponsive); err != nil {
				errs = append(errs, fmt.Errorf("%s Wiederverbindung: %w", k, err))
			} else {
				if err := heartbeat(cc, true); err != nil {
					errs = append(errs, err)
				}
				if _, err := writeLimit(cc, k, 0, false); err != nil {
					if errors.Is(err, ErrControlUnresponsive) {
						r.ControlUnresponsive = true
					}
					errs = append(errs, fmt.Errorf("%s aufheben: %w", k, err))
				}
			}
			cancel()
		}
		if configured {
			for _, k := range []string{"LPC", "LPP"} {
				original, ok := originals[k]
				if !ok {
					continue
				}
				for _, field := range []string{"power", "minimum"} {
					name := k + " Failsafe-Leistung zurückstellen"
					if field == "minimum" {
						name = k + " Failsafe-Mindestdauer zurückstellen"
					}
					startStep(b, name, c.CleanupTimeout)
					cc, cancel := context.WithTimeout(context.Background(), c.CleanupTimeout)
					err := controlReady(cc, b, r.ControlUnresponsive)
					if err == nil {
						err = b.WriteFailsafe(cc, k, field, original)
					}
					cancel()
					if errors.Is(err, ErrControlUnresponsive) {
						r.ControlUnresponsive = true
					}
					if err != nil {
						err = fmt.Errorf("Nicht bestätigt. Notbetriebs-Werte am Gerät prüfen: %w", err)
					}
					detail := fmt.Sprintf("Ursprüngliche Notbetriebs-Leistung %g W geschrieben und durch Geräteantwort bestätigt", original.Power)
					if field == "minimum" {
						detail = fmt.Sprintf("Ursprüngliche Notbetriebs-Mindestdauer %s geschrieben und durch Geräteantwort bestätigt", original.Minimum)
					}
					add(name, detail, "", err, nil)
				}
			}
		}
		cleanupErr := errors.Join(errs...)
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("Freigabe nicht bestätigt; Grenzen können am Gerät weiter gelten. Verbindung wiederherstellen und am Gerät freigeben: %w", cleanupErr)
		}
		add("Grenzen aufheben", "Alle unterstützten Grenzen per SPINE-Quittung aufgehoben", "", cleanupErr, nil)
		if caps.MPC && !r.ControlUnresponsive {
			measurements.current = "Grenzen aufheben"
			cc, cancel := context.WithTimeout(context.Background(), c.CleanupTimeout)
			_, _ = measurements.power(cc)
			cancel()
		}
	}
	r.Finished = time.Now()
	limits.Finish(b, r.Finished)
	r.finishEvaluation()
	return r, nil
}

// Scope is explicit in new reports. Legacy reports retain their original coverage.
func (r Report) optionalStep(name string) bool {
	return r.Mode == ModeOnSite && r.Config.Scope != "" && !r.Config.FullTest() && (name == "LPC Wiederkehr" || name == "LPP Wiederkehr" || name == "LPC Wiederkehr gemessene Wirkung" || name == "LPP Wiederkehr gemessene Wirkung")
}
func (r Report) OutcomeLabel() string { return ResultTitle(r.Config, r.Outcome(), r.AbortReason) }

// Never shorten the observation for a missing or negative optional receipt.
func waitProtocolDeadline(ctx context.Context, deadline time.Time) error {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func controlReady(ctx context.Context, b DeviceBackend, dead bool) error {
	if dead {
		return ErrControlUnresponsive
	}
	return b.Recover(ctx)
}
