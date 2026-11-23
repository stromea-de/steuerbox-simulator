package storage

import (
	"errors"
	"path/filepath"
	"testing"
)

// R4-01: a second holder is refused until the first one unlocks.
func TestTryLockExclusive047(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	unlock, err := TryLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TryLock(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("second lock: %v", err)
	}
	unlock()
	again, err := TryLock(path)
	if err != nil {
		t.Fatal(err)
	}
	again()
}
