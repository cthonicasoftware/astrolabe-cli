# Astrolabe CLI

![astrolabe_cli_logo](images/astrolabe_cli_logo_animated.svg)

Astrolabe is a command-line tool for capturing QA run data from serial devices, TCP sources, and existing files, then validating and uploading those runs to your backend.


## Installation

### Prerequisites
- Go 1.25.0 (or use `mise` with the included `mise.toml`)

### Option 1: Build from source
```bash
git clone <your-repo-url>
cd astrolabe
go build -o astrolabe ./cmd/astrolabe
```

### Option 2: Use mise tasks
```bash
mise trust
mise install
mise run build
```

This creates the binary as `astrolabe`.

### First-time configuration
Astrolabe reads config from `~/.astrolabe/connection.yml` and environment variables.

Common environment variables:
- `ASTROLABE_API_URL`
- `ASTROLABE_PROJECT_ID`
- `ASTROLABE_AUTH_TOKEN`

You can also set metadata defaults:
```bash
astrolabe config set operator "Jane Doe"
astrolabe config set location "Bench A"
astrolabe config set device-id "dev-001"
astrolabe config get operator
```

## Features

- Capture from multiple sources:
  - Serial (`astrolabe capture serial`)
  - TCP (`astrolabe capture tcp`)
  - Files: CSV, JSONL, raw logs (`astrolabe capture file`)
- Interactive workflows via TUI (`astrolabe tui`, `astrolabe config edit`)
- Local offline run cache (defaults to `~/.astrolabe/runs`)
- Run validation before upload (`astrolabe validate <run_dir>`)
- Upload one run or all pending cached runs (`astrolabe upload`)
- JSON output mode for automation (`--json`)

## Examples

### Show help
```bash
astrolabe help
```


### Capture from serial
```bash
astrolabe capture serial --port /dev/ttyUSB0 --baud 115200 --test-plan smoke --tag lab
```

### Capture from TCP
```bash
astrolabe capture tcp --host 192.168.1.50 --port 9000 --test-plan burnin --attr station=west
```

### Ingest an existing file
```bash
astrolabe capture file ./data/results.csv --format csv --operator "Jane Doe" --location "Bench A"
```

### Validate a run before upload
```bash
astrolabe validate ~/.astrolabe/runs/<run-id>
```


### Upload runs
```bash
# upload all pending runs
astrolabe upload

# upload a specific run
astrolabe upload --run-id <run-id>
```

### Machine-readable output
```bash
astrolabe validate ~/.astrolabe/runs/<run-id> --json
```


## Documentation

Detailed guides are in [`docs/`](docs):

- [`docs/USER_GUIDE.md`](docs/USER_GUIDE.md)
- [`docs/OPERATOR_QUICKSTART.md`](docs/OPERATOR_QUICKSTART.md)
- [`docs/OPERATOR_WORKFLOWS.md`](docs/OPERATOR_WORKFLOWS.md)
- [`docs/OPERATOR_TROUBLESHOOTING.md`](docs/OPERATOR_TROUBLESHOOTING.md)
- [`docs/API_REFERENCE.md`](docs/API_REFERENCE.md)
- [`docs/UPLOAD_GUIDE.md`](docs/UPLOAD_GUIDE.md)
- [`docs/FILE_INGESTION.md`](docs/FILE_INGESTION.md)
- [`docs/TCP_CAPTURE.md`](docs/TCP_CAPTURE.md)
- [`docs/CONFIG_TUI.md`](docs/CONFIG_TUI.md)
- [`docs/TUI_WORKFLOW.md`](docs/TUI_WORKFLOW.md)
- [`docs/SYSTEM_REQUIREMENTS.md`](docs/SYSTEM_REQUIREMENTS.md)
