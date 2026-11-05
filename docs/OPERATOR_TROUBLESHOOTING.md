# Operator Troubleshooting Guide

Self-service guide for diagnosing and resolving common Astrolabe issues.

---

## Quick Diagnosis Checklist

Before diving into specific issues, verify these basics:

- [ ] Astrolabe is installed: `astrolabe --help`
- [ ] Configuration exists: `astrolabe config get`
- [ ] Network is reachable: Can you access the API URL in a browser?
- [ ] Device is connected: Check cables and power

---

## Installation Issues

### Binary Not Found / Command Not Recognized

**Symptoms:**
- `astrolabe: command not found` (Linux/macOS)
- `'astrolabe' is not recognized` (Windows)

**Solutions:**

1. **Check if installed:**
   ```bash
   # Linux/macOS
   which astrolabe
   ls ~/.local/bin/astrolabe

   # Windows (PowerShell)
   Get-Command astrolabe
   ```

2. **Add to PATH:**
   ```bash
   # Linux/macOS - add to ~/.bashrc or ~/.zshrc
   export PATH="$HOME/.local/bin:$PATH"
   source ~/.bashrc

   # Windows - check System Properties → Environment Variables
   # User PATH should include: %USERPROFILE%\.local\bin
   ```

3. **Reinstall:**
   ```bash
   # Linux/macOS
   ./scripts/install.sh

   # Windows (PowerShell as Administrator)
   .\scripts\install.ps1
   ```

### Permission Denied During Install

**Symptoms:**
- Cannot write to installation directory
- Access denied errors

**Solutions:**

- **Linux/macOS:** Use user-local install (default) or run with sudo for system-wide
- **Windows:** Run PowerShell as Administrator for system-wide install

---

## Serial Port Issues

### Port Not Detected

**Symptoms:**
- Port not shown in the Capture → Serial tab

**Solutions:**

1. **Verify physical connection:**
   - Check USB cable is firmly connected
   - Try a different USB port
   - Try a different cable

2. **Check system detection:**
   ```bash
   # Linux
   ls /dev/ttyUSB* /dev/ttyACM*
   dmesg | tail -20  # Check for recent USB events

   # macOS
   ls /dev/cu.* /dev/tty.*

   # Windows (PowerShell)
   Get-WmiObject Win32_SerialPort | Select Name, DeviceID
   # Or check Device Manager → Ports (COM & LPT)
   ```

3. **Install drivers (Windows):**
   - Download driver from device manufacturer
   - Install and restart if required

### Permission Denied (Linux)

**Symptoms:**
- `cannot open /dev/ttyUSB0: Permission denied`
- Port visible but cannot connect

**Solutions:**

1. **Add user to dialout group:**
   ```bash
   sudo usermod -a -G dialout $USER
   ```

2. **Log out and log back in** (required for group change)

3. **Verify group membership:**
   ```bash
   groups | grep dialout
   ```

4. **Temporary fix (not recommended for production):**
   ```bash
   sudo chmod 666 /dev/ttyUSB0
   ```

### Permission Denied (macOS)

**Symptoms:**
- Cannot access `/dev/cu.*` or `/dev/tty.*`

**Solutions:**

1. **Check System Preferences → Security & Privacy → Privacy → Full Disk Access**
2. **Grant terminal app access if prompted**

### Port Busy / Locked

**Symptoms:**
- `Device or resource busy`
- `Port is already in use`

**Solutions:**

1. **Find what's using the port:**
   ```bash
   # Linux
   lsof /dev/ttyUSB0
   fuser /dev/ttyUSB0

   # macOS
   lsof | grep cu.usbserial
   ```

2. **Close other applications** using the port (screen, minicom, Arduino IDE, etc.)

3. **Kill blocking process:**
   ```bash
   # Linux/macOS (use PID from lsof output)
   kill <PID>
   ```

### No Data Received

**Symptoms:**
- Connected successfully but no data appears
- Capture screen stays empty

**Solutions:**

1. **Verify baud rate matches device:**
   - Common rates: 9600, 19200, 38400, 57600, 115200, 921600
   - Check device documentation

2. **Verify device is sending data:**
   - Use separate terminal program to test
   - Check device power and status LEDs

3. **Check cable type:**
   - Some cables are charge-only (no data lines)
   - Try a known-good data cable

---

## TCP Connection Issues

### Connection Refused

**Symptoms:**
- `connection refused`
- Cannot connect to host

**Solutions:**

1. **Verify host is reachable:**
   ```bash
   ping 192.168.1.100
   ```

2. **Verify port is open:**
   ```bash
   # Linux/macOS
   nc -zv 192.168.1.100 8080

   # Windows (PowerShell)
   Test-NetConnection -ComputerName 192.168.1.100 -Port 8080
   ```

3. **Check firewall rules** on both client and server

4. **Verify service is running** on the target device

### Connection Timeout

**Symptoms:**
- Connection hangs then fails
- `connection timed out`

**Solutions:**

1. **Check network connectivity:**
   ```bash
   ping <host>
   traceroute <host>  # Linux/macOS
   tracert <host>     # Windows
   ```

2. **Verify correct IP address and port**

3. **Check for network segmentation** (VLANs, subnets)

4. **Try from another machine** to isolate the issue

### No Data Received (TCP)

**Symptoms:**
- Connected but no data streams

**Solutions:**

1. **Verify device is actively sending data**
2. **Check if device requires a handshake or command to start**
3. **Verify protocol matches** (some devices need specific initialization)

---

## Device Resets Mid-Capture

If the device reboots, is unplugged, or the TCP server restarts during a headless `astrolabe capture serial` or `capture tcp`, the capture ends by default. Add `--reconnect` to keep the run going:

```bash
astrolabe capture serial --port /dev/ttyUSB0 --reconnect
astrolabe capture tcp --host 192.168.1.100 --port 9000 --reconnect --reconnect-timeout 10m
```

- `--reconnect` retries with backoff (250ms growing to 5s) until the link returns. The same run continues.
- `--reconnect-timeout` gives up after the link has been down this long in one outage (`0`, the default, never gives up). It is ignored without `--reconnect`.

Each outage is recorded in `data.jsonl`:

- `link_down` with an `error` payload when the link drops.
- `link_up` with `attempts` and `gap_ms` (and `port` if the serial port name changed) when it returns.

If a USB serial device comes back under a new name (for example `/dev/ttyACM0` to `/dev/ttyACM1`), the capture follows it by its USB serial number, and `link_up` records the new `port`. Devices are never matched by VID/PID alone. Adapters that report no USB serial number are only retried under the original name. So are devices that expose several serial ports under one USB serial number (for example dual CDC-ACM or FT2232).

When `--reconnect-timeout` expires, the command prints "Reconnect timed out after <timeout>; ending capture" and saves the run, which then ends with a `link_down` and no `link_up`.

In the TUI live view, reconnect is on by default and the `r` key toggles it (the status bar shows `↻ on` or `↻ off`; the choice is remembered). While the link is down the status bar shows `RECONNECTING`, and `RECONNECTED` for a few seconds after it returns. Marker lines for the drop and the recovery appear in the buffer. Anything you type while reconnecting is kept in the input field but not sent until the link is back.

The run manifest gets a `reconnects` attribute counting recoveries. Data sent by the device while the link was down is not captured; the gap is bounded by the `link_down` and `link_up` records.

---

## Upload Issues

### Network Unreachable

**Symptoms:**
- `network is unreachable`
- `no route to host`

**Solutions:**

1. **Check internet connectivity:**
   ```bash
   ping 8.8.8.8
   curl -I https://google.com
   ```

2. **Check DNS resolution:**
   ```bash
   nslookup your-api-server.com
   ```

3. **Verify API URL is correct:**
   ```bash
   astrolabe config get api_url
   ```

4. **Check proxy settings** if behind corporate proxy

### Authentication Failed (401/403)

**Symptoms:**
- `authentication failed`
- `unauthorized`
- `forbidden`

**Solutions:**

1. **Verify auth token is set:**
   ```bash
   astrolabe config get auth_token
   ```

2. **Check token is valid:**
   - Tokens may expire - get a fresh one from administrator
   - Ensure no extra whitespace when copying

3. **Reconfigure token:**
   ```bash
   astrolabe tui
   # → Configure Connection → Set Auth Token → Ctrl+S
   ```

### API Errors (400 Bad Request)

**Symptoms:**
- `bad request`
- `invalid data`

**Solutions:**

1. **Verify Project ID is correct:**
   ```bash
   astrolabe config get project_id
   ```

2. **Check data format:**
   - Ensure captured data is valid
   - Check for corrupted files in `~/.astrolabe/runs/`

### API Errors (500 Internal Server Error)

**Symptoms:**
- `internal server error`
- Upload fails repeatedly

**Solutions:**

1. **Wait and retry** - server may be temporarily overloaded
2. **Check backend status** with your administrator
3. **Data will remain in local cache** - retry upload later

### Retry Exhausted

**Symptoms:**
- `max retries exceeded`
- Upload keeps failing

**Solutions:**

1. **Check network stability**
2. **Verify backend is accessible:**
   ```bash
   curl -I <API_URL>
   ```
3. **Data is preserved locally** - retry when issue resolved:
   ```bash
   astrolabe upload
   ```

---

## Configuration Issues

### Invalid Configuration File

**Symptoms:**
- `failed to parse config`
- `invalid YAML`

**Solutions:**

1. **Check YAML syntax:**
   ```bash
   # Linux/macOS
   cat ~/.astrolabe/connection.yml

   # Windows
   type %USERPROFILE%\.astrolabe\connection.yml
   ```

2. **Reset configuration:**
   ```bash
   # Backup and recreate
   mv ~/.astrolabe/connection.yml ~/.astrolabe/connection.yml.bak
   astrolabe tui
   # → Configure Connection → Enter values → Ctrl+S
   ```

### Missing Required Fields

**Symptoms:**
- `api_url not configured`
- `project_id required`

**Solutions:**

1. **Set missing fields via TUI:**
   ```bash
   astrolabe tui
   # → Configure Connection
   ```

2. **Or via environment variables:**
   ```bash
   export ASTROLABE_API_URL="https://your-server.com"
   export ASTROLABE_PROJECT_ID="your-project"
   export ASTROLABE_AUTH_TOKEN="your-token"
   ```

---

## Data Issues

### Capture Not Saving

**Symptoms:**
- Pressed `s` but no run created
- Run not appearing in View Runs

**Solutions:**

1. **Check cache directory permissions:**
   ```bash
   ls -la ~/.astrolabe/
   ```

2. **Verify disk space:**
   ```bash
   df -h ~/.astrolabe/
   ```

3. **Check for errors in output**

### Corrupted or Missing Runs

**Symptoms:**
- Run listed but cannot view
- Upload fails for specific run

**Solutions:**

1. **Check run directory structure:**
   ```bash
   ls -la ~/.astrolabe/runs/<run-id>/
   ```
   Should contain: `manifest.json`, `capture.jsonl`

2. **Validate manifest:**
   ```bash
   cat ~/.astrolabe/runs/<run-id>/manifest.json | python -m json.tool
   ```

3. **If corrupted, may need to delete and re-capture:**
   ```bash
   rm -rf ~/.astrolabe/runs/<run-id>/
   ```

---

## Log Locations

**Cache and Data:**
- Linux/macOS: `~/.astrolabe/`
- Windows: `%USERPROFILE%\.astrolabe\`

**Run Data:**
- `~/.astrolabe/runs/<run-id>/manifest.json`
- `~/.astrolabe/runs/<run-id>/capture.jsonl`

**Configuration:**
- `~/.astrolabe/connection.yml`
- `~/.astrolabe/metadata.json`

**Inspect Run Data Directly:**
```bash
# View run manifest and captured data
ls ~/.astrolabe/runs/
cat ~/.astrolabe/runs/<run-id>/manifest.json
```

---

## Getting Help

If you cannot resolve the issue:

1. **Collect diagnostic info:**
   ```bash
   astrolabe version
   astrolabe config get
   ls -la ~/.astrolabe/
   ```

2. **Note the exact error message**

3. **Document steps to reproduce**

4. **Contact your system administrator** with this information

---

## Related Documentation

- [Operator Quickstart](OPERATOR_QUICKSTART.md) - Getting started
- [User Guide](USER_GUIDE.md) - Complete usage reference
- [Upload Guide](UPLOAD_GUIDE.md) - Backend integration details
- [Directory Structure](DIRECTORY_STRUCTURE.md) - File organization
