# Operator Quickstart Guide

Get capturing data in 10 minutes or less.

## What is Astrolabe?

Astrolabe captures test data from serial ports, TCP connections, and files, then uploads it to the Orrery QA backend for analysis. Data is stored locally first (offline-first), so you can capture without network and upload later.

## Before You Start

**System Check:**
- [ ] Astrolabe installed (`astrolabe --help` works)
- [ ] Serial port access (Linux: user in `dialout` group)
- [ ] Backend credentials (API URL, Project ID, Auth Token)

**Don't have these?** See [System Requirements](SYSTEM_REQUIREMENTS.md) and [Installation Guide](../scripts/README.md).

---

## Quick Start: Your First Capture

### Step 1: Launch the TUI

```bash
astrolabe tui
```

You'll see the welcome menu:

```
  Capture
  View Runs
  Upload Data
  Configure Metadata
  Configure Connection
```

### Step 2: Configure Backend Connection (First Time Only)

1. Select **Configure Connection**
2. Fill in:
   - **API URL**: Your Orrery backend URL
   - **Project ID**: Your project identifier
   - **Auth Token**: Your authentication token (press `Ctrl+T` to show/hide)
3. Press `Ctrl+S` to save

### Step 3: Capture Data

**For Serial Port:**
1. Select **Capture**
2. Choose the **Serial** tab
3. Choose your port from the list
4. Set baud rate (default: 115200)
5. Press Enter — live data streams on screen
6. Press `s` to save when done

**For TCP:**
1. Select **Capture**
2. Choose the **TCP** tab
3. Enter host address (e.g., `192.168.1.100`)
4. Enter port (e.g., `8080`)
5. Press Enter — live data streams on screen
6. Press `s` to save when done

**For File Import:**
```bash
astrolabe capture file data.csv --device-id my-device
```

### Step 4: Upload to Backend

1. Select **Upload Data**
2. Watch the progress bar
3. Green checkmarks = success

---

## Verify Success

Check your captured runs:
1. Select **View Runs**
2. Browse runs with arrow keys
3. Press `v` to view details

Check upload status:
- Uploaded runs show upload timestamp
- Pending runs show "Not uploaded"

---

## Common Commands

| Task | Command |
|------|---------|
| Launch TUI | `astrolabe tui` |
| Capture from serial | `astrolabe capture serial --port /dev/ttyUSB0 --baud 115200` |
| Capture from TCP | `astrolabe capture tcp --host 192.168.1.100 --port 8080` |
| Import file | `astrolabe capture file data.csv` |
| Upload all | `astrolabe upload` |
| Check config | `astrolabe config get api_url` |
| Show version | `astrolabe version` |

---

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑`/`↓` or `j`/`k` | Navigate menu |
| `Enter` | Select |
| `Tab` | Next field |
| `Ctrl+S` | Save configuration |
| `Ctrl+T` | Toggle token visibility |
| `s` | Save capture |
| `q` | Back/Quit |

---

## Quick Fixes

**"API URL not configured"**
→ Run `astrolabe tui` → Configure Connection → Set URL → `Ctrl+S`

**Serial port not found**
→ Check connection, then:
- Windows: Check Device Manager for COM port
- Linux: `ls /dev/ttyUSB* /dev/ttyACM*`
- macOS: `ls /dev/cu.*`

**Permission denied (Linux)**
```bash
sudo usermod -a -G dialout $USER
# Log out and back in
```

**Upload failed**
→ Check network connectivity
→ Verify API URL and auth token in Configure Connection

---

## Next Steps

- [Operator Workflows](OPERATOR_WORKFLOWS.md) - Common task patterns
- [Troubleshooting Guide](OPERATOR_TROUBLESHOOTING.md) - Detailed problem solving
- [Metadata Best Practices](METADATA_BEST_PRACTICES.md) - Effective data tagging
- [TUI Workflow Guide](TUI_WORKFLOW.md) - Complete TUI reference

---

**Need Help?** Check [Troubleshooting Guide](OPERATOR_TROUBLESHOOTING.md) or contact your system administrator.
