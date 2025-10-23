package normalize

import (
	"testing"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

func TestRaw_Basic(t *testing.T) {
	raw := NewRaw()

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest single line
	data := []byte("This is a log line\n")
	records, err := raw.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]

	if rec.Type != "raw" {
		t.Errorf("Expected Type=raw, got %q", rec.Type)
	}

	if rec.Payload["content"] != "This is a log line" {
		t.Errorf("Expected content='This is a log line', got %v", rec.Payload["content"])
	}
}

func TestRaw_MultipleLines(t *testing.T) {
	raw := NewRaw()

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest multiple lines
	data := []byte("line1\nline2\nline3\n")
	records, err := raw.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	expected := 3
	if len(records) != expected {
		t.Errorf("Expected %d records, got %d", expected, len(records))
	}

	if records[0].Payload["content"] != "line1" {
		t.Errorf("Expected first record content='line1', got %v", records[0].Payload["content"])
	}
	if records[1].Payload["content"] != "line2" {
		t.Errorf("Expected second record content='line2', got %v", records[1].Payload["content"])
	}
	if records[2].Payload["content"] != "line3" {
		t.Errorf("Expected third record content='line3', got %v", records[2].Payload["content"])
	}
}

func TestRaw_CustomRecordType(t *testing.T) {
	cfg := DefaultRawConfig()
	cfg.RecordType = "log_line"
	raw := NewRawWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	data := []byte("test\n")
	records, err := raw.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if records[0].Type != "log_line" {
		t.Errorf("Expected Type=log_line, got %q", records[0].Type)
	}
}

func TestRaw_SkipEmpty(t *testing.T) {
	cfg := DefaultRawConfig()
	cfg.SkipEmpty = true
	raw := NewRawWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest data with empty lines
	data := []byte("line1\n\nline2\n\n\n")
	records, err := raw.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	// Should skip empty lines
	expected := 2
	if len(records) != expected {
		t.Errorf("Expected %d records (empty lines skipped), got %d", expected, len(records))
	}
}

func TestRaw_NoSkipEmpty(t *testing.T) {
	cfg := DefaultRawConfig()
	cfg.SkipEmpty = false
	raw := NewRawWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest data with empty lines
	data := []byte("line1\n\nline2\n")
	records, err := raw.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	// Should include empty lines
	// Split creates: ["line1", "", "line2", ""] (4 elements)
	expected := 4
	if len(records) != expected {
		t.Errorf("Expected %d records (including empty), got %d", expected, len(records))
	}
}

func TestRaw_NoTrimSpace(t *testing.T) {
	cfg := DefaultRawConfig()
	cfg.TrimSpace = false
	raw := NewRawWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	data := []byte("  spaced line  \n")
	records, err := raw.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	// Should preserve spaces
	if records[0].Payload["content"] != "  spaced line  " {
		t.Errorf("Expected content with spaces preserved, got %v", records[0].Payload["content"])
	}
}

func TestRaw_PreserveRaw(t *testing.T) {
	cfg := DefaultRawConfig()
	cfg.PreserveRaw = true
	raw := NewRawWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	data := []byte("  line  \n")
	records, err := raw.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	// Should have both raw and trimmed content
	// Note: newline is stripped during split, so raw won't include it
	payload := records[0].Payload
	if payload["raw"] != "  line  " {
		t.Errorf("Expected raw field to preserve original (without newline), got %v", payload["raw"])
	}
	if payload["content"] != "line" {
		t.Errorf("Expected content to be trimmed, got %v", payload["content"])
	}
}

func TestRaw_ChunkMode(t *testing.T) {
	cfg := DefaultRawConfig()
	cfg.SplitOnLines = false // Treat chunks as single records
	raw := NewRawWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest chunk with multiple lines
	data := []byte("line1\nline2\nline3\n")
	records, err := raw.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	// Should create single record for entire chunk
	if len(records) != 1 {
		t.Fatalf("Expected 1 record (chunk mode), got %d", len(records))
	}

	// Content should include all lines
	content := records[0].Payload["content"].(string)
	if content != "line1\nline2\nline3" {
		t.Errorf("Expected full chunk as content, got %q", content)
	}
}

func TestRaw_PartialLine(t *testing.T) {
	raw := NewRaw()

	run := &core.Run{ID: "test-run"}
	if err := raw.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest partial line (no newline)
	records, err := raw.Ingest([]byte("partial "))
	if err != nil {
		t.Fatalf("Ingest partial failed: %v", err)
	}

	// Should not produce a record yet
	if len(records) != 0 {
		t.Errorf("Expected 0 records for partial line, got %d", len(records))
	}

	// Complete the line
	records, err = raw.Ingest([]byte("line\n"))
	if err != nil {
		t.Fatalf("Ingest completion failed: %v", err)
	}

	// Now should get the complete record
	if len(records) != 1 {
		t.Errorf("Expected 1 record after completion, got %d", len(records))
	}

	if len(records) > 0 && records[0].Payload["content"] != "partial line" {
		t.Errorf("Expected content='partial line', got %v", records[0].Payload["content"])
	}
}
