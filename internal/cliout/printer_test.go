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

func TestJSONModeOutputMethods(t *testing.T) {
	tests := []struct {
		name      string
		call      func(p *Printer)
		wantLevel string
		wantMsg   string
	}{
		{"Success", func(p *Printer) { p.Success("ok") }, "success", "ok"},
		{"Error", func(p *Printer) { p.Error("bad") }, "error", "bad"},
		{"Warning", func(p *Printer) { p.Warning("warn") }, "warning", "warn"},
		{"Step", func(p *Printer) { p.Step("step1") }, "step", "step1"},
		{"Header", func(p *Printer) { p.Header("title") }, "header", "title"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := NewPrinter(&buf, true)
			tc.call(p)
			var got map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON: %v\noutput=%q", err, buf.String())
			}
			if got["level"] != tc.wantLevel {
				t.Fatalf("level = %v, want %q", got["level"], tc.wantLevel)
			}
			if got["message"] != tc.wantMsg {
				t.Fatalf("message = %v, want %q", got["message"], tc.wantMsg)
			}
		})
	}
}

func TestPlainModeOutputMethods(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("NO_ICONS", "1")

	tests := []struct {
		name string
		call func(p *Printer)
		want string
	}{
		{"Info", func(p *Printer) { p.Info("hello") }, "hello\n"},
		{"Error", func(p *Printer) { p.Error("bad") }, "bad\n"},
		{"Warning", func(p *Printer) { p.Warning("warn") }, "warn\n"},
		{"Step", func(p *Printer) { p.Step("next") }, "next\n"},
		{"Header", func(p *Printer) { p.Header("title") }, "\ntitle\n"},
		{"Muted", func(p *Printer) { p.Muted("dim") }, "dim\n"},
		{"Print", func(p *Printer) { p.Print("raw") }, "raw"},
		{"Println", func(p *Printer) { p.Println("line") }, "line\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			p := NewPrinter(&buf, false)
			tc.call(p)
			if got := buf.String(); got != tc.want {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBlankPlainOutputsNewline(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	p := NewPrinter(&buf, false)
	p.Blank()
	if got := buf.String(); got != "\n" {
		t.Fatalf("Blank() output = %q, want newline", got)
	}
}

func TestStyledMethodsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("NO_ICONS", "1")
	p := NewPrinter(&bytes.Buffer{}, false)
	if got := p.StyledHeader("H"); got != "H" {
		t.Fatalf("StyledHeader = %q, want %q", got, "H")
	}
	if got := p.StyledMuted("M"); got != "M" {
		t.Fatalf("StyledMuted = %q, want %q", got, "M")
	}
	if got := p.StyledItem("I"); got != "I" {
		t.Fatalf("StyledItem = %q, want %q", got, "I")
	}
	if got := p.StyledIcon("★"); got != "" {
		t.Fatalf("StyledIcon with NO_ICONS = %q, want empty", got)
	}
}

func TestStyledIconEmptyInput(t *testing.T) {
	t.Setenv("NO_ICONS", "")
	p := NewPrinter(&bytes.Buffer{}, false)
	if got := p.StyledIcon(""); got != "" {
		t.Fatalf("StyledIcon(\"\") = %q, want empty", got)
	}
	if got := p.StyledIcon("   "); got != "" {
		t.Fatalf("StyledIcon(\"   \") = %q, want empty", got)
	}
}
