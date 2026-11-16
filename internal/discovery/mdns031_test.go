package discovery

import "testing"

// DNS packets are assembled in memory. No multicast/system DNS is used.
func TestMDNS031NamesSurviveTXTDisplayName(t *testing.T) {
	d := Device{Name: "Opta im Schaltschrank", SKI: "0123456789abcdef0123456789abcdef01234567", Address: "127.0.0.1:4711"}
	got := parse(records(d))
	if len(got) != 1 || got[0].Name != d.Name || got[0].Host != "sim-01234567.local." || got[0].ServiceName != "Stromea-Steuerbox-Simulator-01234567."+Service {
		t.Fatalf("DNS-SD host and instance must survive TXT name: %+v", got)
	}
}
