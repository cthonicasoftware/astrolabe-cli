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
