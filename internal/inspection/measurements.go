package inspection

import (
	"context"
	"math"
	"time"
)

const SampleCapacity = 600

type Sample struct {
	At    time.Time `json:"at"`
	Watts *float64  `json:"watts"` // null explicitly marks a failed read / gap
	Step  string    `json:"step"`
}
type LowLoad struct {
	Directions []LowLoad `json:"directions,omitempty"`
	Watts      float64   `json:"watts"`
	Required   float64   `json:"required"`
	Direction  string    `json:"direction"`
}

type measurements struct {
	report            *Report
	backend           Backend
	current, previous string
	last              time.Time
	// device: the device's own measurement time of the last reading, zero if unknown;
	// validDevice keeps the newest known one across failed or untimed reads (046).
	device, validDevice time.Time
}

// fresh reports whether the last reading was measured by the device after prior
// (both device clock); known is false when the device sends no measurement time.
// A reply with an old measurement time is a repeated value, no new evidence (R046-10).
func (m *measurements) fresh(prior time.Time) (fresh, known bool) {
	if m.device.IsZero() {
		return true, false
	}
	return prior.IsZero() || m.device.After(prior), true
}

func newMeasurements(r *Report, b Backend) *measurements { return &measurements{report: r, backend: b} }
func (m *measurements) record(value float64, err error) {
	now := time.Now()
	if m.current == m.previous && now.Sub(m.last) < 900*time.Millisecond {
		return
	}
	sample := Sample{At: now, Step: m.current}
	if err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
		sample.Watts = &value
	}
	if len(m.report.Samples) == SampleCapacity {
		copy(m.report.Samples, m.report.Samples[1:])
		m.report.Samples = m.report.Samples[:SampleCapacity-1]
	}
	m.report.Samples = append(m.report.Samples, sample)
	m.previous = m.current
	m.last = now
	if observer, ok := m.backend.(interface{ PowerSample(Sample) }); ok {
		observer.PowerSample(sample)
	}
}
func (m *measurements) power(ctx context.Context) (float64, error) {
	v, err := m.backend.Power(ctx)
	m.device = time.Time{}
	if source, ok := m.backend.(interface{ PowerMeasuredAt() (time.Time, bool) }); ok && err == nil {
		if t, known := source.PowerMeasuredAt(); known {
			m.device = t
			if t.After(m.validDevice) {
				m.validDevice = t
			}
		}
	}
	if err == nil && ctx.Err() != nil {
		// 046 (R046-11): an answer after its read deadline is recorded as a gap.
		m.record(v, ctx.Err())
		return v, err
	}
	m.record(v, err)
	return v, err
}

// Serial reads avoid competing SPINE requests. Long waits keep yielding samples,
// each with a bounded read deadline; no background sampler outlives cleanup.
func (m *measurements) wait(ctx context.Context, duration, readTimeout time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		case <-ticker.C:
			c, cancel := context.WithTimeout(ctx, readTimeout)
			_, _ = m.power(c)
			cancel()
		}
	}
}

func (m *measurements) finish(value float64) {
	if len(m.report.Samples) > 0 {
		last := m.report.Samples[len(m.report.Samples)-1]
		if last.Step == m.current && last.Watts != nil && *last.Watts == value {
			return
		}
	}
	m.last = time.Time{}
	m.record(value, nil)
}
