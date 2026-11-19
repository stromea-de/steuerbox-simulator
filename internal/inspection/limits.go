package inspection

import "time"

// LimitChange is the limit in effect for one direction from At on (UX-S21).
// It is recorded at acknowledged limit writes and heartbeat changes of the run,
// never inferred from step names. Watts is signed: consumption (LPC) positive,
// feed-in (LPP) negative; nil means no limit was in effect.
type LimitChange struct {
	At        time.Time `json:"at"`
	Direction string    `json:"direction"`
	Watts     *float64  `json:"watts"`
	Failsafe  bool      `json:"failsafe,omitempty"`
	// Unconfirmed: the controller ended failsafe, but no measurement proved the
	// device left it; the failsafe value may still apply.
	Unconfirmed bool `json:"unconfirmed,omitempty"`
}

// limitTimeline applies the LPC/LPP state machine to what the controller
// observed: acknowledged limit writes or deactivations, its heartbeat stop and
// restart, and measurements (UX-S21, Review 037c).
//   - Failsafe begins when the device's heartbeat timeout expires (reference =
//     stop or later heartbeat receipt, plus the timeout), or earlier when a
//     measurement shows the failsafe effect. A heartbeat restart before that
//     deadline means failsafe never began.
//   - The simulator device leaves failsafe as soon as the heartbeat resumes
//     (its implemented behaviour). A real device's return is only confirmed by
//     a measurement; an acknowledged write alone marks it unconfirmed.
type limitTimeline struct {
	report      *Report
	failsafe    float64
	includes    func(string) bool
	immediate   bool
	active      map[string]*float64
	inFS        map[string]bool
	unconfirmed map[string]bool
	heartbeat   bool
	deadline    time.Time
}

func newLimitTimeline(r *Report, failsafe float64, includes func(string) bool, immediate bool) *limitTimeline {
	return &limitTimeline{report: r, failsafe: failsafe, includes: includes, immediate: immediate, active: map[string]*float64{}, inFS: map[string]bool{}, unconfirmed: map[string]bool{}, heartbeat: true}
}

func limitSign(direction string) float64 {
	if direction == "LPP" {
		return -1
	}
	return 1
}

func (t *limitTimeline) effective(direction string) (*float64, bool, bool) {
	if t.inFS[direction] {
		v := limitSign(direction) * t.failsafe
		return &v, true, t.unconfirmed[direction]
	}
	if v := t.active[direction]; v != nil {
		w := limitSign(direction) * *v
		return &w, false, false
	}
	return nil, false, false
}

func (t *limitTimeline) last(direction string) (*LimitChange, bool) {
	for i := len(t.report.Limits) - 1; i >= 0; i-- {
		if t.report.Limits[i].Direction == direction {
			return &t.report.Limits[i], true
		}
	}
	return nil, false
}

// emit appends a change for each direction whose effective limit changed.
func (t *limitTimeline) emit(at time.Time, b Backend) {
	for _, direction := range []string{"LPC", "LPP"} {
		if !t.includes(direction) {
			continue
		}
		value, failsafe, unconfirmed := t.effective(direction)
		previous, seen := t.last(direction)
		if !seen && value == nil {
			continue
		}
		if seen && previous.Failsafe == failsafe && previous.Unconfirmed == unconfirmed && ((previous.Watts == nil && value == nil) || (previous.Watts != nil && value != nil && *previous.Watts == *value)) {
			continue
		}
		change := LimitChange{At: at, Direction: direction, Watts: value, Failsafe: failsafe, Unconfirmed: unconfirmed}
		t.report.Limits = append(t.report.Limits, change)
		if observer, ok := b.(interface{ LimitChanged(LimitChange) }); ok {
			observer.LimitChanged(change)
		}
	}
}

// begin starts failsafe at the given moment for every direction under test.
func (t *limitTimeline) begin(at time.Time, b Backend) {
	t.deadline = time.Time{}
	for _, direction := range []string{"LPC", "LPP"} {
		if t.includes(direction) {
			t.inFS[direction] = true
			// 046 (R046-12): a real device's failsafe is expected, not shown, until
			// a measurement or its reported state confirms it.
			if t.immediate {
				delete(t.unconfirmed, direction)
			} else {
				t.unconfirmed[direction] = true
			}
		}
	}
	t.emit(at, b)
}

// due starts a pending failsafe whose deadline has passed by now.
func (t *limitTimeline) due(now time.Time, b Backend) {
	if !t.deadline.IsZero() && !now.Before(t.deadline) {
		t.begin(t.deadline, b)
	}
}

// Limit records an acknowledged write (active) or deactivation for direction.
func (t *limitTimeline) Limit(b Backend, direction string, value float64, active bool) {
	now := time.Now()
	t.due(now, b)
	if active {
		v := value
		t.active[direction] = &v
	} else {
		delete(t.active, direction)
	}
	if t.heartbeat && t.inFS[direction] {
		if t.immediate {
			delete(t.inFS, direction)
		} else {
			t.unconfirmed[direction] = true
		}
	}
	t.emit(now, b)
}

// HeartbeatStopped records a successful stop; failsafe becomes due at deadline.
func (t *limitTimeline) HeartbeatStopped(deadline time.Time) {
	t.heartbeat = false
	t.deadline = deadline
}

// HeartbeatStarted records a successful (re)start.
func (t *limitTimeline) HeartbeatStarted(b Backend) {
	now := time.Now()
	t.due(now, b)
	t.deadline = time.Time{}
	t.heartbeat = true
	if t.immediate {
		for direction := range t.inFS {
			delete(t.inFS, direction)
			delete(t.unconfirmed, direction)
		}
	}
	t.emit(now, b)
}

// FailsafeMeasured: a measurement (or the device's state) showed the failsafe
// effect of direction, so failsafe began at the latest now. The expected
// failsafe change is confirmed in place, live and in the report (046).
func (t *limitTimeline) FailsafeMeasured(b Backend, direction string) {
	now := time.Now()
	t.due(now, b)
	if !t.deadline.IsZero() {
		t.begin(now, b)
	}
	if !t.inFS[direction] || !t.unconfirmed[direction] {
		return
	}
	delete(t.unconfirmed, direction)
	if c, ok := t.last(direction); ok && c.Failsafe && c.Unconfirmed {
		c.Unconfirmed = false
		if observer, ok := b.(interface{ LimitChanged(LimitChange) }); ok {
			observer.LimitChanged(*c) // same time and direction: replaces the live entry
		}
	}
}

// ReturnMeasured: a measurement showed the device left failsafe.
func (t *limitTimeline) ReturnMeasured(b Backend, direction string) {
	now := time.Now()
	t.due(now, b)
	delete(t.inFS, direction)
	delete(t.unconfirmed, direction)
	t.emit(now, b)
}

// Finish starts a failsafe that became due before the end of the run.
func (t *limitTimeline) Finish(b Backend, end time.Time) { t.due(end, b) }

// failsafeDeadline: reference is the stop or a later heartbeat receipt.
func failsafeDeadline(stopped, receipt time.Time, timeout time.Duration) time.Time {
	reference := stopped
	if receipt.After(reference) {
		reference = receipt
	}
	return reference.Add(timeout)
}
