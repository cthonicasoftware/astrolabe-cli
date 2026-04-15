# TCP Capture Guide

The TCP capture feature allows you to connect to TCP servers and stream data into Astrolabe for archiving, analysis, and upload to the QA backend.

## Overview

TCP capture connects to a remote host:port and streams data until interrupted (Ctrl+C). Data is normalized line-by-line using the same LineJSON normalizer as serial capture.

## Basic Usage

```bash
astrolabe capture tcp --host <host> --port <port>
```

### Examples

```bash
# Capture from localhost:8080
astrolabe capture tcp --host localhost --port 8080

# Capture from remote server
astrolabe capture tcp --host 192.168.1.100 --port 9000

# Capture with test plan metadata
astrolabe capture tcp --host 10.0.0.50 --port 5000 --test-plan "network-test"
```

## Interactive TUI

The TCP capture feature is integrated into the interactive TUI with connection validation:

```bash
astrolabe tui
```

### TUI Workflow

1. **Select "Capture"** from the welcome menu and choose the **TCP** tab
2. **Enter host/IP address** (e.g., localhost, 192.168.1.100)
3. **Select port** from common ports or enter custom port
4. **Connection test** - automatically validates connection before proceeding
5. **Confirm** - launch capture TUI if connection succeeds

### Connection Validation

The TUI automatically tests the TCP connection before launching the capture:

- **Testing**: Shows "Testing connection..." while validating (3 second timeout)
- **Success**: Shows "✓ Connection successful!" and allows you to proceed
- **Failure**: Shows error message with option to retry or go back

This ensures you don't start a capture session with invalid connection settings. The connection is only established once you've confirmed your settings and the validation succeeds.

### Common Ports

The TUI provides quick selection for common TCP ports:

- 8080 (HTTP Alt)
- 9000 (Common)
- 5000 (Development)
- 5025 (SCPI Instruments)
- 7000 (Common)
- 3000 (Development)
- Custom port option

## Command Reference

### Syntax

```bash
astrolabe capture tcp [flags]
```

### Connection Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--host` | `-H` | Hostname or IP address of the TCP server | `localhost` |
| `--port` | `-p` | TCP port number (1-65535) | `9000` |

### TCP-Specific Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--connect-timeout` | Connection timeout | `10s` |
| `--read-timeout` | Read timeout (0 = no timeout) | `0` |
| `--buffer-size` | Read buffer size in bytes | `4096` |

### Metadata Flags

Same as serial and file capture:

| Flag | Description |
|------|-------------|
| `--operator` | Operator name |
| `--location` | Test location/bench identifier |
| `--device-id` | Device identifier |
| `--device-firmware` | Device firmware version |
| `--test-plan` | Test plan name |
| `--tag` | Tag (can be repeated) |
| `--attr` | Custom attribute key=value (can be repeated) |

## Use Cases

### Instrument Data Streaming

Many test instruments expose TCP interfaces for streaming measurement data:

```bash
# Connect to oscilloscope
astrolabe capture tcp --host 192.168.1.10 --port 5025 \
  --test-plan "signal-analysis" \
  --device-id "scope-001" \
  --operator "alice"
```

### Network Protocol Testing

Capture data from network services for analysis:

```bash
# Capture from custom test server
astrolabe capture tcp --host test-server.local --port 8888 \
  --test-plan "protocol-validation" \
  --tag "network" \
  --tag "automated"
```

### Data Logger Integration

Connect to remote data loggers:

```bash
# Capture environmental sensor data
astrolabe capture tcp --host sensor-gateway --port 9000 \
  --test-plan "environmental-monitoring" \
  --location "lab-3" \
  --read-timeout 60s
```

### CI/CD Integration

Capture test output from CI runners:

```bash
# Capture test results streaming from CI agent
astrolabe capture tcp --host ci-agent --port 7000 \
  --test-plan "ci-integration-test" \
  --operator "jenkins" \
  --tag "ci" \
  --tag "automated"
```

## Configuration Options

### Connection Timeout

Controls how long to wait for initial connection:

```bash
astrolabe capture tcp --host slow-server --port 9000 --connect-timeout 30s
```

Good for:
- Servers with slow connection handshakes
- High-latency networks
- Servers that queue connections

### Read Timeout

Controls how long to wait between reads (0 = infinite):

```bash
astrolabe capture tcp --host sporadic-server --port 9000 --read-timeout 120s
```

Good for:
- Servers that send data sporadically
- Detecting disconnections
- Preventing hangs on idle connections

**Note**: With `read-timeout 0` (default), capture will wait indefinitely for data.

### Buffer Size

Controls how much data to read at once:

```bash
# Larger buffer for high-throughput streams
astrolabe capture tcp --host fast-server --port 9000 --buffer-size 65536

# Smaller buffer for line-oriented protocols
astrolabe capture tcp --host line-server --port 9000 --buffer-size 1024
```

Good for:
- High-throughput: Larger buffers (16KB-64KB)
- Low-latency: Smaller buffers (1KB-4KB)
- Line-oriented: Default (4KB)

## Data Flow

1. **Connect**: Establish TCP connection to host:port
2. **Stream**: Read data from socket continuously
3. **Normalize**: Parse data line-by-line (LineJSON normalizer)
4. **Store**: Save normalized records to local cache
5. **Stop**: Ctrl+C to gracefully stop and save
6. **Upload**: Use `astrolabe upload` to send to backend

## Output Structure

Captured data is stored in `~/.astrolabe/runs/<run-id>/`:

```
~/.astrolabe/runs/run-20241015-103045/
├── manifest.json       # Run metadata including TCP address
└── data.jsonl          # Normalized records in JSONL format
```

### Manifest Example

```json
{
  "run_id": "run-20241015-103045",
  "source": {
    "kind": "tcp",
    "addr": "192.168.1.100:9000"
  },
  "manifest": {
    "test": {"plan": "network-test"},
    "attributes": {
      "source_kind": "tcp",
      "tcp_host": "192.168.1.100",
      "tcp_port": "9000",
      "tcp_addr": "192.168.1.100:9000"
    }
  },
  "capture": {
    "channels": ["tcp"],
    "notes": "TCP capture from 192.168.1.100:9000"
  }
}
```

### Record Structure

Data is normalized using LineJSON (same as serial):

```json
{
  "ts": "2024-01-15T10:00:00Z",
  "seq": 1,
  "type": "sample",
  "payload": {
    "line": "measurement: 3.3V"
  }
}
```

## Error Handling

### Connection Refused

```
Error: failed to connect to 127.0.0.1:9000: connection refused
```

**Solutions:**
- Verify server is running
- Check firewall rules
- Verify port number is correct
- Check host/IP address

### Connection Timeout

```
Error: failed to connect to slow-server:9000: context deadline exceeded
```

**Solutions:**
- Increase `--connect-timeout`
- Check network connectivity
- Verify server is accepting connections
- Check for network latency issues

### Read Timeout

If `--read-timeout` is set and server stops sending data:

```
# Capture will stop after timeout expires
```

**Solutions:**
- Increase timeout value
- Check server is still running
- Verify data is being generated

### Connection Closed by Server

If server closes connection:

```
✓ TCP capture complete
  Records: 150
```

Capture completes normally when connection closes.

## Best Practices

### 1. Use Appropriate Timeouts

```bash
# For slow/unreliable networks
astrolabe capture tcp --host remote-server --port 9000 \
  --connect-timeout 30s \
  --read-timeout 120s

# For fast local connections
astrolabe capture tcp --host localhost --port 8080 \
  --connect-timeout 2s
```

### 2. Add Descriptive Metadata

```bash
astrolabe capture tcp --host instrument --port 5025 \
  --test-plan "frequency-sweep" \
  --device-id "sig-gen-001" \
  --operator "bob" \
  --location "rf-lab" \
  --tag "rf" \
  --attr "frequency_start=1GHz" \
  --attr "frequency_stop=2GHz"
```

### 3. Use Ctrl+C for Clean Shutdown

Always use Ctrl+C (SIGINT) to stop capture cleanly. This ensures:
- All data is saved
- Manifest is written
- Connection is closed gracefully
- Upload state is properly set

### 4. Monitor Capture Progress

Output shows connection status and messages:

```
Connecting to TCP socket: 192.168.1.100:9000
Connected! Capturing data (press Ctrl+C to stop)...

^C
Interrupted. Saving capture...

✓ TCP capture complete
  Run ID:       run-20241015-103045
  Records:      1234
```

### 5. Verify Data Before Upload

```bash
# Check what was captured
ls -lh ~/.astrolabe/runs/run-*/

# View captured data
head ~/.astrolabe/runs/run-*/data.jsonl

# Then upload
astrolabe upload
```

## Comparison with Other Sources

| Feature | TCP | Serial | File |
|---------|-----|--------|------|
| **Mode** | Network stream | Hardware port | Disk read |
| **Duration** | Until Ctrl+C | Until Ctrl+C | Until EOF |
| **Reconnect** | Manual | Manual | N/A |
| **Latency** | Network-dependent | Low | Immediate |
| **Throughput** | Network-dependent | Baud-limited | Disk-limited |
| **Use Case** | Remote instruments | Local devices | Historical data |

## Advanced Usage

### Capture from Multiple Sources

Run multiple captures in parallel (different terminals):

```bash
# Terminal 1
astrolabe capture tcp --host instrument-1 --port 9000 --test-plan "multi-source-test"

# Terminal 2
astrolabe capture tcp --host instrument-2 --port 9001 --test-plan "multi-source-test"

# Terminal 3
astrolabe capture tcp --host instrument-3 --port 9002 --test-plan "multi-source-test"
```

Then upload all at once:
```bash
astrolabe upload
```

### Script-Based Automation

```bash
#!/bin/bash

# Start capture in background
astrolabe capture tcp --host data-source --port 9000 \
  --test-plan "automated-capture" \
  &

CAPTURE_PID=$!

# Run test for 60 seconds
sleep 60

# Stop capture
kill -INT $CAPTURE_PID

# Wait for capture to finish
wait $CAPTURE_PID

# Upload results
astrolabe upload
```

### Docker Container Integration

Capture from containerized services:

```bash
# Start test container
docker run -d -p 9000:9000 my-test-service

# Capture data
astrolabe capture tcp --host localhost --port 9000 --test-plan "container-test"

# Stop container
docker stop $(docker ps -q --filter ancestor=my-test-service)
```

## Troubleshooting

### No Data Received

**Symptoms**: Capture connects but receives no records

**Solutions**:
- Verify server is sending data
- Check if server requires initial handshake
- Try `telnet <host> <port>` to verify data flow
- Check firewall/network configuration

### Partial Data Loss

**Symptoms**: Some data appears missing

**Solutions**:
- Increase `--buffer-size`
- Check network stability
- Verify server isn't rate-limiting
- Look for error messages in output

### High Memory Usage

**Symptoms**: astrolabe consuming excessive memory

**Solutions**:
- Data is buffered in memory before writing
- Stop and restart capture periodically
- Check for extremely high data rates
- Consider file-based capture for very large datasets

## Security Considerations

### Network Security

- TCP connections are **not encrypted** by default
- Sensitive data should use VPN or SSH tunnel
- Example using SSH tunnel:

```bash
# Create SSH tunnel
ssh -L 9000:instrument-server:9000 user@gateway

# Capture through tunnel
astrolabe capture tcp --host localhost --port 9000 --test-plan "secure-capture"
```

### Firewall Configuration

Ensure firewall allows outbound TCP connections:

```bash
# Linux: Check firewall
sudo iptables -L

# Allow outbound to specific port
sudo iptables -A OUTPUT -p tcp --dport 9000 -j ACCEPT
```

## Next Steps

After capturing TCP data:

1. **Validate**: Check the run was created
   ```bash
   ls ~/.astrolabe/runs/
   ```

2. **Inspect**: View captured data
   ```bash
   head ~/.astrolabe/runs/run-*/data.jsonl
   ```

3. **Upload**: Send to QA backend
   ```bash
   astrolabe upload
   ```

## See Also

- [Serial Capture](TUI_WORKFLOW.md#capture-serial) - Capturing from serial ports
- [File Ingestion](FILE_INGESTION.md) - Importing existing files
- [Upload Guide](UPLOAD_GUIDE.md) - Uploading captured data
- [Configuration](CONFIG_TUI.md) - Setting up backend connection
