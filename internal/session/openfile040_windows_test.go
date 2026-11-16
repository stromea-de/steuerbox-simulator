//go:build windows

package session

import (
	"os"
	"path/filepath"
	"testing"
)

// R040-07: a symlink in place of a report is not followed on Windows (requires symlink privilege; skipped otherwise).
func TestOpenRegularRejectsSymlinkWindows040(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "2.json")
	if err := os.WriteFile(regular, []byte(`{"started":"2026-01-01T00:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "3.json")
	if err := os.Symlink(regular, link); err != nil {
		t.Skip("symlink privilege missing:", err)
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
