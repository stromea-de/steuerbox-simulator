//go:build windows

package session

import (
	"errors"
	"os"
	"syscall"
)

// openRegular opens a report without following reparse points (symlinks, junctions) and checks the opened
// handle itself: a plain disk file, no directory, no reparse point (R040-05, R040-07).
func openRegular(path string) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	var info syscall.ByHandleFileInformation
	kind, typeErr := syscall.GetFileType(h)
	if typeErr != nil || kind != syscall.FILE_TYPE_DISK || syscall.GetFileInformationByHandle(h, &info) != nil ||
		info.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		syscall.CloseHandle(h)
		return nil, errors.New("keine reguläre Datei")
	}
	return os.NewFile(uintptr(h), path), nil
}
