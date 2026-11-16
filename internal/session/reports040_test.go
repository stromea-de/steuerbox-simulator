package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// 040 F-02: one foreign or damaged *.json in the report folder must not block list, delete or correction.
func TestForeignFilesDoNotBlockReports040(t *testing.T) {
	dir := t.TempDir()
	a := inspection.Report{Started: time.Unix(100, 0)}
	b := inspection.Report{Started: time.Unix(200, 0)}
	for _, r := range []*inspection.Report{&a, &b} {
		if err := SaveReport(dir, r); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string]string{"notizen.json": `{"x":1}`, "123.json": "{not json", "junk.json": "{not json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	reports, skipped, err := ScanReports(dir)
	if err != nil || len(reports) != 2 || skipped != 3 {
		t.Fatalf("scan: %d reports, %d skipped, %v", len(reports), skipped, err)
	}
	if list, err := ListReports(dir); err != nil || len(list) != 2 {
		t.Fatalf("list: %d %v", len(list), err)
	}
	removed, err := DeleteReportSeries(dir, a.ID)
	if err != nil || len(removed) != 1 || removed[0] != a.ID {
		t.Fatalf("delete: %v %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notizen.json")); err != nil {
		t.Fatal("foreign file must stay untouched")
	}
}

// R040-08: a file above the size limit is rejected even if its beginning is a complete report.
func TestOversizedReportRejected040(t *testing.T) {
	dir := t.TempDir()
	head := []byte(`{"started":"2026-01-01T00:00:00Z"}`)
	data := append(head, make([]byte, maxReportBytes)...)
	for i := len(head); i < len(data); i++ {
		data[i] = ' '
	}
	data = append(data, []byte("garbage")...)
	if err := os.WriteFile(filepath.Join(dir, "123.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReport(dir, "123"); err == nil {
		t.Fatal("oversized report accepted")
	}
	exact := append(append([]byte(nil), head...), make([]byte, maxReportBytes-len(head))...)
	for i := len(head); i < len(exact); i++ {
		exact[i] = ' '
	}
	if err := os.WriteFile(filepath.Join(dir, "124.json"), exact, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadReport(dir, "124"); err != nil {
		t.Fatalf("report at the limit rejected: %v", err)
	}
}
