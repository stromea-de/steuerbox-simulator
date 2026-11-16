package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stromea-de/steuerbox-simulator/internal/branding"
)

// Validate the standard data path without hiding access or corruption errors.
func defaultStateDir(base string) (string, error) {
	path := filepath.Join(base, branding.Slug)
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Datenpfad ist kein Verzeichnis: %s", path)
	}
	return path, nil
}
