package tui

import (
	"reflect"
	"testing"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
		wantErr  bool
	}{
		{"", nil, false},
		{"tag1", []string{"tag1"}, false},
		{"tag1, tag2", []string{"tag1", "tag2"}, false},
		{"tag1, , tag3", nil, true},
	}

	for _, tt := range tests {
		got, err := parseTags(tt.input)
		if tt.wantErr && err == nil {
			t.Fatalf("parseTags(%q) expected error", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Fatalf("parseTags(%q) unexpected error: %v", tt.input, err)
		}
		if !reflect.DeepEqual(got, tt.expected) {
			t.Fatalf("parseTags(%q) = %#v, want %#v", tt.input, got, tt.expected)
		}
	}
}

func TestParseAttributes(t *testing.T) {
	tests := []struct {
		input    string
		expected map[string]string
		wantErr  bool
	}{
		{"", map[string]string{}, false},
		{"key=value", map[string]string{"key": "value"}, false},
		{"key=value\nfoo=bar", map[string]string{"key": "value", "foo": "bar"}, false},
		{"invalid", nil, true},
		{"key=", nil, true},
	}

	for _, tt := range tests {
		got, err := parseAttributes(tt.input)
		if tt.wantErr && err == nil {
			t.Fatalf("parseAttributes(%q) expected error", tt.input)
		}
		if !tt.wantErr && err != nil {
			t.Fatalf("parseAttributes(%q) unexpected error: %v", tt.input, err)
		}
		if tt.wantErr {
			continue
		}
		if !reflect.DeepEqual(got, tt.expected) {
			t.Fatalf("parseAttributes(%q) = %#v, want %#v", tt.input, got, tt.expected)
		}
	}
}

func TestFormatAttributeLines(t *testing.T) {
	input := map[string]string{"b": "2", "a": "1"}
	got := formatAttributeLines(input)
	want := "a=1\nb=2"
	if got != want {
		t.Fatalf("formatAttributeLines mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}
