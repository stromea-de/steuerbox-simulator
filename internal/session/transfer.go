package session

// 041 (Nutzer 2026-11-14, ADR 064): Export und Import aller lokalen Daten als ZIP-Datei.
//
// Aufbau (plattformunabhängig, nur „/“ als Trenner, ASCII-Namen, Zeiten in UTC):
//
//	manifest.json              Format, Version, Erstellzeit, SHA-256 jeder weiteren Datei
//	reports/<Protokoll-ID>.json  gespeicherte Berichte, byteidentisch (Prüfsummen im PDF bleiben gültig)
//	profile.json               Profil inkl. Logo (liegt im Browser; vom Client mitgeschickt)
//	identity/energyguard.pem   Simulator-Kennung: Zertifikat und privater Schlüssel (vertraulich)
//
// Der Import ist fail-closed: Erst wird das ganze Archiv geprüft (Namen, Größen, Prüfsummen, Inhalte),
// erst danach wird geschrieben. Inhalte werden nie ausgeführt, nur als JSON/PEM gelesen.

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
	"github.com/stromea-de/steuerbox-simulator/internal/storage"
)

const (
	ExportFormat  = "stromea-steuerbox-simulator-export"
	ExportVersion = 1
	// MaxImportBytes bounds the uploaded archive; reports are a few hundred KB each.
	MaxImportBytes   = 1 << 30
	maxImportEntries = 20000
	maxImportTotal   = 2 << 30
	maxManifestBytes = 4 << 20
	maxProfileBytes  = 2 << 20
	maxIdentityBytes = 64 << 10
	identityEntry    = "identity/energyguard.pem"
	profileEntry     = "profile.json"
	manifestEntry    = "manifest.json"
)

type ImportOptions struct{ Reports, Profile, Identity bool }

// ImportResult: Imported new reports, Skipped reports already present (same ID), Profile/IdentityPEM only
// when selected and contained.
type ImportResult struct {
	Imported    int `json:"imported"`
	Skipped     int `json:"skipped"`
	Conflicts   int `json:"conflicts"` // Review R041-07: other version with the same number already present
	written     []string
	Profile     json.RawMessage `json:"profile,omitempty"`
	IdentityPEM []byte          `json:"-"`
	Identity    bool            `json:"identity"`
}

// Rollback removes the reports written by this import (Review R041-01: a later failing step, e.g. the
// identity, must not leave a partial import behind).
func (r *ImportResult) Rollback() {
	for _, p := range r.written {
		_ = os.Remove(p)
	}
	r.written, r.Imported = nil, 0
}

// writeImported is replaceable in tests to provoke write failures.
var writeImported = storage.WriteFile

// supportedSchema is the newest report schema this version can evaluate (0 = legacy without the field).
const supportedSchema = 1

// validImportedReport adds the import rules to parseReport (Review R041-02): known schema and mode, a start
// time, a recorded ID equal to the file name and a consistent revision.
func validImportedReport(id string, data []byte) (inspection.Report, error) {
	r, e := parseReport(id, data)
	if e != nil {
		return r, e
	}
	var recorded struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &recorded) != nil || recorded.ID != "" && recorded.ID != id {
		return r, errors.New("ID passt nicht zum Dateinamen")
	}
	if r.SchemaVersion < 0 || r.SchemaVersion > supportedSchema {
		return r, errors.New("Berichtsformat aus einer neueren Simulator-Version")
	}
	// Review R041-09: required fields of the supported formats, present and of the right JSON type. Every saved
	// report contains them (json.Marshal of inspection.Report); legacy reports (schema 0) lack id/mode/finished.
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return r, errors.New("Berichtsformat ungültig")
	}
	kind := func(name string) byte {
		v := bytes.TrimSpace(fields[name])
		if len(v) == 0 || string(v) == "null" {
			return 0
		}
		return v[0]
	}
	required := map[string]byte{"started": '"', "config": '{', "device": '{', "steps": '[', "business": '"', "inspector": '"'}
	if r.SchemaVersion >= 1 {
		for k, v := range map[string]byte{"id": '"', "mode": '"', "finished": '"'} {
			required[k] = v
		}
	}
	for name, want := range required {
		if kind(name) != want {
			return r, fmt.Errorf("Pflichtfeld %s fehlt", name)
		}
	}
	if p := string(bytes.TrimSpace(fields["passed"])); p != "true" && p != "false" {
		return r, errors.New("Pflichtfeld passed fehlt")
	}
	if r.SchemaVersion >= 1 && (recorded.ID != id || r.Finished.IsZero() || r.Mode != inspection.ModeSimulator && r.Mode != inspection.ModeOnSite) {
		return r, errors.New("Pflichtfeld id, mode oder finished ungültig")
	}
	if r.Started.IsZero() || r.Started.Year() < 2000 {
		return r, errors.New("Prüfbeginn fehlt")
	}
	if r.Mode != "" && r.Mode != inspection.ModeSimulator && r.Mode != inspection.ModeOnSite {
		return r, errors.New("unbekannter Prüfmodus")
	}
	if v := r.Revision; v != nil {
		if v.Number < 2 || v.RootID == id || v.PreviousID == id || v.Created.IsZero() || v.PreviousDate.IsZero() {
			return r, errors.New("Berichtsversion widersprüchlich")
		}
		for _, ref := range []string{v.RootID, v.PreviousID} {
			if _, e := reportPath("x", ref); e != nil {
				return r, errors.New("Berichtsversion widersprüchlich")
			}
		}
	}
	return r, nil
}

func seriesRoot(r inspection.Report) string {
	if r.Revision != nil && r.Revision.RootID != "" {
		return r.Revision.RootID
	}
	return r.ID
}

func versionKey(r inspection.Report) string {
	if r.Revision != nil && r.Revision.RootID != "" {
		return r.Revision.RootID + "\x00" + strconv.Itoa(r.Revision.Number)
	}
	return r.ID + "\x00" + "1"
}

type exportManifest struct {
	Format   string            `json:"format"`
	Version  int               `json:"version"`
	Created  string            `json:"created"`
	Software string            `json:"software,omitempty"`
	Files    map[string]string `json:"files"`
}

func sha256Hex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

var reportEntry = regexp.MustCompile(`^reports/([0-9A-Za-z-]{1,20})\.json$`)

// readReportFile returns the exact stored bytes of one valid report (same checks as LoadReport).
func readReportFile(dir, id string) ([]byte, error) {
	path, e := reportPath(dir, id)
	if e != nil {
		return nil, e
	}
	f, e := openRegular(path)
	if e != nil {
		return nil, errors.New("Protokoll nicht lesbar")
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, maxReportBytes+1))
	if e != nil || len(data) > maxReportBytes {
		return nil, errors.New("Protokoll nicht lesbar")
	}
	if _, e = parseReport(id, data); e != nil {
		return nil, e
	}
	return data, nil
}

// Export writes the ZIP. profile is the browser profile as JSON object (nil: none).
func Export(w io.Writer, state string, profile json.RawMessage, now time.Time) error {
	files := map[string][]byte{}
	reports, _, e := ScanReports(filepath.Join(state, "reports"))
	if e != nil {
		return e
	}
	for _, r := range reports {
		data, e := readReportFile(filepath.Join(state, "reports"), r.ID)
		if e != nil {
			continue // damaged files are skipped like in the list (040 F-02)
		}
		files["reports/"+r.ID+".json"] = data
	}
	if len(profile) > 0 && string(profile) != "null" {
		if _, e := validProfile(profile); e != nil {
			return e
		}
		files[profileEntry] = profile
	}
	if pem, e := os.ReadFile(filepath.Join(state, "energyguard", "identity.pem")); e == nil {
		files[identityEntry] = pem
	} else if !errors.Is(e, os.ErrNotExist) {
		return errors.New("Simulator-Kennung nicht lesbar")
	}
	m := exportManifest{Format: ExportFormat, Version: ExportVersion, Created: now.UTC().Format(time.RFC3339), Software: report.BuildVersion(), Files: map[string]string{}}
	names := make([]string, 0, len(files))
	for name, data := range files {
		m.Files[name] = sha256Hex(data)
		names = append(names, name)
	}
	sort.Strings(names)
	manifest, e := json.MarshalIndent(m, "", " ")
	if e != nil {
		return e
	}
	z := zip.NewWriter(w)
	modified := now.UTC().Truncate(time.Second)
	for _, name := range append([]string{manifestEntry}, names...) {
		data := manifest
		if name != manifestEntry {
			data = files[name]
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: modified}
		h.SetMode(0600)
		f, e := z.CreateHeader(h)
		if e != nil {
			return e
		}
		if _, e = f.Write(data); e != nil {
			return e
		}
	}
	return z.Close()
}

// safeEntryName accepts only the fixed layout; everything else (zip slip, absolute paths, backslashes,
// drive letters, unknown files) rejects the whole archive.
func safeEntryName(name string) bool {
	if name == "" || !utf8.ValidString(name) || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." || part == "." || part == "" {
			return false
		}
	}
	return name == manifestEntry || name == profileEntry || name == identityEntry || reportEntry.MatchString(name)
}

func readEntry(f *zip.File, limit int64, total *int64) ([]byte, error) {
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("Datei %s zu groß", f.Name)
	}
	rc, e := f.Open()
	if e != nil {
		return nil, fmt.Errorf("Datei %s nicht lesbar", f.Name)
	}
	defer rc.Close()
	data, e := io.ReadAll(io.LimitReader(rc, limit+1)) // never trust the header size
	if e != nil {
		return nil, fmt.Errorf("Datei %s nicht lesbar", f.Name)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("Datei %s zu groß", f.Name)
	}
	*total += int64(len(data))
	if *total > maxImportTotal {
		return nil, errors.New("Exportdatei zu groß")
	}
	return data, nil
}

type exportProfile struct {
	Business   string `json:"business,omitempty"`
	Street     string `json:"street,omitempty"`
	PostalCode string `json:"postalCode,omitempty"`
	City       string `json:"city,omitempty"`
	Country    string `json:"country,omitempty"`
	Inspector  string `json:"inspector,omitempty"`
	Logo       string `json:"logo,omitempty"`
}

var logoPattern = regexp.MustCompile(`^data:image/(png|jpeg);base64,[A-Za-z0-9+/]+={0,2}$`)

// validProfile mirrors the browser rules (profile.js): known text fields up to 200 characters, logo only as
// PNG/JPEG data URL up to 1,000,000 characters. Unknown fields are rejected.
func validProfile(data []byte) (json.RawMessage, error) {
	var p exportProfile
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(&p); e != nil {
		return nil, errors.New("Profil in der Exportdatei ungültig")
	}
	for _, v := range []string{p.Business, p.Street, p.PostalCode, p.City, p.Country, p.Inspector} {
		if utf8.RuneCountInString(v) > 200 {
			return nil, errors.New("Profil in der Exportdatei ungültig")
		}
	}
	if p.Logo != "" && (len(p.Logo) > 1000000 || !logoPattern.MatchString(p.Logo)) {
		return nil, errors.New("Logo in der Exportdatei ungültig")
	}
	out, e := json.Marshal(p)
	return out, e
}

// Import validates the archive completely before it writes anything.
func Import(zipPath, state string, opt ImportOptions) (ImportResult, error) {
	var res ImportResult
	if !opt.Reports && !opt.Profile && !opt.Identity {
		return res, errors.New("Nichts zum Importieren ausgewählt")
	}
	z, e := zip.OpenReader(zipPath)
	if e != nil {
		return res, errors.New("Keine gültige Exportdatei (ZIP)")
	}
	defer z.Close()
	if len(z.File) > maxImportEntries {
		return res, errors.New("Exportdatei enthält zu viele Dateien")
	}
	seen := map[string]*zip.File{}
	for _, f := range z.File {
		if f.FileInfo().IsDir() && (f.Name == "reports/" || f.Name == "identity/") {
			continue
		}
		if !safeEntryName(f.Name) || !f.Mode().IsRegular() || seen[f.Name] != nil {
			return res, fmt.Errorf("Unzulässiger Eintrag in der Exportdatei: %q", f.Name)
		}
		seen[f.Name] = f
	}
	var total int64
	mf := seen[manifestEntry]
	if mf == nil {
		return res, errors.New("Keine Exportdatei des Simulators (manifest.json fehlt)")
	}
	raw, e := readEntry(mf, maxManifestBytes, &total)
	if e != nil {
		return res, e
	}
	var m exportManifest
	if json.Unmarshal(raw, &m) != nil || m.Format != ExportFormat {
		return res, errors.New("Keine Exportdatei des Simulators")
	}
	if m.Version > ExportVersion {
		return res, errors.New("Exportdatei aus einer neueren Simulator-Version. Bitte den Simulator aktualisieren.")
	}
	if m.Version != ExportVersion {
		return res, errors.New("Version der Exportdatei nicht unterstützt")
	}
	for name := range seen {
		if name != manifestEntry && m.Files[name] == "" {
			return res, fmt.Errorf("Datei %s fehlt im Inhaltsverzeichnis", name)
		}
	}
	for name := range m.Files {
		if seen[name] == nil {
			return res, fmt.Errorf("Datei %s fehlt in der Exportdatei", name)
		}
	}
	verified := func(name string, limit int64) ([]byte, error) {
		data, e := readEntry(seen[name], limit, &total)
		if e != nil {
			return nil, e
		}
		if sha256Hex(data) != strings.ToLower(m.Files[name]) {
			return nil, fmt.Errorf("Prüfsumme von %s stimmt nicht", name)
		}
		return data, nil
	}
	// Pass 1: verify every entry (also unselected ones: a manipulated archive is rejected as a whole).
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	type pendingReport struct {
		id, key, previous, root string
		number                  int
		data                    []byte
	}
	var reports []pendingReport
	for _, name := range names {
		switch {
		case name == manifestEntry:
		case name == profileEntry:
			data, e := verified(name, maxProfileBytes)
			if e != nil {
				return res, e
			}
			p, e := validProfile(data)
			if e != nil {
				return res, e
			}
			if opt.Profile {
				res.Profile = p
			}
		case name == identityEntry:
			data, e := verified(name, maxIdentityBytes)
			if e != nil {
				return res, e
			}
			if _, e = identity.Parse(data); e != nil {
				return res, errors.New("Simulator-Kennung in der Exportdatei ungültig")
			}
			if opt.Identity {
				res.IdentityPEM, res.Identity = data, true
			}
		default:
			id := reportEntry.FindStringSubmatch(name)[1]
			if _, e := reportPath(filepath.Join(state, "reports"), id); e != nil {
				return res, fmt.Errorf("Unzulässige Protokoll-ID %q", id)
			}
			data, e := verified(name, maxReportBytes)
			if e != nil {
				return res, e
			}
			parsed, e := validImportedReport(id, data)
			if e != nil {
				return res, fmt.Errorf("Protokoll %s nicht importierbar: %v", id, e)
			}
			if opt.Reports {
				p := pendingReport{id: id, data: data, key: versionKey(parsed), number: parsed.VersionNumber(), root: seriesRoot(parsed)}
				if parsed.Revision != nil {
					p.previous = parsed.Revision.PreviousID
				}
				reports = append(reports, p)
			}
		}
	}
	// Pass 2: write. Existing reports with the same ID are kept and counted as skipped; another report with the
	// same series and version number is a conflict (two installations corrected the same report) and is skipped.
	// Any write failure removes everything this import wrote.
	dir := filepath.Join(state, "reports")
	existing, e := ListReports(dir)
	if e != nil && opt.Reports && len(reports) > 0 {
		return res, e
	}
	type link struct {
		root   string
		number int
	}
	versions, known := map[string]string{}, map[string]link{}
	for _, r := range existing {
		versions[versionKey(r)] = r.ID
		known[r.ID] = link{seriesRoot(r), r.VersionNumber()}
	}
	// Review R041-11: resolve version chains in order (lowest version first); a version is only taken over if
	// its predecessor is present afterwards. A conflicting version rejects its successors, too.
	sort.SliceStable(reports, func(i, j int) bool { return reports[i].number < reports[j].number })
	for _, r := range reports {
		// Review R041-13: the predecessor belongs to the same series and carries the previous number.
		if r.previous != "" {
			prev, ok := known[r.previous]
			if !ok || prev.root != r.root || prev.number != r.number-1 {
				res.Conflicts++
				continue
			}
		}
		path, _ := reportPath(dir, r.id)
		if _, e := os.Lstat(path); e == nil {
			res.Skipped++
			continue
		}
		if other, ok := versions[r.key]; ok && other != r.id {
			res.Conflicts++
			continue
		}
		if e := os.MkdirAll(dir, 0700); e != nil {
			res.Rollback()
			return res, errors.New("Protokollordner nicht beschreibbar")
		}
		if e := writeImported(path, r.data); e != nil {
			res.Rollback()
			return res, errors.New("Protokoll konnte nicht gespeichert werden")
		}
		res.written = append(res.written, path)
		versions[r.key] = r.id
		known[r.id] = link{r.root, r.number}
		res.Imported++
	}
	return res, nil
}
