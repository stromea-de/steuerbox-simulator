package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClaimRecognizesOnlyOurInstance(t *testing.T) {
	for _, body := range []string{`{"application":"stromea-steuerbox-simulator/desktop-v1"}`, `{"application":"other"}`, `not json`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/instance" {
				t.Error(r.URL.Path)
			}
			w.Write([]byte(body))
		}))
		l, existing, e := Claim(s.Listener.Addr().String())
		s.Close()
		expected := body == `{"application":"stromea-steuerbox-simulator/desktop-v1"}`
		if l != nil || existing != expected || (e == nil) != expected {
			t.Fatalf("%s %v %v %v", body, l, existing, e)
		}
	}
	l, existing, e := Claim("127.0.0.1:0")
	if e != nil || existing || l == nil {
		t.Fatal(l, existing, e)
	}
	l.Close()
}
func TestClaimDoesNotFollowRedirect(t *testing.T) {
	visited := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visited = true
		w.Write([]byte(`{"application":"stromea-steuerbox-simulator/desktop-v1"}`))
	}))
	defer other.Close()
	s := httptest.NewServer(http.RedirectHandler(other.URL, 302))
	defer s.Close()
	_, existing, e := Claim(s.Listener.Addr().String())
	if e == nil || existing || visited {
		t.Fatal(existing, e, visited)
	}
}
