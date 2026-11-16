package discovery

import (
	"golang.org/x/sys/windows"
	"net"
)

func platformInterfaceKinds(all []net.Interface) map[string]string {
	out := map[string]string{}
	for _, i := range all {
		row := windows.MibIfRow2{InterfaceIndex: uint32(i.Index)}
		if windows.GetIfEntry2Ex(0, &row) == nil {
			out[i.Name] = adapterKind(row.Type)
		}
	}
	return out
}
