//go:build !windows

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func Alert(message string) {
	fmt.Fprintln(os.Stderr, message)
	if runtime.GOOS == "darwin" {
		_ = exec.Command("osascript", "-e", `on run argv
 display alert "Stromea Steuerbox-Simulator" message (item 1 of argv)
 end run`, message).Run()
	}
}

func ConfirmExit(message string) bool {
	if runtime.GOOS != "darwin" {
		return true
	}
	return exec.Command("osascript", "-e", `on run argv
 display dialog (item 1 of argv) with title "Stromea Steuerbox-Simulator" buttons {"Abbrechen", "Beenden"} default button "Abbrechen" cancel button "Abbrechen"
 end run`, message).Run() == nil
}
