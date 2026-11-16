// Package storage commits private output files only after a complete write.
package storage

import (
	"os"
	"path/filepath"
)

func WriteFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".stromea-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
