package discovery

import (
	"context"
	"net"
	"os/exec"
	"time"
)

func platformInterfaceKinds(_ []net.Interface) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// Local configuration query only. No scan, DNS or network traffic.
	data, err := exec.CommandContext(ctx, "/usr/sbin/networksetup", "-listallhardwareports").Output()
	if err != nil {
		return nil
	}
	return parseHardwarePorts(string(data))
}
