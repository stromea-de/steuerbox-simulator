package storage

import (
	"errors"
	"os"
)

// ErrLocked reports that another process (or session) holds the lock.
var ErrLocked = errors.New("bereits durch einen anderen Vorgang belegt")

// TryLock takes an exclusive, non-blocking lock on path (R4-01). The lock lives as
// long as the returned unlock is not called and the process runs; the operating
// system frees it after a crash. The file itself is kept: removing it would let a
// second process lock a new file while the first still holds the old one.
func TryLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() { _ = unlockFile(f); _ = f.Close() }, nil
}
