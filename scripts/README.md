# Astrolabe Installation Scripts

This directory contains installation and uninstallation scripts for the Astrolabe CLI/TUI tool.

## Quick Start

### Installation (Linux/macOS)

```bash
# Default installation (recommended)
./scripts/install.sh

# System-wide installation
sudo ./scripts/install.sh --system-wide

# Preview what will be done
./scripts/install.sh --dry-run
```

### Installation (Windows)

```powershell
# Default installation (recommended)
.\scripts\install.ps1

# System-wide installation (requires Administrator)
.\scripts\install.ps1 -SystemWide

# Preview what will be done
.\scripts\install.ps1 -DryRun
```

### Uninstallation (Linux/macOS)

```bash
# Interactive uninstallation (asks about each item)
./scripts/uninstall.sh

# Complete removal (including cache and configs)
./scripts/uninstall.sh --complete

# Keep cached run data
./scripts/uninstall.sh --keep-cache
```

### Uninstallation (Windows)

```powershell
# Interactive uninstallation (asks about each item)
.\scripts\uninstall.ps1

# Complete removal (including cache and configs)
.\scripts\uninstall.ps1 -Complete

# Keep cached run data
.\scripts\uninstall.ps1 -KeepCache
```

## Scripts

### `install.sh` - Linux/macOS Installation

Installs the Astrolabe binary and sets up the environment.

**Features:**
- Detects platform (Linux/macOS) and architecture
- Installs binary to `~/.local/bin` (user-local) or `/usr/local/bin` (system-wide)
- Checks and configures PATH
- Installs shell completion (bash/zsh/fish)
- Creates configuration directory (`~/.astrolabe/`)
- Checks serial port permissions (Linux)
- Validates installation

**Options:**
- `--system-wide` - Install to `/usr/local/bin` (requires sudo)
- `--no-completion` - Skip shell completion installation
- `--dry-run` - Preview installation without making changes
- `--help` - Show help message

**Examples:**
```bash
# User-local installation (no sudo required)
./scripts/install.sh

# System-wide installation
sudo ./scripts/install.sh --system-wide

# Skip shell completion
./scripts/install.sh --no-completion

# Preview installation
./scripts/install.sh --dry-run
```

**Installation Locations:**
- Binary: `~/.local/bin/astrolabe` (default) or `/usr/local/bin/astrolabe`
- Config: `~/.astrolabe/`
- Completion:
  - Bash: `~/.local/share/bash-completion/completions/astrolabe`
  - Zsh: `~/.zsh/completion/_astrolabe`
  - Fish: `~/.config/fish/completions/astrolabe.fish`

### `uninstall.sh` - Linux/macOS Uninstallation

Removes Astrolabe and optionally cleans up configuration and cache.

**Features:**
- Removes binary from installation location
- Removes shell completion files
- Cleans up PATH modifications
- Interactive cleanup of cache and configs
- Safe defaults with confirmation prompts

**Options:**
- `--complete` - Remove everything (binary, configs, cache)
- `--keep-cache` - Remove binary and configs, keep cache
- `--dry-run` - Preview uninstallation without making changes
- `--help` - Show help message

**Examples:**
```bash
# Interactive uninstallation (asks what to keep)
./scripts/uninstall.sh

# Complete removal (WARNING: deletes all data)
./scripts/uninstall.sh --complete

# Keep cached run data
./scripts/uninstall.sh --keep-cache

# Preview uninstallation
./scripts/uninstall.sh --dry-run
```

**Cleanup Options:**
1. **Keep everything** - Only removes binary and completion
2. **Remove configs only** - Keeps cached run data in `~/.astrolabe/runs/`
3. **Complete removal** - Deletes everything (requires confirmation)

### `install.ps1` - Windows Installation

Installs the Astrolabe binary and sets up the Windows environment.

**Features:**
- Checks PowerShell version (5.1+ required)
- Detects Windows version and architecture
- Validates Administrator rights (for system-wide install)
- Installs binary to `%LOCALAPPDATA%\Programs\Astrolabe\` (user) or `C:\Program Files\Astrolabe\` (system)
- Updates PATH persistently via registry
- Installs PowerShell completion
- Creates configuration directory (`%USERPROFILE%\.astrolabe\`)
- Displays available COM ports
- Validates installation

**Parameters:**
- `-SystemWide` - Install to Program Files (requires Administrator)
- `-NoCompletion` - Skip PowerShell completion installation
- `-DryRun` - Preview installation without making changes
- `-Help` - Show help message

**Examples:**
```powershell
# User-local installation (no admin required)
.\scripts\install.ps1

# System-wide installation
.\scripts\install.ps1 -SystemWide

# Skip PowerShell completion
.\scripts\install.ps1 -NoCompletion

# Preview installation
.\scripts\install.ps1 -DryRun

# Get detailed help
Get-Help .\scripts\install.ps1 -Detailed
```

**Installation Locations:**
- Binary (user): `%LOCALAPPDATA%\Programs\Astrolabe\astrolabe.exe` (~\AppData\Local\Programs\Astrolabe\)
- Binary (system): `C:\Program Files\Astrolabe\astrolabe.exe`
- Config: `%USERPROFILE%\.astrolabe\` (~\.astrolabe\)
- Completion: `%USERPROFILE%\.astrolabe\astrolabe-completion.ps1`

### `uninstall.ps1` - Windows Uninstallation

Removes Astrolabe and optionally cleans up configuration and cache.

**Features:**
- Locates binary (user or system installation)
- Checks Administrator rights (if needed)
- Removes binary and installation directory
- Removes from PATH (registry modification)
- Removes PowerShell completion from profile
- Interactive cache/config cleanup
- Display summary of removed items

**Parameters:**
- `-Complete` - Remove everything including cache
- `-KeepCache` - Remove binary and configs, preserve cache
- `-DryRun` - Preview uninstallation without making changes
- `-Help` - Show help message

**Examples:**
```powershell
# Interactive uninstallation (asks what to keep)
.\scripts\uninstall.ps1

# Complete removal (WARNING: deletes all data)
.\scripts\uninstall.ps1 -Complete

# Keep cached run data
.\scripts\uninstall.ps1 -KeepCache

# Preview uninstallation
.\scripts\uninstall.ps1 -DryRun

# Get detailed help
Get-Help .\scripts\uninstall.ps1 -Detailed
```

**Cleanup Options:**
1. **Keep everything** - Only removes binary and completion
2. **Remove configs only** - Keeps cached run data in `%USERPROFILE%\.astrolabe\runs\`
3. **Complete removal** - Deletes everything (requires confirmation)

## Configuration Templates

Sample configuration files are provided in the `config/` directory:

### `config/connection.yml.example`

Template for backend API connection settings. Copy to `~/.astrolabe/connection.yml` and customize.

**Key settings:**
- `api_url` - Backend API endpoint
- `project_id` - Project identifier
- `auth_token` - Authentication token (keep secure!)
- `offline_cache` - Local cache directory
- `upload` - Upload retry and batch settings

### `config/metadata.json.example`

Template for default metadata stamped on captures. Copy to `~/.astrolabe/metadata.json` and customize.

**Key settings:**
- `operator` - Person or system performing capture
- `location` - Physical location of test bench
- `device` - Device under test information
- `test` - Test execution information
- `tags` - Freeform tags for categorization
- `attributes` - Custom key-value metadata

## Platform Support

### Linux
- **Supported**: Ubuntu, Debian, Fedora, RHEL, Arch, and other distributions
- **Architecture**: x86_64, aarch64
- **Shells**: bash, zsh, fish
- **Note**: Serial port access requires `dialout` group membership

### macOS
- **Supported**: macOS 10.15+ (Catalina and later)
- **Architecture**: x86_64 (Intel), arm64 (Apple Silicon)
- **Shells**: bash, zsh, fish
- **Note**: Serial port drivers may need to be installed separately

### Windows
- **Supported**: Windows 10, Windows 11, Windows Server 2019+
- **Architecture**: x64 (AMD64)
- **PowerShell**: 5.1 or later (PowerShell 7+ also supported)
- **Shells**: PowerShell (completion supported)
- **Serial Ports**: COM1, COM2, etc. (standard Windows COM ports)
- **Notes**:
  - User-local installation requires no admin rights
  - System-wide installation requires Administrator privileges
  - PATH changes are persistent via registry
  - USB-to-Serial drivers (FTDI, CH340, etc.) must be installed separately
  - **WSL**: Use Linux scripts in Windows Subsystem for Linux (WSL mode)

## Requirements

### Prerequisites (All Platforms)
1. **Binary must be built first**
   ```bash
   # Linux/macOS/WSL
   mise run build
   # OR
   go build ./cmd/astrolabe

   # Windows (PowerShell)
   go build -o astrolabe.exe ./cmd/astrolabe
   ```

### Linux/macOS Specific
2. **Scripts must be executable**
   ```bash
   chmod +x scripts/install.sh scripts/uninstall.sh
   ```

3. **For system-wide installation**
   - Requires sudo/root access
   - Must have write permissions to `/usr/local/bin`

4. **For serial port access (Linux)**
   - User must be in `dialout` group
   - Command: `sudo usermod -a -G dialout $USER`
   - Requires logout/login to take effect

### Windows Specific
2. **PowerShell Execution Policy**
   - May need to allow script execution:
     ```powershell
     # For current session only (safest)
     Set-ExecutionPolicy Bypass -Scope Process

     # For current user (persistent)
     Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
     ```

3. **For system-wide installation**
   - Requires Administrator privileges
   - Right-click PowerShell → "Run as Administrator"
   - Must have write permissions to `C:\Program Files`

4. **For serial port access**
   - Install appropriate USB-to-Serial drivers (FTDI, CH340, etc.)
   - COM ports appear as `COM1`, `COM2`, etc.
   - Check Device Manager for available COM ports

### Optional (All Platforms)
- Shell completion requires bash 4.0+, zsh 5.0+, fish 3.0+, or PowerShell 5.1+
- Colored output requires terminal with ANSI support (Windows Terminal recommended on Windows)

## Troubleshooting

### Binary not found (All Platforms)
```bash
# Linux/macOS
ls -la ./astrolabe
mise run build

# Windows
dir astrolabe.exe
go build -o astrolabe.exe ./cmd/astrolabe
```

### Permission denied (Linux/macOS)
```bash
# For user-local installation (recommended)
./scripts/install.sh

# For system-wide installation
sudo ./scripts/install.sh --system-wide
```

### PowerShell Execution Policy Error (Windows)
```
File cannot be loaded because running scripts is disabled on this system.
```

**Solution:**
```powershell
# Option 1: Bypass for current session only (safest)
Set-ExecutionPolicy Bypass -Scope Process
.\scripts\install.ps1

# Option 2: Allow signed scripts for current user
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser

# Check current policy
Get-ExecutionPolicy -List
```

### Administrator Rights Required (Windows)
```
Access to the path 'C:\Program Files' is denied.
```

**Solution:**
```powershell
# Option 1: Use user-local installation (no admin needed)
.\scripts\install.ps1

# Option 2: Run PowerShell as Administrator
# Right-click PowerShell → "Run as Administrator"
.\scripts\install.ps1 -SystemWide
```

### Binary not in PATH (Linux/macOS)
```bash
# Check PATH
echo $PATH | grep .local/bin

# Add to PATH manually (bash)
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Add to PATH manually (zsh)
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

### Binary not in PATH (Windows)
```powershell
# Check PATH
$env:Path -split ';' | Select-String "Astrolabe"

# Add to User PATH manually
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
[Environment]::SetEnvironmentVariable("Path", "$userPath;$env:LOCALAPPDATA\Programs\Astrolabe", "User")

# Restart PowerShell to pick up changes
```

### Shell completion not working (Linux/macOS)
```bash
# Bash: Source completion file
source ~/.local/share/bash-completion/completions/astrolabe

# Zsh: Reload completion
rm -f ~/.zcompdump && compinit

# Fish: Restart fish shell
exec fish
```

### PowerShell completion not working (Windows)
```powershell
# Check if profile exists
Test-Path $PROFILE

# Check if completion is in profile
Get-Content $PROFILE | Select-String "astrolabe"

# Manually source completion
. "$env:USERPROFILE\.astrolabe\astrolabe-completion.ps1"

# Restart PowerShell
```

### Serial port access denied (Linux)
```bash
# Check current groups
groups

# Add to dialout group
sudo usermod -a -G dialout $USER

# IMPORTANT: Log out and log back in
# Verify group membership
groups | grep dialout
```

### COM port not found (Windows)
```powershell
# List all COM ports
Get-WmiObject Win32_PnPEntity | Where-Object { $_.Caption -match "COM\d+" } | Select-Object Caption

# Check Device Manager
devmgmt.msc

# Install drivers if needed (FTDI, CH340, etc.)
```

### Windows Terminal Color Issues
If colors don't display correctly in PowerShell:
```powershell
# Use Windows Terminal (recommended)
# Download from Microsoft Store

# Or disable colors
$env:NO_COLOR = "1"
.\scripts\install.ps1
```

## Development

### Testing Installation
```bash
# Always test with dry-run first
./scripts/install.sh --dry-run

# Test actual installation in safe location
# (uses ~/.local/bin by default)
./scripts/install.sh

# Verify installation
astrolabe version
```

### Testing Uninstallation
```bash
# Test with dry-run
./scripts/uninstall.sh --dry-run

# Test actual uninstallation
./scripts/uninstall.sh
```

### Testing on Fresh Systems
```bash
# Docker for Linux testing
docker run -it --rm -v $(pwd):/app ubuntu:22.04 bash
cd /app && ./scripts/install.sh

# Virtual machine for full system testing
# Test on: Ubuntu, Fedora, macOS (Intel), macOS (Apple Silicon)
```

## Contributing

When making changes to installation scripts:

1. **Test with `--dry-run` first**
2. **Test on multiple platforms** (Linux, macOS)
3. **Test with different shells** (bash, zsh, fish)
4. **Test edge cases** (existing installation, missing permissions, etc.)
5. **Update this README** with any new features or options
6. **Update TODO.md** with completion status

## License

See [LICENSE](../LICENSE) for details.
