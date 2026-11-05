package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsMissingFileUsesDefaults(t *testing.T) {
	got, err := LoadSettingsFrom(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != DefaultSettings() {
		t.Errorf("settings = %+v, want defaults %+v", got, DefaultSettings())
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", settingsFileName)
	want := Settings{LiveView: LiveViewPrefs{LineEnding: "CRLF", Echo: false, Timestamps: true}}
	if err := SaveSettingsTo(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadSettingsFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSettingsPartialFileKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), settingsFileName)
	if err := os.WriteFile(path, []byte(`{"live_view":{"line_ending":"CR"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettingsFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.LiveView.LineEnding != "CR" || !got.LiveView.Echo || got.LiveView.Timestamps {
		t.Errorf("got %+v, want CR with echo on, timestamps off", got.LiveView)
	}
}

func TestSettingsSaveKeepsUnknownSections(t *testing.T) {
	path := filepath.Join(t.TempDir(), settingsFileName)
	if err := os.WriteFile(path, []byte(`{"future":{"x":1},"live_view":{"echo":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := UpdateSettingsAt(path, func(s *Settings) { s.LiveView.Timestamps = true })
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatal(err)
	}
	var future struct{ X int }
	if err := json.Unmarshal(raw["future"], &future); err != nil || future.X != 1 {
		t.Errorf("future section = %s, want it kept", raw["future"])
	}
	got, _ := LoadSettingsFrom(path)
	if !got.LiveView.Timestamps {
		t.Error("timestamps not saved")
	}
}

func TestUpdateSettingsRepairsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), settingsFileName)
	if err := os.WriteFile(path, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpdateSettingsAt(path, func(s *Settings) { s.LiveView.Echo = false }); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := LoadSettingsFrom(path)
	if err != nil {
		t.Fatalf("load after repair: %v", err)
	}
	if got.LiveView.Echo || got.LiveView.LineEnding != "LF" {
		t.Errorf("got %+v, want defaults with echo off", got.LiveView)
	}
}
