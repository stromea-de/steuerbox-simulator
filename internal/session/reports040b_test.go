//go:build !windows

package session

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// R040-01/R040-02: a FIFO or symlink named like a report never blocks the scan, and foreign JSON with a valid
// report name (null, {}, notes) is skipped instead of being listed as an empty report.
func TestSpecialAndForeignReportFiles040(t *testing.T) {
	dir := t.TempDir()
	good := inspection.Report{Started: time.Unix(100, 0)}
	if err := SaveReport(dir, &good); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "123.json"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, good.ID+".json"), filepath.Join(dir, "456.json")); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"777.json": "null", "778.json": "{}", "779.json": `{"notiz":"privat"}`, "780.json": `{"started":null}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan struct{})
	var reports []inspection.Report
	var skipped int
	go func() { reports, skipped, _ = ScanReports(dir); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("scan blocked on a FIFO")
	}
	if len(reports) != 1 || reports[0].ID != good.ID || skipped != 6 {
		t.Fatalf("reports %d skipped %d", len(reports), skipped)
	}
	if _, err := LoadReport(dir, "123"); err == nil {
		t.Fatal("FIFO loaded as report")
	}
}

// R040-05: the opened file itself is checked, so a FIFO swapped in after any earlier check cannot block.
func TestOpenRegularRejectsFIFOAndSymlink040(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "1.json")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		f, err := openRegular(fifo)
		if f != nil {
			f.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO opened as regular file")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("open blocked on FIFO")
	}
	regular := filepath.Join(dir, "2.json")
	if err := os.WriteFile(regular, []byte(`{"started":"2026-01-01T00:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "3.json")
	if err := os.Symlink(regular, link); err != nil {
		t.Fatal(err)
	}
	if f, err := openRegular(link); err == nil {
		f.Close()
		t.Fatal("symlink followed")
	}
	f, err := openRegular(regular)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}
