package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stromea-de/steuerbox-simulator/internal/report"
)

func TestReleaseVersionDoesNotCreateState203(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", root)
	for _, arg := range []string{"--version", "version"} {
		var out, errout bytes.Buffer
		if code := execute(context.Background(), []string{arg}, &out, &errout); code != 0 {
			t.Fatalf("version exit %d", code)
		}
		if !strings.Contains(out.String(), report.BuildVersion()) || errout.Len() != 0 {
			t.Fatal("missing build version or unexpected error")
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("version must not create local identity or state", filepath.Base(root))
	}
}
