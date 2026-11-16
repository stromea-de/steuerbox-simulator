package session

import (
	"context"
	"errors"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"testing"
	"time"
)

func TestLowLoadHasDeadlineAndTimeout023(t *testing.T) {
	s := &Session{lowLoadTimeout: 20 * time.Millisecond}
	done := make(chan error, 1)
	go func() { _, e := s.LowLoad(context.Background(), inspection.LowLoad{}); done <- e }()
	until := time.Now().Add(time.Second)
	for s.Progress().LowLoad == nil && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	p := s.Progress()
	if p.Deadline.IsZero() {
		t.Fatal("missing deadline")
	}
	if e := <-done; !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	if s.ResolveLowLoad(false) == nil {
		t.Fatal("stale decision accepted")
	}
}

func TestMissingAcknowledgementLatchesAndStopsFurtherWrites023(t *testing.T) {
	s := &Session{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if e := s.waitControlResult(ctx, 23, make(chan model.ResultDataType)); !errors.Is(e, inspection.ErrControlUnresponsive) {
		t.Fatal(e)
	}
	// Even an unchanged restoration target must not be called OK after a missing ACK.
	start := time.Now()
	if e := s.WriteFailsafe(context.Background(), "LPC", "minimum", inspection.FailsafeValues{Minimum: 2 * time.Hour}); !errors.Is(e, inspection.ErrControlUnresponsive) {
		t.Fatal(e)
	}
	if _, e := s.Limit(context.Background(), "LPC", 0, false); !errors.Is(e, inspection.ErrControlUnresponsive) {
		t.Fatal(e)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("second timeout instead of fail fast")
	}
}
func TestAcknowledgementResultMustBePositive023(t *testing.T) {
	for _, n := range []model.ErrorNumberType{0, 1} {
		s := &Session{}
		ch := make(chan model.ResultDataType, 1)
		ch <- model.ResultDataType{ErrorNumber: &n}
		e := s.waitControlResult(context.Background(), 7, ch)
		if (e == nil) != (n == 0) {
			t.Fatal(n, e)
		}
		if s.ControlError() != nil {
			t.Fatal("explicit rejection treated as unresponsive")
		}
	}
}

// Real TLS/SPINE, loopback only: reading an equal value must not mask dead writes.
func TestFreshReadButNoWriteAcknowledgement023(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	d := s.Device
	if e = s.Pair(d.SKI); e != nil {
		t.Fatal(e)
	}
	if e = s.SelectDevice(d, true); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if e = s.Connect(ctx); e != nil {
		t.Fatal(e)
	}
	original, e := s.ReadFailsafe(ctx, "LPC")
	if e != nil {
		t.Fatal(e)
	}
	if original.Source == "" || original.ReadAt.IsZero() {
		t.Fatal("fresh read lacks provenance", original)
	}
	feature := s.CS.Entity.FeatureOfTypeAndRole(model.FeatureTypeTypeDeviceConfiguration, model.RoleTypeServer)
	if e = feature.AddWriteApprovalCallback(func(*spineapi.Message) {}); e != nil {
		t.Fatal(e)
	}
	short, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()
	if e = s.WriteFailsafe(short, "LPC", "minimum", original); !errors.Is(e, inspection.ErrControlUnresponsive) {
		t.Fatal("equal fresh read hid missing write ACK", e)
	}
	got, e := s.ReadFailsafe(ctx, "LPC")
	if e != nil || got.Minimum != original.Minimum || !got.ReadAt.After(original.ReadAt) {
		t.Fatal("read path should remain fresh and usable", e, got)
	}
	before := s.EG
	if e = s.Rebuild(ctx); e != nil {
		t.Fatal("rebuild", e)
	}
	if before == s.EG || s.ControlError() != nil {
		t.Fatal("stale endpoint/latch survived rebuild")
	}
	caps, e := s.Capabilities(ctx)
	if e != nil || !caps.LPC {
		t.Fatal("detection not repeated", caps, e)
	}
	if len(s.ConnectionEvents()) < 3 {
		t.Fatal("missing timestamped connection events", s.ConnectionEvents())
	}
}

func TestChangingToPracticeClearsDeadEndpointState023(t *testing.T) {
	s, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	d := s.Device
	if e = s.SelectDevice(d, true); e != nil {
		t.Fatal(e)
	}
	s.controlErr = inspection.ErrControlUnresponsive
	if e = s.SelectSimulator(); e != nil {
		t.Fatal(e)
	}
	if s.IsDeviceMode() || s.ControlError() != nil {
		t.Fatal("old real-device failure leaked into new practice endpoint")
	}
}

func TestConnectionEventsDoNotLeakIntoNextReport023(t *testing.T) {
	s := &Session{}
	s.RecordConnection("Verbindungsabbruch", "Vorherige Prüfung", false)
	saved := s.ConnectionEvents()
	s.ResetProgress()
	if len(s.Progress().Events) != 0 {
		t.Fatal("previous customer's connection events leaked into new report")
	}
	if len(saved) != 1 {
		t.Fatal("reset changed saved snapshot")
	}
}
