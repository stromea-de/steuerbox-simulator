package session

import (
	"encoding/json"
	"errors"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"github.com/stromea-de/steuerbox-simulator/internal/report"
	"github.com/stromea-de/steuerbox-simulator/internal/storage"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// maxReportBytes bounds one saved report file (reports are a few hundred KB).
const maxReportBytes = 64 << 20

func (s *Session) ReportDir() string { return filepath.Join(s.state, "reports") }
func reportPath(dir, id string) (string, error) {
	if dir == "" || len(id) < 1 || len(id) > 20 || (strings.Trim(id, "0123456789") != "" && !inspection.ValidReportID(id) && !(len(id) == 12 && strings.Trim(id, "0123456789abcdef") == "")) {
		return "", errors.New("ungültige Protokoll-ID")
	}
	return filepath.Join(dir, id+".json"), nil
}
func SaveReport(dir string, r *inspection.Report) error {
	if r.ID == "" {
		r.ID = strconv.FormatInt(r.Started.UnixNano(), 10)
	}
	if r.SchemaVersion == 0 && r.LegacyProtocolID == "" {
		r.LegacyProtocolID = report.ProtocolIdentity(*r)
	}
	path, e := reportPath(dir, r.ID)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return errors.New("Protokollordner nicht beschreibbar")
	}
	data, e := json.Marshal(r)
	if e != nil {
		return errors.New("Protokoll nicht speicherbar")
	}
	if e = storage.WriteFile(path, data); e != nil {
		return errors.New("Protokoll konnte nicht lokal gespeichert werden")
	}
	return nil
}
func LoadReport(dir, id string) (inspection.Report, error) {
	var r inspection.Report
	path, e := reportPath(dir, id)
	if e != nil {
		return r, e
	}
	// R040-01/05: only regular files; a FIFO, device or symlink named like a report must never block or
	// redirect. The type is checked on the opened file itself.
	f, e := openRegular(path)
	if e != nil {
		return r, errors.New("Protokoll nicht lesbar")
	}
	// R040-08: read one byte beyond the limit; larger files are rejected, never truncated.
	data, e := io.ReadAll(io.LimitReader(f, maxReportBytes+1))
	f.Close()
	if e != nil || len(data) > maxReportBytes {
		return r, errors.New("Protokoll nicht lesbar")
	}
	return parseReport(id, data)
}

// parseReport applies the report file rules to bytes (also used by the import, 041).
func parseReport(id string, data []byte) (inspection.Report, error) {
	var r inspection.Report
	// R040-02: foreign JSON (null, {}, notes) is no report; every saved report, also the legacy format,
	// carries the field "started".
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields["started"] == nil || string(fields["started"]) == "null" || json.Unmarshal(data, &r) != nil {
		return r, errors.New("Protokolldatei beschädigt")
	}
	r.ID = id
	if r.SchemaVersion == 0 && r.LegacyProtocolID == "" {
		r.LegacyProtocolID = report.ProtocolIdentity(r)
	}
	return r, nil
}
func ListReports(dir string) ([]inspection.Report, error) {
	reports, _, e := ScanReports(dir)
	return reports, e
}

// ScanReports lists all readable reports. Foreign, misnamed or damaged *.json files are skipped and
// only counted (040 F-02); they must never block the list, deletion or correction of other reports.
func ScanReports(dir string) ([]inspection.Report, int, error) {
	result := []inspection.Report{}
	if dir == "" {
		return result, 0, nil
	}
	entries, e := os.ReadDir(dir)
	if errors.Is(e, os.ErrNotExist) {
		return result, 0, nil
	}
	if e != nil {
		return nil, 0, errors.New("Protokollordner nicht lesbar")
	}
	skipped := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !entry.Type().IsRegular() {
			skipped++
			continue
		}
		r, e := LoadReport(dir, strings.TrimSuffix(entry.Name(), ".json"))
		if e != nil {
			skipped++
			continue
		}
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Started.After(result[j].Started) })
	return result, skipped, nil
}

// DeleteReportSeries removes a saved report together with all its corrected versions (039, C-01).
// It returns the removed IDs. Other reports stay untouched.
func DeleteReportSeries(dir, id string) ([]string, error) {
	target, e := LoadReport(dir, id)
	if e != nil {
		return nil, e
	}
	root := target.ID
	if target.Revision != nil && target.Revision.RootID != "" {
		root = target.Revision.RootID
	}
	all, e := ListReports(dir)
	if e != nil {
		return nil, e
	}
	removed := []string{}
	for _, r := range all {
		if r.ID != root && (r.Revision == nil || r.Revision.RootID != root) {
			continue
		}
		path, e := reportPath(dir, r.ID)
		if e != nil {
			return removed, e
		}
		if e := os.Remove(path); e != nil && !errors.Is(e, os.ErrNotExist) {
			return removed, errors.New("Protokoll konnte nicht gelöscht werden")
		}
		removed = append(removed, r.ID)
	}
	return removed, nil
}
