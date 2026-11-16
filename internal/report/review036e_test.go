package report

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReleaseSentence036e(t *testing.T) {
	for _, confirmed := range []bool{true, false} {
		mock := &releaseMock036e{confirmed: confirmed, powers: []float64{11000, 4200, 500, 4200, -3000, -500, -3000, -8000}}
		c := inspection.DefaultConfig()
		c.Business = "Elektro Muster"
		c.Inspector = "Beispiel Fachkraft"
		c.StepTimeout = 30 * time.Millisecond
		r, err := inspection.Run(context.Background(), c, inspection.Device{Name: "Mock 036e", SKI: "0123456789abcdef0123456789abcdef01234567"}, mock)
		if err != nil {
			t.Fatal(err)
		}
		release := r.Steps[len(r.Steps)-1]
		if release.Name != "Grenzen aufheben" || release.Passed != confirmed {
			t.Fatal("mock release not observed", release)
		}

		for _, row := range r.FunctionMatrix() {
			if strings.Contains(strings.ToLower(row.Title), "aufheben") {
				t.Fatal(row)
			}
		}
		if !confirmed && r.Outcome() == "Bestanden" {
			t.Fatal("unconfirmed release must not be reported as passed")
		}
		if note := r.FailsafeNote(); note != "" {
			t.Fatal("practice run needs no failsafe note", note)
		}
		data, err := PDF(r)
		if err != nil {
			t.Fatal(err)
		}
		if out := os.Getenv("RELEASE036E_OUTPUT"); out != "" {
			if err := os.MkdirAll(out, 0700); err != nil {
				t.Fatal(err)
			}
			name := "unconfirmed"
			if confirmed {
				name = "confirmed"
			}
			jsonData, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			// 036f: web fixtures reuse the overall status of this exact run.
			var fields map[string]any
			if err := json.Unmarshal(jsonData, &fields); err != nil {
				t.Fatal(err)
			}
			fields["outcome"] = r.Outcome()
			if jsonData, err = json.Marshal(fields); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name+".json"), jsonData, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, name+".pdf"), data, 0600); err != nil {
				t.Fatal(err)
			}
		}

		d := &document{}
		d.newPage()
		d.resultMatrix(r)
		want := "Aufheben der Testgrenzen gesendet, keine Bestätigung vom Gerät."
		if confirmed {
			want = "Die Testgrenzen wurden wieder aufgehoben."
		}
		if !strings.Contains(d.current().String(), literal(want)) {
			t.Fatal("release sentence missing", want)
		}
	}
}

// Mock transport runs the production inspection including cleanup. Only inactive
// limit commands lose their ACK in the negative case; measurements stay equal.
type releaseMock036e struct {
	confirmed bool
	powers    []float64
}

func (m *releaseMock036e) Connect(context.Context) error                      { return nil }
func (m *releaseMock036e) Supports(context.Context) error                     { return nil }
func (m *releaseMock036e) Configure(context.Context, inspection.Config) error { return nil }
func (m *releaseMock036e) Heartbeat(context.Context, bool) error              { return nil }
func (m *releaseMock036e) Limit(_ context.Context, _ string, _ float64, active bool) (string, error) {
	if !active && !m.confirmed {
		return "", errors.New("mock: no release acknowledgement")
	}
	return "SPINE result 0", nil
}
func (m *releaseMock036e) Power(context.Context) (float64, error) {
	if len(m.powers) == 0 {
		return 0, errors.New("mock: no measurement")
	}
	v := m.powers[0]
	m.powers = m.powers[1:]
	return v, nil
}
