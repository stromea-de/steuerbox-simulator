package session

import (
	"context"
	spineapi "github.com/enbility/spine-go/api"
	"github.com/enbility/spine-go/model"
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"strings"
	"testing"
	"time"
)

func TestFullLibraryInspection(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Pair("0000000000000000000000000000000000000000"); err == nil {
		t.Fatal("wrong SKI accepted")
	}
	if err := s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	c := inspection.DefaultConfig()
	c.Business = "Elektro Muster"
	c.Inspector = "Erika Prüfer"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := s.Run(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range r.Steps {
		t.Log(step.Name, step.Passed, step.Detail)
	}
	if !r.Passed {
		t.Fatal("inspection failed")
	}
	again, err := s.Run(ctx, c)
	if err != nil || !again.Passed {
		t.Fatalf("repeat failed: %+v %v", again, err)
	}
}

func TestTrustSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	first, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.Pair(first.Device.SKI); err != nil {
		t.Fatal(err)
	}
	first.Close()
	second, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if !second.Paired() {
		t.Fatal("explicit mutual trust was not restored")
	}
}

func TestSPINENegativeAcknowledgement(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err = s.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	f := s.CS.Entity.FeatureOfTypeAndRole(model.FeatureTypeTypeLoadControl, model.RoleTypeServer)
	if err = f.AddWriteApprovalCallback(func(msg *spineapi.Message) {
		f.ApproveOrDenyWrite(msg, model.ErrorType{ErrorNumber: model.ErrorNumberTypeCommandRejected})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Limit(ctx, "LPC", 4200, true); err == nil || !strings.Contains(err.Error(), "SPINE-Quittung") {
		t.Fatalf("negative result did not fail: %v", err)
	}
}
