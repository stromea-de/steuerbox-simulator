package session

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The archive namespace must behave identically on case-sensitive and case-insensitive hosts.
// Import validates before writing, including entries that the user did not select.
func TestImportPortablePaths045(t *testing.T) {
	for _, name := range []string{
		`reports\123.json`, `C:\profile.json`, `C:/profile.json`, `/profile.json`, `../profile.json`,
		`Profile.json`, `REPORTS/123.json`, `reports/CON.json`, `reports/con.json`, `reports/NUL.json`,
		`reports/AUX.json`, `reports/COM1.json`, `reports/LPT1.json`, `reports/123.json.`, `reports/123.json `,
		"reports/" + strings.Repeat("a", 260) + ".json", "reports/é.json", "reports/e\u0301.json",
	} {
		t.Run(name, func(t *testing.T) {
			dst := t.TempDir()
			sentinel := filepath.Join(dst, "sentinel")
			if err := os.WriteFile(sentinel, []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			archive := buildZip041(t, []entry041{{name: profileEntry, data: profile041}, {name: name, data: []byte(`{}`)}}, nil)
			if _, err := Import(archive, dst, ImportOptions{Profile: true}); err == nil {
				t.Fatal("nonportable archive accepted on " + runtime.GOOS)
			}
			files, err := os.ReadDir(dst)
			data, readErr := os.ReadFile(sentinel)
			if err != nil || readErr != nil || len(files) != 1 || !bytes.Equal(data, []byte("unchanged")) {
				t.Fatal("partial mutation")
			}
		})
	}
	// All accepted report filenames are short ASCII, with one canonical spelling per ID.
	for _, id := range []string{"123", "012345abcdef", "0123-4567-89AB-CDEF"} {
		p, err := reportPath("reports", id)
		if err != nil || !filepath.IsLocal(p) {
			t.Fatalf("portable report path on %s: %q %v", runtime.GOOS, p, err)
		}
	}
}
