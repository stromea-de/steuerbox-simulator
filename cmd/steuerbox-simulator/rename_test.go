package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenamedCLIHelp(t *testing.T) {
	for _, command := range []string{"--help", "identity", "discover", "pair", "run", "demo", "serve"} {
		args := []string{command}
		if command != "--help" {
			args = append(args, "--help")
		}
		var out bytes.Buffer
		if code := execute(context.Background(), args, &out, &out); code != 0 {
			t.Fatal(code, out.String())
		}
		for _, want := range []string{"Stromea Steuerbox-Simulator", "Prüft §14a- und Einspeisebegrenzung per EEBUS (LPC, LPP, MPC)", "stromea-steuerbox-simulator", "Software-Simulation", "keine Steuerbox-Hardware"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s: missing %q", command, want)
			}
		}
	}
}

func TestExplicitStateDirectory(t *testing.T) {
	// Corrupt PEM stops before any listener: verify the selected path by its
	// error, without requiring network permissions or changing default data.
	chosen := t.TempDir()
	if err := os.Mkdir(filepath.Join(chosen, "energyguard"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chosen, "energyguard", "identity.pem"), []byte("existing damaged identity"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")
	var out bytes.Buffer
	if code := execute(context.Background(), []string{"identity", "--state", chosen}, &out, &out); code != 1 || !strings.Contains(out.String(), "lokale Identität beschädigt") {
		t.Fatal(code, out.String())
	}
}

func TestStandardStateDirectory(t *testing.T) {
	base := t.TempDir()
	want := filepath.Join(base, "stromea-steuerbox-simulator")
	got, err := defaultStateDir(base)
	if err != nil || got != want {
		t.Fatalf("got %s, %v", got, err)
	}
	if err := os.WriteFile(want, []byte("obstruction"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := defaultStateDir(base); err == nil {
		t.Fatal("file accepted as state directory")
	}
	if err := os.Remove(want); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(want, 0700); err != nil {
		t.Fatal(err)
	}
	got, err = defaultStateDir(base)
	if err != nil || got != want {
		t.Fatalf("got %s, %v", got, err)
	}
}
