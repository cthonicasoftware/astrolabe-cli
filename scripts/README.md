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

### Uninstallation (Linux/macOS)

```bash
# Interactive uninstallation (asks about each item)
./scripts/uninstall.sh

# Complete removal (including cache and configs)
./scripts/uninstall.sh --complete

# Keep cached run data
./scripts/uninstall.sh --keep-cache
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
- **Status**: PowerShell scripts not yet implemented
- **Planned**: `install.ps1` and `uninstall.ps1` for Windows support
- **WSL**: Use Linux scripts in Windows Subsystem for Linux

## Requirements

### Prerequisites
1. **Binary must be built first**
   ```bash
   # From project root
   mise run build
   # OR
   go build ./cmd/astrolabe
   ```

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

### Optional
- Shell completion requires bash 4.0+, zsh 5.0+, or fish 3.0+
- Colored output requires terminal with ANSI support

## Troubleshooting

### Binary not found
```bash
# Check if binary exists
ls -la ./astrolabe

# If not, build it
mise run build
```

### Permission denied
```bash
# For user-local installation (recommended)
./scripts/install.sh

# For system-wide installation
sudo ./scripts/install.sh --system-wide
```

### Binary not in PATH
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

### Shell completion not working
```bash
# Bash: Source completion file
source ~/.local/share/bash-completion/completions/astrolabe

# Zsh: Reload completion
rm -f ~/.zcompdump && compinit

# Fish: Restart fish shell
exec fish
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
