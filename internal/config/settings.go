package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const settingsFileName = "settings.json"

// Settings are app preferences remembered between sessions. They live in
// their own file because the config editor rewrites config.yaml wholesale.
// Each feature gets its own section so new settings can be added later.
type Settings struct {
	LiveView LiveViewPrefs `json:"live_view"`
}

// LiveViewPrefs are the live-capture view settings.
type LiveViewPrefs struct {
	LineEnding string `json:"line_ending"`
	Echo       bool   `json:"echo"`
	Timestamps bool   `json:"timestamps"`
}

// DefaultSettings returns LF line endings, echo on and timestamps off.
func DefaultSettings() Settings {
	return Settings{
		LiveView: LiveViewPrefs{LineEnding: "LF", Echo: true},
	}
}

// SettingsPath returns ~/.astrolabe/settings.json.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate settings file: %w", err)
	}
	return filepath.Join(home, ".astrolabe", settingsFileName), nil
}

// LoadSettingsFrom reads settings from path. A missing file, or fields
// missing from the file, fall back to DefaultSettings.
func LoadSettingsFrom(path string) (Settings, error) {
	settings := DefaultSettings()
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("read settings: %w", err)
	}
	if err := json.Unmarshal(payload, &settings); err != nil {
		return DefaultSettings(), fmt.Errorf("decode settings: %w", err)
	}
	return settings, nil
}

// SaveSettingsTo writes settings to path, creating its directory. Top-level
// sections this version does not know about are kept, so an older build
// never drops settings written by a newer one.
func SaveSettingsTo(path string, settings Settings) error {
	merged := map[string]json.RawMessage{}
	if existing, err := os.ReadFile(path); err == nil {
		// A corrupt file is simply replaced.
		_ = json.Unmarshal(existing, &merged)
	}

	known, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(known, &sections); err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	for k, v := range sections {
		merged[k] = v
	}

	payload, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}

// UpdateSettingsAt loads settings from path, applies fn and saves the result,
// so changing one section leaves the others as they were.
func UpdateSettingsAt(path string, fn func(*Settings)) error {
	// A load error still returns usable defaults; saving then repairs a
	// corrupt file instead of failing every update.
	settings, _ := LoadSettingsFrom(path)
	fn(&settings)
	return SaveSettingsTo(path, settings)
}

// LoadSettings reads settings from the default location.
func LoadSettings() (Settings, error) {
	path, err := SettingsPath()
	if err != nil {
		return DefaultSettings(), err
	}
	return LoadSettingsFrom(path)
}

// UpdateSettings applies fn to the settings at the default location.
func UpdateSettings(fn func(*Settings)) error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	return UpdateSettingsAt(path, fn)
}
