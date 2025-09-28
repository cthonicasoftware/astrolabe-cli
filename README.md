# CLI Agent Outline

## 1) Purpose & scope
- Standardize data acquisition from test benches, devices, and instruments.  
- Provide reliable capture, normalization, and upload of test artifacts into the QA app.  
- Replace ad-hoc scripts with a single, consistent tool that engineers can trust.

## 2) End-to-end flow
1. User/CI invokes `qa-agent capture …`  
2. Agent connects to source (serial, TCP, file, or instrument plugin).  
3. Data is normalized into JSONL + manifest.  
4. Artifacts are stored locally with metadata.  
5. Upload job posts artifacts + metadata to the QA app.  
6. If offline, artifacts remain cached until connection is restored.

## 3) Core commands
- `qa-agent capture` – start a capture from a port/device.  
- `qa-agent upload` – send cached runs to server.  
- `qa-agent config` – manage API tokens, defaults (baud, port).  
- `qa-agent validate` – check file/manifest consistency before upload.  
- `qa-agent version` – report agent + schema versions.

## 4) Configuration
- Defaults stored in `.qa-agent.yml`.  
- Fields: API URL, project ID, auth token, capture defaults (baud, port, sample rate), offline cache path.  
- Environment variables override config (e.g., `QA_AGENT_TOKEN`).  
- Configurable retry/backoff and upload batch size.

## 5) Supported sources (MVP)
- **Serial ports** (USB-UART, RS-485, etc.)  
- **Files** (CSV, JSONL, logs) for retroactive ingestion.  
- **TCP sockets** (simple streaming sources).  
- Future: USB (via libusb), SCPI instruments (DMM, scope).

## 6) Normalization & metadata
- Capture saved with:
  - Run manifest: device ID, firmware hash, test plan, operator, timestamp.  
  - Data file: JSONL or chunked binary with sidecar metadata.  
  - Checksums for integrity.  
- Schema version stamped in every run.

## 7) Offline & resilience
- Local cache directory holds artifacts until uploaded.  
- Auto-retry with exponential backoff.  
- Resume partial uploads.  
- CLI flags for `--offline` and `--force-upload`.

## 8) Integration with QA app
- Uses presigned URLs or API tokens for upload.  
- All artifacts tied to a `run_id` created in the Rails backend.  
- Agent reports parser version + capture conditions.  
- Server treats agent uploads just like manual file uploads.

## 9) Implementation notes
- Language: Go (static binary, cross-platform) or Python (if packaging is acceptable).  
- Logging: structured JSON logs for CI parsing.  
- Packaging: prebuilt binaries for Linux/macOS/Windows.  
- Tests: simulate serial/TCP streams, offline caches, and upload failures.

## 10) Success criteria (v0)
- Reliable serial capture → local JSONL + manifest.  
- Upload works with API token + presigned URLs.  
- Offline cache + retry proven in tests.  
- Deterministic schema versioning.  
- Command help/docs are self-contained (`qa-agent --help`).  
