package webui

// 041 (Nutzer 2026-11-14): Export/Import aller Daten, „Simulator beenden“ und Berichtsversionen.

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/session"
)

func (a *App) state041() string { return filepath.Dir(a.reportDir) }

// exportData returns a ZIP with all reports, the browser profile (sent by the page) and the simulator
// identity. The archive is built in memory from local files only; nothing leaves the computer.
func (a *App) exportData(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	var in struct {
		Profile json.RawMessage `json:"profile"`
	}
	if !decode(w, r, &in) {
		return
	}
	// Report files are written atomically (rename); reading them needs no App.mu, so /api/state stays responsive.
	var out bytes.Buffer
	err := session.Export(&out, a.state041(), in.Profile, time.Now())
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="stromea-simulator-export-`+time.Now().Format("2006-01-02")+`.zip"`)
	_, _ = w.Write(out.Bytes())
}

// importData accepts the raw ZIP (Content-Type application/zip). The body is spooled to a private temporary
// file outside App.mu (040 F-03), validated completely and only then written.
func (a *App) importData(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	if r.Header.Get("Content-Type") != "application/zip" {
		http.Error(w, "ZIP erforderlich", 415)
		return
	}
	q := r.URL.Query()
	opt := session.ImportOptions{Reports: q.Get("reports") == "1", Profile: q.Get("profile") == "1", Identity: q.Get("identity") == "1"}
	if !opt.Reports && !opt.Profile && !opt.Identity {
		http.Error(w, "Nichts zum Importieren ausgewählt", 400)
		return
	}
	if a.blocked041() {
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	tmp, err := os.CreateTemp("", "stromea-import-*.zip")
	if err != nil {
		http.Error(w, "Import nicht möglich", 500)
		return
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, http.MaxBytesReader(w, r.Body, session.MaxImportBytes))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		http.Error(w, "Exportdatei zu groß oder unvollständig", 400)
		return
	}
	// Review R041-03: reserve the import under App.mu, validate and write outside it (state polling and
	// "Simulator beenden" stay responsive); every other change is refused while importing.
	a.mu.Lock()
	if a.blockedLocked() {
		a.mu.Unlock()
		http.Error(w, "Prüfung/Verbindung läuft", 409)
		return
	}
	// Review R041-10: the import belongs to the service lifecycle; shutdown and Close wait for it.
	done := make(chan struct{})
	a.importing, a.importDone = true, done
	a.wg.Add(1)
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.importing, a.importDone = false, nil
		a.mu.Unlock()
		close(done)
		a.wg.Done()
	}()
	res, err := session.Import(tmp.Name(), a.state041(), opt)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if res.IdentityPEM != nil {
		if err = a.s.ImportIdentity(res.IdentityPEM); err != nil {
			res.Rollback() // Review R041-01: no partial import
			http.Error(w, err.Error(), 400)
			return
		}
		a.connected = false
		a.kind = ""
		a.runError = ""
	}
	send(w, res)
}

func (a *App) blocked041() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.blockedLocked()
}

// blockedLocked: no new operation or data change while a run/connection/search, an import or the shutdown
// is in progress (Review R041-03/04). Called under App.mu.
func (a *App) blockedLocked() bool {
	return a.busy || a.search.Active || a.shuttingDown || a.importing
}

// shutdown ends only this simulator process through the hook of the local server owner (never a proxy or
// another process). A running inspection is aborted first; its own cleanup lifts the test limits and
// restores failsafe values before the hook runs.
func (a *App) shutdown(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "POST") {
		return
	}
	if r.ContentLength != 0 {
		var in struct{}
		if !decode(w, r, &in) {
			return
		}
	}
	if a.shutdownHook == nil {
		http.Error(w, "Beenden ist in dieser Betriebsart nicht verfügbar", http.StatusNotImplemented)
		return
	}
	a.mu.Lock()
	aborting := a.busy && a.kind == "run"
	cancel, done := a.runCancel, a.operationDone
	searchCancel, searchDone := a.searchCancel, a.searchDone
	importDone := a.importDone
	if cancel != nil {
		if a.kind == "run" {
			a.s.CancelRecovery()
			a.operation = "Abbruch angefordert; Grenzen werden aufgehoben"
		}
		cancel()
	}
	if searchCancel != nil {
		searchCancel()
	}
	first := !a.shuttingDown
	a.shuttingDown = true
	a.mu.Unlock()
	w.WriteHeader(http.StatusAccepted)
	send(w, map[string]bool{"shuttingDown": true, "aborting": aborting})
	if !first {
		return
	}
	hook := a.shutdownHook
	go func() {
		if cancel != nil && done != nil {
			<-done
		}
		if searchCancel != nil && searchDone != nil {
			<-searchDone
		}
		if importDone != nil {
			<-importDone
		}
		hook()
	}()
}

type versionRef struct {
	ID      string    `json:"id"`
	Number  int       `json:"number"`
	Created time.Time `json:"created"`
	Current bool      `json:"-"`
}

func seriesRoot(r inspection.Report) string {
	if r.Revision != nil && r.Revision.RootID != "" {
		return r.Revision.RootID
	}
	return r.ID
}

// versionsOf lists all stored versions of the series of report id, newest first. Called under App.mu.
func (a *App) versionsOf(id string) ([]versionRef, error) {
	target, err := session.LoadReport(a.reportDir, id)
	if err != nil {
		return nil, err
	}
	all, err := session.ListReports(a.reportDir)
	if err != nil {
		return nil, err
	}
	root := seriesRoot(target)
	out := []versionRef{}
	for _, r := range all {
		if seriesRoot(r) != root {
			continue
		}
		created := r.Started
		if r.Revision != nil {
			created = r.Revision.Created
		}
		out = append(out, versionRef{ID: r.ID, Number: r.VersionNumber(), Created: created, Current: r.ID == id})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Number > out[j].Number })
	return out, nil
}

func (a *App) reportVersions(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, "GET") {
		return
	}
	id := r.URL.Query().Get("id")
	a.mu.Lock()
	if id == "" && a.result != nil {
		id = a.result.ID
	}
	versions, err := a.versionsOf(id)
	a.mu.Unlock()
	if err != nil {
		http.Error(w, "Protokoll nicht gefunden", 404)
		return
	}
	send(w, versions)
}

// newerVersionExists: only the newest version of a series may be corrected (041). Called under App.mu.
func (a *App) newerVersionExists(v *inspection.Report) (bool, error) {
	if v.ID == "" {
		return false, nil
	}
	versions, err := a.versionsOf(v.ID)
	if errors.Is(err, os.ErrNotExist) || err != nil && strings.Contains(err.Error(), "nicht lesbar") {
		return false, nil // unsaved live report
	}
	if err != nil {
		return false, err
	}
	return len(versions) > 0 && versions[0].Number > v.VersionNumber(), nil
}
