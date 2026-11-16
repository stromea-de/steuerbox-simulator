package webui

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileRoute024b(t *testing.T) {
	a := New(nil)
	defer a.Close()
	r := httptest.NewRequest("GET", "http://127.0.0.1:8080/profil", nil)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if !strings.Contains(w.Body.String(), `href="/profil"`) {
		t.Fatal("missing profile navigation")
	}
	if strings.Contains(w.Body.String(), `id="remember-site"`) {
		t.Fatal("customer persistence still present")
	}
}
