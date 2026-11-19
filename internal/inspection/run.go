// Package inspection executes the reproducible simulator acceptance profile.
package inspection

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const Notice = "Dokumentierter Funktionstest. Ersetzt keine Konformitätsprüfung."

// LegacyNotices are earlier wordings stored in older reports; they mean the
// same and are replaced by Notice on output (PO decision 11.11.2026, ADR 012).
var LegacyNotices = []string{"Keine Zertifizierung. Dieses Protokoll ersetzt keine Konformitätsprüfung."}

const Profile = "Loopback-Simulator v0.1; Heartbeat 1 s, Ausfall 2,5 s, verkürzte Wiederkehr; Failsafe-Mindestdauer konfiguriert: 2 h. Keine physikalische Messung."

type Device struct {
	Type     string `json:"type,omitempty"`
	Name     string `json:"name"`
	SKI      string `json:"ski"`
	Brand    string `json:"brand"`
	Model    string `json:"model"`
	Serial   string `json:"serial,omitempty"`
	Firmware string `json:"firmware,omitempty"`
	Source   string `json:"source,omitempty"`
}

const ScopeConsumption = "consumption"
const ScopeBoth = "both"
const ScopeProduction = "production"

// Includes keeps direction selection consistent for validation, writes and reporting.
func (c Config) Includes(kind string) bool {
	return !(kind == "LPP" && c.Scope == ScopeConsumption || kind == "LPC" && c.Scope == ScopeProduction)
}

// Site contains optional locally stored assignment data, never diagnostic log data.
type Site struct {
	Customer        string `json:"customer"`
	CustomerAddress string `json:"customerAddress,omitempty"`
	Address         string `json:"address"`
	Meter           string `json:"meter"`
	Note            string `json:"note"`
}

func (s Site) Validate() error {
	for _, v := range []string{s.Customer, s.CustomerAddress, s.Address, s.Meter, s.Note} {
		if len([]rune(v)) > 1000 {
			return errors.New("Kunden-/Anlagenangaben: höchstens 1.000 Zeichen je Feld")
		}
	}
	return nil
}

type Config struct {
	Profile         InspectorProfile `json:"profile,omitempty"`
	Site            Site             `json:"site,omitempty"`
	Scope           string           `json:"scope,omitempty"`
	PauseLowLoad    bool             `json:"pauseLowLoad,omitempty"`
	TestProfile     string           `json:"testProfile"`
	LongTest        bool             `json:"longTest"`
	SignOperator    bool             `json:"signOperator"`
	SignInspector   bool             `json:"signInspector"`
	Business        string           `json:"business"`
	Inspector       string           `json:"inspector"`
	LPC             float64          `json:"lpc"`
	LPP             float64          `json:"lpp"`
	Failsafe        float64          `json:"failsafe"`
	Tolerance       float64          `json:"tolerance"`
	StepTimeout     time.Duration    `json:"stepTimeout"`
	EffectTimeout   time.Duration    `json:"effectTimeout"`
	FailsafeTimeout time.Duration    `json:"failsafeTimeout"`
	// TestHeartbeatTimeout replaces the 120 s protocol minimum of device runs only
	// for test partners (Go tests, --loopback-partner E2E); never via API/JSON (046).
	TestHeartbeatTimeout time.Duration `json:"-"`
	RecoveryTimeout      time.Duration `json:"recoveryTimeout"`
	CleanupTimeout       time.Duration `json:"cleanupTimeout"`
	FailsafeMinimum      time.Duration `json:"failsafeMinimum"`
	// Authorized is retained for compatibility with older JSON/CLI clients.
	Authorized bool `json:"authorized"`
}

func DefaultConfig() Config {
	return Config{LPC: 4200, LPP: 3000, Failsafe: 500, Tolerance: 50, StepTimeout: 6 * time.Second}
}
func (c Config) Validate() error {
	if err := c.Profile.Validate(); err != nil {
		return err
	}
	if err := c.Site.Validate(); err != nil {
		return err
	}
	if c.TestProfile != "" && c.TestProfile != "quick" && c.TestProfile != "full" {
		return errors.New("ungültiges Prüfprofil")
	}
	// 043 (Nutzer 2026-11-14): only the inspector's name is required; the company is optional.
	if strings.TrimSpace(c.Inspector) == "" || len([]rune(c.Business)) > 200 || len([]rune(c.Inspector)) > 200 {
		return errors.New("Prüfer/in ist erforderlich; Firma und Prüfer/in höchstens 200 Zeichen")
	}
	for _, v := range []float64{c.LPC, c.LPP, c.Failsafe, c.Tolerance} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1000000 || math.Abs(v*1000-math.Round(v*1000)) > 0.000001 {
			return errors.New("Leistungswerte: 0 bis 1.000.000 W, höchstens drei Nachkommastellen")
		}
	}
	if c.Scope != "" && c.Scope != ScopeConsumption && c.Scope != ScopeBoth && c.Scope != ScopeProduction {
		return errors.New("ungültiger Prüfumfang")
	}
	limit := c.LPC
	if c.Scope == ScopeProduction {
		limit = c.LPP
	} else if c.Scope != ScopeConsumption {
		limit = math.Min(limit, c.LPP)
	}
	if (c.Includes("LPC") && c.LPC <= 0) || (c.Includes("LPP") && c.LPP <= 0) || c.Tolerance*2 >= limit-c.Failsafe {
		return errors.New("Grenzen müssen mit ausreichendem Abstand über Failsafe liegen")
	}
	if c.StepTimeout <= 0 || c.StepTimeout > 10*time.Minute {
		return errors.New("ungültiges Zeitlimit")
	}
	return nil
}

type Step struct {
	Category   string    `json:"category,omitempty"`
	Required   *bool     `json:"required,omitempty"`
	ReasonCode string    `json:"reason,omitempty"`
	Status     string    `json:"status,omitempty"`
	Name       string    `json:"name"`
	Passed     bool      `json:"passed"`
	Detail     string    `json:"detail"`
	At         time.Time `json:"at"`
	Watts      *float64  `json:"watts,omitempty"`
}
type Report struct {
	Revision            *ReportRevision           `json:"revision,omitempty"`
	LegacyProtocolID    string                    `json:"protocolId,omitempty"`
	SchemaVersion       int                       `json:"schemaVersion,omitempty"`
	Software            string                    `json:"software,omitempty"`
	AbortReason         string                    `json:"abortReason,omitempty"`
	AbortText           string                    `json:"abortText,omitempty"`
	ID                  string                    `json:"id,omitempty"`
	ControlUnresponsive bool                      `json:"controlUnresponsive,omitempty"`
	Events              []Step                    `json:"events,omitempty"`
	Heartbeats          []HeartbeatObservation    `json:"heartbeats,omitempty"`
	Capabilities        *Capabilities             `json:"capabilities,omitempty"`
	Samples             []Sample                  `json:"samples,omitempty"`
	Limits              []LimitChange             `json:"limits,omitempty"`
	FailsafeOriginals   map[string]FailsafeValues `json:"failsafeOriginals,omitempty"`
	Mode                string                    `json:"mode"`
	LocalSKI            string                    `json:"localSki,omitempty"`
	Business            string                    `json:"business"`
	Inspector           string                    `json:"inspector"`
	Started             time.Time                 `json:"started"`
	Finished            time.Time                 `json:"finished"`
	Device              Device                    `json:"device"`
	Config              Config                    `json:"config"`
	Profile             string                    `json:"profile"`
	Notice              string                    `json:"notice"`
	Steps               []Step                    `json:"steps"`
	Passed              bool                      `json:"passed"`
}
type Backend interface {
	Connect(context.Context) error
	Supports(context.Context) error
	Configure(context.Context, Config) error
	Limit(context.Context, string, float64, bool) (string, error)
	Power(context.Context) (float64, error)
	Heartbeat(context.Context, bool) error
}

func Run(ctx context.Context, c Config, d Device, b Backend) (Report, error) {
	if err := c.Validate(); err != nil {
		return Report{}, err
	}
	c.NormalizeProfile()
	id, err := NewReportID()
	if err != nil {
		return Report{}, err
	}
	r := Report{ID: id, SchemaVersion: 1, Business: c.Business, Inspector: c.Inspector, Started: time.Now(), Device: d, Config: c, Mode: ModeSimulator, Profile: Profile, Notice: Notice, Passed: true}
	measurements := newMeasurements(&r, b)
	limits := newLimitTimeline(&r, c.Failsafe, c.Includes, true)
	// The simulator device enters failsafe after its own heartbeat timeout.
	failsafeDelay := c.FailsafeTimeout
	if d, ok := b.(interface{ FailsafeDelay() time.Duration }); ok {
		failsafeDelay = d.FailsafeDelay()
	}
	heartbeat := func(ctx context.Context, on bool) error {
		err := b.Heartbeat(ctx, on)
		if err != nil {
			return err
		}
		if on {
			limits.HeartbeatStarted(b)
			return nil
		}
		stopped := time.Now()
		var receipt time.Time
		if r, ok := b.(interface{ LastHeartbeatReceipt() (time.Time, string) }); ok {
			receipt, _ = r.LastHeartbeatReceipt()
		}
		limits.HeartbeatStopped(failsafeDeadline(stopped, receipt, failsafeDelay))
		return nil
	}
	release := func(ctx context.Context, kind string) error {
		_, err := b.Limit(ctx, kind, 0, false)
		if err == nil {
			limits.Limit(b, kind, 0, false)
		}
		return err
	}
	step := func(name string, fn func(context.Context) (string, *float64, error)) bool {
		startStep(b, name, c.StepTimeout)
		measurements.current = name
		sctx, cancel := context.WithTimeout(ctx, c.StepTimeout)
		defer cancel()
		detail, watts, err := fn(sctx)
		if err != nil {
			r.Passed = false
			detail = UserError(err)
			if r.AbortReason == "" {
				r.AbortReason = AbortReason(err)
			}
		}
		r.Steps = append(r.Steps, Step{Name: name, Passed: err == nil, Detail: detail, At: time.Now(), Watts: watts})
		r.Steps[len(r.Steps)-1] = r.classify(r.Steps[len(r.Steps)-1])
		stepFinished(b, r.Steps[len(r.Steps)-1])
		return err == nil
	}
	action := func(fn func(context.Context) error) func(context.Context) (string, *float64, error) {
		return func(ctx context.Context) (string, *float64, error) { return "Bestätigt", nil, fn(ctx) }
	}
	limit := func(kind string, value float64) func(context.Context) (string, *float64, error) {
		return func(ctx context.Context) (string, *float64, error) {
			ack, err := b.Limit(ctx, kind, value, true)
			if err == nil {
				limits.Limit(b, kind, value, true)
			}
			return fmt.Sprintf("%s; Soll %.0f W", ack, value), nil, err
		}
	}
	power := func(predicate func(float64) bool, expect string) func(context.Context) (string, *float64, error) {
		return func(ctx context.Context) (string, *float64, error) {
			var value float64
			var lastErr error
			for {
				v, err := measurements.power(ctx)
				value = v
				lastErr = err
				if err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) && predicate(v) {
					measurements.finish(v)
					return fmt.Sprintf("MPC %.1f W; %s; Toleranz %.1f W", v, expect, c.Tolerance), &value, nil
				}
				select {
				case <-ctx.Done():
					return "", nil, fmt.Errorf("Messwirkung nicht nachgewiesen (%s), letzter Wert %.1f W, Lesefehler %v: %w", expect, value, lastErr, ctx.Err())
				case <-time.After(25 * time.Millisecond):
				}
			}
		}
	}
	// Cleanup has a fresh deadline so cancellation or a failed write cannot skip release.
	flow := func() {
		if !step("Verbindung", action(b.Connect)) {
			return
		}
		if !step("Unterstützte Use Cases", action(b.Supports)) {
			return
		}
		if !step("Failsafe-Werte setzen", action(func(ctx context.Context) error { return b.Configure(ctx, c) })) {
			return
		}
		if !step("Heartbeat starten", action(func(ctx context.Context) error { return heartbeat(ctx, true) })) {
			return
		}
		if c.Includes("LPC") {
			if !step("Ausgangsmessung MPC", power(func(v float64) bool { return v > c.LPC+c.Tolerance }, "ausreichender Bezug für LPC")) {
				return
			}
			if !step("LPC Grenze + Quittung", limit("LPC", c.LPC)) {
				return
			}
			if !step("LPC gemessene Wirkung", power(func(v float64) bool { return math.Abs(v-c.LPC) <= c.Tolerance }, "LPC Soll")) {
				return
			}

			if !step("LPC Heartbeat-Ausfall", action(func(ctx context.Context) error { return heartbeat(ctx, false) })) {
				return
			}
			if !step("LPC Failsafe gemessene Wirkung", power(func(v float64) bool { return math.Abs(v-c.Failsafe) <= c.Tolerance }, "Failsafe Soll, positiver Bezug")) {
				return
			}
			limits.FailsafeMeasured(b, "LPC")
			if !step("LPC Wiederkehr", func(ctx context.Context) (string, *float64, error) {
				if err := heartbeat(ctx, true); err != nil {
					return "", nil, err
				}
				return power(func(v float64) bool { return math.Abs(v-c.LPC) <= c.Tolerance }, "LPC nach Wiederkehr")(ctx)
			}) {
				return
			}
		}
		if c.Scope == ScopeConsumption {
			return
		}
		if c.Scope == ScopeProduction && !step("LPP Ausgangsmessung MPC", power(func(v float64) bool { return v < -c.LPP-c.Tolerance }, "ausreichende Einspeisung für LPP")) {
			return
		}
		if !step("LPP Grenze + Quittung", limit("LPP", c.LPP)) {
			return
		}
		if !step("LPP gemessene Wirkung", power(func(v float64) bool { return math.Abs(v+c.LPP) <= c.Tolerance }, "LPP Soll, negative Einspeisung")) {
			return
		}
		if !step("LPP Heartbeat-Ausfall", action(func(ctx context.Context) error { return heartbeat(ctx, false) })) {
			return
		}
		if !step("LPP Failsafe gemessene Wirkung", power(func(v float64) bool { return math.Abs(v+c.Failsafe) <= c.Tolerance }, "Failsafe Soll, negative Einspeisung")) {
			return
		}
		limits.FailsafeMeasured(b, "LPP")
		if !step("LPP Wiederkehr", func(ctx context.Context) (string, *float64, error) {
			if err := heartbeat(ctx, true); err != nil {
				return "", nil, err
			}
			return power(func(v float64) bool { return math.Abs(v+c.LPP) <= c.Tolerance }, "LPP nach Wiederkehr")(ctx)
		}) {
			return
		}
	}
	flow()
	if observer, ok := b.(interface{ StepStarted(string) }); ok {
		observer.StepStarted("Grenzen aufheben")
	}
	measurements.current = "Grenzen aufheben"
	cleanupCtx, cancel := context.WithTimeout(context.Background(), c.StepTimeout)
	defer cancel()
	var errs []error
	if err := heartbeat(cleanupCtx, true); err != nil {
		errs = append(errs, err)
	}
	for _, kind := range []string{"LPC", "LPP"} {
		if !c.Includes(kind) {
			continue
		}
		if err := release(cleanupCtx, kind); err != nil {
			errs = append(errs, fmt.Errorf("%s aufheben: %w", kind, err))
		}
	}
	var watts *float64
	detail := "Beide Grenzen per Quittung aufgehoben"
	if len(errs) == 0 {
		var err error
		if c.Scope == ScopeConsumption {
			detail, watts, err = power(func(v float64) bool { return v > c.LPC+c.Tolerance }, "Bezug nach Freigabe")(cleanupCtx)
		} else {
			detail, watts, err = power(func(v float64) bool { return v < -c.LPP-c.Tolerance }, "Einspeisung nach Freigabe")(cleanupCtx)
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	cleanupErr := errors.Join(errs...)
	if cleanupErr != nil {
		r.Passed = false
		detail = UserError(cleanupErr)
	}
	r.Steps = append(r.Steps, Step{Name: "Grenzen aufheben", Passed: cleanupErr == nil, Detail: detail, At: time.Now(), Watts: watts})
	r.Steps[len(r.Steps)-1] = r.classify(r.Steps[len(r.Steps)-1])
	stepFinished(b, r.Steps[len(r.Steps)-1])
	r.Finished = time.Now()
	limits.Finish(b, r.Finished)
	r.finishEvaluation()
	return r, nil
}

// Progress is a snapshot of actual completed steps, never a timer estimate.
type Progress struct {
	Started  time.Time     `json:"started,omitempty"`
	Deadline time.Time     `json:"deadline,omitempty"`
	Events   []Step        `json:"events,omitempty"`
	Samples  []Sample      `json:"samples,omitempty"`
	Limits   []LimitChange `json:"limits,omitempty"`
	LowLoad  *LowLoad      `json:"lowLoad,omitempty"`
	Current  string        `json:"current"`
	Steps    []Step        `json:"steps"`
}

func stepFinished(b Backend, s Step) {
	if observer, ok := b.(interface{ StepFinished(Step) }); ok {
		observer.StepFinished(s)
	}
}

func startStep(b Backend, name string, timeout time.Duration) {
	if observer, ok := b.(interface{ StepStarted(string) }); ok {
		observer.StepStarted(name)
	}
	if observer, ok := b.(interface{ StepWaiting(string, time.Time) }); ok {
		observer.StepWaiting(name, time.Now().Add(timeout))
	}
}
