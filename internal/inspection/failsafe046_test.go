package inspection

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// 046: Real run 2026-11-18 (simulator -> energy management box -> heat pump): LPC 600 W,
// failsafe 400 W. The heat pump was already at 234 W under the LPC limit. The
// failsafe step passed 7 ms after the heartbeat stop, long before the device's
// 120 s heartbeat timeout; the device never entered failsafe.
//
// failsafe046 is a device with a real clock: after the heartbeat stop it keeps
// the controlled value until stop + timeout and only then (if enters) applies
// the failsafe value. Every power read is recorded with its step and time.
type failsafe046 struct {
	onsiteFake
	mu        sync.Mutex
	timeout   time.Duration
	baseline  float64
	limited   []float64 // values under the active limit, the last one repeats
	failsafe  float64
	free      float64
	enters    bool
	dip       float64 // value read between heartbeat stop and timeout, 0 = none
	state     *bool
	limit     bool
	hbOff     bool
	inFS      bool
	stopped   time.Time
	current   string
	reads     []read046
	baselined bool
	// block delays dip reads without honouring ctx (a read across the timeout).
	block time.Duration
	// lateReceipt: from the second query on, a positive receipt this long after the stop.
	lateReceipt  time.Duration
	receiptCalls int
	receiptFrom  int // first query that sees the late receipt (default 2)
	// lost: reads after the release fail with a connection loss.
	lost     bool
	released bool
	// cancelAfter cancels the run this long after the heartbeat stop.
	cancelAfter time.Duration
	cancel      context.CancelFunc
	// freeze: after the stop the device stops measuring and repeats its first
	// post-stop value with that measurement time (R046-10).
	freeze   bool
	frozen   *read046
	measured time.Time
	// backwards: after the timeout one fresh high reading, then low readings
	// with an older measurement time (R046-23).
	backwards bool
	sawNew    bool
}

func (f *failsafe046) PowerMeasuredAt() (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.measured, !f.measured.IsZero()
}

type read046 struct {
	step string
	at   time.Time
	w    float64
}

func (f *failsafe046) StepStarted(name string) { f.mu.Lock(); f.current = name; f.mu.Unlock() }
func (f *failsafe046) Heartbeat(ctx context.Context, on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hbOff = !on
	if !on {
		f.stopped = time.Now()
		if f.cancelAfter > 0 {
			time.AfterFunc(f.cancelAfter, f.cancel)
		}
	}
	return ctx.Err()
}
func (f *failsafe046) LastHeartbeatReceipt() (time.Time, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.receiptCalls++
	if f.lateReceipt > 0 && f.receiptCalls >= max(2, f.receiptFrom) {
		return f.stopped.Add(f.lateReceipt), "Test: späte Quittung"
	}
	return f.stopped, "Test: Empfang am Gerät"
}
func (f *failsafe046) Limit(ctx context.Context, k string, v float64, on bool) (string, error) {
	f.mu.Lock()
	f.update(time.Now())
	f.limit = on
	if !on && !f.hbOff && !f.stopped.IsZero() {
		f.released = true
	}
	if !f.hbOff {
		f.inFS = false // heartbeat + new write ends failsafe (LPC/LPP 1.0.0)
	}
	f.mu.Unlock()
	return f.onsiteFake.Limit(ctx, k, v, on)
}
func (f *failsafe046) update(now time.Time) {
	if f.hbOff && f.enters && !now.Before(f.stopped.Add(f.timeout)) {
		f.inFS = true
	}
}
func (f *failsafe046) Power(ctx context.Context) (float64, error) {
	f.mu.Lock()
	if f.block > 0 && f.hbOff && f.dip != 0 && time.Now().Before(f.stopped.Add(f.timeout)) {
		f.mu.Unlock()
		time.Sleep(f.block) // ignores ctx: the answer arrives after the timeout
		f.mu.Lock()
		f.reads = append(f.reads, read046{step: f.current, at: time.Now(), w: f.dip})
		f.mu.Unlock()
		return f.dip, nil
	}
	defer f.mu.Unlock()
	now := time.Now()
	f.update(now)
	if f.lost && f.released {
		f.reads = append(f.reads, read046{step: f.current, at: now})
		return 0, ErrConnectionLost
	}
	f.measured = now
	if f.backwards && f.hbOff && !now.Before(f.stopped.Add(f.timeout)) {
		v := f.failsafe
		if !f.sawNew {
			f.sawNew, v = true, f.limited[0]
		} else {
			f.measured = f.stopped.Add(time.Millisecond)
		}
		f.reads = append(f.reads, read046{step: f.current, at: now, w: v})
		return v, ctx.Err()
	}
	if f.freeze && f.hbOff && f.frozen != nil {
		f.measured = f.frozen.at
		f.reads = append(f.reads, read046{step: f.current, at: now, w: f.frozen.w})
		return f.frozen.w, ctx.Err()
	}
	var v float64
	switch {
	case !f.baselined:
		f.baselined = true
		v = f.baseline
	case f.inFS:
		v = f.failsafe
	case f.hbOff && f.dip != 0 && now.Before(f.stopped.Add(f.timeout)):
		v = f.dip
	case f.limit:
		v = f.limited[0]
		if len(f.limited) > 1 {
			f.limited = f.limited[1:]
		}
	default:
		v = f.free
	}
	f.reads = append(f.reads, read046{step: f.current, at: now, w: v})
	if f.freeze && f.hbOff && f.frozen == nil {
		f.frozen = &read046{at: now, w: v}
	}
	return v, ctx.Err()
}

// FailsafeState is only offered when the device reports its state.
type reporting046 struct{ *failsafe046 }

func (f reporting046) FailsafeState(context.Context, string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.update(time.Now())
	if f.state != nil {
		return *f.state, nil
	}
	return f.inFS, nil
}

func config046(scope string) Config {
	c := onsiteConfig()
	c.Scope = scope
	c.LPC = 600
	c.LPP = 600
	c.Failsafe = 400
	c.Tolerance = 50
	c.FailsafeTimeout = 300 * time.Millisecond
	c.EffectTimeout = 2 * time.Second
	return c
}

func step046(t *testing.T, r Report, name string) Step {
	t.Helper()
	for _, s := range r.Steps {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("step %q missing: %+v", name, r.Steps)
	return Step{}
}

// The deciding (last) read of the failsafe step is taken after the device's
// heartbeat timeout; reads before it only feed the curve.
func readsAfterTimeout046(t *testing.T, f *failsafe046, name string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	due := f.stopped.Add(f.timeout)
	var last *read046
	for i := range f.reads {
		if f.reads[i].step == name {
			last = &f.reads[i]
		}
	}
	if last == nil {
		t.Fatalf("%s: no read", name)
	}
	if last.at.Before(due) {
		t.Fatalf("%s decided by %.0f W at %s, %s before the heartbeat timeout", name, last.w, last.at.Format(time.RFC3339Nano), due.Sub(last.at))
	}
}

// Replay of 2026-11-18: power already below the failsafe value under LPC,
// device never enters failsafe. The simulator must not report a passed failsafe.
func TestFailsafeRun20261118NotPassedBeforeTimeout046(t *testing.T) {
	for _, tc := range []struct {
		scope, prefix string
		sign          float64
	}{{ScopeConsumption, "LPC", 1}, {ScopeProduction, "LPP", -1}} {
		t.Run(tc.prefix, func(t *testing.T) {
			c := config046(tc.scope)
			f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, LPP: true, MPC: true}}, timeout: c.FailsafeTimeout,
				baseline: tc.sign * 832, limited: []float64{tc.sign * 234}, failsafe: tc.sign * 234, free: tc.sign * 281, enters: false}
			r, err := RunOnSite(context.Background(), c, Device{}, f)
			if err != nil {
				t.Fatal(err)
			}
			if s := step046(t, r, tc.prefix+" gemessene Wirkung"); !s.Passed {
				t.Fatalf("limit effect must pass: %+v", s)
			}
			s := step046(t, r, tc.prefix+" Failsafe gemessene Wirkung")
			if s.Passed || s.Status != NotChecked || !strings.Contains(s.Detail, "nicht unterscheidbar") {
				t.Fatalf("failsafe must stay unproven when power was already below it: %+v", s)
			}
			readsAfterTimeout046(t, f, tc.prefix+" Failsafe gemessene Wirkung")
			back := step046(t, r, tc.prefix+" Vorzeitige Rückkehr gemessene Wirkung")
			if back.Passed || back.Status != NotChecked || !strings.Contains(back.Detail, "Notbetrieb nicht nachgewiesen") {
				t.Fatalf("return cannot be evaluated without proven failsafe: %+v", back)
			}
			if r.Passed || r.Outcome() != "Unvollständig" {
				t.Fatalf("outcome %s passed=%t", r.Outcome(), r.Passed)
			}
			if !strings.Contains(s.Reason(), "nicht unterscheidbar") {
				t.Fatal(s.Reason())
			}
			for _, l := range r.Limits {
				if l.Failsafe && l.At.Before(r.Heartbeats[0].Reference.Add(c.FailsafeTimeout)) {
					t.Fatalf("failsafe drawn before timeout: %+v", l)
				}
			}
		})
	}
}

// A dip below the failsafe value before the timeout is no failsafe evidence,
// also when it is actually read during the wait, or its answer arrives only
// after the timeout (R046-07). The device stays above it afterwards.
func TestFailsafeDipBeforeTimeoutIsNoEvidence046(t *testing.T) {
	t.Parallel()
	for _, block := range []time.Duration{0, 1200 * time.Millisecond} {
		t.Run(block.String(), func(t *testing.T) {
			t.Parallel()
			c := config046(ScopeConsumption)
			c.LPC = 2000
			c.FailsafeTimeout = 1500 * time.Millisecond // the 1 s sampling tick reads the dip
			f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
				baseline: 3000, limited: []float64{1500}, dip: 300, failsafe: 300, free: 2500, enters: false, block: block}
			r, err := RunOnSite(context.Background(), c, Device{}, f)
			if err != nil {
				t.Fatal(err)
			}
			s := step046(t, r, "LPC Failsafe gemessene Wirkung")
			if s.Passed || r.Outcome() != "Nicht bestanden" {
				t.Fatalf("dip before timeout passed: %+v %s", s, r.Outcome())
			}
			f.mu.Lock()
			dips := 0
			for _, read := range f.reads {
				if read.step == s.Name && read.w == 300 {
					dips++
				}
			}
			f.mu.Unlock()
			if dips == 0 {
				t.Fatal("the dip was never read; the test does not exercise it")
			}
			readsAfterTimeout046(t, f, s.Name)
			// R046-11: a late answer to a read started before the timeout is a gap in the curve.
			due := r.Heartbeats[0].Reference.Add(c.FailsafeTimeout)
			for _, sample := range r.Samples {
				if sample.Step == s.Name && !sample.At.Before(due) && sample.Watts != nil && *sample.Watts == 300 {
					t.Fatalf("pre-timeout dip stored as post-timeout value: %+v", sample)
				}
			}
		})
	}
}

// Distinguishable transition after the timeout passes; the return is measured.
func TestFailsafeAfterTimeoutPasses046(t *testing.T) {
	for _, tc := range []struct {
		scope, prefix string
		sign          float64
	}{{ScopeConsumption, "LPC", 1}, {ScopeProduction, "LPP", -1}} {
		t.Run(tc.prefix, func(t *testing.T) {
			c := config046(tc.scope)
			c.LPC, c.LPP = 2000, 2000
			f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, LPP: true, MPC: true}}, timeout: c.FailsafeTimeout,
				baseline: tc.sign * 3000, limited: []float64{tc.sign * 1500}, failsafe: tc.sign * 300, free: tc.sign * 2500, enters: true}
			r, err := RunOnSite(context.Background(), c, Device{}, f)
			if err != nil {
				t.Fatal(err)
			}
			s := step046(t, r, tc.prefix+" Failsafe gemessene Wirkung")
			if !s.Passed || s.Watts == nil || *s.Watts != tc.sign*300 {
				t.Fatalf("failsafe after timeout must pass: %+v", s)
			}
			readsAfterTimeout046(t, f, tc.prefix+" Failsafe gemessene Wirkung")
			if back := step046(t, r, tc.prefix+" Vorzeitige Rückkehr gemessene Wirkung"); !back.Passed {
				t.Fatalf("return must pass: %+v", back)
			}
			// R046-12: the measured failsafe is confirmed in place; the decisive value is stored after due.
			confirmed := false
			for _, l := range r.Limits {
				confirmed = confirmed || l.Direction == tc.prefix && l.Failsafe && !l.Unconfirmed
			}
			due := r.Heartbeats[0].Reference.Add(c.FailsafeTimeout)
			stored := false
			for _, sample := range r.Samples {
				stored = stored || sample.Step == s.Name && !sample.At.Before(due) && sample.Watts != nil && *sample.Watts == tc.sign*300
			}
			if !confirmed || !stored {
				t.Fatalf("confirmed=%t stored=%t %+v", confirmed, stored, r.Limits)
			}
			if !r.Passed {
				t.Fatalf("run must pass: %+v", r.Steps)
			}
		})
	}
}

// A device that reports its state must show failsafe: its report decides an
// otherwise indistinguishable value, and a contradicting report fails.
func TestFailsafeReportedState046(t *testing.T) {
	t.Parallel()
	c := config046(ScopeConsumption)
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 832, limited: []float64{234}, failsafe: 234, free: 900, enters: true}
	r, err := RunOnSite(context.Background(), c, Device{}, reporting046{f})
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); !s.Passed || !strings.Contains(s.Detail, "Gerät meldet Notbetrieb") {
		t.Fatalf("reported failsafe must decide: %+v", s)
	}
	readsAfterTimeout046(t, f, "LPC Failsafe gemessene Wirkung")

	no := false
	c.LPC = 2000
	f = &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 3000, limited: []float64{1500}, failsafe: 300, free: 2500, enters: true, state: &no}
	r, err = RunOnSite(context.Background(), c, Device{}, reporting046{f})
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); s.Passed || !strings.Contains(s.Detail, "meldet keinen Notbetrieb") || s.Reason() != "Das Gerät meldet keinen Notbetrieb." || r.Outcome() != "Nicht bestanden" {
		t.Fatalf("contradicting state must fail: %+v %s", s, r.Outcome())
	}
}

// R046-01: a configured outage timeout below 120 s (here the scaled minimum)
// never starts the evaluation before the device's protocol timeout.
func TestFailsafeConfiguredTimeoutCannotUndercutProtocol046(t *testing.T) {
	c := config046(ScopeConsumption)
	c.LPC = 2000
	c.FailsafeTimeout = 50 * time.Millisecond
	c.TestHeartbeatTimeout = 400 * time.Millisecond // scaled 120 s
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.TestHeartbeatTimeout,
		baseline: 3000, limited: []float64{1500}, dip: 300, failsafe: 300, free: 2500, enters: true}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	s := step046(t, r, "LPC Failsafe gemessene Wirkung")
	readsAfterTimeout046(t, f, s.Name)
	if !s.Passed || r.Config.FailsafeTimeout != c.TestHeartbeatTimeout || !strings.Contains(r.Profile, "angehoben") {
		t.Fatalf("%+v timeout=%s profile=%s", s, r.Config.FailsafeTimeout, r.Profile)
	}
	if d := r.Heartbeats[0].Deadline.Sub(r.Heartbeats[0].Reference); d != c.TestHeartbeatTimeout+HeartbeatReserve {
		t.Fatal(d)
	}
	raised := c
	raised.FailsafeTimeout = c.TestHeartbeatTimeout
	if c.RunDeadline() != raised.RunDeadline() || c.TimeBudget() != raised.TimeBudget() {
		t.Fatal("run deadline and budget must use the protocol timeout")
	}
	production := c
	production.TestHeartbeatTimeout = 0
	if production.DeviceOutageTimeout() != ProtocolHeartbeatTimeout {
		t.Fatal(production.DeviceOutageTimeout())
	}
	if raw, _ := json.Marshal(c); strings.Contains(string(raw), "TestHeartbeat") || strings.Contains(string(raw), "testHeartbeat") {
		t.Fatal("test timeout must never be serialised")
	}
	var decoded Config
	if json.Unmarshal([]byte(`{"TestHeartbeatTimeout":1,"testHeartbeatTimeout":1}`), &decoded) != nil || decoded.TestHeartbeatTimeout != 0 {
		t.Fatal("test timeout must not be settable via JSON")
	}
}

// A positive receipt that arrives after the reference moves the evaluation (R046-03).
func TestFailsafeLateReceiptMovesDue046(t *testing.T) {
	c := config046(ScopeConsumption)
	c.LPC = 2000
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout + 200*time.Millisecond,
		baseline: 3000, limited: []float64{1500}, dip: 300, failsafe: 300, free: 2500, enters: true, lateReceipt: 200 * time.Millisecond}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	s := step046(t, r, "LPC Failsafe gemessene Wirkung")
	readsAfterTimeout046(t, f, s.Name)
	h := r.Heartbeats[0]
	if !s.Passed || !h.Reference.Equal(f.stopped.Add(200*time.Millisecond)) || !strings.Contains(h.Basis, "Quittung nach dem Stopp") {
		t.Fatalf("%+v %+v", s, h)
	}
}

// The device answers 28 s after its timeout, inside the 30 s reserve.
func TestFailsafeLateEffectInsideReservePasses046(t *testing.T) {
	t.Parallel()
	c := config046(ScopeConsumption)
	c.LPC = 2000
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout + 28*time.Second,
		baseline: 3000, limited: []float64{1500}, failsafe: 300, free: 2500, enters: true}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); !s.Passed {
		t.Fatalf("%+v", s)
	}
}

// R046-02: the long-duration test evaluates no return without shown failsafe.
func TestFailsafeLongTestWithoutShownFailsafe046(t *testing.T) {
	c := config046(ScopeConsumption)
	c.TestProfile = "full"
	c.RecoveryTimeout = 3 * time.Hour
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 832, limited: []float64{234}, failsafe: 234, free: 900, enters: false}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"LPC Wiederkehr", "LPC Wiederkehr gemessene Wirkung"} {
		if s := step046(t, r, name); s.Passed || s.Status != NotChecked || s.Detail != ReturnNotEvaluable {
			t.Fatalf("%+v", s)
		}
	}
	unconfirmed := false
	for _, l := range r.Limits {
		unconfirmed = unconfirmed || l.Failsafe && l.Unconfirmed
	}
	if !unconfirmed || r.Passed {
		t.Fatalf("unproven failsafe must be drawn unconfirmed: %+v", r.Limits)
	}
}

// R046-04: a cancel during the wait and a connection loss during the return
// measurement keep their cause and never pass.
func TestFailsafeCancelAndConnectionLoss046(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := config046(ScopeConsumption)
	c.LPC = 2000
	c.FailsafeTimeout = time.Second
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 3000, limited: []float64{1500}, failsafe: 300, free: 2500, enters: true, cancelAfter: 100 * time.Millisecond, cancel: cancel}
	r, err := RunOnSite(ctx, c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); s.Passed || s.Status != NotChecked || s.ReasonCode != "user_cancel" {
		t.Fatalf("%+v", s)
	}

	c.FailsafeTimeout = 300 * time.Millisecond
	f = &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 3000, limited: []float64{1500}, failsafe: 300, free: 2500, enters: true, lost: true}
	r, err = RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); !s.Passed {
		t.Fatalf("%+v", s)
	}
	if s := step046(t, r, "LPC Vorzeitige Rückkehr gemessene Wirkung"); s.Passed || s.ReasonCode != "connection_lost" {
		t.Fatalf("%+v", s)
	}
}

// R046-10: a device that keeps answering with its last measurement (old
// measurement time) gives no post-timeout evidence, even below the failsafe value.
func TestFailsafeRepeatedMeasurementIsNoEvidence046(t *testing.T) {
	t.Parallel()
	c := config046(ScopeConsumption)
	c.LPC = 2000
	c.FailsafeTimeout = 1500 * time.Millisecond // the wait reads the frozen value once
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 3000, limited: []float64{1500}, dip: 300, failsafe: 300, free: 2500, enters: false, freeze: true}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); s.Passed || r.Outcome() != "Nicht bestanden" {
		t.Fatalf("repeated pre-timeout measurement passed: %+v", s)
	}
}

// R046-16: a drop earlier than any failsafe can start (before reference +
// timeout/2) is a load change; a later drop may be an early compliant entry.
func TestFailsafeEarlyDropVersusEarlyEntry046(t *testing.T) {
	t.Parallel()
	c := config046(ScopeConsumption)
	c.LPC = 2000
	c.FailsafeTimeout = 3 * time.Second // wait reads at +1 s and +2 s; failsafe possible from +1.5 s
	early := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 3000, limited: []float64{1500}, dip: 300, failsafe: 300, free: 2500, enters: true}
	r, err := RunOnSite(context.Background(), c, Device{}, early)
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); s.Passed || s.Status != NotChecked || !strings.Contains(s.Detail, "früher als ein Notbetrieb möglich") {
		t.Fatalf("early load drop counted as failsafe: %+v", s)
	}
	plausible := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: 1800 * time.Millisecond,
		baseline: 3000, limited: []float64{1500}, failsafe: 300, free: 2500, enters: true}
	r, err = RunOnSite(context.Background(), c, Device{}, plausible)
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); !s.Passed {
		t.Fatalf("compliant early entry must pass: %+v", s)
	}
	plausible.mu.Lock()
	defer plausible.mu.Unlock()
	low := false
	for _, read := range plausible.reads {
		low = low || read.step == "LPC Failsafe gemessene Wirkung" && read.w == 300 && read.at.Before(plausible.stopped.Add(c.FailsafeTimeout))
	}
	if !low {
		t.Fatal("the early entry was not read during the wait; the test does not exercise it")
	}
}

// R046-19: a receipt arriving during the deciding read moves the window; the
// candidate is discarded and the device is evaluated after the new due time.
func TestFailsafeReceiptDuringDecidingRead046(t *testing.T) {
	c := config046(ScopeConsumption)
	c.LPC = 2000
	c.FailsafeTimeout = time.Second // the discarded candidate (dip) lies after timeout/2
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout + 200*time.Millisecond,
		baseline: 3000, limited: []float64{1500}, dip: 300, failsafe: 300, free: 2500, enters: true, lateReceipt: 200 * time.Millisecond, receiptFrom: 3}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	s := step046(t, r, "LPC Failsafe gemessene Wirkung")
	readsAfterTimeout046(t, f, s.Name)
	if !s.Passed || !r.Heartbeats[0].Reference.Equal(f.stopped.Add(200*time.Millisecond)) {
		t.Fatalf("%+v %+v", s, r.Heartbeats[0])
	}
}

// R046-22: a positive receipt later than stop + timeout cannot move the window;
// the reference is unclear and the failsafe is not evaluated.
func TestFailsafeImpossibleLateReceiptNotEvaluable046(t *testing.T) {
	c := config046(ScopeConsumption)
	c.LPC = 2000
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 3000, limited: []float64{1500}, failsafe: 300, free: 2500, enters: true, lateReceipt: c.FailsafeTimeout + 100*time.Millisecond}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	s := step046(t, r, "LPC Failsafe gemessene Wirkung")
	if s.Passed || s.Status != NotChecked || !strings.Contains(s.Detail, "nicht bewertbar") || s.Reason() != "Notbetrieb nicht bewertbar: Zeitbezug der Lebenszeichen unklar." {
		t.Fatalf("%+v", s)
	}
}

// R046-23: a reply measured before an already known newer measurement is no evidence.
func TestFailsafeBackwardsMeasurementTimeIsNoEvidence046(t *testing.T) {
	t.Parallel()
	c := config046(ScopeConsumption)
	c.LPC = 2000
	f := &failsafe046{onsiteFake: onsiteFake{caps: Capabilities{LPC: true, MPC: true}}, timeout: c.FailsafeTimeout,
		baseline: 3000, limited: []float64{1500}, failsafe: 300, free: 2500, enters: false, backwards: true}
	r, err := RunOnSite(context.Background(), c, Device{}, f)
	if err != nil {
		t.Fatal(err)
	}
	if s := step046(t, r, "LPC Failsafe gemessene Wirkung"); s.Passed {
		t.Fatalf("older measurement passed: %+v", s)
	}
}
