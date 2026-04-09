package runs

import (
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

func TestFormatDuration(t *testing.T) {
	if got := FormatDuration(0, true); got != "-" {
		t.Fatalf("expected '-', got %q", got)
	}
	if got := FormatDuration(0, false); got != "in-progress" {
		t.Fatalf("expected in-progress, got %q", got)
	}
	if got := FormatDuration(90, true); got != "1m30s" {
		t.Fatalf("unexpected formatted duration: %q", got)
	}
}

func TestFormatSource(t *testing.T) {
	meta := core.SourceMeta{Kind: "serial", Port: "/dev/ttyUSB0"}
	if got := FormatSource(meta); got != "serial:/dev/ttyUSB0" {
		t.Fatalf("unexpected source format: %q", got)
	}

	meta = core.SourceMeta{Kind: "file", Path: "/tmp/data.csv"}
	if got := FormatSource(meta); got != "file:/tmp/data.csv" {
		t.Fatalf("unexpected source path format: %q", got)
	}
}

func TestFormatTestPlan(t *testing.T) {
	testInfo := core.TestInfo{Plan: "smoke", Variant: "fast"}
	if got := FormatTestPlan(testInfo); got != "smoke/fast" {
		t.Fatalf("unexpected test plan format: %q", got)
	}

	testInfo = core.TestInfo{Plan: "burn-in"}
	if got := FormatTestPlan(testInfo); got != "burn-in" {
		t.Fatalf("unexpected variantless format: %q", got)
	}
}
