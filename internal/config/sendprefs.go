package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const sendPrefsFileName = "send_prefs.json"

// SendPrefs are the live-capture send settings remembered between sessions.
// They live in their own file because the config editor rewrites config.yaml
// wholesale.
type SendPrefs struct {
	LineEnding string `json:"line_ending"`
	Echo       bool   `json:"echo"`
}

// DefaultSendPrefs returns LF line endings with echo on.
func DefaultSendPrefs() SendPrefs {
	return SendPrefs{LineEnding: "LF", Echo: true}
}

// SendPrefsPath returns ~/.astrolabe/send_prefs.json.
func SendPrefsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate send prefs file: %w", err)
	}
	return filepath.Join(home, ".astrolabe", sendPrefsFileName), nil
}

// LoadSendPrefsFrom reads prefs from path. A missing file, or fields missing
// from the file, fall back to DefaultSendPrefs.
func LoadSendPrefsFrom(path string) (SendPrefs, error) {
	prefs := DefaultSendPrefs()
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return prefs, nil
	}
	if err != nil {
		return prefs, fmt.Errorf("read send prefs: %w", err)
	}
	if err := json.Unmarshal(payload, &prefs); err != nil {
		return DefaultSendPrefs(), fmt.Errorf("decode send prefs: %w", err)
	}
	return prefs, nil
}

// SaveSendPrefsTo writes prefs to path, creating its directory.
func SaveSendPrefsTo(path string, prefs SendPrefs) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create send prefs directory: %w", err)
	}
	payload, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return fmt.Errorf("encode send prefs: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write send prefs: %w", err)
	}
	return nil
}

// LoadSendPrefs reads prefs from the default location.
func LoadSendPrefs() (SendPrefs, error) {
	path, err := SendPrefsPath()
	if err != nil {
		return DefaultSendPrefs(), err
	}
	return LoadSendPrefsFrom(path)
}

// SaveSendPrefs writes prefs to the default location.
func SaveSendPrefs(prefs SendPrefs) error {
	path, err := SendPrefsPath()
	if err != nil {
		return err
	}
	return SaveSendPrefsTo(path, prefs)
}
