package webui

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRenamedPagesAndTrademarkNotice(t *testing.T) {
	const notice = "Die EEBUS-Wort-/Bildmarke ist eine eingetragene Marke der EEBus Initiative e.V. EEBUS bezeichnet hier den unterstützten Kommunikationsstandard. Der Stromea Steuerbox-Simulator ist ein unabhängiges Open-Source-Projekt und wird weder von der EEBus Initiative e.V. noch von der EEBUService GmbH herausgegeben oder zertifiziert. Er ist keine Steuerbox-Hardware und ersetzt keine Konformitätsprüfung."
	a := New(nil)
	defer a.Close()
	for _, path := range []string{"/", "/protokoll"} {
		w := httptest.NewRecorder()
		a.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:33287"+path, nil))
		body := w.Body.String()
		if regexp.MustCompile(`(?i)\b(konform|zertifiziert)(e|er|es|en|em)?\b`).MatchString(strings.ReplaceAll(body, notice, "")) {
			t.Errorf("%s contains a conformity/certification assertion", path)
		}
		for _, want := range []string{"Stromea Steuerbox-Simulator", "Prüft §14a- und Einspeisesteuerung über die EEBUS-Schnittstelle.", notice} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: missing %q", path, want)
			}
		}
		if path == "/" && !strings.Contains(body, "Über den Stromea Steuerbox-Simulator") {
			t.Error("missing About section")
		}
		for _, forbidden := range []string{"Lucas", "Spohn", "Stromea EEBUS Prüftool", "Stromea Prüftool", "0100-600"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q", path, forbidden)
			}
		}
		if os.Getenv("STROMEA_007_UI_ARTIFACTS") == "1" {
			// Rendered by the real handlers, static preview only: no script,
			// live session token, remote assets or network needed for capture.
			body = strings.ReplaceAll(body, a.token, "static-preview")
			body = strings.ReplaceAll(body, `<script src="/app.js" defer></script>`, "")
			body = strings.ReplaceAll(body, `href="/style.css"`, `href="007-style.css"`)
			name := "007-web-start.html"
			if path == "/protokoll" {
				name = "007-web-download.html"
			}
			if err := os.MkdirAll("../../artifacts", 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join("../../artifacts", name), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			css, err := assets.ReadFile("assets/style.css")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("../../artifacts/007-style.css", css, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
