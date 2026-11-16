//go:build !windows && (!darwin || !cgo)

package desktop

import "context"

func menu(ctx context.Context, version string, open, logs, stop func(), done <-chan struct{}) {
	open()
	select {
	case <-ctx.Done():
		stop()
		<-done
	case <-done:
	}
}
