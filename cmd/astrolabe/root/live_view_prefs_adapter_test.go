package root

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSettings(t *testing.T, home, body string) {
	t.Helper()
	dir := filepath.Join(home, ".astrolabe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadSettings_MissingReconnectDefaultsOn: settings written before
// reconnect existed must load with reconnect on.
func TestLoadSettings_MissingReconnectDefaultsOn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	var a liveViewPrefsAdapter

	writeSettings(t, home, `{"live_view":{"line_ending":"LF","echo":true}}`)
	p, err := a.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !p.Reconnect {
		t.Error("missing reconnect key loaded as off, want on")
	}

	writeSettings(t, home, `{"live_view":{"reconnect":false}}`)
	p, err = a.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p.Reconnect {
		t.Error("reconnect:false loaded as on")
	}

	writeSettings(t, home, `{"live_view":{"line_ending":"LF","echo":true}}`)
	p.Reconnect = false
	if err := a.Save(p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := a.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Reconnect {
		t.Error("saved reconnect off, loaded on")
	}
}
