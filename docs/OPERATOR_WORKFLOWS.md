# Operator Workflows

Common task patterns for day-to-day Astrolabe operations.

---

## Serial Capture Workflow

**Use case:** Firmware testing, device validation, production line testing

### TUI Method (Recommended)

```bash
astrolabe tui
```

1. Select **List Ports** to identify your device
2. Select **Capture Serial**
3. Choose port from list
4. Set baud rate (common: 9600, 115200)
5. Press Enter to start capture
6. Run your test on the device
7. Press `s` to save when complete
8. Select **Upload Data** to send to backend

### CLI Method

```bash
# Identify available ports
astrolabe ports list

# Start capture
astrolabe capture serial /dev/ttyUSB0 \
  --baud 115200 \
  --device-id "DUT-001" \
  --test-plan "functional-test"

# Press Ctrl+C when done

# Upload
astrolabe upload
```

### Tips

- Match baud rate to device configuration
- Use consistent device IDs for tracking
- Add firmware version to metadata for traceability

---

## File Ingestion Workflow

**Use case:** Historical data import, CI/CD results, instrument exports

### Single File Import

```bash
# CSV with headers
astrolabe capture file test-results.csv \
  --device-id "archive-001" \
  --test-plan "historical-import"

# Upload
astrolabe upload
```

### Batch File Import

```bash
# Linux/macOS: Process all CSV files in directory
for file in /path/to/data/*.csv; do
  astrolabe capture file "$file" \
    --test-plan "batch-import" \
    --tag "migration"
done

# Windows PowerShell
Get-ChildItem D:\data\*.csv | ForEach-Object {
  astrolabe capture file $_.FullName --test-plan "batch-import"
}

# Upload all at once
astrolabe upload
```

### Supported Formats

| Extension | Format | Notes |
|-----------|--------|-------|
| `.csv` | CSV | Auto-detects headers |
| `.jsonl`, `.ndjson` | JSONL | One JSON object per line |
| `.log`, `.txt` | Raw | Each line becomes a record |

### Common Flags

```bash
# CSV without headers
astrolabe capture file data.csv --no-headers

# Tab-separated
astrolabe capture file data.tsv --delimiter '\t'

# Skip header lines
astrolabe capture file data.csv --skip-lines 2

# Custom column names
astrolabe capture file data.csv --columns "time,voltage,current"
```

---

## TCP Capture Workflow

**Use case:** Network instruments, remote sensors, protocol testing

### TUI Method

```bash
astrolabe tui
```

1. Select **Capture TCP**
2. Enter host address (e.g., `192.168.1.100`)
3. Select or enter port number
4. Connection validates automatically
5. Press Enter to start streaming
6. Press `s` to save when complete

### CLI Method

```bash
# Capture from network instrument
astrolabe capture tcp 192.168.1.100 5025 \
  --device-id "oscilloscope-01" \
  --test-plan "signal-measurement"

# With timeouts for slow connections
astrolabe capture tcp slow-device 9000 \
  --connect-timeout 30s \
  --read-timeout 120s

# Upload
astrolabe upload
```

### Common Ports

| Port | Common Use |
|------|------------|
| 5025 | SCPI instruments |
| 8080 | HTTP services |
| 9000 | Data loggers |
| 5000/3000 | Development servers |

---

## Batch Processing Workflow

**Use case:** Processing multiple files or devices systematically

### Multiple Files with Metadata

```bash
#!/bin/bash
# batch-import.sh

BATCH_ID="BATCH-$(date +%Y%m%d)"
DATA_DIR="/path/to/data"

for file in "$DATA_DIR"/*.csv; do
  filename=$(basename "$file")

  astrolabe capture file "$file" \
    --test-plan "production-data" \
    --tag "$BATCH_ID" \
    --attr "source_file=$filename"

  echo "Processed: $filename"
done

# Upload all runs
astrolabe upload

echo "Batch $BATCH_ID complete"
```

### Multiple Devices (Serial)

```bash
#!/bin/bash
# multi-device-capture.sh

DEVICES=(
  "/dev/ttyUSB0:DUT-001"
  "/dev/ttyUSB1:DUT-002"
  "/dev/ttyUSB2:DUT-003"
)

for entry in "${DEVICES[@]}"; do
  PORT="${entry%%:*}"
  DEVICE_ID="${entry##*:}"

  echo "Capturing from $DEVICE_ID on $PORT..."

  # Run capture in background
  astrolabe capture serial "$PORT" \
    --baud 115200 \
    --device-id "$DEVICE_ID" \
    --test-plan "multi-device-test" \
    &
done

# Wait for user to stop
echo "Press Enter to stop all captures..."
read

# Stop all captures
killall -INT astrolabe

# Upload
astrolabe upload
```

---

## Continuous Monitoring Setup

**Use case:** Long-running data collection, environmental monitoring

### Timed Capture Script

```bash
#!/bin/bash
# timed-capture.sh

DURATION_SECONDS=3600  # 1 hour
PORT="/dev/ttyUSB0"

# Start capture in background
astrolabe capture serial "$PORT" \
  --baud 115200 \
  --device-id "monitor-01" \
  --test-plan "environmental-monitoring" \
  --tag "long-running" \
  &

CAPTURE_PID=$!

# Wait for duration
sleep $DURATION_SECONDS

# Stop capture gracefully
kill -INT $CAPTURE_PID
wait $CAPTURE_PID

# Upload
astrolabe upload
```

### Periodic Capture with Cron

```bash
# Run hourly capture (add to crontab)
0 * * * * /path/to/scripts/hourly-capture.sh >> /var/log/astrolabe.log 2>&1
```

```bash
#!/bin/bash
# hourly-capture.sh

TIMESTAMP=$(date +%Y%m%d-%H%M)

# Capture for 55 minutes (leave 5 min buffer)
timeout 3300 astrolabe capture tcp sensor-gateway 9000 \
  --test-plan "hourly-monitoring" \
  --attr "capture_time=$TIMESTAMP"

# Upload
astrolabe upload
```

---

## CI/CD Integration

**Use case:** Automated testing in pipelines

### Jenkins Pipeline

```groovy
pipeline {
  agent any

  stages {
    stage('Test') {
      steps {
        // Run tests, generate output
        sh 'make test > test-output.log'
      }
    }

    stage('Capture Results') {
      steps {
        sh '''
          astrolabe capture file test-output.log \
            --test-plan "ci-regression" \
            --operator "jenkins" \
            --attr "build_number=${BUILD_NUMBER}" \
            --attr "git_commit=${GIT_COMMIT}" \
            --tag "ci" \
            --tag "automated"
        '''
      }
    }

    stage('Upload') {
      steps {
        sh 'astrolabe upload'
      }
    }
  }
}
```

### GitHub Actions

```yaml
name: Test and Upload
on: [push]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Tests
        run: make test > test-results.csv

      - name: Setup Astrolabe
        run: ./scripts/install.sh

      - name: Configure Astrolabe
        env:
          ASTROLABE_API_URL: ${{ secrets.ASTROLABE_API_URL }}
          ASTROLABE_PROJECT_ID: ${{ secrets.ASTROLABE_PROJECT_ID }}
          ASTROLABE_AUTH_TOKEN: ${{ secrets.ASTROLABE_AUTH_TOKEN }}
        run: |
          astrolabe capture file test-results.csv \
            --test-plan "ci-regression" \
            --operator "github-actions" \
            --attr "commit=${{ github.sha }}" \
            --attr "branch=${{ github.ref_name }}" \
            --tag "ci"

          astrolabe upload
```

---

## Quick Reference: Common Commands

| Task | Command |
|------|---------|
| Launch TUI | `astrolabe tui` |
| List serial ports | `astrolabe ports list` |
| Capture serial | `astrolabe capture serial <port> --baud <rate>` |
| Capture TCP | `astrolabe capture tcp <host> <port>` |
| Import file | `astrolabe capture file <path>` |
| List runs | `astrolabe runs list` |
| Upload all | `astrolabe upload` |
| Upload specific run | `astrolabe upload --run-id <id>` |
| Check config | `astrolabe config get` |
| Show version | `astrolabe version` |

---

## Quick Reference: Metadata Flags

These flags work with all capture commands:

| Flag | Description | Example |
|------|-------------|---------|
| `--device-id` | Device identifier | `--device-id "DUT-001"` |
| `--device-firmware` | Firmware version | `--device-firmware "1.2.3"` |
| `--test-plan` | Test plan name | `--test-plan "regression"` |
| `--operator` | Operator name | `--operator "alice"` |
| `--location` | Test location | `--location "lab-1"` |
| `--tag` | Tag (repeatable) | `--tag "production" --tag "batch-1"` |
| `--attr` | Custom attribute | `--attr "temp=25" --attr "voltage=3.3"` |

---

## Related Documentation

- [Operator Quickstart](OPERATOR_QUICKSTART.md) - Getting started
- [Troubleshooting Guide](OPERATOR_TROUBLESHOOTING.md) - Problem solving
- [Metadata Best Practices](METADATA_BEST_PRACTICES.md) - Effective tagging
- [File Ingestion Guide](FILE_INGESTION.md) - Detailed file import options
- [TCP Capture Guide](TCP_CAPTURE.md) - Network capture details
