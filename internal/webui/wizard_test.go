package webui

import (
	"github.com/stromea-de/steuerbox-simulator/internal/inspection"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWizardStructure(t *testing.T) {
	a := New(nil)
	defer a.Close()
	w := httptest.NewRecorder()
	a.index(w, httptest.NewRequest("GET", "/", nil))
	body := w.Body.String()
	shared, err := assets.ReadFile("assets/result.js")
	if err != nil {
		t.Fatal(err)
	}
	body += string(shared)
	for _, want := range []string{`id="step-1"`, `id="step-2" hidden`, `id="step-3" hidden`, `aria-current="step"`, `id="pair-error"`, `id="config-error"`, `id="run-error"`, `id="manual-device"`, `id="progress"`, `aria-live="polite"`, "Prüfablauf", "https://stromea.de/simulator", "https://github.com/stromea-de/steuerbox-simulator"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, old := range []string{`id="authorized"`, `id="confirm-ski"`, `id="connect"`, "Ich bin befugt"} {
		if strings.Contains(body, old) {
			t.Errorf("obsolete gate: %q", old)
		}
	}
	if !(strings.Index(body, `id="discover"`) < strings.Index(body, `id="device-choices"`) && strings.Index(body, `id="device-choices"`) < strings.Index(body, `id="manual-device"`)) {
		t.Error("discovery flow out of order")
	}
}

func TestWizardDownloadPreview009(t *testing.T) {
	a := New(nil)
	defer a.Close()
	a.result = &inspection.Report{Mode: inspection.ModeSimulator, Business: "Elektro Muster GmbH", Inspector: "Erika Muster", Device: inspection.Device{Name: "Loopback-Simulator", Brand: "Stromea", Model: "CS/MU"}, Started: time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC), Passed: true}
	for _, group := range a.result.RequiredStepGroups() {
		for _, name := range group {
			a.result.Steps = append(a.result.Steps, inspection.Step{Name: name, Passed: true})
		}
	}
	w := httptest.NewRecorder()
	a.downloadPage(w, httptest.NewRequest("GET", "/protokoll", nil))
	for _, want := range []string{"Prüfergebnis und Prüfbericht", "Erika Muster", "Loopback-Simulator", "26.10.2026", "Bestanden", "https://github.com/stromea-de/steuerbox-simulator"} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
	if os.Getenv("STROMEA_009_UI_ARTIFACTS") == "1" {
		css, err := assets.ReadFile("assets/style.css")
		if err != nil {
			t.Fatal(err)
		}
		html := strings.Replace(w.Body.String(), `<link rel="stylesheet" href="/style.css">`, "<style>"+string(css)+"</style>", 1)
		if err := os.MkdirAll("../../artifacts", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("../../artifacts/009-protokoll.html", []byte(html), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
