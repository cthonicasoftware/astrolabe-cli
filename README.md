## CLI Agent Outline

### 1) Purpose & scope

- Standardize data acquisition from test benches, devices, and instruments.  
- Provide reliable capture, normalization, and upload of test artifacts into the QA app.  
- Replace ad-hoc scripts with a single, consistent tool that engineers can trust.

### 2) End-to-end flow

1. User/CI invokes `astrolabe capture …`  
2. Agent connects to source (serial, TCP, file, or instrument plugin).  
3. Data is normalized into JSONL + manifest.  
4. Artifacts are stored locally with metadata.  
5. Upload job posts artifacts + metadata to the QA app.  
6. If offline, artifacts remain cached until connection is restored.

### 3) Core commands

- `astrolabe capture` – start a capture from a port/device.  
- `astrolabe upload` – send cached runs to server.  
- `astrolabe config` – manage API tokens, defaults (baud, port).  
- `astrolabe validate` – check file/manifest consistency before upload.  
- `astrolabe version` – report agent + schema versions.

### 4) Configuration

- Defaults stored in `.astrolabe.yml`.  
- Fields: API URL, project ID, auth token, capture defaults (baud, port, sample rate), offline cache path.  
- Environment variables override config (e.g., `ASTROLABE_TOKEN`).  
- Configurable retry/backoff and upload batch size.

### 5) Supported sources (MVP)

- **Serial ports** (USB-UART, RS-485, etc.)  
- **Files** (CSV, JSONL, logs) for retroactive ingestion.  
- **TCP sockets** (simple streaming sources).  
- Future: USB (via libusb), SCPI instruments (DMM, scope).

### 6) Normalization & metadata

- Capture saved with:
  - Run manifest: device ID, firmware hash, test plan, operator, timestamp.  
  - Data file: JSONL or chunked binary with sidecar metadata.  
  - Checksums for integrity.  
- Schema version stamped in every run.

### 7) Offline & resilience

- Local cache directory holds artifacts until uploaded.  
- Auto-retry with exponential backoff.  
- Resume partial uploads.  
- CLI flags for `--offline` and `--force-upload`.

### 8) Integration with QA app

- Uses presigned URLs or API tokens for upload.  
- All artifacts tied to a `run_id` created in the Rails backend.  
- Agent reports parser version + capture conditions.  
- Server treats agent uploads just like manual file uploads.

### 9) Implementation notes

- Language: Go (static binary, cross-platform)  
- Logging: structured JSON logs for CI parsing.  
- Packaging: prebuilt binaries for Linux/macOS/Windows.  
- Tests: simulate serial/TCP streams, offline caches, and upload failures.

### 10) Success criteria (v0)
- Reliable serial capture → local JSONL + manifest.  
- Upload works with API token + presigned URLs.  
- Offline cache + retry proven in tests.  
- Deterministic schema versioning.  
- Command help/docs are self-contained (`astrolabe --help`).  


## Project Setup
A stylish, operator-friendly CLI agent skeleton for standardized capture and upload of QA artifacts.

### Requirements
- [mise](https://mise.jdx.dev/) (to install tool versions)
- Go 1.25.1 (pinned via `mise.toml`)

### Quick start
```sh
# activate toolchain
mise trust
mise install
mise activate

# build
mise run build

# run
mise run tui
mise run cli <command>
```
- Go tool invocations use a project-local cache (`.gocache`) so builds/tests work even in sandboxed environments.

### Project layout
```
cmd/astrolabe/       # Cobra commands entrypoints
internal/core/       # Core domain models (Run, Record, SourceMeta)
internal/config/     # Config loader (file + env + flags)
internal/logging/    # Logging helpers
internal/sources/    # Source interfaces & (future) implementations
internal/normalize/  # Normalizers (bytes → records)
internal/storage/    # Filesystem storage (manifest + JSONL)
internal/upload/     # Upload client (presigned URLs / token)
internal/tui/        # TUI (bubbletea)
```

### Data model primitives
- `Run`: capture session envelope that links the source, manifest, capture settings, artifacts, and upload lifecycle.
- `Manifest`: operator-supplied metadata stamped on every run; nests `DeviceInfo` (id, firmware, hardware rev) and `TestInfo` (plan, variant, run number) plus optional tags/attributes.
- `CaptureSettings`: normalized view of how the stream was acquired (sample rate, duration hint, channel list).
- `Record`: single normalized datum emitted by a source; ordered via `seq` and timestamped.
- `Artifact`: on-disk payload belonging to the run (manifest JSON, JSONL data, device logs, attachments) with checksum + media type; `ArtifactRole` distinguishes core data vs. extras.
- `UploadState`: tracks reconciliation with the QA backend (queue status, attempts, timestamps, remote run id).

Supporting types (`SourceMeta`, `Checksum`, etc.) live in `internal/core/types.go` and are intended to be shared across packages.
