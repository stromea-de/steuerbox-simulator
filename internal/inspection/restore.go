package inspection

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"
)

// LimitState is a fresh read of the device's LPC/LPP power limit (236). LPP is
// reported as positive watts like the rest of this package.
type LimitState struct {
	Watts    float64   `json:"watts"`
	HasValue bool      `json:"hasValue,omitempty"`
	Active   bool      `json:"active"`
	Known    bool      `json:"known,omitempty"`
	ReadAt   time.Time `json:"readAt,omitempty"`
}

// LimitReader proves the end state of a limit by reading it back.
type LimitReader interface {
	ReadLimit(context.Context, string) (LimitState, error)
}

// PendingRestore is everything needed to put a device back after a crash (236).
type PendingRestore struct {
	ReportID string                    `json:"reportId"`
	Saved    time.Time                 `json:"saved"`
	Failsafe map[string]FailsafeValues `json:"failsafe"`
	Limits   map[string]LimitState     `json:"limits,omitempty"`
}

// RestoreJournal persists the original values before the first device write and
// drops them only after a confirmed restore and release.
// EntityPinner fixes the device entity of each function before its originals are
// read (R4-02), so reads, the journal and all writes refer to one entity.
type EntityPinner interface {
	PinEntities(kinds []string) error
}

type RestoreJournal interface {
	SaveRestore(PendingRestore) error
	// ClearRestore deletes the record only when it still belongs to reportID.
	ClearRestore(reportID string) error
}

// sameFailsafe compares the field as the device reports it. Raw wire numbers are
// compared as rationals; floats only without wire data.
func sameFailsafe(field string, want, got FailsafeValues) bool {
	if field == "minimum" {
		return want.Minimum == got.Minimum
	}
	if want.PowerNumber != nil && got.PowerNumber != nil {
		return rationalWatts(*want.PowerNumber, want.PowerScale).Cmp(rationalWatts(*got.PowerNumber, got.PowerScale)) == 0
	}
	return math.Abs(want.Power-got.Power) < 0.0005
}
func rationalWatts(n int64, scale *int8) *big.Rat {
	r := new(big.Rat).SetInt64(n)
	s := 0
	if scale != nil {
		s = int(*scale)
	}
	f := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(max(s, -s))), nil))
	if s < 0 {
		return r.Quo(r, f)
	}
	return r.Mul(r, f)
}

// restoreFailsafeField writes one original value, waits for the acknowledgement
// and reads it back. Only a matching readback counts as restored.
func restoreFailsafeField(ctx context.Context, b DeviceBackend, k, field string, original FailsafeValues) (string, error) {
	if err := b.WriteFailsafe(ctx, k, field, original); err != nil {
		return "", err
	}
	got, err := b.ReadFailsafe(ctx, k)
	if err != nil {
		return "", fmt.Errorf("geschrieben, aber nicht zurückgelesen: %w", err)
	}
	if !sameFailsafe(field, original, got) {
		if field == "minimum" {
			return "", fmt.Errorf("Gerät meldet Mindestdauer %s statt %s", got.Minimum, original.Minimum)
		}
		return "", fmt.Errorf("Gerät meldet %g W statt %g W", got.Power, original.Power)
	}
	if field == "minimum" {
		return fmt.Sprintf("Ursprüngliche Notbetriebs-Mindestdauer %s geschrieben, quittiert und zurückgelesen", original.Minimum), nil
	}
	return fmt.Sprintf("Ursprüngliche Notbetriebs-Leistung %g W geschrieben, quittiert und zurückgelesen", original.Power), nil
}

// restoredField is a failsafe field already confirmed during this restore.
type restoredField struct {
	kind, field string
	want        FailsafeValues
}

// restoreFailsafeOnce reads the field first and writes it only when it differs
// (236b: LPC and LPP may share one failsafe minimum; a second write of the same
// value is rejected by the Opta). An unreadable field is not written (R5-01).
// After a write, every field confirmed earlier in this restore is read again
// (R5-02): a shared device value written twice with different originals must not
// count as restored.
func restoreFailsafeOnce(ctx context.Context, b DeviceBackend, k, field string, original FailsafeValues, done []restoredField) (string, error) {
	cur, err := b.ReadFailsafe(ctx, k)
	if err != nil {
		return "", fmt.Errorf("aktueller Wert nicht lesbar (%w); nichts geschrieben", err)
	}
	if sameFailsafe(field, original, cur) {
		return "Gerät meldet bereits den Ursprungswert; nichts geschrieben", nil
	}
	detail, err := restoreFailsafeField(ctx, b, k, field, original)
	if err != nil {
		return "", err
	}
	read := map[string]FailsafeValues{}
	for _, d := range done {
		got, ok := read[d.kind]
		if !ok {
			if got, err = b.ReadFailsafe(ctx, d.kind); err != nil {
				return "", fmt.Errorf("geschrieben; %s nicht erneut zurückgelesen: %w", d.kind, err)
			}
			read[d.kind] = got
		}
		if !sameFailsafe(d.field, d.want, got) {
			return "", fmt.Errorf("geschrieben; danach meldet %s nicht mehr den Ursprungswert (gemeinsamer Gerätewert)", d.kind)
		}
	}
	return detail, nil
}

var errNoLimitReader = errors.New("Grenze nicht zurücklesbar")

// sameLimitWatts allows only the half-milliwatt rounding of the write path (R2-07).
// The extra 1e-9 W absorbs float error of that rounding itself (R3-06: 4200.0005 W
// is written as 4200.001 W); it is far below any representable milliwatt step.
func sameLimitWatts(a, b float64) bool { return math.Abs(a-b) <= 0.0005+1e-9 }

// verifyReleased reads the limit back (R236-01): it must be inactive and, when the
// original value is known, carry that value again. Unknown state is a failure.
func verifyReleased(ctx context.Context, b DeviceBackend, k string, original LimitState, haveOriginal bool) error {
	reader, ok := b.(LimitReader)
	if !ok {
		return errNoLimitReader
	}
	v, err := reader.ReadLimit(ctx, k)
	if err != nil {
		return fmt.Errorf("%s Grenze nicht zurückgelesen: %w", k, err)
	}
	if !v.Known {
		return fmt.Errorf("%s Gerät meldet keinen Aktivstatus der Grenze", k)
	}
	if v.Active {
		return fmt.Errorf("%s Begrenzung laut Gerät weiter aktiv", k)
	}
	want := releaseValue(map[string]LimitState{k: original}, k)
	if !haveOriginal {
		want = 0
	}
	if !v.HasValue || !sameLimitWatts(v.Watts, want) {
		return fmt.Errorf("%s Gerät meldet Grenzwert %g W statt %g W", k, v.Watts, want)
	}
	return nil
}

// releaseValue is the original limit value, written inactive; 0 W without a read.
func releaseValue(limits map[string]LimitState, k string) float64 {
	if v, ok := limits[k]; ok && v.HasValue && !math.IsNaN(v.Watts) && !math.IsInf(v.Watts, 0) && v.Watts >= 0 {
		return v.Watts
	}
	return 0
}

// RestoreOriginals puts a device back after a restart (236): release limits,
// write the saved failsafe values, read everything back. ok only when all of it
// is confirmed by the device.
func RestoreOriginals(ctx context.Context, b DeviceBackend, p PendingRestore, timeout time.Duration) ([]Step, bool) {
	var steps []Step
	ok := true
	add := func(name, detail string, err error) {
		s := Step{Name: name, Detail: detail, Passed: err == nil, At: time.Now()}
		if err != nil {
			s.Detail = UserError(err)
			ok = false
		}
		steps = append(steps, s)
	}
	kinds := make([]string, 0, len(p.Failsafe))
	for k := range p.Failsafe {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	run := func(fn func(context.Context) (string, error)) (string, error) {
		cc, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := b.Recover(cc); err != nil {
			return "", err
		}
		return fn(cc)
	}
	// R236-04: write only what differs from the saved originals.
	// R3-03: a device that still reports every saved original – failsafe values, limit
	// value and its active flag – is left alone. A crash before the first configuration
	// write therefore changes nothing, also when another EMS had an active limit.
	// R2-01: as soon as anything differs, the test touched the device; an active limit
	// is then always released, also when its value equals the original. The end state
	// is "inactive, original value" like a regular end.
	// R2-03/R3-04: without a saved original limit nothing is guessed or written and the
	// restore is not confirmed; the record stays.
	type plan struct{ limit, power, minimum, blocked bool }
	todo := map[string]plan{}
	needed, untouched, unreadable := false, true, false
	for _, k := range kinds {
		var pl plan
		orig, have := p.Limits[k]
		pl.blocked = !have
		same := false
		_, err := run(func(cc context.Context) (string, error) {
			cur, err := b.ReadFailsafe(cc, k)
			if err != nil {
				return "", err
			}
			pl.power = !sameFailsafe("power", p.Failsafe[k], cur)
			pl.minimum = !sameFailsafe("minimum", p.Failsafe[k], cur)
			reader, ok := b.(LimitReader)
			if !ok || !have {
				pl.limit = have
				return "", nil
			}
			l, err := reader.ReadLimit(cc, k)
			if err != nil {
				return "", fmt.Errorf("%s Grenze nicht lesbar: %w", k, err)
			}
			// R5-01: a reply without active flag or without the value that was saved is
			// an incomplete read, not a detected change; nothing is written then.
			if !l.Known || orig.HasValue && !l.HasValue {
				return "", fmt.Errorf("%s Grenze unvollständig gemeldet", k)
			}
			released := l.Known && !l.Active
			valueSame := l.HasValue && sameLimitWatts(l.Watts, releaseValue(p.Limits, k))
			pl.limit = !released || !valueSame
			same = !pl.power && !pl.minimum && l.Known && l.Active == orig.Active && valueSame
			return "", nil
		})
		if err != nil {
			// R4-03: an unreadable state is no detected change. Nothing is written; the
			// record stays and the next restore reads again.
			add(k+" Originalwerte lesen", "", fmt.Errorf("aktueller Gerätezustand nicht lesbar (%w); nichts geschrieben", err))
			unreadable = true
		}
		// 236b (real device): a device in failsafe state rejects new failsafe values
		// (LPC/LPP state machine). Writing the limit – inactive, original value, the
		// end state anyway – after the heartbeat ends that state first; so a failsafe
		// field to restore always comes with the release of its limit.
		if have && (pl.power || pl.minimum) {
			pl.limit = true
		}
		untouched = untouched && same
		todo[k] = pl
	}
	if unreadable {
		return steps, false
	}
	for _, k := range kinds {
		if todo[k].blocked {
			add(k+" Grenzen aufheben", "", fmt.Errorf("%s Ursprungsgrenze nicht gespeichert; Grenze nicht verändert, am Gerät prüfen", k))
		}
		pl := todo[k]
		pl.limit = pl.limit && !pl.blocked
		todo[k] = pl
		needed = needed || pl.limit || pl.power || pl.minimum
	}
	if untouched && ok {
		add("Originalwerte prüfen", "Gerät meldet bereits alle gespeicherten Originalwerte; nichts geschrieben", nil)
		return steps, ok
	}
	if !needed {
		if !ok {
			return steps, ok
		}
		add("Originalwerte prüfen", "Gerät meldet bereits die gespeicherten Originalwerte; nichts geschrieben", nil)
		return steps, ok
	}
	// Like the regular cleanup: a running heartbeat ends the device's failsafe state.
	_, err := run(func(cc context.Context) (string, error) { return "", b.Heartbeat(cc, true) })
	add("Heartbeat starten", "Bestätigt", err)
	// R6-01: fields that already matched at the first read count as confirmed and
	// are re-checked after every later write (shared device values).
	var done []restoredField
	for _, k := range kinds {
		if !todo[k].power {
			done = append(done, restoredField{k, "power", p.Failsafe[k]})
		}
		if !todo[k].minimum {
			done = append(done, restoredField{k, "minimum", p.Failsafe[k]})
		}
	}
	for _, k := range kinds {
		if todo[k].limit {
			detail, err := run(func(cc context.Context) (string, error) {
				_, err := b.Limit(cc, k, releaseValue(p.Limits, k), false)
				return "Grenze inaktiv auf Ursprungswert geschrieben", err
			})
			add(k+" Grenze freigeben", detail, err)
		}
		for _, field := range []string{"power", "minimum"} {
			if field == "power" && !todo[k].power || field == "minimum" && !todo[k].minimum {
				continue
			}
			name := k + " Failsafe-Leistung zurückstellen"
			if field == "minimum" {
				name = k + " Failsafe-Mindestdauer zurückstellen"
			}
			detail, err := run(func(cc context.Context) (string, error) {
				return restoreFailsafeOnce(cc, b, k, field, p.Failsafe[k], done)
			})
			add(name, detail, err)
			if err == nil {
				done = append(done, restoredField{k, field, p.Failsafe[k]})
			}
		}
	}
	for _, k := range kinds {
		if !todo[k].limit {
			continue
		}
		orig, have := p.Limits[k]
		_, err := run(func(cc context.Context) (string, error) { return "", verifyReleased(cc, b, k, orig, have) })
		add(k+" Grenzen aufheben", "Zurückgelesen: keine aktive Begrenzung, Ursprungswert", err)
	}
	return steps, ok
}
