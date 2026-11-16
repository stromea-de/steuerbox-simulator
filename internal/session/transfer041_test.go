package session

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
)

// 041 (Nutzer 2026-11-14): Export/Import aller Daten (Protokolle, Profil inkl. Logo, Simulatorkennung) als ZIP.

func transferFixture041(t *testing.T) (state string, ids []string, pem []byte) {
	t.Helper()
	state = t.TempDir()
	dir := filepath.Join(state, "reports")
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		berlin = time.FixedZone("CET", 3600)
	}
	for i, at := range []time.Time{time.Date(2026, 11, 8, 10, 15, 0, 0, berlin), time.Date(2026, 3, 29, 1, 59, 0, 0, time.FixedZone("UTC+14", 14*3600))} {
		id, e := inspection.NewReportID()
		if e != nil {
			t.Fatal(e)
		}
		r := inspection.Report{ID: id, SchemaVersion: 1, Started: at, Finished: at.Add(time.Minute), Mode: inspection.ModeSimulator, Steps: []inspection.Step{{Name: "Verbindung", Passed: true, At: at}}}
		r.Config.Site.Customer = []string{"Müller & Söhne – Ünïcode 🚀", `Kunde "Ä/\\"`}[i]
		if e = SaveReport(dir, &r); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	if _, err = identity.LoadOrCreate(filepath.Join(state, "energyguard")); err != nil {
		t.Fatal(err)
	}
	pem, err = os.ReadFile(filepath.Join(state, "energyguard", "identity.pem"))
	if err != nil {
		t.Fatal(err)
	}
	return state, ids, pem
}

var profile041 = json.RawMessage(`{"business":"Elektro Muster GmbH","street":"Musterweg 1","postalCode":"12345","city":"Musterstadt","country":"Deutschland","inspector":"Erika Muster","logo":"data:image/png;base64,iVBORw0KGgo="}`)

func TestExportImportRoundTrip041(t *testing.T) {
	src, ids, pem := transferFixture041(t)
	var archive bytes.Buffer
	if err := Export(&archive, src, profile041, time.Date(2026, 11, 14, 9, 0, 0, 0, time.FixedZone("X", -7*3600))); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, f := range z.File {
		names = append(names, f.Name)
		_, offset := f.Modified.Zone()
		if strings.Contains(f.Name, `\`) || strings.HasPrefix(f.Name, "/") || offset != 0 || !f.Modified.Equal(time.Date(2026, 11, 14, 16, 0, 0, 0, time.UTC)) {
			t.Fatalf("portable entry name/time expected: %q %v", f.Name, f.Modified)
		}
	}
	if names[0] != "manifest.json" {
		t.Fatalf("manifest first: %v", names)
	}
	dst := t.TempDir()
	path := filepath.Join(t.TempDir(), "export.zip")
	if err = os.WriteFile(path, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	res, err := Import(path, dst, ImportOptions{Reports: true, Profile: true, Identity: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported != 2 || res.Skipped != 0 || !bytes.Equal(res.IdentityPEM, pem) {
		t.Fatalf("result %+v", res)
	}
	var a, b map[string]any
	_ = json.Unmarshal(profile041, &a)
	_ = json.Unmarshal(res.Profile, &b)
	if a["logo"] != b["logo"] || a["country"] != b["country"] || a["inspector"] != b["inspector"] {
		t.Fatalf("profile round trip: %s", res.Profile)
	}
	for _, id := range ids {
		before, _ := os.ReadFile(filepath.Join(src, "reports", id+".json"))
		after, err := os.ReadFile(filepath.Join(dst, "reports", id+".json"))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("report %s must be byte-identical (checksum stays valid): %v", id, err)
		}
		r1, _ := LoadReport(filepath.Join(src, "reports"), id)
		r2, _ := LoadReport(filepath.Join(dst, "reports"), id)
		if !r1.Started.Equal(r2.Started) || r1.Config.Site.Customer != r2.Config.Site.Customer {
			t.Fatalf("time zone/charset changed: %v %v", r1.Started, r2.Started)
		}
		_, o1 := r1.Started.Zone()
		_, o2 := r2.Started.Zone()
		if o1 != o2 {
			t.Fatalf("time zone offset changed")
		}
	}
	// Duplicates (same report ID) are not imported twice.
	again, err := Import(path, dst, ImportOptions{Reports: true})
	if err != nil || again.Imported != 0 || again.Skipped != 2 || again.Profile != nil || again.IdentityPEM != nil {
		t.Fatalf("second import: %+v %v", again, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(dst, "reports")); len(entries) != 2 {
		t.Fatalf("duplicates written: %d", len(entries))
	}
}

func TestImportSelectiveAndEmpty041(t *testing.T) {
	src, _, _ := transferFixture041(t)
	var archive bytes.Buffer
	if err := Export(&archive, src, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "export.zip")
	_ = os.WriteFile(path, archive.Bytes(), 0600)
	dst := t.TempDir()
	res, err := Import(path, dst, ImportOptions{Profile: true, Identity: true})
	if err != nil || res.Imported != 0 || res.Profile != nil || res.IdentityPEM == nil {
		t.Fatalf("selective: %+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "reports")); !os.IsNotExist(err) {
		t.Fatal("reports written although not selected")
	}
	if _, err := Import(path, dst, ImportOptions{}); err == nil {
		t.Fatal("import without any selection must fail")
	}
}

type entry041 struct {
	name string
	data []byte
	mode os.FileMode
}

func buildZip041(t *testing.T, entries []entry041, manifest func(map[string]any)) string {
	t.Helper()
	sums := map[string]string{}
	for _, e := range entries {
		sums[e.name] = sha256Hex(e.data)
	}
	m := map[string]any{"format": ExportFormat, "version": ExportVersion, "created": "2026-11-14T08:00:00Z", "files": sums}
	if manifest != nil {
		manifest(m)
	}
	data, _ := json.Marshal(m)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	all := append([]entry041{{name: "manifest.json", data: data}}, entries...)
	for _, e := range all {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		f, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write(e.data)
	}
	_ = w.Close()
	path := filepath.Join(t.TempDir(), "x.zip")
	_ = os.WriteFile(path, buf.Bytes(), 0600)
	return path
}

func TestImportRejectsUnsafeArchives041(t *testing.T) {
	src, ids, pem := transferFixture041(t)
	report, _ := os.ReadFile(filepath.Join(src, "reports", ids[0]+".json"))
	good := entry041{name: "reports/" + ids[0] + ".json", data: report}
	cases := map[string]struct {
		entries  []entry041
		manifest func(map[string]any)
	}{
		"zip slip parent":     {entries: []entry041{{name: "../evil.json", data: report}}},
		"zip slip nested":     {entries: []entry041{{name: "reports/../../evil.json", data: report}}},
		"absolute path":       {entries: []entry041{{name: "/tmp/evil.json", data: report}}},
		"windows separator":   {entries: []entry041{{name: `reports\` + ids[0] + ".json", data: report}}},
		"drive letter":        {entries: []entry041{{name: "C:/evil.json", data: report}}},
		"unknown entry":       {entries: []entry041{good, {name: "run.sh", data: []byte("#!/bin/sh\nrm -rf /")}}},
		"invalid report id":   {entries: []entry041{{name: "reports/evil name.json", data: report}}},
		"symlink entry":       {entries: []entry041{{name: "reports/" + ids[1] + ".json", data: []byte("/etc/passwd"), mode: os.ModeSymlink | 0777}}},
		"damaged report":      {entries: []entry041{{name: "reports/" + ids[1] + ".json", data: []byte(`{"note":"no report"}`)}}},
		"id mismatch":         {entries: []entry041{{name: "reports/" + ids[1] + ".json", data: report}}},
		"damaged identity":    {entries: []entry041{good, {name: "identity/energyguard.pem", data: []byte("-----" + "BEGIN CERTIFICATE" + "-----\nAAAA\n-----" + "END CERTIFICATE" + "-----\n")}}},
		"damaged profile":     {entries: []entry041{good, {name: "profile.json", data: []byte(`{"logo":"javascript:alert(1)"}`)}}},
		"duplicate name":      {entries: []entry041{good, good}},
		"newer version":       {entries: []entry041{good}, manifest: func(m map[string]any) { m["version"] = ExportVersion + 1 }},
		"foreign format":      {entries: []entry041{good}, manifest: func(m map[string]any) { m["format"] = "other" }},
		"checksum mismatch":   {entries: []entry041{good}, manifest: func(m map[string]any) { m["files"] = map[string]string{good.name: sha256Hex([]byte("x"))} }},
		"missing in manifest": {entries: []entry041{good}, manifest: func(m map[string]any) { m["files"] = map[string]string{} }},
	}
	_ = pem
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dst := t.TempDir()
			path := buildZip041(t, c.entries, c.manifest)
			if _, err := Import(path, dst, ImportOptions{Reports: true, Profile: true, Identity: true}); err == nil {
				t.Fatal("unsafe archive accepted")
			}
			left := 0
			_ = filepath.Walk(dst, func(p string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					left++
				}
				return nil
			})
			if left != 0 {
				t.Fatalf("rejected archive wrote %d files", left)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(dst), "evil.json")); err == nil {
				t.Fatal("zip slip escaped the target")
			}
		})
	}
	// Not a ZIP at all, and an oversized entry (limits are checked on the actual bytes, not the header).
	junk := filepath.Join(t.TempDir(), "junk.zip")
	_ = os.WriteFile(junk, []byte("PK\x03\x04 not really"), 0600)
	if _, err := Import(junk, t.TempDir(), ImportOptions{Reports: true}); err == nil {
		t.Fatal("non-zip accepted")
	}
	big := buildZip041(t, []entry041{{name: "profile.json", data: bytes.Repeat([]byte(" "), maxProfileBytes+1)}}, nil)
	if _, err := Import(big, t.TempDir(), ImportOptions{Profile: true}); err == nil {
		t.Fatal("oversized profile accepted")
	}
}

func TestImportIdentityReplacesSimulatorIdentity041(t *testing.T) {
	src, _, pem := transferFixture041(t)
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, before, _, _, _ := s.Snapshot()
	if err := s.ImportIdentity([]byte("garbage")); err == nil {
		t.Fatal("garbage identity accepted")
	}
	if err := s.ImportIdentity(pem); err != nil {
		t.Fatal(err)
	}
	_, after, _, _, _ := s.Snapshot()
	want, _ := identity.LoadOrCreate(filepath.Join(src, "energyguard"))
	if after == before || after != want.SKI {
		t.Fatalf("simulator identity not replaced: %s -> %s (want %s)", before, after, want.SKI)
	}
	stored, _ := os.ReadFile(filepath.Join(s.state, "energyguard", "identity.pem"))
	if !bytes.Equal(stored, pem) {
		t.Fatal("identity file not stored")
	}
	if st, _ := os.Stat(filepath.Join(s.state, "energyguard", "identity.pem")); st.Mode().Perm()&0077 != 0 {
		t.Fatalf("private key readable by others: %v", st.Mode())
	}
	_ = io.Discard
}

// Review 041 R041-01: a failure while writing rolls back every report written by this import.
func TestImportRollsBackOnWriteFailure041(t *testing.T) {
	src, _, _ := transferFixture041(t)
	var archive bytes.Buffer
	if err := Export(&archive, src, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "export.zip")
	_ = os.WriteFile(path, archive.Bytes(), 0600)
	dst := t.TempDir()
	calls := 0
	old := writeImported
	writeImported = func(p string, b []byte) error {
		calls++
		if calls == 2 {
			return os.ErrPermission
		}
		return old(p, b)
	}
	defer func() { writeImported = old }()
	if _, err := Import(path, dst, ImportOptions{Reports: true}); err == nil {
		t.Fatal("write failure not reported")
	}
	if entries, _ := os.ReadDir(filepath.Join(dst, "reports")); len(entries) != 0 {
		t.Fatalf("partial import left %d files", len(entries))
	}
	// The caller can roll back a successful report import when a later step (identity) fails.
	writeImported = old
	res, err := Import(path, dst, ImportOptions{Reports: true})
	if err != nil || res.Imported != 2 {
		t.Fatal(err)
	}
	res.Rollback()
	if entries, _ := os.ReadDir(filepath.Join(dst, "reports")); len(entries) != 0 {
		t.Fatalf("rollback left %d files", len(entries))
	}
}

// Review 041 R041-02/R041-07: newer schemas, inconsistent revisions and conflicting versions are not imported.
func TestImportRejectsUnknownSchemasAndVersionConflicts041(t *testing.T) {
	src, ids, _ := transferFixture041(t)
	report, _ := os.ReadFile(filepath.Join(src, "reports", ids[0]+".json"))
	var m map[string]any
	_ = json.Unmarshal(report, &m)
	mutate := func(f func(map[string]any)) []byte {
		c := map[string]any{}
		for k, v := range m {
			c[k] = v
		}
		f(c)
		b, _ := json.Marshal(c)
		return b
	}
	for name, data := range map[string][]byte{
		"newer schema": mutate(func(c map[string]any) { c["schemaVersion"] = 999 }),
		"unknown mode": mutate(func(c map[string]any) { c["mode"] = "cloud" }),
		"bad revision": mutate(func(c map[string]any) { c["revision"] = map[string]any{"number": 1, "rootId": "x"} }),
		"zero start":   mutate(func(c map[string]any) { c["started"] = "0001-01-01T00:00:00Z" }),
		"bad revision root": mutate(func(c map[string]any) {
			c["revision"] = map[string]any{"number": 2, "rootId": "../x", "previousId": ids[1]}
		}),
		"minimal schema 1": []byte(`{"schemaVersion":1,"started":"2026-11-14T08:00:00Z"}`),
		"null steps":       mutate(func(c map[string]any) { c["steps"] = nil }),
		"missing config":   mutate(func(c map[string]any) { delete(c, "config") }),
		"missing finished": mutate(func(c map[string]any) { delete(c, "finished") }),
		"passed not bool":  mutate(func(c map[string]any) { c["passed"] = "yes" }),
		"empty id":         mutate(func(c map[string]any) { c["id"] = "" }),
		"revision no times": mutate(func(c map[string]any) {
			c["revision"] = map[string]any{"number": 2, "rootId": ids[1], "previousId": ids[1]}
		}),
	} {
		t.Run(name, func(t *testing.T) {
			path := buildZip041(t, []entry041{{name: "reports/" + ids[0] + ".json", data: data}}, nil)
			if _, err := Import(path, t.TempDir(), ImportOptions{Reports: true}); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	// Two installations corrected the same v1 independently: the second v2 is a conflict, not a new row.
	root := ids[0]
	v2a, _ := inspection.NewReportID()
	v2b, _ := inspection.NewReportID()
	rev := func(id string) []byte {
		return mutate(func(c map[string]any) {
			c["id"] = id
			c["revision"] = map[string]any{"number": 2, "rootId": root, "previousId": root, "created": "2026-11-14T08:00:00Z", "previousDate": "2026-11-08T09:15:00Z"}
		})
	}
	dst := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dst, "reports"), 0700)
	_ = os.WriteFile(filepath.Join(dst, "reports", root+".json"), report, 0600)
	_ = os.WriteFile(filepath.Join(dst, "reports", v2a+".json"), rev(v2a), 0600)
	path := buildZip041(t, []entry041{{name: "reports/" + v2b + ".json", data: rev(v2b)}}, nil)
	res, err := Import(path, dst, ImportOptions{Reports: true})
	if err != nil || res.Imported != 0 || res.Conflicts != 1 {
		t.Fatalf("conflict: %+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "reports", v2b+".json")); err == nil {
		t.Fatal("conflicting version written")
	}
	// Review R041-11: a successor of a rejected (conflicting) version is a conflict too, in any order.
	v3b, _ := inspection.NewReportID()
	v3 := mutate(func(c map[string]any) {
		c["id"] = v3b
		c["revision"] = map[string]any{"number": 3, "rootId": root, "previousId": v2b, "created": "2026-11-14T09:00:00Z", "previousDate": "2026-11-14T08:00:00Z"}
	})
	for _, order := range [][]entry041{{{name: "reports/" + v2b + ".json", data: rev(v2b)}, {name: "reports/" + v3b + ".json", data: v3}}, {{name: "reports/" + v3b + ".json", data: v3}, {name: "reports/" + v2b + ".json", data: rev(v2b)}}} {
		res, err := Import(buildZip041(t, order, nil), dst, ImportOptions{Reports: true})
		if err != nil || res.Imported != 0 || res.Conflicts != 2 {
			t.Fatalf("successor of a conflict: %+v %v", res, err)
		}
	}
	// Review R041-13: the predecessor must belong to the same series and carry the previous number.
	other := ids[1]
	for name, revision := range map[string]map[string]any{
		"foreign predecessor": {"number": 2, "rootId": root, "previousId": other, "created": "2026-11-14T08:00:00Z", "previousDate": "2026-11-08T09:15:00Z"},
		"skipped number":      {"number": 3, "rootId": other, "previousId": other, "created": "2026-11-14T08:00:00Z", "previousDate": "2026-11-08T09:15:00Z"},
	} {
		id, _ := inspection.NewReportID()
		data := mutate(func(c map[string]any) { c["id"] = id; c["revision"] = revision })
		d := t.TempDir()
		_ = os.MkdirAll(filepath.Join(d, "reports"), 0700)
		other0, _ := os.ReadFile(filepath.Join(src, "reports", other+".json"))
		_ = os.WriteFile(filepath.Join(d, "reports", root+".json"), report, 0600)
		_ = os.WriteFile(filepath.Join(d, "reports", other+".json"), other0, 0600)
		if res, err := Import(buildZip041(t, []entry041{{name: "reports/" + id + ".json", data: data}}, nil), d, ImportOptions{Reports: true}); err != nil || res.Imported != 0 || res.Conflicts != 1 {
			t.Fatalf("%s: %+v %v", name, res, err)
		}
	}
	// A version whose predecessor is missing everywhere is not imported either.
	orphan, _ := inspection.NewReportID()
	lone := mutate(func(c map[string]any) {
		c["id"] = orphan
		c["revision"] = map[string]any{"number": 4, "rootId": root, "previousId": v3b, "created": "2026-11-14T10:00:00Z", "previousDate": "2026-11-14T09:00:00Z"}
	})
	if res, err := Import(buildZip041(t, []entry041{{name: "reports/" + orphan + ".json", data: lone}}, nil), dst, ImportOptions{Reports: true}); err != nil || res.Imported != 0 || res.Conflicts != 1 {
		t.Fatalf("orphan version: %+v %v", res, err)
	}
}
