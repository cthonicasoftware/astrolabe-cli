# Astrolabe CLI

![astrolabe_cli_logo](images/astrolabe_cli_logo_animated.svg)

---
Astrolabe is a command-line tool for capturing QA run data from serial devices, TCP sources, and existing files, then validating and uploading those runs to your backend.
<img width="1431" height="622" alt="image" src="https://github.com/user-attachments/assets/5403ec8c-c65b-4f60-93b6-78337517ea7e" />


## Installation

### Prerequisites
- Go 1.25.1 (or use `mise` with the included `mise.toml`)

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

<img width="799" height="486" alt="image" src="https://github.com/user-attachments/assets/891df31f-6136-4a9b-b728-f049e2d1b314" />


## Examples

### Show help
```bash
astrolabe help
```
<img width="888" height="586" alt="image" src="https://github.com/user-attachments/assets/a3471f47-bba9-4c9e-8bd9-3999f37ffc33" />


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
<img width="954" height="466" alt="image" src="https://github.com/user-attachments/assets/c7635d73-700a-47a6-8322-07888b865434" />


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
<img width="919" height="124" alt="image" src="https://github.com/user-attachments/assets/2053fc16-f715-473f-9ea1-33bace8d5367" />


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
