package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestEmbeddedReportAndMetadata024(t *testing.T) {
	r := sampleReport(t)
	r.ID = "K7Q2-9XMB-4T1R-HZ6P"
	r.SchemaVersion = 1
	r.Mode = inspection.ModeSimulator
	r.Software = BuildVersion()
	for i := range r.Steps {
		r.Steps[i].Category = inspection.StepCategory(r.Steps[i].Name)
		if r.Steps[i].Category == inspection.Workflow {
			r.Steps[i].Status = "done"
		}
	}
	r.Config.SignInspector = true
	r.Config.SignOperator = true
	r.AbortReason = "connection_lost"
	r.AbortText = inspection.ReasonText(r.AbortReason)
	r.Steps[4].Status = inspection.NotChecked
	r.Steps[4].Passed = false
	r.Steps[4].Watts = ptr(0)
	r.Steps[4].Detail = "nicht geprüft (zu wenig Leistung)"
	r.Passed = false
	data, _ := JSON(r)
	pdf, e := PDF(r)
	if e != nil {
		t.Fatal(e)
	}
	for _, want := range []string{"/EmbeddedFiles", "/AF [", "/AFRelationship /Data", "stromea-pruefbericht.json", "/Subtype /application#2Fjson", "/Type /Metadata /Subtype /XML", "xmp:CreateDate", "xmp:CreatorTool", r.ID, "pruefbericht-v1.json"} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Fatal("missing", want)
		}
	}
	re := regexp.MustCompile(`(?s)/Type /EmbeddedFile.*?/Length ([0-9]+).*?stream\n`)
	m := re.FindSubmatchIndex(pdf)
	if m == nil {
		t.Fatal("no attachment stream")
	}
	n, _ := strconv.Atoi(string(pdf[m[2]:m[3]]))
	got := pdf[m[1] : m[1]+n]
	if sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatal("JSON attachment differs")
	}
	// xref offsets must still point at every indirect object after adding streams.
	for _, m := range regexp.MustCompile(`(?m)^([0-9]{10}) 00000 n `).FindAllSubmatch(pdf, -1) {
		off, _ := strconv.Atoi(string(m[1]))
		if !regexp.MustCompile(`^[0-9]+ 0 obj\n`).Match(pdf[off:]) {
			t.Fatal("invalid xref", off)
		}
	}
	if os.Getenv("STROMEA_024_ARTIFACTS") == "1" {
		if e = os.WriteFile("../../artifacts/024-pruefbericht.pdf", pdf, 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile("../../artifacts/024-pruefbericht.json", data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	text := pdfText(t, pdf)
	if strings.Contains(text, "(freiwillig)") || strings.Contains(text, "Die Empfangsbestätigung ist keine Abnahme") {
		t.Fatal("old signature text")
	}
	if !strings.Contains(text, "Bestätigung der Fachkraft") {
		t.Fatal(text)
	}
	r.Config.Site.Note = "Nachtrag"
	other, _ := PDF(r)
	if !bytes.Contains(other, []byte(r.ID)) {
		t.Fatal("ID changed with contents")
	}
}
func TestVersionLabels024(t *testing.T) {
	oldV, oldC := Version, Commit
	defer func() { Version, Commit = oldV, oldC }()
	for _, v := range []struct{ tag, commit, want string }{{"v1.0.0-beta.1", "29a6c6e", "Version 1.0.0-beta.1 (Softwarestand 29a6c6e)"}, {"", "29a6c6e", "Testversion (Softwarestand 29a6c6e)"}, {"29a6c6e-dirty", "29a6c6e", "Testversion (Softwarestand 29a6c6e-dirty)"}} {
		Version, Commit = v.tag, v.commit
		if BuildVersion() != v.want {
			t.Fatal(BuildVersion(), v.want)
		}
	}
}

func TestSchemaCoversReportAndValueTypes024(t *testing.T) {
	data, e := os.ReadFile("../../docs/schema/pruefbericht-v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var schema struct {
		Properties map[string]any `json:"properties"`
		Defs       map[string]struct {
			Properties map[string]any `json:"properties"`
		} `json:"$defs"`
	}
	if e = json.Unmarshal(data, &schema); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		typ   reflect.Type
		props map[string]any
	}{{reflect.TypeOf(inspection.Report{}), schema.Properties}, {reflect.TypeOf(inspection.ReportRevision{}), schema.Defs["reportRevision"].Properties}, {reflect.TypeOf(inspection.Config{}), schema.Defs["config"].Properties}, {reflect.TypeOf(inspection.Step{}), schema.Defs["step"].Properties}, {reflect.TypeOf(inspection.HeartbeatObservation{}), schema.Defs["heartbeat"].Properties}, {reflect.TypeOf(inspection.LimitChange{}), schema.Defs["limitChange"].Properties}} {
		for i := 0; i < tc.typ.NumField(); i++ {
			key := strings.Split(tc.typ.Field(i).Tag.Get("json"), ",")[0]
			if key != "" && key != "-" && tc.props[key] == nil {
				t.Error("schema missing", tc.typ.Name(), key)
			}
		}
	}
}
