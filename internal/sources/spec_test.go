package sources

import (
	"path/filepath"
	"testing"
)

// TestSpecConformance asserts the invariants every capture source must hold.
// New sources are covered by adding a single entry here.
func TestSpecConformance(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sample.log")

	specs := []struct {
		name string
		spec Spec
		kind string
	}{
		{"serial", Config{Port: "COM3", Baud: 115200}, "serial"},
		{"tcp", TCPConfig{Host: "localhost", Port: 9000}, "tcp"},
		{"file", FileConfig{Path: tmpFile}, "file"},
	}

	for _, tc := range specs {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.spec.Kind(); got != tc.kind {
				t.Errorf("Kind() = %q, want %q", got, tc.kind)
			}

			attrs := tc.spec.ManifestAttrs()
			if got := attrs["source_kind"]; got != tc.spec.Kind() {
				t.Errorf("ManifestAttrs()[source_kind] = %q, want %q", got, tc.spec.Kind())
			}
			for k, v := range attrs {
				if k == "" || v == "" {
					t.Errorf("ManifestAttrs() has empty key or value: %q=%q", k, v)
				}
			}

			if got := tc.spec.CaptureSettings().Notes; got == "" {
				t.Error("CaptureSettings().Notes is empty, want a human-readable description")
			}
		})
	}
}

// TestSpecManifestAttrsIsolated guards the capture service, which mutates the
// returned map to add run_label and source_format.
func TestSpecManifestAttrsIsolated(t *testing.T) {
	cfg := TCPConfig{Host: "localhost", Port: 9000}

	first := cfg.ManifestAttrs()
	first["run_label"] = "mutated"

	if _, ok := cfg.ManifestAttrs()["run_label"]; ok {
		t.Error("ManifestAttrs() returned a shared map; mutation leaked into a later call")
	}
}
