package webui

import (
	"bytes"
	"errors"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

// Called under App.mu. An explicit ID can always open an earlier saved report.
func (a *App) selectedReport(r *http.Request) (*inspection.Report, error) {
	if id := r.URL.Query().Get("id"); id != "" {
		v, e := session.LoadReport(a.reportDir, id)
		return &v, e
	}
	if a.result == nil {
		return nil, errors.New("Noch kein Protokoll")
	}
	return a.result, nil
}
func (a *App) reports(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	reports, skipped, e := session.ScanReports(a.reportDir)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	entries := summarizeReports(reports)
	w.Header().Set("X-Stromea-Skipped-Reports", strconv.Itoa(skipped))

	send(w, entries)
}
func (a *App) updateSite(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in inspection.Site
	if !decode(w, r, &in) {
		return
	}
	if e := in.Validate(); e != nil {
		http.Error(w, e.Error(), 400)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blockedLocked() { // 040 F-13: same rule as pair
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	v, e := a.selectedReport(r)
	if e != nil {
		http.Error(w, e.Error(), 404)
		return
	}
	// 041: once a newer version exists, an older one can no longer be corrected.
	if newer, e := a.newerVersionExists(v); e != nil || newer {
		http.Error(w, "Es gibt eine neuere Berichtsversion. Korrigiere die neueste Version.", 409)
		return
	}
	copy := *v
	id, e := inspection.NewReportID()
	if e != nil {
		http.Error(w, "Berichtsversion konnte nicht erstellt werden", 500)
		return
	}
	rootID, previousDate := v.ID, v.Started
	if v.Revision != nil {
		rootID, previousDate = v.Revision.RootID, v.Revision.Created
	}
	predecessor := *v
	stored, e := session.ListReports(a.reportDir)
	if e != nil {
		http.Error(w, "Berichtsreihe konnte nicht gelesen werden", 500)
		return
	}
	for _, candidate := range stored {
		if candidate.Revision != nil && candidate.Revision.RootID == rootID && candidate.VersionNumber() > predecessor.VersionNumber() {
			predecessor = candidate
			previousDate = candidate.Revision.Created
		}
	}
	copy.ID = id
	copy.LegacyProtocolID = ""
	copy.Revision = &inspection.ReportRevision{Number: predecessor.VersionNumber() + 1, RootID: rootID, PreviousID: predecessor.ID, PreviousDate: previousDate, Created: time.Now()}
	copy.Config.Site = in
	if e = session.SaveReport(a.reportDir, &copy); e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	if a.result != nil && (a.result.ID == v.ID || a.result.ID == predecessor.ID || r.URL.Query().Get("id") == "") {
		a.result = &copy
	}
	send(w, copy)
}

// Summary metadata must retain the stored mode; never infer it from result or name.
type reportListEntry struct {
	ID       string            `json:"id"`
	Report   inspection.Report `json:"report"`
	Outcome  string            `json:"outcome"`
	Versions int               `json:"versions"`
}

// MeterLines wraps long identifiers once without changing their characters.
func (e reportListEntry) MeterLines() []string {
	value := e.Report.Config.Site.Meter
	if value == "" {
		return []string{"–"}
	}
	chars := []rune(value)
	if len(chars) <= 28 {
		return []string{value}
	}
	at := (len(chars) + 1) / 2
	return []string{string(chars[:at]), string(chars[at:])}
}

func (e reportListEntry) StatusLabel() string {
	if e.Report.Mode == inspection.ModeSimulator {
		return e.Outcome + " (Übung)"
	}
	return e.Outcome
}

// summarizeReports shows one row per inspection (041): the newest version of each series, in the order of
// the given list (newest inspection first).
func summarizeReports(reports []inspection.Report) []reportListEntry {
	latest, count := map[string]inspection.Report{}, map[string]int{}
	for _, v := range reports {
		root := seriesRoot(v)
		count[root]++
		if old, ok := latest[root]; !ok || v.VersionNumber() > old.VersionNumber() {
			latest[root] = v
		}
	}
	entries := []reportListEntry{}
	seen := map[string]bool{}
	for _, r := range reports {
		root := seriesRoot(r)
		if seen[root] {
			continue
		}
		seen[root] = true
		v := latest[root]
		entries = append(entries, reportListEntry{ID: v.ID, Report: inspection.Report{Revision: v.Revision, Mode: v.Mode, Started: v.Started, Config: inspection.Config{Site: v.Config.Site}, Device: v.Device}, Outcome: v.Outcome(), Versions: count[root]})
	}
	return entries
}
func (a *App) reportListFallback() ([]byte, error) {
	a.mu.Lock()
	reports, err := session.ListReports(a.reportDir)
	a.mu.Unlock()
	b, e := assets.ReadFile("assets/report-list.html")
	if e != nil {
		return nil, e
	}
	t, e := template.New("report-list").Parse(string(b))
	if e != nil {
		return nil, e
	}
	var output bytes.Buffer
	e = t.Execute(&output, struct {
		Entries []reportListEntry
		Error   bool
	}{summarizeReports(reports), err != nil})
	return output.Bytes(), e
}

// deleteReport removes one saved report with all its versions after an explicit confirmation in the UI (039, C-01).
func (a *App) deleteReport(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct {
		ID string `json:"id"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.blockedLocked() { // 040 F-13: same rule as pair
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	removed, e := session.DeleteReportSeries(a.reportDir, in.ID)
	if e != nil {
		http.Error(w, e.Error(), 404)
		return
	}
	if a.result != nil {
		for _, id := range removed {
			if a.result.ID == id {
				a.result = nil
				break
			}
		}
	}
	send(w, map[string][]string{"removed": removed})
}
