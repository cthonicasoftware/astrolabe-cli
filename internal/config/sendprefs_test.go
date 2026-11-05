package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSendPrefsMissingFileUsesDefaults(t *testing.T) {
	prefs, err := LoadSendPrefsFrom(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if prefs != DefaultSendPrefs() {
		t.Errorf("prefs = %+v, want defaults %+v", prefs, DefaultSendPrefs())
	}
}

func TestSendPrefsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", sendPrefsFileName)
	want := SendPrefs{LineEnding: "CRLF", Echo: false}
	if err := SaveSendPrefsTo(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadSendPrefsFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSendPrefsPartialFileKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), sendPrefsFileName)
	if err := os.WriteFile(path, []byte(`{"line_ending":"CR"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSendPrefsFrom(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.LineEnding != "CR" || !got.Echo {
		t.Errorf("got %+v, want CR with echo on", got)
	}
}
