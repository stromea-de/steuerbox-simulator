package inspection

import "testing"

// 042 (Nutzer 2026-11-14): the result title names the checked scope ("LPC-Funktionsprüfung bestanden").
func TestResultTitleNamesScope042(t *testing.T) {
	cases := []struct{ scope, profile, outcome, abort, want string }{
		{ScopeConsumption, "quick", "Bestanden", "", "LPC-Funktionsprüfung bestanden"},
		{ScopeProduction, "quick", "Bestanden", "", "LPP-Funktionsprüfung bestanden"},
		{ScopeBoth, "quick", "Bestanden", "", "LPC/LPP-Funktionsprüfung bestanden"},
		{ScopeBoth, "full", "Bestanden", "", "LPC/LPP-Langzeitprüfung bestanden"},
		{ScopeConsumption, "quick", "Nicht bestanden", "", "LPC-Funktionsprüfung nicht bestanden"},
		{ScopeProduction, "full", "Unvollständig", "", "LPP-Langzeitprüfung unvollständig"},
		{ScopeConsumption, "quick", "Unvollständig", "connection_lost", "LPC-Funktionsprüfung unvollständig"},
		{ScopeBoth, "quick", "Unvollständig", "user_cancel", "LPC/LPP-Funktionsprüfung abgebrochen"},
		{ScopeConsumption, "quick", "Nicht bestanden", "user_cancel", "LPC-Funktionsprüfung nicht bestanden"},
		// Legacy reports without a recorded scope keep their previous title.
		{"", "quick", "Bestanden", "", "Funktionsprüfung bestanden"},
		{"", "quick", "Unvollständig", "user_cancel", "Unvollständig"},
	}
	for _, c := range cases {
		cfg := Config{Scope: c.scope, TestProfile: c.profile}
		if got := ResultTitle(cfg, c.outcome, c.abort); got != c.want {
			t.Errorf("%+v: %q, want %q", c, got, c.want)
		}
	}
}

func TestReportTitleNamesScope042(t *testing.T) {
	cases := []struct{ mode, scope, want string }{
		{ModeOnSite, ScopeConsumption, "LPC-Prüfung am Gerät"},
		{ModeOnSite, ScopeProduction, "LPP-Prüfung am Gerät"},
		{ModeOnSite, ScopeBoth, "LPC/LPP-Prüfung am Gerät"},
		{ModeSimulator, ScopeBoth, "LPC/LPP-Prüfung im Simulator"},
		{ModeOnSite, "", "Prüfung am Gerät"},
		{ModeSimulator, "", "Prüfung im Simulator"},
	}
	for _, c := range cases {
		r := Report{Mode: c.mode, Config: Config{Scope: c.scope}}
		if got := r.ReportTitle(); got != c.want {
			t.Errorf("%+v: %q, want %q", c, got, c.want)
		}
	}
}

// 042: the failsafe restore status is classified once; the texts live in the report/web layers.
func TestFailsafeRestoreStatus042(t *testing.T) {
	ok := []Step{{Name: "LPC Failsafe-Leistung zurückstellen", Passed: true}, {Name: "LPC Failsafe-Mindestdauer zurückstellen", Passed: true}}
	originals := map[string]FailsafeValues{"LPC": {Power: 900}}
	if got := (Report{Mode: ModeSimulator}).FailsafeRestore(); got != "simulated" {
		t.Fatal(got)
	}
	if got := (Report{Mode: ModeOnSite, Steps: ok, FailsafeOriginals: originals}).FailsafeRestore(); got != "confirmed" {
		t.Fatal(got)
	}
	failed := []Step{ok[0], {Name: ok[1].Name, Passed: false}}
	if got := (Report{Mode: ModeOnSite, Steps: failed, FailsafeOriginals: originals}).FailsafeRestore(); got != "unconfirmed" {
		t.Fatal(got)
	}
	configured := []Step{{Name: "Failsafe-Werte setzen", Passed: true}}
	if got := (Report{Mode: ModeOnSite, Steps: configured, FailsafeOriginals: originals}).FailsafeRestore(); got != "unconfirmed" {
		t.Fatal("written values without restore step are never confirmed", got)
	}
	// R042-01: cancelled before configuring, or originals unreadable (configuration skipped): nothing was changed.
	for _, steps := range [][]Step{nil, {{Name: "Last vor Start prüfen", Passed: true}}, {{Name: "Failsafe-Werte setzen", Status: NotChecked, Detail: "Keine Änderung ohne lesbare Ursprungswerte"}}} {
		r := Report{Mode: ModeOnSite, Steps: steps}
		if got := r.FailsafeRestore(); got != "unchanged" || r.FailsafeNote() != "" {
			t.Fatal(steps, got, r.FailsafeNote())
		}
	}
	note := (Report{Mode: ModeOnSite, Steps: failed, FailsafeOriginals: originals}).FailsafeNote()
	if note != FailsafeWarningWeb {
		t.Fatal(note)
	}
	if (Report{Mode: ModeOnSite, Steps: ok, FailsafeOriginals: originals}).FailsafeNote() != "" {
		t.Fatal("confirmed restore needs no note")
	}
}
