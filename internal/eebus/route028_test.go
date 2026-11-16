package eebus

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestRoute028DialPolicy(t *testing.T) {
	for _, tc := range []struct {
		address  string
		route    DialRoute
		fallback bool
	}{
		{"169.254.78.213:4711", DialRoute{"Ethernet", "169.254.78.1"}, false},
		{"[fe80::2]:4711", DialRoute{"Ethernet", "fe80::1"}, false},
		{"192.0.2.2:4711", DialRoute{"Ethernet", "192.0.2.1"}, true},
	} {
		t.Run(tc.address, func(t *testing.T) {
			calls := 0
			trace := newConnectionTrace()
			dial := func(_ context.Context, d *net.Dialer, network, address string) (net.Conn, error) {
				calls++
				if calls == 1 {
					if d.LocalAddr == nil {
						t.Fatal("missing source binding")
					}
					if network == "tcp6" && address != "[fe80::2%Ethernet]:4711" {
						t.Fatal(address)
					}
					return nil, &net.OpError{Op: "dial", Err: &os.SyscallError{Syscall: "bind", Err: syscall.EADDRNOTAVAIL}}
				}
				if d.LocalAddr != nil {
					t.Fatal("fallback still bound")
				}
				return nil, errors.New("test end")
			}
			_, _ = dialRoute(context.Background(), tc.address, tc.route, trace, dial)
			want := 1
			if tc.fallback {
				want = 2
			}
			if calls != want {
				t.Fatalf("calls %d want %d", calls, want)
			}
			if tc.fallback && trace.snapshot()[0].Detail == "" {
				t.Fatal("fallback not diagnosed")
			}
		})
	}
}

func TestRoute028NoFallbackAfterConnectFailure(t *testing.T) {
	calls := 0
	_, err := dialRoute(context.Background(), "192.0.2.2:4711", DialRoute{"Ethernet", "192.0.2.1"}, newConnectionTrace(), func(context.Context, *net.Dialer, string, string) (net.Conn, error) {
		calls++
		return nil, syscall.ECONNREFUSED
	})
	if calls != 1 || !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("%d %v", calls, err)
	}
}

func TestRoute028LinkLocalRequiresSource(t *testing.T) {
	for _, a := range []string{"169.254.1.2:4711", "[fe80::2]:4711"} {
		_, err := dialRoute(context.Background(), a, DialRoute{}, newConnectionTrace(), func(context.Context, *net.Dialer, string, string) (net.Conn, error) {
			t.Fatal("unbound link-local dial")
			return nil, nil
		})
		if err == nil {
			t.Fatal("missing route accepted")
		}
	}
	if err := DeviceAddress("[fe80::2%Ethernet]:4711"); err != nil {
		t.Fatal(err)
	}
}

func TestRoute028LoopbackSocketAndFallback(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ifaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	loop := ""
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			loop = iface.Name
			break
		}
	}
	for _, name := range []string{loop, "stromea-missing-adapter-028"} {
		trace := newConnectionTrace()
		c, err := dialRoute(context.Background(), listener.Addr().String(), DialRoute{name, "127.0.0.1"}, trace, func(ctx context.Context, d *net.Dialer, network, address string) (net.Conn, error) {
			return d.DialContext(ctx, network, address)
		})
		if err != nil {
			t.Fatal(err)
		}
		c.Close()
		trace.finish(nil)
		detail := trace.snapshot()[0].Detail
		if !strings.Contains(detail, "127.0.0.1") || !strings.Contains(detail, name) {
			t.Fatal(detail)
		}
		if (name != loop) != strings.Contains(detail, "Rückfall auf System-Routing") {
			t.Fatal(detail)
		}
	}
}

func TestRoute028CanceledBindDoesNotFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := dialRoute(ctx, "192.0.2.2:4711", DialRoute{"Ethernet", "192.0.2.1"}, newConnectionTrace(), func(context.Context, *net.Dialer, string, string) (net.Conn, error) {
		calls++
		cancel()
		return nil, &bindingError{context.Canceled}
	})
	if calls != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("%d %v", calls, err)
	}
}

func TestRoute028NumericZone(t *testing.T) {
	ifaces, e := net.Interfaces()
	if e != nil {
		t.Fatal(e)
	}
	if len(ifaces) == 0 {
		t.Fatal("no local interfaces")
	}
	iface := ifaces[0]
	zone := strconv.Itoa(iface.Index)
	if !sameRouteInterface(zone, iface.Name) {
		t.Fatal("numeric zone not resolved")
	}
	_, e = dialRoute(context.Background(), "[fe80::2%"+zone+"]:4711", DialRoute{iface.Name, "fe80::1"}, newConnectionTrace(), func(_ context.Context, d *net.Dialer, network, address string) (net.Conn, error) {
		if network != "tcp6" || address != "[fe80::2%"+iface.Name+"]:4711" || d.LocalAddr.(*net.TCPAddr).Zone != iface.Name {
			t.Fatalf("%s %s %v", network, address, d.LocalAddr)
		}
		return nil, errors.New("test end")
	})
	if e == nil {
		t.Fatal("missing test sentinel")
	}
}
