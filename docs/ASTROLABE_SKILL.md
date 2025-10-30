# Astrolabe CLI/TUI Development Skill

> **⚠️ IMPORTANT: MANDATORY SKILL USAGE**
> **This skill MUST be used for ALL work on the Astrolabe project.**
> When working on this codebase, Claude Code should always reference this skill document to ensure consistency with the project's architecture, patterns, and conventions. This skill contains critical context about the domain model, testing patterns, TUI architecture, and implementation guidelines that are essential for maintaining code quality and consistency.

## Overview

Astrolabe is a Go-based CLI/TUI agent that standardizes data acquisition from test benches, devices, and instruments. It provides reliable capture, normalization, and upload of test artifacts into a QA application. The tool emphasizes operator-friendliness, offline resilience, and deterministic data handling.

**Current Implementation Status**: ✅ **MVP COMPLETE**. All MVP sources (Serial, File, TCP) are fully implemented and tested. The core capture, normalization, storage, and upload pipeline is operational. A comprehensive TUI is implemented with styled CLI output. Telemetry, testing infrastructure, and operator-friendly features are in place.

## Core Philosophy

- **Single source of truth**: Replace ad-hoc scripts with one consistent tool
- **Offline-first**: Cache artifacts locally; upload when connection is restored
- **Operator-friendly**: Clear feedback, structured logging, helpful error messages
- **Deterministic**: Schema versioning, checksums, and reproducible captures
- **Extensible**: Plugin architecture for new sources and normalizers

## Project Structure

```
cmd/astrolabe/       # Cobra command entrypoints
  └── root/          # Command implementations (capture, upload, config, etc.)
internal/core/       # Domain models (Run, Record, Manifest, etc.)
internal/config/     # Configuration loader (file + env + flags)
internal/logging/    # Structured logging helpers
internal/sources/    # Source interfaces & implementations (Serial, File, TCP)
internal/normalize/  # Data normalizers (bytes → records: CSV, JSONL, Raw)
internal/storage/    # Filesystem storage (manifest + JSONL)
internal/upload/     # Upload client (presigned URLs / API tokens)
internal/capture/    # Capture pipeline orchestration
internal/runs/       # Run listing and formatting
internal/tui/        # Bubbletea-based TUI (comprehensive implementation)
internal/cliout/     # Styled CLI output (printer with consistent formatting)
internal/telemetry/  # Metrics collection (Prometheus, expvar)
internal/testutil/   # Test helpers and mock implementations
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
- Role-based classification (manifest, data, logs, attachments)
- Checksum for integrity validation
- Media type for proper handling
- Relative path within run directory

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

**Connection settings** (`connection.yml`):
```yaml
api_url: https://qa.example.com
project_id: proj_abc123
auth_token: tok_xyz789
offline_cache_path: ~/.astrolabe/cache
upload_batch_size: 10
retry_max_attempts: 5
retry_backoff_seconds: [1, 2, 4, 8, 16]
```

**Metadata defaults** (`metadata.json`):
```json
{
  "device": {
    "id": "bench-01",
    "hardware_rev": "v2.1"
  },
  "capture": {
    "baud_rate": 115200,
    "sample_rate_hz": 1000
  }
}
```

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
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Parse flags
        deviceID, _ := cmd.Flags().GetString("device-id")
        
        // Load config
        cfg, err := config.Load()
        if err != nil {
            return fmt.Errorf("load config: %w", err)
        }
        
        // Create run
        run, err := startCapture(args[0], deviceID, cfg)
        if err != nil {
            return fmt.Errorf("start capture: %w", err)
        }
        
        // Output result
        cmd.Printf("Capture started: run_id=%s\n", run.ID)
        return nil
    },
}
```

## Source Architecture

### Source Interface

All sources implement a common interface for uniform capture:

```go
type Source interface {
    // Connect establishes the connection
    Connect(ctx context.Context) error
    
    // Read returns raw bytes from the source
    Read(ctx context.Context) ([]byte, error)
    
    // Close releases resources
    Close() error
    
    // Meta returns source metadata
    Meta() SourceMeta
}
```

### Normalizer Interface

Normalizers transform raw bytes into typed records:

```go
type Normalizer interface {
    // Normalize converts raw bytes to records
    Normalize(data []byte) ([]Record, error)
    
    // SchemaVersion returns the normalizer version
    SchemaVersion() string
}
```

### MVP Sources (All Implemented ✅)

1. **Serial ports** (USB-UART, RS-485) ✅ **COMPLETE**
   - Uses `go.bug.st/serial`
   - Handles baud rate, parity, stop bits, flow control configuration
   - Implements read timeouts for resilience
   - Frame-based reading with channel interface
   - Full test coverage with MockSerial implementation
   - CLI: `astrolabe capture serial <port> --baud=115200`

2. **Files** (CSV, JSONL, logs) ✅ **COMPLETE**
   - Retroactive ingestion of existing data
   - Auto-detects format from extension/content
   - Streams large files (doesn't load entirely into memory)
   - Multiple normalizers: CSV, JSONL, Raw
   - Full test coverage
   - CLI: `astrolabe capture file <path>`

3. **TCP sockets** ✅ **COMPLETE**
   - Simple streaming sources
   - Handles reconnection logic
   - Configurable read buffer sizes
   - Frame-based reading
   - Full test coverage
   - CLI: `astrolabe capture tcp <host:port>`

### Future Sources

- **USB devices** (via `libusb`)
- **SCPI instruments** (DMMs, oscilloscopes)
- **HTTP endpoints** (webhook targets)

### Implementation Guidelines

- **Context-aware**: All I/O operations accept `context.Context`
- **Timeout handling**: Set reasonable defaults; allow override
- **Resource cleanup**: Always implement proper `Close()` behavior
- **Error wrapping**: Use `fmt.Errorf("context: %w", err)` for stack traces
- **Reconnection**: Implement exponential backoff for transient failures

## Storage & Artifacts

### Directory Structure

```
~/.astrolabe/cache/
└── runs/
    └── {run_id}/
        ├── manifest.json      # Run metadata
        ├── data.jsonl         # Normalized records
        ├── raw.log            # Original source output (optional)
        ├── checksums.txt      # Artifact checksums
        └── attachments/       # Additional files
```

### Artifact Types (ArtifactRole)

- `RoleManifest`: Run metadata (always required)
- `RoleData`: Primary capture data (JSONL format)
- `RoleRaw`: Original source output (debugging)
- `RoleLog`: Device/system logs
- `RoleAttachment`: User-uploaded files

### Storage Operations

**Writing artifacts**:
1. Create run directory atomically
2. Write artifacts with `.tmp` suffix
3. Calculate checksums as files are written
4. Rename to final name (atomic on POSIX)
5. Write `checksums.txt` last

**Reading artifacts**:
1. Verify checksums before processing
2. Fail fast on corruption detection
3. Report specific artifact failures

**Cleanup policy**:
- Keep uploaded runs for 7 days (configurable)
- Keep failed runs until manually deleted
- Provide `astrolabe clean` command for manual cleanup

### Checksum Strategy

- Use SHA-256 for all artifacts
- Format: `{algo}:{hex_digest}`
- Store in `checksums.txt`: `<checksum> <relative_path>`
- Verify on: read, upload, and validation commands

## Upload Strategy

### Upload Flow

1. Scan cache directory for runs in `UploadStatePending`
2. Create backend run record (get remote `run_id`)
3. Request presigned URLs for each artifact
4. Upload artifacts concurrently (with rate limiting)
5. Confirm upload completion with backend
6. Update local `UploadState` to `UploadStateCompleted`

### Resilience Features

**Exponential backoff**:
- Start: 1 second
- Max: 60 seconds
- Jitter: ±25% to avoid thundering herd

**Partial upload recovery**:
- Track which artifacts succeeded
- Resume from last successful artifact
- Don't re-upload completed artifacts

**Network detection**:
- Ping health endpoint before upload attempts
- Cache failures; retry on next upload command
- Support `--offline` flag to skip upload entirely

**Batch processing**:
- Upload multiple runs concurrently
- Configurable batch size (`upload_batch_size`)
- Progress reporting for long-running uploads

### API Integration

**Authentication**:
- API token in `Authorization: Bearer {token}` header
- Support for token refresh (future)
- Fail clearly on 401/403 errors

**Idempotency**:
- `Idempotency-Key` header uses local run.ID (ULID) for safe retries
- Backend can deduplicate create requests based on this key
- Enables reliable retry logic without creating duplicate runs

**Endpoints**:
- `POST /api/runs/` - Create run record (with Idempotency-Key header)
- `GET /api/runs/{id}/upload-urls` - Get presigned URLs
- `POST /api/runs/{id}/complete` - Finalize upload

**Error handling**:
- 4xx errors: User issue (bad config, invalid data)
- 5xx errors: Server issue (retry with backoff)
- Timeout: Network issue (retry with backoff)

## Logging Strategy

### Structured Logging

Use structured JSON logs for machine parseability:

```go
log.Info("capture started",
    "run_id", run.ID,
    "source_type", run.SourceMeta.Type,
    "device_id", run.Manifest.Device.ID,
)
```

### Log Levels

- **Debug**: Internal state changes, config values
- **Info**: User-visible progress (capture started, upload complete)
- **Warn**: Recoverable issues (retry attempts, partial failures)
- **Error**: Failures requiring user action

### Logging Library

Use `go.uber.org/zap` for:
- High performance (zero-allocation in hot paths)
- Structured output (JSON for CI, console for users)
- Level-based filtering
- Context propagation

### CI/Automation Output

In `--json` mode:
- One JSON object per line (JSONL)
- Include `timestamp`, `level`, `message`, and context fields
- Emit progress events: `capture.started`, `capture.completed`, `upload.progress`, `upload.completed`

## CLI Output Styling (internal/cliout) ✅ IMPLEMENTED

### Printer Pattern

The `cliout.Printer` provides consistent, styled output across all CLI commands:

```go
p := cliout.DefaultPrinter(jsonMode)

p.Header("Capture Session")
p.Info("Connecting to device...")
p.Success("Connection established")
p.KeyValue("Device ID", "bench-01")
p.Warning("High temperature detected")
p.Error("Upload failed")
```

### Features

- **Consistent styling**: Reuses TUI color palette and icons
- **JSON mode**: Automatic structured output with `--json` flag
- **Color detection**: Respects `NO_COLOR` and `TERM=dumb` environment variables
- **Icon support**: Optional icons (disable with `NO_ICONS=1`)
- **Multiple output levels**: Info, Success, Error, Warning, Step, Header, Muted

### Usage Guidelines

- Use `Printer` in all CLI commands (not TUI)
- Always respect the `--json` flag for CI/automation
- Use semantic methods (Success/Error/Warning) not just colors
- KeyValue for structured data display
- Header for section separators

## Telemetry & Metrics (internal/telemetry) ✅ IMPLEMENTED

### Metrics Backends

Astrolabe supports multiple metrics backends for monitoring:

1. **Prometheus** (`/metrics` endpoint)
   - Standard counters, gauges, histograms
   - Useful for production monitoring
   - Scrape-based collection

2. **expvar** (Go's built-in metrics)
   - Lightweight, no external dependencies
   - Good for development and debugging
   - Access via HTTP endpoint

### Key Metrics

Track these operational metrics:
- Capture sessions started/completed
- Records processed per source type
- Bytes transferred per source
- Upload attempts/successes/failures
- Upload retry counts
- Cache size and artifact counts

### Implementation Notes

- Metrics are optional; don't fail if unavailable
- Use structured names: `astrolabe_captures_total`, `astrolabe_upload_errors`
- Include labels: source type, device ID, error type
- Expose metrics endpoint only when explicitly enabled

## TUI Design (Bubbletea) ✅ IMPLEMENTED

### TUI Philosophy

- **Real-time feedback**: Show live capture progress ✅
- **Interruptible**: Graceful shutdown on Ctrl+C ✅
- **Informative**: Display key metrics (records/sec, bytes captured, upload status) ✅
- **Error-friendly**: Clear error messages with recovery suggestions ✅
- **Operator-friendly**: Visual navigation with arrow keys and intuitive controls ✅

### Implemented TUI Screens

The TUI is fully implemented with the following screens:

1. **Welcome Screen** (`welcome.go`) - Main navigation hub
   - Options to start capture, configure settings, view runs, upload data
   - Styled with consistent color palette
   - Keyboard navigation support

2. **Capture Configuration** (`capture_tabs.go`) - Multi-tab capture setup
   - Serial port selection with device listing
   - TCP endpoint configuration
   - File selection
   - Real-time validation

3. **Metadata Screen** (`metadata.go`) - Device and test metadata entry
   - Device ID, firmware hash, hardware revision
   - Test plan, variant, run number
   - Custom tags and attributes
   - Form validation

4. **Connection Settings** (`config.go`) - API and connection configuration
   - API URL and project ID
   - Authentication token management
   - Offline cache settings
   - Retry and backoff parameters

5. **Run Listing** (`runs.go`) - View and manage cached runs
   - List all local runs with metadata
   - Run details view
   - Upload status tracking
   - Delete/archive options

6. **Upload View** (`upload.go`) - Batch upload progress
   - Per-run upload status
   - Progress bars and counters
   - Retry attempt tracking
   - Error display with recovery suggestions

7. **Advanced Settings** (`advanced_settings.go`) - Additional configuration
   - Logging levels
   - Telemetry settings
   - Performance tuning
   - Debug options

### TUI Architecture Notes

The TUI follows the Elm architecture strictly:
- Pure model updates in `Update()`
- Side effects via `Cmd` return values
- No direct I/O in view rendering
- Consistent state management patterns

### Bubbletea Model Structure

```go
type model struct {
    run       *core.Run
    progress  captureProgress
    logs      []string
    err       error
    quitting  bool
}

type captureProgress struct {
    recordCount   int
    bytesReceived int64
    duration      time.Duration
    recordsPerSec float64
}
```

### TUI Screens

1. **Capture view**: Live metrics, recent records, status
2. **Upload view**: Batch progress, per-run status, retry attempts
3. **Error view**: Detailed error, suggested actions, logs

### Update Loop Pattern

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "ctrl+c" {
            m.quitting = true
            return m, tea.Quit
        }
    case recordMsg:
        m.progress.recordCount++
        return m, waitForNextRecord()
    case errorMsg:
        m.err = msg.err
        return m, tea.Quit
    }
    return m, nil
}
```

### View Rendering Guidelines

- Use `lipgloss` for consistent styling
- Show spinner during long operations
- Update progress bar for known-duration tasks
- Limit log display to last 10 lines
- Use colors sparingly (not all terminals support them)

## Testing Strategy

### Test Categories

1. **Unit tests**: Pure functions, data model validation
2. **Integration tests**: Source implementations, storage operations
3. **End-to-end tests**: Full capture → upload flow (mocked backend)
4. **Simulation tests**: Synthetic serial/TCP streams

### Mock Sources for Testing

```go
type mockSource struct {
    data   [][]byte
    cursor int
    delay  time.Duration
}

func (m *mockSource) Read(ctx context.Context) ([]byte, error) {
    if m.cursor >= len(m.data) {
        return nil, io.EOF
    }
    time.Sleep(m.delay)
    data := m.data[m.cursor]
    m.cursor++
    return data, nil
}
```

### Key Test Scenarios

**Capture flow**:
- Source connection failures
- Partial data reads
- Normalizer errors
- Disk full conditions

**Upload flow**:
- Network timeouts
- Partial upload failures (resume logic)
- Backend 5xx errors (retry logic)
- Corrupted artifacts (checksum mismatch)

**Offline mode**:
- Capture works without network
- Cache persists across restarts
- Uploads process on reconnection

### Test Helpers ✅ IMPLEMENTED

The `internal/testutil/` package provides comprehensive test helpers:

**Implemented:**
- ✅ `MockSerial`: Full-featured mock serial device with configurable frames, delays, and error injection
  - Frame-based emission with configurable timing
  - Context cancellation support
  - Error injection for testing failure paths
  - Reusable across tests with Reset()
  - Dynamic frame addition during test execution
  - See `internal/testutil/README.md` for comprehensive documentation

**Future additions:**
- `NewTempRunDir()`: Temporary run directory
- `MockHTTPServer()`: Fake QA backend
- `AssertManifest()`: Validate manifest structure
- `MockTCPSource`: Mock TCP socket source
- `MockFileSource`: Mock file source with configurable content

### Testing MockSerial Example

```go
import "github.com/LostinTimeandspaceYT/qa_cli_agent/internal/testutil"

// Create mock with test data
mock := testutil.NewMockSerial(
    testutil.WithFrames([][]byte{
        []byte("TEMP:23.5C\n"),
        []byte("TEMP:23.7C\n"),
    }),
    testutil.WithFrameDelay(10*time.Millisecond),
)

ctx := context.Background()
mock.Open(ctx)
defer mock.Close()

// Read and process frames
for frame := range mock.Frames() {
    // Test your capture logic
}
```

### Coverage Goals

- Core domain logic: 90%+
- Source implementations: 80%+
- Storage operations: 80%+
- Upload client: 75%+
- CLI commands: 60%+ (harder to test UI)

## Build & Release

### Build System (mise)

Use `mise run` tasks for all operations:

```toml
[tasks.build]
run = "go build -o bin/astrolabe ./cmd/astrolabe"
description = "Build the CLI binary"

[tasks.test]
run = "go test ./..."
description = "Run all tests"

[tasks.lint]
run = "golangci-lint run"
description = "Run linters"

[tasks.tui]
run = "go run ./cmd/astrolabe tui"
description = "Run TUI in development mode"
```

### Binary Packaging

- **Linux**: Static binary (CGO_ENABLED=0)
- **macOS**: Universal binary (amd64 + arm64)
- **Windows**: Signed executable

Distribution formats:
- `.tar.gz` archives (Linux, macOS)
- `.zip` archives (Windows)
- Homebrew formula (macOS)
- `.deb` and `.rpm` packages (Linux)

### Versioning

- Use semantic versioning (v1.2.3)
- Embed version at build time: `go build -ldflags "-X main.version=v1.2.3"`
- Include schema version in binaries
- Show both in `astrolabe version` output

## Common Implementation Patterns

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("connect to source: %w", err)
}

// Check for specific error types
if errors.Is(err, io.EOF) {
    // Handle end of stream
}

// Extract wrapped errors
var netErr *net.OpError
if errors.As(err, &netErr) {
    // Handle network error
}
```

### Context Propagation

```go
func (s *serialSource) capture(ctx context.Context) error {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := s.readAndProcess(); err != nil {
                return err
            }
        }
    }
}
```

### Resource Cleanup

```go
func processRun(runID string) error {
    f, err := os.Open(filepath.Join(cacheDir, runID, "data.jsonl"))
    if err != nil {
        return err
    }
    defer f.Close() // Always defer Close
    
    // Process file
    return nil
}
```

### Concurrency Patterns

```go
// Worker pool for uploads
func uploadArtifacts(ctx context.Context, artifacts []Artifact) error {
    sem := make(chan struct{}, 4) // Max 4 concurrent uploads
    errCh := make(chan error, len(artifacts))
    
    for _, a := range artifacts {
        sem <- struct{}{}
        go func(artifact Artifact) {
            defer func() { <-sem }()
            errCh <- uploadOne(ctx, artifact)
        }(a)
    }
    
    // Wait and collect errors
    for range artifacts {
        if err := <-errCh; err != nil {
            return err
        }
    }
    return nil
}
```

## Success Criteria Checklist

For v0 (MVP) release, verify:

### Core Capture & Sources ✅ COMPLETE
- [x] Serial capture implementation complete
- [x] File source implementation complete
- [x] TCP socket source implementation complete
- [x] All sources produce valid JSONL + manifest
- [x] Frame-based reading interface for all sources
- [x] Normalizers for CSV, JSONL, and raw formats

### Upload & Storage ✅ COMPLETE
- [x] Upload works with API token authentication
- [x] Presigned URL uploads succeed
- [x] Offline cache persists across restarts
- [x] Retry logic proven with simulated failures
- [x] Checksum validation prevents corrupted uploads
- [x] Exponential backoff with jitter
- [x] Comprehensive upload test coverage

### Metadata & Versioning ✅ COMPLETE
- [x] Schema version stamped on every artifact
- [x] Manifest includes device info, test info, and capture settings
- [x] Run ID (UUID) tracking throughout lifecycle
- [x] Upload state tracking (pending, uploading, completed, failed)

### CLI & UX ✅ COMPLETE
- [x] `astrolabe --help` is comprehensive and clear
- [x] CLI commands work in CI (non-interactive mode)
- [x] Styled CLI output with consistent formatting
- [x] JSON output mode for automation
- [x] Color detection and NO_COLOR support
- [x] Config command for managing settings
- [x] `astrolabe` (no args) launches TUI for operator-friendly experience

### TUI ✅ COMPLETE
- [x] TUI provides real-time capture feedback
- [x] Welcome screen with navigation
- [x] Metadata configuration screen
- [x] Connection/API configuration screen
- [x] Run listing and management
- [x] Upload status and progress tracking
- [x] Advanced settings configuration
- [x] Port selection for serial devices

### Testing & Quality ✅ COMPLETE
- [x] MockSerial test helper with comprehensive features
- [x] All packages have test coverage
- [x] Integration test examples
- [x] All tests passing
- [x] Telemetry implementation (Prometheus + expvar)

### Next Phase: Production Readiness

Remaining work for production deployment:
- [ ] End-to-end integration tests with real backend
- [ ] Backend API implementation (Django)
- [ ] Performance testing with high-throughput sources
- [ ] Binary packaging for Linux, macOS, Windows
- [ ] Installation documentation
- [ ] Operator training materials
- [ ] Production deployment guide

## Development Workflow

### Adding a New Source

1. Define interface implementation in `internal/sources/`
2. Create normalizer in `internal/normalize/`
3. Add tests with mock data
4. Register source type in `internal/sources/registry.go`
5. Add command flags for source-specific options
6. Update documentation in `README.md`

### Adding a New Command

1. Create command file in `cmd/astrolabe/`
2. Define flags and help text
3. Implement `RunE` function with error handling
4. Add command to root in `cmd/astrolabe/root.go`
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

**⚠️ IMPORTANT: Question Feature Value Before Implementation**

Before implementing new features, especially UX features, critically evaluate:

1. **Does this add a third way to do something?**
   - If yes, it's probably not worth it
   - Example: We already have `astrolabe tui` (interactive) and `astrolabe capture serial` (CLI)
   - Adding `astrolabe run` would be a third entry point with unclear value

2. **Does it duplicate existing functionality?**
   - Check if the feature can be achieved by improving existing commands
   - Example: Instead of `astrolabe run`, make `astrolabe` (no args) launch TUI

3. **Does it match the tool's usage patterns?**
   - `mise run` works because tasks are pre-defined and non-interactive
   - Astrolabe operations require configuration (ports, files, run IDs)
   - Interactive menus don't fit this model well

**When proposing features, Claude should:**
- Challenge the value proposition early
- Point out if it creates UI/UX confusion
- Suggest simpler alternatives
- Ask "is the juice worth the squeeze?"
- Help rubber duck without burning tokens on premature implementation

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

### Plugin System

Design extensibility points:
- Source plugins (dynamic loading)
- Normalizer plugins (custom formats)
- Upload targets (multiple backends)

### Advanced Features

- **Streaming uploads**: Upload while capturing (low latency)
- **Compression**: Gzip artifacts before upload
- **Encryption**: E2E encryption for sensitive data
- **Multi-device capture**: Parallel capture from multiple sources
- **Live dashboards**: WebSocket streaming to browser UI

### Performance Optimization

- **Zero-copy reads**: Use `io.ReaderFrom` where possible
- **Memory pooling**: Reuse buffers for high-throughput sources
- **Parallel normalization**: Pipeline stages with goroutines
- **Incremental checksums**: Streaming hash computation

---

## Quick Reference

### Essential Commands During Development

```bash
# Build the binary
mise run build

# Run the TUI (recommended for operators)
mise run tui
# OR simply run astrolabe with no arguments:
./bin/astrolabe

# Capture from serial source ✅
./bin/astrolabe capture serial /dev/ttyUSB0 --device-id bench-01 --baud 115200

# Capture from file ✅
./bin/astrolabe capture file data.csv --device-id test-unit-42

# Capture from TCP source ✅
./bin/astrolabe capture tcp 192.168.1.100:8080 --device-id sensor-01

# Upload cached runs
./bin/astrolabe upload

# List cached runs
./bin/astrolabe runs list

# Validate run artifacts
./bin/astrolabe validate <run-id>

# Configure settings
./bin/astrolabe config set api_url https://qa.example.com
./bin/astrolabe config get

# Show version
./bin/astrolabe version

# Run all tests
mise run test
go test ./...

# Run specific package tests
go test ./internal/sources/... -v
go test ./internal/upload/... -v

# Run tests with coverage
go test -cover ./...

# Run linters
mise run lint

# Clean build artifacts
rm -rf bin/ .gocache/

# Inspect local cache
ls -la ~/.astrolabe/runs/
cat ~/.astrolabe/runs/{run-id}/manifest.json | jq .
cat ~/.astrolabe/runs/{run-id}/data.jsonl | head

# Check telemetry (if enabled)
curl http://localhost:9090/metrics
```

### Batch File Ingestion

```bash
# Simple shell loop for batch ingestion
for file in /mnt/usb/test_data/*.{csv,jsonl}; do
    ./bin/astrolabe capture file "$file" --device-id archive-01
done

# With parallel processing (GNU Parallel)
ls /mnt/usb/test_data/*.csv | parallel ./bin/astrolabe capture file {} --device-id archive-01

# Find with specific criteria
find /data/archive -name "*.csv" -mtime -7 | while read file; do
    ./bin/astrolabe capture file "$file"
done
```

- File ingestion remains CLI-only; the TUI is optimized for real-time streaming sources (serial, TCP).
- Prefer shell scripting for repeatable automation of batch ingests.
- Future: a dedicated `astrolabe capture batch <directory>` command may be added if operators request it.

### Key File Locations

- Config: `~/.astrolabe/connection.yml`, `~/.astrolabe/metadata.json`
- Run cache: `~/.astrolabe/runs/`
- Logs: Stdout/stderr (structured JSON with `--json` flag)
- Build output: `./bin/astrolabe`
- Test cache: `./.gocache/` (project-local Go cache)

### Common Gotchas

- **Serial ports**: Require permissions (`sudo usermod -a -G dialout $USER`)
- **Cache directory**: Must be writable; fails silently if not
- **Checksums**: Calculate before renaming temp files
- **Context cancellation**: Always check `ctx.Done()` in loops
- **Defer order**: Last defer executes first (LIFO)

---

## Document History

This skill document should be updated as the project evolves. Keep it in sync with major architectural changes, new patterns, and lessons learned from production use.

**Last Updated**: 2025-10-25

**Major Updates:**
- 2025-10-25 (v2): Design guidance update
  - Added "Design Anti-Patterns" section with feature evaluation criteria
  - Documented decision to make `astrolabe` launch TUI by default
  - Added guidance for Claude to challenge low-value features early

- 2025-10-25 (v1): Updated to reflect MVP completion status
  - Added mandatory skill usage directive
  - Updated all implementation statuses (Serial ✅, File ✅, TCP ✅, Upload ✅, TUI ✅)
  - Added new sections: CLI Output Styling (cliout), Telemetry & Metrics
  - Updated Test Helpers section with MockSerial documentation
  - Reorganized Success Criteria Checklist with completion status
  - Updated Quick Reference with current command syntax
  - Added TUI screen details
  - Marked project as "MVP COMPLETE"

**Implementation Status Summary:**
- ✅ All MVP sources implemented and tested (Serial, File, TCP)
- ✅ Complete capture pipeline (sources → normalization → storage)
- ✅ Full upload system with retry logic and checksum validation
- ✅ Comprehensive TUI with 7+ screens
- ✅ Styled CLI output with JSON mode
- ✅ Telemetry and metrics collection
- ✅ MockSerial test helper with full documentation
- ✅ All tests passing
- 🚧 Backend API integration (in progress)
- 🚧 Production packaging and deployment (planned)
