package webui

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
)

func Test036bMeterLinesPreserveIdentifier(t *testing.T) {
	for _, value := range []string{"", "DE-123", "DE-000123456789012345678901234567890", strings.Repeat("α😀", 20)} {
		e := reportListEntry{Report: inspection.Report{Config: inspection.Config{Site: inspection.Site{Meter: value}}}}
		lines := e.MeterLines()
		if len(lines) > 2 {
			t.Fatal("identifier takes more than two lines")
		}
		want := value
		if want == "" {
			want = "–"
		}
		if strings.Join(lines, "") != want {
			t.Fatal("identifier changed")
		}
	}
}
