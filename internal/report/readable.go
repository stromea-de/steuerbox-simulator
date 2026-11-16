package report

import (
	"fmt"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func fallback(value, other string) string {
	if strings.TrimSpace(value) == "" {
		return other
	}
	return value
}
func humanDuration(v time.Duration) string {
	if v > 0 && v%time.Hour == 0 {
		return fmt.Sprintf("%s Std.", germanNumber(v.Hours()))
	}
	if v > 0 && v%time.Minute == 0 {
		return fmt.Sprintf("%s Min.", germanNumber(v.Minutes()))
	}
	return germanNumber(v.Seconds()) + " s"
}

var goDuration = regexp.MustCompile(`\b[0-9]+(?:h[0-9]+m[0-9.]+s|m[0-9.]+s|h|m|s)\b`)
var internals = regexp.MustCompile(`(?i)(?:[;,] ?)?(?:changeable|isValueChangeable|LPC|LPP|MPC)\s*[:=]\s*(?:true|false)|(?:[;,] ?)?kein Cache`)

func readableDetail(text string) string {
	text = goDuration.ReplaceAllStringFunc(text, func(v string) string {
		d, e := time.ParseDuration(v)
		if e != nil {
			return v
		}
		return humanDuration(d)
	})
	text = internals.ReplaceAllString(text, "")
	text = strings.NewReplacer("true", "ja", "false", "nein", "context canceled", "Prüfung abgebrochen", "context deadline exceeded", "Zeitlimit erreicht").Replace(text)
	return strings.TrimSpace(strings.NewReplacer("Std..", "Std.", "Min..", "Min.").Replace(text))
}

func presentStepDetail(r inspection.Report, s inspection.Step) string {
	kind := "LPC"
	direction := "Bezug"
	if strings.HasPrefix(s.Name, "LPP ") {
		kind = "LPP"
		direction = "Einspeisung"
	}
	if strings.HasSuffix(s.Name, "Failsafe Ursprungswerte") {
		if v, ok := r.FailsafeOriginals[kind]; ok {
			return "Notbetriebswerte vorher (" + direction + ", " + kind + "): " + watts(v.Power) + ", " + humanDuration(v.Minimum) + " – direkt vom Gerät gelesen"
		}
	}
	if s.Name == "Unterstützte Use Cases" {
		return functionSummary(r)
	}
	if !s.Passed && s.Status != inspection.NotChecked {
		if stepConnectionLost(r, s) {
			return "Keine Antwort. Verbindung riss ab."
		}
		return s.Reason()
	}
	return stepDetail(s)
}

func functionSummary(r inspection.Report) string {
	if r.Capabilities == nil {
		return "Gerätefunktionen nicht gespeichert"
	}
	var parts []string
	for _, kind := range []string{"LPC", "LPP", "MPC"} {
		if kind != "MPC" && !r.Config.Includes(kind) {
			continue
		}
		yes := r.Capabilities.MPC
		if kind == "LPC" {
			yes = r.Capabilities.LPC
		}
		if kind == "LPP" {
			yes = r.Capabilities.LPP
		}
		// 038: one line per function, plain wording.
		label := map[string]string{"LPC": "Bezug begrenzen (LPC)", "LPP": "Einspeisung begrenzen (LPP)", "MPC": "Leistung messen (MPC)"}[kind] + ": vom Gerät nicht angeboten"
		if yes {
			label = map[string]string{"LPC": "Bezug begrenzen (LPC)", "LPP": "Einspeisung begrenzen (LPP)", "MPC": "Leistung messen (MPC)"}[kind] + ": vom Gerät angeboten"
		}
		var names []string
		for _, n := range r.Capabilities.Scenarios[kind] {
			name := fmt.Sprintf("Funktion %d", n)
			if kind == "MPC" && n == 1 {
				name = "Leistungsmessung"
			} else if n == 1 {
				name = "Grenze"
			} else if n == 2 {
				name = "Notbetriebswerte"
			} else if n == 3 {
				name = "Lebenszeichen"
			}
			names = append(names, name)
		}
		if len(names) > 0 {
			label += " (" + strings.Join(names, ", ") + ")"
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "\n")
}

func displayAbort(r inspection.Report) string {
	if r.AbortReason != "" {
		return inspection.ReasonText(r.AbortReason)
	}
	for _, event := range r.Events {
		if event.Name == "Verbindungsabbruch" {
			return "Verbindung zum Gerät um " + event.At.Format("15:04:05") + " unterbrochen."
		}
	}
	return ""
}

var historicalMPC = regexp.MustCompile(`^MPC (-?[0-9]+(?:\.[0-9]+)?) W(?:;.*)?$`)

// Conservative display migration of an evidenced insufficient baseline. Preserve
// original steps and JSON for attachment/hash, including historical verdicts.
func renderReport(r inspection.Report) inspection.Report {
	r.Steps = append([]inspection.Step(nil), r.Steps...)
	for i, s := range r.Steps {
		if !s.Passed || s.Status == inspection.NotChecked || (s.Name != "Ausgangsmessung MPC" && s.Name != "LPP Ausgangsmessung MPC") {
			continue
		}
		var value float64
		if s.Watts != nil {
			value = *s.Watts
		} else {
			m := historicalMPC.FindStringSubmatch(s.Detail)
			if m == nil {
				continue
			}
			v, e := strconv.ParseFloat(m[1], 64)
			if e != nil {
				continue
			}
			value = v
		}
		limit := r.Config.LPC
		if s.Name == "LPP Ausgangsmessung MPC" {
			limit = r.Config.LPP
			value = -value
		}
		if value <= limit+r.Config.Tolerance {
			s.Passed = false
			s.Status = inspection.NotChecked
			s.ReasonCode = "insufficient_load"
			s.Detail = "Nicht geprüft: zu wenig Leistung"
			v := value
			if s.Name == "LPP Ausgangsmessung MPC" {
				v = -value
			}
			s.Watts = &v
			r.Steps[i] = s
		}
	}
	return r
}
