//go:build !windows

package session

import (
	"errors"
	"os"
	"syscall"
)

// openRegular opens a report without following symlinks and without blocking on a FIFO, then checks the type
// of the opened file itself (R040-05: no check-then-open race between Lstat and ReadFile).
func openRegular(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("keine reguläre Datei")
	}
	return f, nil
}
