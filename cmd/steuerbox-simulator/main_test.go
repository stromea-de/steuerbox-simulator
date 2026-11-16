package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIRejectsUnknownCommandAndLANListener(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"serve", "--listen", "0.0.0.0:33287"}, {"demo", "--betrieb", "Test", "--pruefer", ""}} {
		var b bytes.Buffer
		if code := execute(context.Background(), args, &b, &b); code != 2 {
			t.Fatalf("%v: %d %s", args, code, b.String())
		}
	}
}

func TestLANDiscoveryOptions(t *testing.T) {
	for _, args := range [][]string{
		{"discover", "--lan"},
		{"discover", "--interface", "en0"},
		{"discover", "--lan", "--interface", "en0", "--mdns-loopback"},
	} {
		var out bytes.Buffer
		if code := execute(context.Background(), args, &out, &out); code != 2 {
			t.Fatalf("%v: %d %s", args, code, out.String())
		}
	}
	var out bytes.Buffer
	if code := execute(context.Background(), []string{"discover", "--interfaces"}, &out, &out); code != 0 {
		t.Fatalf("list interfaces: %d %s", code, out.String())
	}
}

func TestDeviceOptionsRequireExplicitSKI(t *testing.T) {
	ski := "0123456789012345678901234567890123456789"
	for _, args := range [][]string{
		{"run", "--device", "--betrieb", "Test", "--pruefer", "Test", "--address", "127.0.0.1:4711", "--ski", ski},
		{"pair", "--device", "--address", "example.com:4711", "--ski", ski, "--confirm-ski", ski},
	} {
		var out bytes.Buffer
		if code := execute(context.Background(), args, &out, &out); code != 2 {
			t.Fatalf("%v: %d %s", args, code, out.String())
		}
	}
}

func TestCLISignatureOptions(t *testing.T) {
	for _, command := range []string{"run", "demo"} {
		var out bytes.Buffer
		args := []string{command, "--unterschrift-betreiber", "--unterschrift-fachkraft", "--betrieb", "Test", "--pruefer", ""}
		if code := execute(context.Background(), args, &out, &out); code != 2 || !strings.Contains(out.String(), "Prüfer/in ist erforderlich") {
			t.Fatal("signature flags not accepted before config validation", code, out.String())
		}
		out.Reset()
		if code := execute(context.Background(), []string{command, "--help"}, &out, &out); code != 0 {
			t.Fatal(code)
		}
		for _, name := range []string{"unterschrift-betreiber", "unterschrift-fachkraft"} {
			if !strings.Contains(out.String(), name) {
				t.Fatal("missing help", name)
			}
		}
	}
}

func TestAuthorizationCompatibilityFlagIsIgnored(t *testing.T) {
	for _, flag := range []string{"--authorized", "--authorized=false"} {
		var out bytes.Buffer
		code := execute(context.Background(), []string{"run", flag, "--betrieb", "Test", "--pruefer", ""}, &out, &out)
		if code != 2 || !strings.Contains(out.String(), "Prüfer/in ist erforderlich") {
			t.Fatal(code, out.String())
		}
	}
}

func TestDeviceRunWithoutAuthorizationReachesStateInitialization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	ski := "0123456789012345678901234567890123456789"
	for _, extra := range [][]string{nil, {"--authorized"}, {"--authorized=false"}} {
		args := []string{"run", "--device", "--address", "127.0.0.1:4711", "--ski", ski, "--confirm-ski", ski, "--betrieb", "Test", "--pruefer", "Test", "--state", path}
		args = append(args, extra...)
		var out bytes.Buffer
		code := execute(context.Background(), args, &out, &out)
		if code != 1 || !strings.Contains(out.String(), "not a directory") {
			t.Fatal("expected filesystem failure after successful validation", code, out.String())
		}
	}
}
