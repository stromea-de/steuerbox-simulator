package inspection

import (
	"regexp"
	"strconv"
)

// FunctionMatrix is the screen projection, shared with the no-JavaScript page.
// It never changes evaluation, stored checkpoints or the PDF/JSON evidence.
func (r Report) FunctionMatrix() []MatrixRow {
	titles := []string{"Grenze angenommen", "Grenze wirkt", "Notbetrieb bei Ausfall", "Rückkehr zum Normalbetrieb"}
	if r.Config.FullTest() {
		titles = append(titles, "Notbetrieb hält Mindestdauer")
	}
	rows := make([]MatrixRow, len(titles))
	for i, title := range titles {
		rows[i] = MatrixRow{Title: title, LPC: "open", LPP: "open"}
	}
	for _, kind := range []string{"LPC", "LPP"} {
		if !r.Config.Includes(kind) {
			for i := range rows {
				if kind == "LPC" {
					rows[i].LPC = "excluded"
				} else {
					rows[i].LPP = "excluded"
				}
			}
			continue
		}
		baseline := "Ausgangsmessung MPC"
		if kind == "LPP" {
			baseline = "LPP " + baseline
		}
		effect := []string{kind + " gemessene Wirkung"}
		if kind == "LPC" || r.Config.Scope == ScopeProduction || r.SchemaVersion >= 1 && r.Mode == ModeOnSite {
			effect = append([]string{baseline}, effect...)
		}
		recovery := []string{kind + " Wiederkehr"}
		if r.Mode == ModeOnSite && r.Config.Scope != "" {
			if r.Config.FullTest() {
				recovery = append(recovery, kind+" Wiederkehr gemessene Wirkung")
			} else {
				recovery = []string{kind + " Vorzeitige Freigabe", kind + " Vorzeitige Rückkehr gemessene Wirkung"}
			}
		}
		groups := [][]string{{kind + " Grenze + Quittung"}, effect, {kind + " Heartbeat-Ausfall", kind + " Failsafe gemessene Wirkung"}, recovery}
		if r.Config.FullTest() {
			groups = append(groups, []string{kind + " Wiederkehr"})
		}
		for i, names := range groups {
			missing, absent, failed := false, false, false
			for _, name := range names {
				seen := false
				for _, s := range r.Steps {
					if s.Name != name {
						continue
					}
					seen = true
					unchecked := s.Status == NotChecked
					if name == baseline {
						value := s.Watts
						if value == nil {
							if m := baseline034.FindStringSubmatch(s.Detail); m != nil {
								v, _ := strconv.ParseFloat(m[1], 64)
								value = &v
							}
						}
						if value != nil {
							power, limit := *value, r.Config.LPC
							if kind == "LPP" {
								power = -power
								limit = r.Config.LPP
							}
							if power <= limit+r.Config.Tolerance {
								unchecked = true
							}
						}
					}
					if unchecked {
						missing = true
					} else if !s.Passed {
						failed = true
					}
				}
				if !seen {
					missing = true
					absent = true
				}
			}
			status := "passed"
			if failed {
				status = "failed"
			} else if absent {
				status = "open"
			} else if missing {
				status = "not_checked"
			}
			if kind == "LPC" {
				rows[i].LPC = status
			} else {
				rows[i].LPP = status
			}
		}
	}
	return rows
}

var baseline034 = regexp.MustCompile(`^MPC (-?[0-9]+(?:\.[0-9]+)?) W(?:;.*)?$`)

type FunctionCounts struct{ Passed, Failed, NotChecked, Open int }

func (r Report) FunctionCounts() FunctionCounts {
	rank := map[string]int{"excluded": -1, "passed": 0, "not_checked": 1, "open": 2, "failed": 3}
	var c FunctionCounts
	for _, row := range r.FunctionMatrix() {
		status := row.LPC
		if rank[row.LPP] > rank[status] {
			status = row.LPP
		}
		switch status {
		case "passed":
			c.Passed++
		case "failed":
			c.Failed++
		case "not_checked":
			c.NotChecked++
		case "open":
			c.Open++
		}
	}
	return c
}

// Labels are for the German no-JavaScript fallback; browser dictionaries localize them.
func (r MatrixRow) LPCLabel() string { return functionLabel034(r.LPC) }
func (r MatrixRow) LPPLabel() string { return functionLabel034(r.LPP) }
func functionLabel034(status string) string {
	return map[string]string{"passed": "Bestanden", "failed": "Fehlgeschlagen", "not_checked": "Nicht geprüft", "open": "Ausstehend", "excluded": "Nicht im Prüfumfang"}[status]
}
