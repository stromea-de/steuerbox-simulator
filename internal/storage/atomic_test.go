package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteKeepsDestinationOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "keep")
	if err := os.Mkdir(dest, 0700); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(dest, []byte("new")); err == nil {
		t.Fatal("rename should fail")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 || files[0].Name() != "keep" {
		t.Fatal(files)
	}
	dest = filepath.Join(dir, "report.json")
	if err := WriteFile(dest, []byte("complete")); err != nil {
		t.Fatal(err)
	}
	v, _ := os.ReadFile(dest)
	info, _ := os.Stat(dest)
	if string(v) != "complete" || info.Mode().Perm() != 0600 {
		t.Fatal(string(v), info)
	}
}
