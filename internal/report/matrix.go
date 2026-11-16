package report

import (
	"fmt"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
)

func (d *document) resultMatrix(r inspection.Report) {
	r = renderReport(r)
	for _, kind := range []string{"LPC", "LPP"} {
		if !r.Config.Includes(kind) {
			continue
		}
		direction, limit := "Bezug", r.Config.LPC
		if kind == "LPP" {
			direction, limit = "Einspeisung", r.Config.LPP
		}
		header := func() {
			d.fill(margin, d.y, bodyWidth, 22, "#F4F4F6")
			d.text(margin+5, d.y+5, 9, "F2", ink, direction+" ("+kind+") · Grenze "+watts(limit))
			d.y += 22
			if kind == "LPC" || !r.Config.Includes("LPC") {
				for i, label := range []string{"Prüfpunkt", "Bestanden", "Fehler", "Nicht geprüft", "Bemerkung"} {
					d.text(margin+[]float64{4, 169, 229, 275, 349}[i], d.y+5, 7, "F2", muted, label)
				}
				d.y += 23
				d.line(margin, d.y, pageWidth-margin, d.y, .9, ink)
			}
		}
		checks := formChecks(r, kind)
		d.reserve(45 + float64(len(checks))*31)
		header()
		for _, c := range checks {
			title := wrapText(c.text, 155, 8.5, "F1")
			note := wrapText(formRemark(r, c), bodyWidth-355, 8, "F1")
			h := max(24, float64(max(len(title), len(note)))*10.5+8)
			if d.y+h > bodyBottom {
				d.newPage()
				header()
			}
			d.lines(margin+4, d.y+7, 8.5, 10.5, "F1", ink, title)
			status := checkpointStatus(r, c)
			for i, value := range []string{"bestanden", "nicht bestanden", "nicht geprüft"} {
				x := margin + 185 + float64(i)*55
				d.frame(x, d.y+8, 12, 12, rule)
				if status == value {
					color := muted
					if i == 0 {
						color = green
					}
					if i == 1 {
						color = red
					}
					mark := "nicht bestanden"
					if i == 0 {
						mark = "bestanden"
					}
					d.formMark(x, d.y+8, mark, color)
				}
			}
			d.lines(margin+349, d.y+7, 8, 10.5, "F1", muted, note)
			d.y += h
			d.line(margin, d.y, pageWidth-margin, d.y, .35, rule)
		}
		d.y += 12
	}
	d.reserve(30)
	lines := wrapText(r.ReleaseNote(), bodyWidth, 9, "F1")
	d.lines(margin, d.y, 9, 12, "F1", ink, lines)
	d.y += float64(len(lines))*12 + 12
}
func (d *document) evidenceGroups(r inspection.Report) {
	groups := r.EvidenceGroups()
	if connectionLost(r) {
		missing := []inspection.Step{}
		for _, kind := range []string{"LPC", "LPP"} {
			if !r.Config.Includes(kind) {
				continue
			}
			for _, c := range formChecks(r, kind) {
				if checkpointStatus(r, c) != "bestanden" {
					missing = append(missing, inspection.Step{Name: c.text})
				}
			}
		}
		if len(missing) > 0 {
			groups = append([]inspection.EvidenceGroup{{Reason: "Verbindungsabbruch", Steps: missing}}, groups...)
		}
	}
	if len(groups) == 0 {
		return
	}
	d.reserve(50)
	d.heading("Offene Nachweise · nach Grund")
	for _, g := range groups {
		reason := strings.TrimSuffix(g.Reason, ".")
		// The baseline and the later interruption are distinct evidence gaps.
		if connectionLost(r) && g.Reason == "Dieser Nachweis fehlt. Prüfschritt öffnen." {
			reason = "Verbindungsabbruch"
		}
		noun := "Nachweise"
		if len(g.Steps) == 1 {
			noun = "Nachweis"
		}
		lines := wrapText(fmt.Sprintf("%s · %d %s", reason, len(g.Steps), noun), bodyWidth, 9, "F2")
		d.reserve(float64(len(lines))*12 + 20)
		d.lines(margin, d.y, 9, 12, "F2", ink, lines)
		d.y += float64(len(lines))*12 + 12
	}
}
