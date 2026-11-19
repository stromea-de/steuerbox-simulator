package eebus

import (
	"testing"
	"time"

	"github.com/enbility/spine-go/model"
)

// 046 (R046-18): only absolute SPINE times are measurement evidence.
func TestMeasuredAtOnlyAbsolute046(t *testing.T) {
	at := time.Date(2026, 11, 18, 9, 26, 22, 0, time.UTC)
	absolute := model.NewAbsoluteOrRelativeTimeTypeFromTime(at)
	if got := measuredAt(absolute); !got.Equal(at) {
		t.Fatal(got)
	}
	relative := model.NewAbsoluteOrRelativeTimeTypeFromDuration(-5 * time.Second)
	if got := measuredAt(relative); !got.IsZero() {
		t.Fatal("relative time must not count", got)
	}
	if !measuredAt(nil).IsZero() {
		t.Fatal("missing time")
	}
}
