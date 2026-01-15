# System Requirements

Pre-installation checklist and platform-specific requirements for Astrolabe.

---

## Supported Operating Systems

### Linux

| Distribution | Versions | Status |
|--------------|----------|--------|
| Ubuntu | 20.04 LTS, 22.04 LTS, 24.04 LTS | Verified |
| Debian | 11 (Bullseye), 12 (Bookworm) | Verified |
| Fedora | 38, 39, 40 | Expected to work |
| RHEL/CentOS | 8, 9 | Expected to work |
| Arch Linux | Rolling | Expected to work |

**Requirements:**
- x86_64 (AMD64) architecture
- glibc 2.31 or later

### Windows

| Version | Status |
|---------|--------|
| Windows 10 (1903+) | Verified |
| Windows 11 | Verified |
| Windows Server 2019+ | Expected to work |

**Requirements:**
- x86_64 (AMD64) architecture
- PowerShell 5.1+ (for installer)

### macOS

| Version | Status |
|---------|--------|
| macOS 10.13 (High Sierra)+ | Not verified |
| macOS 11 (Big Sur)+ | Not verified |
| macOS 12 (Monterey)+ | Not verified |

**Note:** macOS support is implemented but not verified due to lack of test hardware. Please report issues if encountered.

**Requirements:**
- Intel (x86_64) or Apple Silicon (arm64)
- Bash or Zsh (for installer)

---

## Hardware Requirements

### Minimum

| Resource | Requirement |
|----------|-------------|
| CPU | Any modern x86_64 processor |
| RAM | 256 MB available |
| Disk | 100 MB for application + data storage |
| Network | Required for upload (optional for capture) |

### Recommended

| Resource | Recommendation |
|----------|----------------|
| CPU | Multi-core for concurrent captures |
| RAM | 512 MB+ available |
| Disk | 1 GB+ for data caching |
| Network | Stable connection for uploads |

### Storage Considerations

- **Capture data** is stored locally before upload
- **Large captures** may require additional disk space
- **Default location:** `~/.astrolabe/` (user home directory)
- **Typical run size:** 1-100 MB depending on capture duration

---

## Serial Port Requirements

### Linux

**Driver:** Built-in kernel drivers for most USB-serial adapters

**Permissions:** User must be in `dialout` group

```bash
# Check current groups
groups

# Add user to dialout group
sudo usermod -a -G dialout $USER

# Log out and back in for changes to take effect
```

**Common device paths:**
- `/dev/ttyUSB0`, `/dev/ttyUSB1` - USB-serial adapters
- `/dev/ttyACM0`, `/dev/ttyACM1` - USB CDC devices (Arduino, etc.)

### Windows

**Driver:** Install manufacturer-provided drivers

**Common adapters:**
- FTDI: [FTDI Drivers](https://ftdichip.com/drivers/)
- CH340/CH341: [CH340 Drivers](http://www.wch-ic.com/downloads/CH341SER_ZIP.html)
- CP210x: [Silicon Labs Drivers](https://www.silabs.com/developers/usb-to-uart-bridge-vcp-drivers)

**Device paths:** `COM1`, `COM2`, `COM3`, etc. (check Device Manager)

### macOS

**Driver:** Most USB-serial adapters work with built-in drivers

**Device paths:**
- `/dev/cu.usbserial-*` - USB-serial adapters
- `/dev/tty.usbserial-*` - Alternative naming
- `/dev/cu.usbmodem*` - USB CDC devices

**Note:** Use `cu.*` paths for outgoing connections (recommended)

---

## Network Requirements

### Backend Connectivity

| Requirement | Details |
|-------------|---------|
| Protocol | HTTPS (TLS 1.2+) |
| Port | 443 (default HTTPS) |
| DNS | Must resolve backend hostname |

### Firewall Rules

Ensure outbound connections are allowed to:
- Your Orrery backend server (port 443)
- S3-compatible storage for file uploads (if applicable)

### Proxy Configuration

If behind a corporate proxy, set environment variables:

```bash
# Linux/macOS
export HTTP_PROXY="http://proxy.company.com:8080"
export HTTPS_PROXY="http://proxy.company.com:8080"
export NO_PROXY="localhost,127.0.0.1"

# Windows PowerShell
$env:HTTP_PROXY = "http://proxy.company.com:8080"
$env:HTTPS_PROXY = "http://proxy.company.com:8080"
```

### Offline Operation

Astrolabe supports **offline-first** operation:
- Capture data without network connectivity
- Data stored locally in `~/.astrolabe/runs/`
- Upload when connectivity is restored
- Automatic retry with exponential backoff

---

## Dependencies

### Runtime Dependencies

**None** - Astrolabe is distributed as a statically linked binary with no external dependencies.

### Build Dependencies (Development Only)

| Dependency | Version | Purpose |
|------------|---------|---------|
| Go | 1.25+ | Compilation |
| mise | Latest | Task runner (optional) |

---

## Installation Checklist

Use this checklist before installing Astrolabe:

### All Platforms

- [ ] System meets minimum hardware requirements
- [ ] Sufficient disk space available
- [ ] Network access to Orrery backend (or plan for offline use)
- [ ] Backend credentials obtained (API URL, Project ID, Auth Token)

### Linux

- [ ] glibc 2.31+ installed (check with `ldd --version`)
- [ ] User added to `dialout` group (for serial port access)
- [ ] Write access to `~/.local/bin/` or installation directory

### Windows

- [ ] PowerShell 5.1+ available
- [ ] Serial port drivers installed (if using serial capture)
- [ ] Administrator access (for system-wide install, optional)

### macOS

- [ ] Bash or Zsh available
- [ ] Write access to `~/.local/bin/` or installation directory
- [ ] Terminal granted necessary permissions (System Preferences → Security)

---

## Verification

After installation, verify Astrolabe is working:

```bash
# Check installation
astrolabe --help
astrolabe version

# Check serial port access (if applicable)
astrolabe tui
# → Select "List Ports"

# Check configuration
astrolabe config get
```

---

## Known Limitations

### Platform-Specific

| Platform | Limitation |
|----------|------------|
| Linux | Requires dialout group for serial access |
| Windows | Some USB drivers require restart after install |
| macOS | Not verified - please report issues |

### General

| Feature | Limitation |
|---------|------------|
| Serial ports | One capture per port at a time |
| TCP capture | No TLS/SSL support (use SSH tunnel for security) |
| File size | Limited by available memory during processing |
| Concurrent uploads | Sequential by default |

---

## Related Documentation

- [Installation Guide](../scripts/README.md) - Step-by-step installation
- [Operator Quickstart](OPERATOR_QUICKSTART.md) - Getting started
- [Troubleshooting](OPERATOR_TROUBLESHOOTING.md) - Problem resolution
