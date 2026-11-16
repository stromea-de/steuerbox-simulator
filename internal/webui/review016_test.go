package webui

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdvertisedScopeAndNoControlGate016(t *testing.T) {
	for _, tc := range []struct {
		caps   inspection.Capabilities
		scope  string
		reject bool
	}{
		{inspection.Capabilities{LPC: true}, "consumption", false},
		{inspection.Capabilities{LPP: true}, "production", false},
		{inspection.Capabilities{LPC: true, LPP: true}, "both", false},
		{inspection.Capabilities{MPC: true}, "", true},
	} {
		c := inspection.DefaultConfig()
		err := deviceConfig(&c, &tc.caps, true)
		if (err != nil) != tc.reject {
			t.Fatal(tc, err)
		}
		if !tc.reject && (c.Scope != tc.scope || !tc.caps.LPC && c.LPC != 0 || !tc.caps.LPP && c.LPP != 0) {
			t.Fatal(tc, c)
		}
	}
	c := inspection.DefaultConfig()
	caps := inspection.Capabilities{LPC: true}
	if deviceConfig(&c, &caps, false) == nil || deviceConfig(&c, nil, true) == nil {
		t.Fatal("no connection or unknown capabilities accepted")
	}
}
func TestEmbeddedOfflineLanguageAssets016(t *testing.T) {
	app := New(nil)
	defer app.Close()
	for _, name := range []string{"i18n-data.js", "i18n.js"} {
		w := httptest.NewRecorder()
		app.handler.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:8080/"+name, nil))
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "javascript") || w.Body.Len() < 1000 {
			t.Fatal(name, w.Code, w.Body.Len())
		}
	}
	for _, lang := range []string{"de", "en", "pl", "ru", "uk", "tr", "ro"} {
		if _, err := assets.ReadFile("assets/i18n/" + lang + ".json"); err != nil {
			t.Fatal(err)
		}
	}
}
