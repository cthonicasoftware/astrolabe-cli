package cliout

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestInfoJSONEscapesMessage(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, true)

	p.Info("hello \"json\"\nworld")

	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput=%q", err, buf.String())
	}

	if got["level"] != "info" {
		t.Fatalf("level = %v, want info", got["level"])
	}
	if got["message"] != "hello \"json\"\nworld" {
		t.Fatalf("message = %q, want %q", got["message"], "hello \"json\"\nworld")
	}
}

func TestKeyValueJSONIncludesObjectData(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, true)

	p.KeyValue("run_id", "abc-123")

	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput=%q", err, buf.String())
	}

	if got["level"] != "kv" {
		t.Fatalf("level = %v, want kv", got["level"])
	}
	if got["message"] != "" {
		t.Fatalf("message = %q, want empty string", got["message"])
	}

	data, ok := got["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data has unexpected type: %T", got["data"])
	}
	if data["run_id"] != "abc-123" {
		t.Fatalf("data.run_id = %v, want abc-123", data["run_id"])
	}
}

func TestJSONModeMutedAndBlankAreNoop(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinter(&buf, true)

	p.Muted("hidden")
	p.Blank()

	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

func TestSuccessPlainRespectsNoColorAndNoIcons(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("NO_ICONS", "1")

	var buf bytes.Buffer
	p := NewPrinter(&buf, false)
	p.Success("done")

	if got := buf.String(); got != "done\n" {
		t.Fatalf("output = %q, want %q", got, "done\n")
	}
}

func TestKeyValuePlainFormatting(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	var buf bytes.Buffer
	p := NewPrinter(&buf, false)
	p.KeyValue("Run ID", "01ABC")

	if got := buf.String(); got != "  Run ID: 01ABC\n" {
		t.Fatalf("output = %q, want %q", got, "  Run ID: 01ABC\n")
	}
}
