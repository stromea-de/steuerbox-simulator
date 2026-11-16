package session

import (
	"github.com/stromea-de/steuerbox-simulator/internal/identity"
	"path/filepath"
	"testing"
)

func TestIdentityResetIsExplicitAndPersists(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	before := s.EG.Identity.SKI
	if err = s.ResetIdentity(false); err == nil || s.EG.Identity.SKI != before {
		t.Fatal("confirmation missing")
	}
	if err = s.Pair(s.Device.SKI); err != nil {
		t.Fatal(err)
	}
	if err = s.ResetIdentity(true); err != nil {
		t.Fatal(err)
	}
	id, err := identity.LoadOrCreate(filepath.Join(dir, "energyguard"))
	if err != nil || id.SKI == before || id.SKI != s.EG.Identity.SKI || s.Paired() {
		t.Fatal(err, id.SKI, before)
	}
	restarted, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if restarted.Paired() || restarted.EG.Identity.SKI != id.SKI {
		t.Fatal("stale trust restored")
	}
}
