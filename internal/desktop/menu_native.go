//go:build windows || (darwin && cgo)

package desktop

import (
	"context"
	"fyne.io/systray"
)

func menu(ctx context.Context, version string, open, logs, stop func(), done <-chan struct{}) {
	systray.Run(func() {
		systray.SetIcon(icon())
		systray.SetTitle("Stromea")
		systray.SetTooltip("Stromea Steuerbox-Simulator")
		show := systray.AddMenuItem("Simulator öffnen", "")
		systray.AddMenuItem("Version "+version, "").Disable()
		folder := systray.AddMenuItem("Protokollordner öffnen", "")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Beenden", "")
		go open()
		go func() {
			for {
				select {
				case <-show.ClickedCh:
					go open()
				case <-folder.ClickedCh:
					go logs()
				case <-quit.ClickedCh:
					if ConfirmExit(exitMessage(activeInstance())) {
						quit.Disable()
						stop()
					}
				case <-ctx.Done():
					stop()
					<-done
					systray.Quit()
					return
				case <-done:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}
