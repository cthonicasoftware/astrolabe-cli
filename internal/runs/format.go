package runs

import (
	"fmt"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// FormatDuration renders a human-friendly duration string from seconds.
func FormatDuration(seconds float64, completed bool) string {
	if seconds <= 0 {
		if completed {
			return "-"
		}
		return "in-progress"
	}
	d := time.Duration(seconds * float64(time.Second))
	if d >= time.Hour {
		return d.Round(time.Second).String()
	}
	if d >= time.Minute {
		return d.Round(time.Second).String()
	}
	return d.Round(100 * time.Millisecond).String()
}

// FormatSource summarizes the capture source for display.
func FormatSource(meta core.SourceMeta) string {
	if meta.Kind == "" {
		return "unknown"
	}
	switch meta.Kind {
	case "serial":
		if meta.Port != "" {
			return fmt.Sprintf("serial:%s", meta.Port)
		}
	}
	if meta.Path != "" {
		return fmt.Sprintf("%s:%s", meta.Kind, meta.Path)
	}
	if meta.Addr != "" {
		return fmt.Sprintf("%s:%s", meta.Kind, meta.Addr)
	}
	return meta.Kind
}

// FormatTestPlan produces a compact representation of the test plan/variant.
func FormatTestPlan(test core.TestInfo) string {
	if test.Plan == "" {
		return "unspecified"
	}
	if test.Variant == "" {
		return test.Plan
	}
	return fmt.Sprintf("%s/%s", test.Plan, test.Variant)
}
