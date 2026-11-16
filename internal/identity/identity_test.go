package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentityPersistsAndRejectsBrokenState(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreate(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreate(dir)
	if err != nil || first.SKI != second.SKI || len(first.SKI) != 40 {
		t.Fatalf("unstable identity: %v", err)
	}
	info, _ := os.Stat(filepath.Join(dir, "identity.pem"))
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
	if err := os.WriteFile(filepath.Join(dir, "identity.pem"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreate(dir); err == nil {
		t.Fatal("must not replace broken identity")
	}
}

func TestNormalizeSKI(t *testing.T) {
	s := "0123456789abcdef0123456789abcdef01234567"
	if got, err := NormalizeSKI("01:23:45:67:89:AB:CD:EF:01:23:45:67:89:AB:CD:EF:01:23:45:67"); err != nil || got != s {
		t.Fatalf("%q %v", got, err)
	}
	for _, s := range []string{"", "abc", "../../key", "zz23456789abcdef0123456789abcdef01234567"} {
		if _, err := NormalizeSKI(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
