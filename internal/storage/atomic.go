// Package storage commits private output files only after a complete write.
package storage

import (
	"errors"
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

// CreateExclusive commits a complete file only if path does not exist yet (236):
// a hard link fails atomically on an existing target. Without link support it
// falls back to O_EXCL; a torn file then fails to parse instead of being trusted.
func CreateExclusive(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".stromea-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err = os.Link(tmp, path); err == nil || errors.Is(err, os.ErrExist) {
		return err
	}
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = out.Write(data); err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
