---
name: astrolabe
description: Go-based CLI/TUI development for Astrolabe project - data acquisition tool for test benches using Cobra, Bubbletea, serial/TCP sources, and offline-first architecture. Use when working on Astrolabe codebase, implementing capture pipeline, sources, normalizers, TUI screens, or testing.
---

# Astrolabe CLI/TUI Development Skill

## Overview

Astrolabe is a Go-based CLI/TUI agent that standardizes data acquisition from test benches, devices, and instruments. It provides reliable capture, normalization, and upload of test artifacts into a QA application. The tool emphasizes operator-friendliness, offline resilience, and deterministic data handling.

**Current Implementation Status**: ✅ **MVP COMPLETE**. All MVP sources (Serial, File, TCP) are fully implemented and tested. The core capture, normalization, storage, and upload pipeline is operational. A comprehensive TUI is implemented with styled CLI output. Telemetry, testing infrastructure, and operator-friendly features are in place.

**Upload System**: ✅ **Aligned with Orrery Backend Contract**. The upload client now uses batch format for presign/confirm endpoints, stores backend-assigned artifact IDs, and supports deduplication. Compatible with Orrery Phase 3 implementation.

## Core Philosophy

- **Single source of truth**: Replace ad-hoc scripts with one consistent tool
- **Offline-first**: Cache artifacts locally; upload when connection is restored
- **Operator-friendly**: Clear feedback, structured logging, helpful error messages
- **Deterministic**: Schema versioning, checksums, and reproducible captures
- **Extensible**: Plugin architecture for new sources and normalizers

## Project Structure

```
cmd/astrolabe/     # Cobra command entrypoints
internal/
  ├── core/        # Domain models (Run, Record, Manifest)
  ├── config/      # Config loader (file + env + flags)
  ├── logging/     # Structured logging helpers
  ├── sources/     # Source implementations (Serial, File, TCP)
  ├── normalize/   # Data normalizers (CSV, JSONL, Raw)
  ├── storage/     # Filesystem storage (manifest + JSONL)
  ├── upload/      # Upload client (presigned URLs / API tokens)
  ├── capture/     # Capture pipeline orchestration
  ├── runs/        # Run listing and formatting
  ├── tui/         # Bubbletea-based TUI
  ├── cliout/      # Styled CLI output
  ├── telemetry/   # Metrics collection (Prometheus, expvar)
  └── testutil/    # Test helpers and mock implementations
```

### Key Principles

1. **Internal packages only**: All business logic lives in `internal/` to prevent external dependencies
2. **Domain-driven design**: `internal/core/` defines the canonical data model
3. **Interface-based sources**: All data sources implement common interfaces for capture
4. **Dependency injection**: Pass dependencies explicitly; avoid global state

## Core Data Model

### Run Lifecycle

```
[Source] → [Normalize] → [Run + Artifacts] → [Storage] → [Upload Queue] → [QA App]
```

### Key Types (internal/core)

**Run**: Capture session envelope
- Links source metadata, manifest, settings, artifacts, and upload state
- Identified by `run_id` (ULID - Universally Unique Lexicographically Sortable Identifier)
- ULID properties: 26 characters, time-sortable, globally unique, compatible with backend expectations
- Immutable once created; state changes tracked in `UploadState`

**Manifest**: Operator-supplied metadata
- `DeviceInfo`: device ID, firmware hash, hardware revision
- `TestInfo`: test plan, variant, run number
- Tags and custom attributes
- Stamped on every run for traceability

**Record**: Single normalized datum
- Sequential ordering via `seq` field
- Timestamp for temporal correlation
- Flexible payload (JSON object)
- Source-specific fields preserved

**Artifact**: On-disk file belonging to a run
- Role-based classification (manifest, data, logs, attachments, raw)
- Checksum for integrity validation (SHA-256)
- Media type for proper handling
- Relative path within run directory
- Remote artifact ID (backend-assigned ULID after presign request)
- Upload tracking (timestamps, remote URL)

**UploadState**: Backend reconciliation tracker
- Queue status (pending, uploading, completed, failed)
- Retry attempts and timestamps
- Remote run ID after successful upload
- Error messages for debugging

### Supporting Types

- `SourceMeta`: Describes the capture source (type, address, protocol)
- `CaptureSettings`: How the stream was acquired (sample rate, duration, channels)
- `Checksum`: Hash algorithm + digest for integrity
- `ArtifactRole`: Enum for artifact classification

## Configuration Management

### Configuration Hierarchy (lowest to highest priority)

1. Default values (hardcoded)
2. Config file (`~/.astrolabe/connection.yml`, `~/.astrolabe/metadata.json`)
3. Environment variables (`ASTROLABE_*`)
4. Command-line flags

### Key Configuration Areas

**Connection settings** (`connection.yml`): api_url, project_id, auth_token, offline_cache_path, upload_batch_size, retry_max_attempts, retry_backoff_seconds

**Metadata defaults** (`metadata.json`): device.id, device.hardware_rev, capture.baud_rate, capture.sample_rate_hz

### Implementation Notes

- Use `viper` for configuration management
- Validate config on load; fail fast with clear errors
- Support `--config` flag to override default paths
- Log effective configuration at startup (debug level)

## Command Structure (Cobra)

### Command Hierarchy

```
astrolabe
├── capture      # Start a new capture
├── upload       # Upload cached runs
├── config       # Manage configuration
├── validate     # Validate artifacts
└── version      # Show version info
```

### Command Patterns

**Flag conventions**:
- Use long flags with descriptive names (`--device-id`, not `-d`)
- Provide short flags only for frequently used options
- Group related flags in command help output

**Error handling**:
- Return `cobra.Command` errors for user-facing issues
- Use structured logging for internal errors
- Exit codes: 0 (success), 1 (user error), 2 (system error)

**Output modes**:
- Default: Human-friendly progress and status
- `--quiet`: Minimal output (errors only)
- `--json`: Structured JSON output for CI/automation
- `--verbose`: Debug-level logging

### Example Command Implementation

```go
var captureCmd = &cobra.Command{
    Use:   "capture [source]",
    Short: "Start a data capture from a source",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := config.Load()
        if err != nil { return fmt.Errorf("load config: %w", err) }
        run, err := startCapture(args[0], deviceID, cfg)
        if err != nil { return fmt.Errorf("start capture: %w", err) }
        cmd.Printf("Capture started: run_id=%s\n", run.ID)
        return nil
    },
}
```

## Source Architecture

### Source Interface

```go
type Source interface {
    Connect(ctx context.Context) error
    Read(ctx context.Context) ([]byte, error)
    Close() error
    Meta() SourceMeta
}
```

### Normalizer Interface

```go
type Normalizer interface {
    Normalize(data []byte) ([]Record, error)
    SchemaVersion() string
}
```

### Implemented Sources ✅

**Serial** (`internal/sources/serial.go`):
- Connects to serial devices (UART, USB-Serial)
- Configurable baud rate, parity, stop bits
- Reads until EOF, timeout, or context cancellation
- Status: ✅ Implemented and tested

**File** (`internal/sources/file.go`):
- Reads from local files (CSV, JSONL, raw logs)
- Supports batch ingestion from directories
- Memory-efficient streaming for large files
- Status: ✅ Implemented and tested

**TCP** (`internal/sources/tcp.go`):
- Connects to TCP servers (test equipment APIs)
- Supports connection pooling and reconnection
- Configurable timeouts and keep-alive
- Status: ✅ Implemented and tested

### Normalizers ✅

**CSV Normalizer**: Parses CSV rows into records with field mapping
**JSONL Normalizer**: One JSON object per line, preserves structure
**Raw Normalizer**: Captures raw bytes as base64-encoded payloads (for binary protocols)

All normalizers assign sequential `seq` numbers and preserve timestamps for temporal ordering.

## Capture Pipeline

### Pipeline Flow

```
1. Source.Connect()     # Establish connection
2. Source.Read()        # Stream raw bytes
3. Normalizer.Parse()   # Convert to records
4. Storage.Write()      # Persist to disk
5. UploadQueue.Add()    # Queue for backend sync
```

### Error Handling Strategy

**Capture errors**: Log and continue (don't abort on single read failure)
**Normalization errors**: Store raw bytes + error record for manual inspection
**Storage errors**: Fatal (data loss risk)
**Upload errors**: Retry with exponential backoff, queue indefinitely

### Concurrency Model

- **Main goroutine**: Orchestrates pipeline stages
- **Reader goroutine**: Streams from source into channel
- **Normalizer goroutines**: Worker pool (configurable size)
- **Writer goroutine**: Flushes to disk with buffering
- **Uploader**: Separate process or background goroutine

Use `context.Context` for graceful shutdown on SIGTERM/SIGINT.

## Storage Layer

### Directory Structure

```
~/.astrolabe/runs/
  └── {run_id}/
      ├── manifest.json    # Run metadata
      ├── data.jsonl       # Normalized records
      ├── raw.bin          # Raw capture (optional)
      └── checksum.sha256  # Integrity verification
```

### Checksum Strategy

- Calculate SHA-256 for all artifacts on write
- Store in `checksum.sha256` file (one line per artifact)
- Verify before upload to detect corruption
- Reject upload if checksums don't match

### Retention Policy

- Keep runs locally until successfully uploaded
- Optionally purge after N days (configurable)
- Archive mode: never delete (for regulatory compliance)

## Upload System ✅

### Upload Flow

```
1. List pending runs (query storage)
2. For each run:
   a. Verify checksums
   b. Request presigned URLs from backend (batch format)
      - Send artifacts array with role, filename, checksum
      - Receive artifact_id for each artifact (store locally)
      - Check for deduplication (status: "existing")
   c. Upload artifacts to presigned URLs
      - Skip if status was "existing" (already uploaded)
      - Use PUT method with file content
      - Apply retry logic with exponential backoff
   d. Confirm upload with backend
      - Send artifact_id (from step b) in batch format
      - Backend verifies file in storage and marks as uploaded
   e. Update local state (save upload_state.json)
3. Retry failed uploads (exponential backoff)
```

### Backend Contract Alignment

**Presign Request** (batch format):
```json
{
  "artifacts": [{
    "filename": "manifest.json",
    "role": "manifest",
    "content_type": "application/json",
    "size_bytes": 1024,
    "checksum_sha256": "abc...",
    "source": "cli"
  }]
}
```

**Presign Response**:
```json
{
  "artifacts": [{
    "artifact_id": "01JARTIFACT123",  // Store this!
    "url": "https://...",
    "method": "PUT",
    "headers": {...},
    "expires_at": "2025-11-16T12:00:00Z",
    "status": null  // or "existing" for deduplication
  }]
}
```

**Confirm Request** (uses artifact_id):
```json
{
  "artifacts": [{
    "artifact_id": "01JARTIFACT123",  // From presign response
    "filename": "manifest.json",
    "checksum_sha256": "abc...",
    "uploaded_at": "2025-11-16T11:55:00Z"
  }]
}
```

### Deduplication Support

The upload system supports artifact deduplication:
- Backend detects duplicate artifacts by checksum during presign request
- Returns `status: "existing"` in presign response for duplicates
- CLI skips upload step for deduplicated artifacts
- Confirmation still required (to link artifact to current run)
- Reduces bandwidth and storage costs for repeated uploads

### Retry Logic

- Max attempts: 5 (configurable)
- Backoff: [1s, 2s, 4s, 8s, 16s]
- Jitter: ±20% to prevent thundering herd
- Permanent failures: Mark for manual investigation
- Retries preserve artifact_id from original presign request

### Offline Behavior

- Queue uploads when backend unreachable
- Retry periodically (every 5 minutes)
- Display pending count in TUI status bar
- Gracefully handle network transitions

## TUI Architecture (Bubbletea)

### Screen Flow

```
HomeScreen → CaptureScreen → RunningCaptureScreen → ResultScreen
         ↓
       UploadScreen
         ↓
       ListRunsScreen
```

### Key Screens

**HomeScreen**: Main menu (Capture, Upload, View Runs, Settings, Quit)
**CaptureScreen**: Source selection and configuration
**RunningCaptureScreen**: Live progress, record count, duration, stop button
**ResultScreen**: Capture summary with run ID and artifact details
**UploadScreen**: Upload queue with progress bars and status
**ListRunsScreen**: Filterable/sortable run history
**SettingsScreen**: Configuration editor

### TUI Patterns

- Use `tea.Model` for each screen
- Implement `Init()`, `Update()`, `View()` for state management
- Handle `tea.KeyMsg`, `tea.WindowSizeMsg`, custom messages
- Use `lipgloss` for consistent styling
- Show spinner during blocking operations
- Provide clear error messages with recovery suggestions

## CLI Output Styling (`internal/cliout`)

### Printer Interface

```go
type Printer interface {
    Success(msg string)
    Info(msg string)
    Warning(msg string)
    Error(msg string)
    Progress(current, total int)
}
```

### Output Modes

- **Pretty**: Colored output with icons (default)
- **Plain**: No colors (for CI/logs)
- **JSON**: Structured output (for automation)

Use `--no-color` flag to disable styling globally.

## Testing Strategy

### Test Coverage Requirements

- **Unit tests**: All core logic (>80% coverage)
- **Integration tests**: End-to-end capture workflows
- **Mock tests**: Source and normalizer implementations
- **Table-driven tests**: Edge cases and error paths

### Test Helpers (`internal/testutil`)

**MockSerial**: Simulates serial device with configurable behavior
```go
mock := testutil.NewMockSerial()
mock.SetReadData([]byte("sensor,123\n"))
mock.SetError(io.EOF) // Simulate disconnect
```

**TempStorage**: Isolated filesystem for storage tests
**MockUploader**: Simulates backend responses

### Testing Patterns

- Use `t.Parallel()` for independent tests
- Use `testify/assert` for assertions
- Use `httptest.Server` for upload tests
- Use golden files for TUI rendering tests
- Test context cancellation paths

### Example Test

```go
func TestSerialCapture(t *testing.T) {
    mock := testutil.NewMockSerial()
    mock.SetReadData([]byte("temp,25.3\n"))
    
    src := sources.NewSerial("/dev/ttyUSB0", 115200)
    src.SetReader(mock) // Inject mock
    
    data, err := src.Read(context.Background())
    assert.NoError(t, err)
    assert.Equal(t, "temp,25.3\n", string(data))
}
```

## Telemetry & Metrics

### Prometheus Metrics

**Capture metrics**:
- `astrolabe_captures_total` (counter)
- `astrolabe_records_captured` (counter)
- `astrolabe_capture_duration_seconds` (histogram)

**Upload metrics**:
- `astrolabe_uploads_total{status}` (counter)
- `astrolabe_upload_retries` (counter)
- `astrolabe_upload_duration_seconds` (histogram)

**Source metrics** (per source type):
- `astrolabe_source_reads_total{source}` (counter)
- `astrolabe_source_errors_total{source}` (counter)

### Expvar Endpoint

Expose runtime metrics at `http://localhost:9090/debug/vars`:
- Goroutine count
- Memory stats
- Active captures
- Pending uploads

Enable with `--telemetry` flag or `ASTROLABE_TELEMETRY=true`.

## Development Workflow

### Adding a New Source

1. Define struct implementing `Source` interface
2. Implement `Connect()`, `Read()`, `Close()`, `Meta()`
3. Add constructor in `internal/sources/`
4. Register in source factory
5. Write unit tests with mock connection
6. Write integration test with real device (if available)
7. Update command help text

### Adding a New Normalizer

1. Define struct implementing `Normalizer` interface
2. Implement `Normalize()` and `SchemaVersion()`
3. Add constructor in `internal/normalize/`
4. Register in normalizer factory
5. Write table-driven tests with edge cases
6. Update documentation with format examples

### Adding a New Command

1. Create command file in `cmd/astrolabe/root/`
2. Define `cobra.Command` with flags and `RunE`
3. Implement business logic in `internal/` packages
4. Add command to root command in `init()`
5. Write integration test
6. Update `--help` output verification test

### Debugging Tips

- Set `ASTROLABE_LOG_LEVEL=debug` for verbose logging
- Use `--offline` to test capture without network
- Run `astrolabe validate <run_id>` to check artifact integrity
- Inspect `~/.astrolabe/cache/runs/` for raw artifacts
- Mock the QA backend with `httptest.Server` in tests

## Anti-Patterns to Avoid

### Code Anti-Patterns

- **Global state**: Pass dependencies explicitly
- **Panics**: Return errors; let caller decide handling
- **Unbounded goroutines**: Always use worker pools or semaphores
- **Ignoring context**: Check `ctx.Done()` in loops
- **Magic numbers**: Use named constants
- **Silent failures**: Log or return all errors
- **Blocking writes**: Use buffered channels for async operations
- **Infinite retries**: Always set max attempts and backoff cap

### Design Anti-Patterns

**Question Feature Value Before Implementation**

Before implementing new features, especially UX features, critically evaluate:

1. **Does this add a third way to do something?** If yes, it's likely unnecessary. Example: We have `astrolabe tui` (interactive) and `astrolabe capture serial` (CLI) - a third entry point adds confusion.

2. **Does it duplicate existing functionality?** Check if the feature can be achieved by improving existing commands.

3. **Does it match the tool's usage patterns?** Astrolabe operations require configuration (ports, files, run IDs) - interactive menus don't fit this model well.

**When proposing features**:
- Challenge the value proposition early
- Point out if it creates UI/UX confusion
- Suggest simpler alternatives
- Ask "is the juice worth the squeeze?"

## External Dependencies

### Core Libraries

- **github.com/spf13/cobra**: CLI framework
- **github.com/spf13/viper**: Configuration management
- **github.com/charmbracelet/bubbletea**: TUI framework
- **github.com/charmbracelet/lipgloss**: TUI styling
- **go.uber.org/zap**: Structured logging
- **github.com/google/uuid**: UUID generation

### Source-Specific

- **github.com/tarm/serial** or **go.bug.st/serial**: Serial port access
- **golang.org/x/net/websocket**: WebSocket sources (future)

### Testing

- **github.com/stretchr/testify**: Test assertions
- **github.com/golang/mock**: Mock generation

## Future Considerations

**Plugin system**: Source/normalizer plugins with dynamic loading
**Streaming uploads**: Upload while capturing for low latency
**Compression**: Gzip artifacts before upload
**Multi-device capture**: Parallel capture from multiple sources

**Performance optimizations**: Zero-copy reads, memory pooling, parallel normalization, incremental checksums

---

## Quick Reference

### Build & Run

```bash
mise run build        # Build binary
./bin/astrolabe       # Launch TUI (default)
mise run tui          # Explicit TUI launch
```

### Capture Commands

```bash
# Serial source ✅
./bin/astrolabe capture serial /dev/ttyUSB0 --device-id ID --baud 115200

# File source ✅
./bin/astrolabe capture file data.csv --device-id ID

# TCP source ✅
./bin/astrolabe capture tcp 192.168.1.100:8080 --device-id ID
```

### Management

```bash
./bin/astrolabe upload           # Upload cached runs
./bin/astrolabe runs list        # List cached runs
./bin/astrolabe validate <id>    # Validate run artifacts
./bin/astrolabe config set KEY VAL  # Update config
./bin/astrolabe config get       # Show current config
./bin/astrolabe version          # Show version
```

### Testing

```bash
mise run test                    # Run all tests
mise run lint                    # Run linters
go test ./...                    # All tests
go test ./internal/sources/... -v   # Specific package
go test -cover ./...             # With coverage
```

### Batch File Ingestion

```bash
# Simple shell loop
for file in /mnt/usb/test_data/*.csv; do
    ./bin/astrolabe capture file "$file" --device-id archive-01
done

# With GNU Parallel
ls /mnt/usb/test_data/*.csv | parallel ./bin/astrolabe capture file {} --device-id archive-01
```

File ingestion remains CLI-only; the TUI is optimized for real-time streaming sources.

### Key File Locations

- **Config**: `~/.astrolabe/{connection.yml,metadata.json}`
- **Runs**: `~/.astrolabe/runs/`
- **Binary**: `./bin/astrolabe`
- **Logs**: Stdout/stderr (structured JSON with `--json` flag)

### Common Gotchas

- Serial ports require dialout group membership: `sudo usermod -a -G dialout $USER`
- Verify cache directory is writable
- Calculate checksums before renaming temp files
- Always check `ctx.Done()` in loops
- Defer order is LIFO (last defer executes first)

---

**Implementation Status:**
✅ All MVP sources (Serial, File, TCP) | ✅ Complete capture pipeline | ✅ Upload system with retry logic | ✅ Comprehensive TUI (7+ screens) | ✅ Styled CLI output | ✅ Telemetry and metrics | ✅ Testing infrastructure