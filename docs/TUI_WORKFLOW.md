# TUI Workflow Guide

## Welcome Screen Navigation

Launch the interactive TUI:

```bash
astrolabe tui
```

## Available Actions

The welcome screen presents these options:

```
┌─────────────────────────────────────┐
│  🎯 Capture Serial                  │
│  󰛶  Capture TCP                     │
│  📡 List Ports                      │
│  📊 View Runs                       │
│  ⬆️  Upload Data                    │
│  📝 Configure Metadata              │
│  🔌 Configure Connection            │
│  📄 New File                        │
└─────────────────────────────────────┘
```

Use `↑` / `↓` or `j` / `k` to navigate, `Enter` to select.

## Workflow Examples

### First-Time Setup

1. **Launch TUI**: `astrolabe tui`
2. **Select "Configure Connection"** (🔌)
   - Set API URL (your backend server)
   - Set Project ID
   - Set Auth Token
   - Press `Ctrl+S` to save
3. **Return to welcome screen**
4. **Select "Configure Metadata"** (📝)
   - Set operator, location, device info
   - Press `Ctrl+S` to save
5. **Ready to capture!**

### Capture and Upload Workflow

1. **Select "Capture Serial"** (🎯) or **"Capture TCP"** (󰛶)
   - For serial: Choose port from list, configure baud rate
   - For TCP: Enter host/IP and port
   - Watch live data stream
   - Press `s` to save when done
2. **Return to welcome screen** (shows success message)
3. **Select "View Runs"** (📊) to verify capture
4. **Select "Upload Data"** (⬆️)
   - Uploads all pending runs to backend server
5. **Done!**

### View Captured Data

1. **Select "View Runs"** (📊)
   - Browse all captured runs
   - See metadata, timestamps, record counts
   - Navigate with arrow keys
   - Press `v` to view run details

## Menu Item Details

### 🎯 Capture Serial
- Interactive serial port selection
- Live data streaming TUI
- Press `s` to save, `q` to quit without saving
- Returns to welcome screen when done

### 󰛶 Capture TCP
- Enter TCP host/IP address
- Select from common ports or enter custom port
- Live data streaming TUI
- Press `s` to save, `q` to quit without saving
- Returns to welcome screen when done

### 📡 List Ports
- Shows all available serial ports
- Displays port paths and descriptions
- Press `q` to return

### 📝 Configure Metadata
- Set operator, location, device info, test details
- Pre-populates future captures
- Navigate with `↑`/`↓`/`Tab`
- `Ctrl+S` to save

### 📊 View Runs
- Browse all captured runs in offline cache
- See run ID, timestamp, records, upload status
- `v` to view details, `q` to return

### ⬆️ Upload Data
- Uploads all pending runs to backend server
- Shows animated progress bar with spinner
- Displays checkmarks (✓) for successful uploads
- Displays X marks (✗) for failures
- Live counter: X/Y runs uploaded
- Requires configuration (API URL, token, project ID)
- Returns to welcome screen when complete

### 🔌 Configure Connection
- Interactive backend configuration
- Set API URL, Project ID, Auth Token
- Configure offline cache location
- Set max retry attempts
- Press `Ctrl+T` to toggle token visibility
- `Ctrl+S` to save

### 📄 New File
- (Future feature)

## Status Messages

After completing an action, you'll see a status banner:

**Success** (green):
```
✓ Configuration Updated
Configuration saved. You can now upload runs.
```

**Info** (blue):
```
ℹ Run Discarded
Capture discarded. Start a new run when you are ready.
```

**Error** (red):
```
✗ Upload Failed
Connection refused: check your API URL and network
```

## Keyboard Shortcuts

### Global (all screens)
- `↑` / `k` - Move up
- `↓` / `j` - Move down
- `Enter` - Select / Confirm
- `q` or `Ctrl+C` - Quit / Back
- `Esc` - Exit (some screens)

### Configuration Screen
- `Tab` / `Shift+Tab` - Navigate fields
- `Ctrl+T` - Toggle token visibility
- `Ctrl+S` - Save

### Metadata Screen
- `Tab` / `Shift+Tab` - Navigate fields
- `Ctrl+S` - Save

### Capture Screen
- `s` - Save and exit
- `q` - Quit without saving

## Tips

1. **Use Configure Connection first**: Set up your backend connection before trying to upload
2. **Set metadata defaults**: Configure metadata once, reuse for all captures
3. **Check View Runs**: Verify captures before uploading
4. **Status messages persist**: Return to welcome screen to see results of previous action
5. **Safe to quit**: Captured data is saved locally, upload when ready

## Troubleshooting

### "API URL not configured" when uploading
→ Select "Configure Connection" and set your backend server URL

### Can't find serial port
→ Select "List Ports" to see available ports

### Lost metadata between captures
→ Use "Configure Metadata" to set defaults that persist

### Upload fails
→ Check "Configure Connection" settings (API URL, token, project ID)
→ Verify network connection to backend server
