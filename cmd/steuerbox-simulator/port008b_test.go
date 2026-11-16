package main

import (
	"bytes"
	"context"
	"net"
	"strings"
	"testing"
)

func TestServeDefaultPort008b(t *testing.T) {
	var out bytes.Buffer
	if code := execute(context.Background(), []string{"serve", "--help"}, &out, &out); code != 0 {
		t.Fatal(code, out.String())
	}
	if !strings.Contains(out.String(), `default "127.0.0.1:33287"`) {
		t.Fatal("missing new default", out.String())
	}
}

func TestServeOccupiedPort008b(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var out bytes.Buffer
	// A nil session is intentional: binding must fail before constructing the UI.
	if code := serve(context.Background(), nil, l.Addr().String(), &out, &out); code != 1 {
		t.Fatal(code, out.String())
	}
	for _, want := range []string{"bereits belegt", l.Addr().String(), "--listen 127.0.0.1:<port>"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q: %s", want, out.String())
		}
	}
}

func TestServeAnnouncesBoundURL008b(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	// Port 0 asks the OS for a free loopback port; the printed URL must use it.
	if code := serve(ctx, nil, "127.0.0.1:0", &out, &out); code != 0 {
		t.Fatal(code, out.String())
	}
	line := strings.Split(out.String(), "\n")[0]
	addr := strings.TrimPrefix(line, "Lokale Oberfläche: http://")
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host != "127.0.0.1" || port == "0" || port == "" {
		t.Fatal("actual bound URL missing", line)
	}
}
