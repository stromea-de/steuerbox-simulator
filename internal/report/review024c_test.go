package report

import (
	"bytes"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"regexp"
	"strings"
	"testing"
)

func TestConceptMatrix024c(t *testing.T) {
	r := sampleReport(t)
	d := &document{}
	d.newPage()
	d.resultMatrix(r)
	text := d.current().String()
	for _, want := range []string{"Grenze angenommen", "Grenze wirkt", "Notbetrieb bei Ausfall", "Rückkehr zum Normalbetrieb", "Bemerkung"} {
		expected := 2
		if want == "Bemerkung" {
			expected = 1
		}
		if strings.Count(text, literal(want)) != expected {
			t.Errorf("expected two rows/headings for %q", want)
		}
	}
	for _, bad := range []string{"Ausgangsleistung messen", "Mindestdauer abwarten", "Haken: bestanden"} {
		if strings.Contains(text, literal(bad)) {
			t.Error("internal/obsolete text", bad)
		}
	}
}
func TestSelectedOutcomeIsCheck024c(t *testing.T) {
	r := sampleReport(t)
	d := &document{}
	d.newPage()
	d.summary(r)
	// A check has two joined strokes; the old cross had two opposite diagonals.
	strokes := regexp.MustCompile(`2\.20 w ([0-9.]+) ([0-9.]+) m ([0-9.]+) ([0-9.]+) l`).FindAllStringSubmatch(d.current().String(), -1)
	if len(strokes) != 2 || strokes[0][3] != strokes[1][1] || strokes[0][4] != strokes[1][2] {
		t.Fatal("selected result needs two joined check strokes, not crossed diagonals", strokes)
	}
}
func TestEmptyHeaderBlocks024c(t *testing.T) {
	r := sampleReport(t)
	r.Business = ""
	r.Inspector = ""
	d := &document{}
	d.newPage()
	d.formHeader(r)
	for _, bad := range []string{"Anlagenbetreiber", "Anlage", "Prüfer", "Notiz"} {
		if strings.Contains(d.current().String(), "("+literal(bad)+")") {
			t.Error("empty block", bad)
		}
	}
}

// syntheticAbortReport024c exercises disconnect and low-load advice without
// depending on private hardware reports or customer/device identifiers.
func syntheticAbortReport024c(t *testing.T) inspection.Report {
	t.Helper()
	r := sampleReport(t)
	r.Mode = inspection.ModeOnSite
	r.SchemaVersion = 1
	r.Config.Scope = inspection.ScopeConsumption
	r.Config.TestProfile = "quick"
	r.AbortReason = "connection_lost"
	r.Passed = false
	r.Events = []inspection.Step{{Name: "Verbindungsabbruch", At: r.Started}}
	r.Steps = []inspection.Step{
		{Name: "Verbindung", Passed: true, At: r.Started},
		{Name: "LPC Grenze + Quittung", Passed: false, At: r.Started, ReasonCode: "connection_lost"},
		{Name: "Ausgangsmessung MPC", Status: inspection.NotChecked, ReasonCode: "insufficient_load", At: r.Started},
		{Name: "Grenzen aufheben", Passed: true, At: r.Finished},
	}
	return r
}

func TestAbortAndLoadAdvice024c(t *testing.T) {
	r := syntheticAbortReport024c(t)
	for _, want := range []string{"Verbindung stabil machen, etwa per Netzwerkkabel.", "Vor dem Start Last einschalten: mehr als 4.250 W.", "Prüfung wiederholen."} {
		if !strings.Contains(strings.Join(nextActions(r), "\n"), want) {
			t.Fatal("missing synthetic advice", want)
		}
	}
	before, _ := JSON(r)
	pdf, e := PDF(r)
	if e != nil {
		t.Fatal(e)
	}
	text := pdfText(t, pdf)
	for _, want := range []string{"Verbindung stabil machen, etwa per Netzwerkkabel.", "Vor dem Start Last einschalten: mehr als 4.250 W.", "Prüfung wiederholen.", "Verbindungsabbruch", "1 Nachweis", "Bemerkung"} {
		if !strings.Contains(text, want) {
			t.Error("missing", want)
		}
	}
	for _, bad := range []string{"Std..", "SPINE DeviceClassification; fehlende Angaben", "Marke/Modell", "Kontrollsignal", "keine Angabe einer amtlichen Prüfdauer", "Prüfnummer · Protokoll-ID"} {
		if strings.Contains(text, bad) {
			t.Error("obsolete text", bad)
		}
	}
	after, _ := JSON(r)
	if !bytes.Equal(before, after) || !bytes.Contains(pdf, before) {
		t.Fatal("original evidence changed")
	}
}

func TestGroupedEvidenceCannotHideFailures024c(t *testing.T) {
	r := sampleReport(t)
	r.Config.Scope = inspection.ScopeConsumption
	r.Mode = inspection.ModeOnSite
	r.Steps = append(r.Steps, inspection.Step{Name: "LPC Vorzeitige Freigabe", Passed: true}, inspection.Step{Name: "LPC Vorzeitige Rückkehr gemessene Wirkung", Passed: true})
	checks := formChecks(r, "LPC")
	if len(checks) != 4 || checkpointStatus(r, checks[3]) != "bestanden" {
		t.Fatal(checks)
	}
	r.Steps = append(r.Steps, inspection.Step{Name: "LPC Vorzeitige Freigabe", Passed: false})
	if checkpointStatus(r, checks[3]) != "nicht bestanden" {
		t.Fatal("later/earlier success hides failed release")
	}
	r.Config.TestProfile = "full"
	checks = formChecks(r, "LPC")
	if len(checks) != 5 || checks[4].text != "Notbetrieb hält Mindestdauer" {
		t.Fatal(checks)
	}
}
func TestLaterDisconnectDoesNotExplainEarlierRejection024c(t *testing.T) {
	r := sampleReport(t)
	r.AbortReason = "connection_lost"
	r.Steps[5].Passed = false
	r.Steps[5].Detail = "SPINE result 7, Nachricht 80"
	if got := formRemark(r, formChecks(r, "LPC")[0]); strings.Contains(got, "Verbindung riss ab") {
		t.Fatal("negative device response is not a lost connection:", got)
	}
}
