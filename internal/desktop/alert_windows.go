package desktop

import (
	"syscall"
	"unsafe"
)

func Alert(message string) {
	text, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("Stromea Steuerbox-Simulator")
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}

func ConfirmExit(message string) bool {
	text, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("Stromea Steuerbox-Simulator")
	// Yes/No, question icon, default No, foreground. No shell or external dependency.
	result, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x4|0x20|0x100|0x40000)
	return result == 6
}
