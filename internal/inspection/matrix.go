package inspection

import "strings"

// MatrixRow folds repeated steps using the worst recorded status. A missing cell
// is open; an unselected direction is excluded. Neither implies a passed test.
type MatrixRow struct{ Key, Title, LPC, LPP string }

func (r Report) ResultMatrix() []MatrixRow {
	var rows []MatrixRow
	rank := map[string]int{"passed": 0, "not_checked": 1, "open": 2, "failed": 3}
	seen := map[string]bool{}
	for _, s := range r.Steps {
		kind, key := "", s.Name
		if key == "Ausgangsmessung MPC" {
			kind = "LPC"
		}
		for _, k := range []string{"LPC", "LPP"} {
			if strings.HasPrefix(key, k+" ") {
				kind = k
				key = strings.TrimPrefix(key, k+" ")
				break
			}
		}
		at := -1
		for i := range rows {
			if rows[i].Key == key {
				at = i
				break
			}
		}
		if at < 0 {
			title := StepName(s.Name)
			title = strings.TrimPrefix(strings.TrimPrefix(title, "Bezug: "), "Einspeisung: ")
			row := MatrixRow{Key: key, Title: title, LPC: "open", LPP: "open"}
			if !r.Config.Includes("LPC") {
				row.LPC = "excluded"
			}
			if !r.Config.Includes("LPP") {
				row.LPP = "excluded"
			}
			rows = append(rows, row)
			at = len(rows) - 1
		}
		status := "failed"
		if s.Status == NotChecked {
			status = "not_checked"
		} else if s.Passed {
			status = "passed"
		}
		for _, k := range []string{"LPC", "LPP"} {
			if kind != "" && kind != k {
				continue
			}
			cell := &rows[at].LPC
			if k == "LPP" {
				cell = &rows[at].LPP
			}
			if *cell == "excluded" {
				continue
			}
			id := key + "/" + k
			if !seen[id] || rank[status] > rank[*cell] {
				*cell = status
			}
			seen[id] = true
		}
	}
	return rows
}

type EvidenceGroup struct {
	Reason string
	Steps  []Step
}

func (r Report) EvidenceGroups() []EvidenceGroup {
	var out []EvidenceGroup
	for _, s := range r.OpenEvidence() {
		reason := s.Reason()
		at := -1
		for i := range out {
			if out[i].Reason == reason {
				at = i
				break
			}
		}
		if at < 0 {
			out = append(out, EvidenceGroup{Reason: reason})
			at = len(out) - 1
		}
		out[at].Steps = append(out[at].Steps, s)
	}
	return out
}
