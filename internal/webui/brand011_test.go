package webui

import (
	"strings"
	"testing"
)

func TestStromeaBrand011(t *testing.T) {
	css, _ := assets.ReadFile("assets/style.css")
	html, _ := assets.ReadFile("assets/index.html")
	for _, want := range []string{"--primary:#4C7A1E", "--stromea-lime:#7CB342", "accent-color:var(--stromea-lime)", "--primary:#8AC44B"} {
		if !strings.Contains(string(css), want) {
			t.Errorf("missing brand style %s", want)
		}
	}
	if !strings.Contains(string(html), `<svg class="brand-mark"`) || !strings.Contains(string(html), "M587 364C587 324") {
		t.Error("missing Stromea S vector")
	}
	for _, old := range []string{"#284D7C", "#a3c8ff", "#6084bc"} {
		if strings.Contains(string(css), old) {
			t.Errorf("obsolete blue %s", old)
		}
	}
}
