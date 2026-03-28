package root

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	validatepkg "github.com/cthonicasoftware/astrolabe-cli/internal/validate"
)

func TestOutputJSON_Valid(t *testing.T) {
	result := validatepkg.Result{
		Valid:  true,
		RunID:  "run-123",
		RunDir: "/tmp/run-123",
		Checks: []validatepkg.Check{
			{Name: "directory_exists", Status: validatepkg.StatusPassed},
		},
		Warnings: []string{"warning-1"},
	}

	output := captureStdout(t, func() {
		if err := outputJSON(result); err != nil {
			t.Fatalf("outputJSON: %v", err)
		}
	})

	var decoded validatepkg.Result
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, output)
	}

	if !decoded.Valid {
		t.Fatalf("expected decoded result to be valid")
	}
	if decoded.RunID != result.RunID {
		t.Fatalf("RunID: got %q, want %q", decoded.RunID, result.RunID)
	}
	if decoded.RunDir != result.RunDir {
		t.Fatalf("RunDir: got %q, want %q", decoded.RunDir, result.RunDir)
	}
	if len(decoded.Checks) != 1 || decoded.Checks[0].Name != "directory_exists" {
		t.Fatalf("Checks: got %+v", decoded.Checks)
	}
}

func TestOutputJSON_InvalidReturnsValidationError(t *testing.T) {
	result := validatepkg.Result{
		Valid:  false,
		RunDir: "/tmp/run-456",
		Checks: []validatepkg.Check{
			{Name: "manifest_parse", Status: validatepkg.StatusFailed, Error: "invalid json"},
		},
		Errors: []string{"invalid json"},
	}

	output := captureStdout(t, func() {
		err := outputJSON(result)
		if !errors.Is(err, errValidationFailed) {
			t.Fatalf("expected errValidationFailed, got %v", err)
		}
	})

	var decoded validatepkg.Result
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("unmarshal output: %v\noutput: %s", err, output)
	}
	if decoded.Valid {
		t.Fatalf("expected decoded result to be invalid")
	}
	if len(decoded.Errors) != 1 || decoded.Errors[0] != "invalid json" {
		t.Fatalf("Errors: got %+v", decoded.Errors)
	}
}

func TestOutputStyled_Valid(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("NO_ICONS", "1")

	result := validatepkg.Result{
		Valid:  true,
		RunID:  "run-789",
		RunDir: "/tmp/run-789",
		Checks: []validatepkg.Check{
			{Name: "directory_exists", Status: validatepkg.StatusPassed},
			{Name: "upload_state", Status: validatepkg.StatusSkipped},
		},
		Warnings: []string{"warning-1"},
	}

	output := captureStdout(t, func() {
		if err := outputStyled(result); err != nil {
			t.Fatalf("outputStyled: %v", err)
		}
	})

	for _, want := range []string{
		"Validating run directory: /tmp/run-789",
		"directory_exists",
		"upload_state (skipped)",
		"warning-1",
		"Validation passed for run run-789",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected output to contain %q, got %q", want, output)
		}
	}
}

func TestOutputStyled_InvalidReturnsValidationError(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("NO_ICONS", "1")

	result := validatepkg.Result{
		Valid:  false,
		RunDir: "/tmp/run-999",
		Checks: []validatepkg.Check{
			{Name: "data_checksum", Status: validatepkg.StatusFailed, Error: "checksum mismatch"},
		},
		Errors: []string{"checksum mismatch"},
	}

	output := captureStdout(t, func() {
		err := outputStyled(result)
		if !errors.Is(err, errValidationFailed) {
			t.Fatalf("expected errValidationFailed, got %v", err)
		}
	})

	for _, want := range []string{
		"Validating run directory: /tmp/run-999",
		"data_checksum: checksum mismatch",
		"Validation failed",
		"checksum mismatch",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected output to contain %q, got %q", want, output)
		}
	}
}

func TestPrintCheck(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("NO_ICONS", "1")

	var buf bytes.Buffer
	out := cliout.NewPrinter(&buf, false)

	printCheck(out, validatepkg.Check{Name: "directory_exists", Status: validatepkg.StatusPassed})
	printCheck(out, validatepkg.Check{Name: "manifest_parse", Status: validatepkg.StatusFailed, Error: "invalid json"})
	printCheck(out, validatepkg.Check{Name: "upload_state", Status: validatepkg.StatusSkipped})

	output := buf.String()
	for _, want := range []string{
		"directory_exists",
		"manifest_parse: invalid json",
		"upload_state (skipped)",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected output to contain %q, got %q", want, output)
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	previousStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = previousStdout
	})

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return buf.String()
}
