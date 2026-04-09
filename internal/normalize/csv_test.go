package normalize

import (
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

func TestCSV_BasicWithHeaders(t *testing.T) {
	csv := NewCSV()

	// Initialize
	run := &core.Run{ID: "test-run"}
	if err := csv.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest header row
	headerData := []byte("name,age,city\n")
	records, err := csv.Ingest(headerData)
	if err != nil {
		t.Fatalf("Ingest header failed: %v", err)
	}

	// Should not create a record for header
	if len(records) != 0 {
		t.Errorf("Expected 0 records for header row, got %d", len(records))
	}

	// Ingest data row
	rowData := []byte("Alice,30,NYC\n")
	records, err = csv.Ingest(rowData)
	if err != nil {
		t.Fatalf("Ingest row failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]

	// Check record type
	if rec.Type != "csv_row" {
		t.Errorf("Expected Type=csv_row, got %q", rec.Type)
	}

	// Check payload fields
	payload := rec.Payload
	if payload["name"] != "Alice" {
		t.Errorf("Expected name=Alice, got %v", payload["name"])
	}
	if payload["age"] != "30" {
		t.Errorf("Expected age=30, got %v", payload["age"])
	}
	if payload["city"] != "NYC" {
		t.Errorf("Expected city=NYC, got %v", payload["city"])
	}
}

func TestCSV_NoHeaders(t *testing.T) {
	cfg := DefaultCSVConfig()
	cfg.HasHeaders = false
	csv := NewCSVWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := csv.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest data row (no header)
	rowData := []byte("Alice,30,NYC\n")
	records, err := csv.Ingest(rowData)
	if err != nil {
		t.Fatalf("Ingest row failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	payload := rec.Payload

	// Should use generated column names
	if payload["col_0"] != "Alice" {
		t.Errorf("Expected col_0=Alice, got %v", payload["col_0"])
	}
	if payload["col_1"] != "30" {
		t.Errorf("Expected col_1=30, got %v", payload["col_1"])
	}
	if payload["col_2"] != "NYC" {
		t.Errorf("Expected col_2=NYC, got %v", payload["col_2"])
	}
}

func TestCSV_ExplicitColumnNames(t *testing.T) {
	cfg := DefaultCSVConfig()
	cfg.ColumnNames = []string{"person", "years", "location"}
	csv := NewCSVWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := csv.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest data row
	rowData := []byte("Alice,30,NYC\n")
	records, err := csv.Ingest(rowData)
	if err != nil {
		t.Fatalf("Ingest row failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	payload := rec.Payload

	// Should use explicit column names
	if payload["person"] != "Alice" {
		t.Errorf("Expected person=Alice, got %v", payload["person"])
	}
	if payload["years"] != "30" {
		t.Errorf("Expected years=30, got %v", payload["years"])
	}
	if payload["location"] != "NYC" {
		t.Errorf("Expected location=NYC, got %v", payload["location"])
	}
}

func TestCSV_CustomDelimiter(t *testing.T) {
	cfg := DefaultCSVConfig()
	cfg.Delimiter = '\t' // Tab-separated
	cfg.HasHeaders = false
	csv := NewCSVWithConfig(cfg)

	run := &core.Run{ID: "test-run"}
	if err := csv.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest tab-separated data
	rowData := []byte("Alice\t30\tNYC\n")
	records, err := csv.Ingest(rowData)
	if err != nil {
		t.Fatalf("Ingest row failed: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	payload := rec.Payload

	if payload["col_0"] != "Alice" {
		t.Errorf("Expected col_0=Alice, got %v", payload["col_0"])
	}
}

func TestCSV_MultipleLines(t *testing.T) {
	csv := NewCSV()

	run := &core.Run{ID: "test-run"}
	if err := csv.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest multiple lines at once
	data := []byte("name,age\nAlice,30\nBob,25\n")
	records, err := csv.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	// Should get 2 data records (header doesn't count)
	expected := 2
	if len(records) != expected {
		t.Errorf("Expected %d records, got %d", expected, len(records))
	}

	// Check first record
	if records[0].Payload["name"] != "Alice" {
		t.Errorf("Expected first record name=Alice, got %v", records[0].Payload["name"])
	}

	// Check second record
	if records[1].Payload["name"] != "Bob" {
		t.Errorf("Expected second record name=Bob, got %v", records[1].Payload["name"])
	}
}

func TestCSV_EmptyLines(t *testing.T) {
	csv := NewCSV()

	run := &core.Run{ID: "test-run"}
	if err := csv.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest data with empty lines
	data := []byte("name,age\n\nAlice,30\n\n")
	records, err := csv.Ingest(data)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	// Should get 1 data record (empty lines ignored)
	expected := 1
	if len(records) != expected {
		t.Errorf("Expected %d record, got %d", expected, len(records))
	}
}

func TestCSV_PartialLine(t *testing.T) {
	csv := NewCSV()

	run := &core.Run{ID: "test-run"}
	if err := csv.Init(run); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Ingest header
	_, err := csv.Ingest([]byte("name,age\n"))
	if err != nil {
		t.Fatalf("Ingest header failed: %v", err)
	}

	// Ingest partial line (no newline)
	records, err := csv.Ingest([]byte("Alice,"))
	if err != nil {
		t.Fatalf("Ingest partial failed: %v", err)
	}

	// Should not produce a record yet (line incomplete)
	if len(records) != 0 {
		t.Errorf("Expected 0 records for partial line, got %d", len(records))
	}

	// Complete the line
	records, err = csv.Ingest([]byte("30\n"))
	if err != nil {
		t.Fatalf("Ingest completion failed: %v", err)
	}

	// Now should get the complete record
	if len(records) != 1 {
		t.Errorf("Expected 1 record after completion, got %d", len(records))
	}

	if len(records) > 0 {
		if records[0].Payload["name"] != "Alice" {
			t.Errorf("Expected name=Alice, got %v", records[0].Payload["name"])
		}
		if records[0].Payload["age"] != "30" {
			t.Errorf("Expected age=30, got %v", records[0].Payload["age"])
		}
	}
}
