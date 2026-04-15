# Astrolabe CLI User Guide

## Prerequisites

Before using Astrolabe, ensure you have:

**Required Access:**
- **Windows**: Serial port drivers installed for your device (check Device Manager under "Ports (COM & LPT)")
- **macOS**: No special permissions needed; serial ports accessible at `/dev/tty.*` or `/dev/cu.*`
- **Linux**: Dialout group membership for serial access:
  ```bash
  sudo usermod -a -G dialout $USER
  # Log out and back in for changes to take effect
  ```
- **Writable cache directory**:
  - Windows: `%USERPROFILE%\.astrolabe\`
  - macOS/Linux: `~/.astrolabe/`
- **Backend access**: Network connection to QA backend server
- **Authentication token**: Obtain from your system administrator

**Required Information:**
- Backend API URL (e.g., `https://qa.yourcompany.com`)
- Project ID (e.g., `project-123`)
- Device ID for your test equipment

## Overview

Astrolabe lets you capture test data from devices and upload it to the QA backend for analysis. You'll configure your connection once, then capture data from serial ports or TCP connections using an interactive interface. Data is saved locally first (offline-first design), so you can upload whenever network is available. By the end of this guide, you'll have captured a test run and uploaded it successfully.

## Step-by-Step Instructions

### First-Time Setup

**Step 1: Launch the TUI**
```bash
# Windows (Command Prompt or PowerShell)
astrolabe.exe tui

# macOS/Linux
astrolabe tui
```
**Expected result:** Welcome screen appears with menu options: Capture, View Runs, Upload Data, Configure Metadata, Configure Connection.

**Step 2: Configure Connection**
- Select "Configure Connection" (🔌)
- Press `Tab` to navigate between fields:
  - **API URL**: Enter your backend server URL
  - **Project ID**: Enter your project identifier
  - **Auth Token**: Enter authentication token (press `Ctrl+T` to toggle visibility)
- Press `Ctrl+S` to save

**Expected result:** Green message appears: `✓ Configuration Updated` with "Configuration saved. You can now upload runs."

**Step 3: Configure Metadata (Optional but Recommended)**
- Return to welcome screen (press `q`)
- Select "Configure Metadata" (📝)
- Set default values (device ID, hardware revision, location, operator name)
- Press `Ctrl+S` to save

**Expected result:** Configuration saved; these defaults will auto-populate for all future captures.

### Capturing Data

**Step 4: Identify Your Port (Serial Only)**
- Check your OS for available serial ports:
  - **Windows**: Device Manager → Ports (COM & LPT) — note the COM number (e.g., `COM3`)
  - **macOS**: `ls /dev/cu.*` in terminal
  - **Linux**: `ls /dev/ttyUSB* /dev/ttyACM*` in terminal

**Step 5: Start Capture**
- Select **Capture** from the welcome screen
- Choose the **Serial** or **TCP** tab
- Configure connection:
  - **Serial**: Choose your port and set baud rate (e.g., `115200`)
  - **TCP**: Enter host address and port number separately
- Press Enter to start

**Expected result:** Live data streams in real-time on screen.

**Step 6: Save Capture**
- Press `s` when capture is complete

**Expected result:** Blue message: `ℹ Run Saved` with run ID. Data stored in cache directory under `runs/{run_id}/`

### Uploading Data

**Step 7: Verify Captured Runs**
- Select "View Runs" (📊) from welcome screen
- Browse your captured runs
- Press `v` to view details, `q` to return

**Expected result:** List of runs with timestamps and record counts.

**Step 8: Upload to Backend**
- Select "Upload Data" (⬆️)
- Progress bar appears with spinner
- Successful uploads marked with ✓

**Expected result:** All pending runs uploaded. Message shows "X/Y runs uploaded" with green checkmarks.

## Code Examples

### CLI Alternative (Advanced Users)

```bash
# Windows: Capture from COM port
# --device-id: Your equipment identifier
# --baud: Communication speed (common: 9600, 115200)
astrolabe.exe capture serial --port COM3 --device-id test-bench-01 --baud 115200

# macOS: Capture from USB serial
./bin/astrolabe capture serial --port /dev/cu.usbserial-0001 --device-id test-bench-01 --baud 115200

# Linux: Capture from USB serial
./bin/astrolabe capture serial --port /dev/ttyUSB0 --device-id test-bench-01 --baud 115200

# Capture from file (batch import) - all platforms
# Useful for importing historical CSV data
astrolabe capture file data.csv --device-id archive-01

# Upload specific run by ID
astrolabe upload --run-id <run-id>

# Check a configuration value
astrolabe config get api_url
```

### Environment Variables (Alternative to TUI Config)

```bash
# Windows (PowerShell)
$env:ASTROLABE_API_URL="https://qa.yourcompany.com"
$env:ASTROLABE_PROJECT_ID="project-123"
$env:ASTROLABE_AUTH_TOKEN="your-token"

# macOS/Linux (Bash/Zsh)
export ASTROLABE_API_URL="https://qa.yourcompany.com"
export ASTROLABE_PROJECT_ID="project-123"
export ASTROLABE_AUTH_TOKEN="your-token"
```

### Batch File Processing

```powershell
# Windows PowerShell: Process multiple CSV files
Get-ChildItem D:\test_data\*.csv | ForEach-Object {
    astrolabe.exe capture file $_.FullName --device-id archive-01
}
```

```bash
# macOS/Linux: Process multiple CSV files from USB drive
for file in /mnt/usb/test_data/*.csv; do
    ./bin/astrolabe capture file "$file" --device-id archive-01
done
```

## Common Errors

### Error 1: "API URL not configured"
**When:** Attempting upload without setting backend URL
**Fix:**
```bash
astrolabe tui
# Select "Configure Connection" and set API URL, then save with Ctrl+S
```

### Error 2: "Can't find serial port"
**When:** Port doesn't exist or insufficient permissions
**Fix (Windows):** Check Device Manager → Ports (COM & LPT). Verify device is connected and driver installed.
**Fix (macOS):** List ports with `ls /dev/cu.*` to verify device connection.
**Fix (Linux):** Check dialout group membership:
```bash
groups | grep dialout
# If missing:
sudo usermod -a -G dialout $USER
# Log out and back in
```

### Error 3: "Connection refused: check your API URL and network"
**When:** Upload fails due to network or incorrect URL
**Fix:** Verify backend URL is correct and server is reachable:
```bash
astrolabe config get api_url
```

### Error 4: "Permission denied"
**When:** Cannot read/write config file
**Fix (Windows):** Check folder permissions in File Explorer → Right-click `.astrolabe` folder → Properties → Security
**Fix (macOS/Linux):**
```bash
chmod 700 ~/.astrolabe/
chmod 600 ~/.astrolabe/connection.yml
```

### Error 5: "auth token not configured"
**When:** Attempting upload without authentication token
**Fix:** Launch TUI, configure connection, and set auth token (use `Ctrl+T` to toggle visibility when entering).

## Troubleshooting

**IF upload fails:**
1. **Check configuration exists:**
   ```bash
   astrolabe config get api_url
   ```
   - If empty → Run `astrolabe tui` and configure connection

2. **Check network connectivity:**
   - Verify backend server URL is accessible
   - Test with browser or `curl <API_URL>`

3. **Check authentication:**
   ```bash
   astrolabe config get auth_token
   ```
   - If missing → Reconfigure with valid token

**IF serial port not found:**
1. **List available ports:**
   - **Windows**: Check Device Manager → Ports (COM & LPT) for the COM number
   - **macOS**: Run `ls /dev/cu.*` in terminal
   - **Linux**: Run `ls /dev/ttyUSB* /dev/ttyACM*` in terminal
   - Verify your device is connected

2. **Check drivers/permissions:**
   - **Windows**: Install manufacturer's USB driver
   - **macOS**: No action needed (drivers built-in)
   - **Linux**: Add to dialout group (see Error 2)

**IF data not saving:**
1. **Check cache directory:**
   - **Windows**: `dir %USERPROFILE%\.astrolabe\runs\`
   - **macOS/Linux**: `ls -la ~/.astrolabe/runs/`
   - If permission denied → Fix folder permissions

2. **Verify disk space:**
   - Ensure sufficient storage in cache directory

**IF captures missing metadata:**
1. **Set metadata defaults:**
   - Launch TUI → "Configure Metadata"
   - Set device ID, operator, location
   - Save with `Ctrl+S`

## Related Resources

- **[TUI Workflow Guide](TUI_WORKFLOW.md)** - Detailed interactive interface documentation
- **[Configuration Guide](CONFIG_TUI.md)** - Advanced configuration options
- **[Upload Guide](UPLOAD_GUIDE.md)** - Backend integration and upload process details

---

**Keyboard Reference:**
- `↑`/`↓` or `k`/`j` - Navigate menus
- `Enter` - Select option
- `Tab`/`Shift+Tab` - Navigate form fields
- `Ctrl+S` - Save configuration
- `Ctrl+T` - Toggle token visibility
- `s` - Save capture
- `q` or `Ctrl+C` - Quit/Back
