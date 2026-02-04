# Astrolabe Project TODOs

This TODO list is organized by priority based on the Astrolabe MVP completion roadmap.

---

## 🔴 HIGH PRIORITY - Production Readiness Blockers

### Verify Upload and Validation to API endpoint
**Status**: Complete ✅
**Blocks**: Production deployment, end-to-end validation

- [x] Generate random capture files for testing
- [x] Test upload flow with real Django backend
- [x] Verify presigned URL upload mechanism
- [x] Test retry logic with simulated failures
- [x] Validate checksum verification on backend
- [x] Test offline cache and reconnection flow

### End-to-End Integration Tests with Real Backend
**Status**: Complete ✅
**Blocks**: Production confidence

- [x] Set up test backend instance
- [x] Test full capture → upload → retrieval flow
- [x] Test error scenarios (network failures, auth errors, corrupted data)
- [x] Verify manifest and artifact integrity end-to-end
- [x] Test batch upload functionality

---

## 🟡 MEDIUM PRIORITY - Deployment & Usability

### Create Install & Uninstall Script
**Status**: Complete ✅ (Linux/macOS/Windows)
**Required for**: Field deployment

- [x] Create installation script that adds astrolabe to PATH
- [x] During installation, offer to add shell completion for user's shell
- [x] Support Linux, macOS installation paths
- [x] Create uninstall script with cleanup of config/cache (optional)
- [x] Test installation on fresh systems (dry-run validated)
- [x] Create Windows PowerShell install script (install.ps1)
- [x] Create Windows PowerShell uninstall script (uninstall.ps1)
- [x] Test on Windows system (dry-run validated - Windows 11)

**Files created:**
- `scripts/install.sh` - Bash installer for Linux/macOS
- `scripts/uninstall.sh` - Bash uninstaller for Linux/macOS
- `scripts/install.ps1` - PowerShell installer for Windows
- `scripts/uninstall.ps1` - PowerShell uninstaller for Windows
- `config/connection.yml.example` - Sample connection config
- `config/metadata.json.example` - Sample metadata config
- `scripts/README.md` - Comprehensive installation documentation

**Features:**
- User-local and system-wide installation options
- PATH configuration (persistent via registry on Windows)
- Shell completion for bash/zsh/fish/PowerShell
- Safe uninstallation with data protection
- Interactive cache/config cleanup
- Dry-run mode for all scripts
- Cross-platform compatibility
- ASCII-based icons for universal terminal compatibility

**Test Results:**
- ✅ Linux - Full installation verified on desktop
- ✅ Windows 11 - Full installation verified on laptop
- ⚠️ macOS - Not verified (no hardware available for testing)
- ✅ Binary detection and validation working
- ✅ Serial port detection working (COM8, COM9 detected on Windows)
- ✅ Configuration directory detection working

**Production Ready:** Scripts verified for Linux and Windows deployment. macOS untested.

### Binary Packaging for Distribution
**Status**: Complete ✅
**Required for**: Easy deployment to test benches

- [x] Linux: Static binary (CGO_ENABLED=0) - AMD64 and ARM64
- [x] macOS: Binaries for Intel (AMD64) and Apple Silicon (ARM64)
- [x] Windows: AMD64 executable
- [x] Create `.tar.gz` archives (Linux, macOS)
- [x] Create `.zip` archives (Windows)
- [x] Version embedding via ldflags
- [x] SHA256 checksums generation
- [ ] Optional: Windows code signing
- [ ] Optional: `.deb` and `.rpm` packages (Linux)
- [ ] Optional: Homebrew formula (macOS)

**Build Tasks (mise.toml):**
- `mise run build` - Build for current platform
- `mise run build:all` - Build all platform binaries
- `mise run release` - Build all + create archives
- `mise run release:checksums` - Generate SHA256 checksums

**GitHub Actions:**
- `.github/workflows/ci.yml` - CI on push/PR (test + build)
- `.github/workflows/release.yml` - Release on version tags (v*)

**To create a release:**
```bash
git tag v1.0.0
git push origin v1.0.0
# GitHub Actions automatically builds and creates release
```

### Performance Testing with High-Throughput Sources
**Status**: Not Started
**Required for**: Production confidence with fast devices

- [x] Test serial capture at high baud rates (921600+)
- [ ] Test TCP capture with high-frequency data streams
- [ ] Profile memory usage during long-running captures
- [ ] Test large file ingestion (multi-GB CSV/JSONL files)
- [ ] Identify and fix performance bottlenecks

---

## 🟢 MEDIUM PRIORITY - Documentation & Training

### Operator Training Materials
**Status**: Complete ✅
**Required for**: Field deployment success

- [x] Create operator quickstart guide (`docs/OPERATOR_QUICKSTART.md`)
- [x] Document common workflows (`docs/OPERATOR_WORKFLOWS.md`)
- [x] Create troubleshooting guide (`docs/OPERATOR_TROUBLESHOOTING.md`)
- [x] Document metadata configuration best practices (`docs/METADATA_BEST_PRACTICES.md`)
- [ ] Create video walkthrough of TUI usage

**Files created:**
- `docs/OPERATOR_QUICKSTART.md` - 10-minute getting started guide
- `docs/OPERATOR_WORKFLOWS.md` - Common task patterns (serial, TCP, file, batch, CI/CD)
- `docs/OPERATOR_TROUBLESHOOTING.md` - Self-service problem resolution guide
- `docs/METADATA_BEST_PRACTICES.md` - Effective metadata configuration guide
- `docs/SYSTEM_REQUIREMENTS.md` - Pre-installation checklist

### Installation Documentation
**Status**: Complete ✅
**Required for**: Field deployment

- [x] Document system requirements (`docs/SYSTEM_REQUIREMENTS.md`)
- [x] Create step-by-step installation guide (`scripts/README.md`)
- [x] Document configuration file setup (covered in quickstart and troubleshooting)
- [x] Explain serial port permissions (covered in system requirements and troubleshooting)
- [x] Document API authentication setup (covered in quickstart and troubleshooting)

### Production Deployment Guide
**Status**: Not Started
**Required for**: Operations team handoff

- [ ] Document backend API deployment requirements
- [ ] Create network architecture diagram
- [ ] Document firewall/proxy configuration
- [ ] Create monitoring and alerting setup guide
- [ ] Document backup and recovery procedures

---

## 🔵 LOW PRIORITY - Enhancements & Polish

### Improve Menu Navigation
**Status**: Not Started
**Impact**: UX polish

- [x] Add scroll wrapping in TUI menus
- [x] Test with keyboard navigation
- [x] Ensure consistent behavior across all TUI screens

### Create Temperature Calibration Demo
**Status**: Complete ✅
**Purpose**: MVP demonstration with real hardware
**Goal**: Demo Astrolabe MVP once backend API is online
**Location**: `../zephyr/cthonica/`

- [x] Set up MCP9808 with Zephyr Driver
- [x] Hardware configured and ready
- [ ] Create capture workflow for temperature data
- [ ] Document setup and execution steps
- [ ] Prepare demo script for stakeholders

---

## 🟣 FUTURE / DEFERRED

### Add SCPI & VISA Capture Sources
**Status**: Not Started
**Priority**: Low (future enhancement)

- [ ] Investigate Go libraries (e.g., go-visa)
- [ ] Define SCPI source interface
- [ ] Implement SCPI/VISA source in `internal/sources/`
- [ ] Create normalizer for SCPI command/response format
- [ ] Add TUI tab for SCPI device configuration
- [ ] Add tests with mock SCPI instruments
- [ ] Document SCPI instrument configuration

### Example Workflows Documentation
**Status**: Complete ✅
**Purpose**: Help operators understand common patterns

- [x] Document batch file ingestion workflow (`docs/OPERATOR_WORKFLOWS.md`)
- [x] Document continuous monitoring setup (`docs/OPERATOR_WORKFLOWS.md`)
- [x] Document multi-device capture scenarios (`docs/OPERATOR_WORKFLOWS.md`)
- [x] Create example scripts for common tasks (`docs/OPERATOR_WORKFLOWS.md`)

---

## Completion Status

**MVP Core**: ✅ Complete (Serial, File, TCP sources; Upload system; TUI)
**Backend Integration**: ✅ Complete (E2E tests passed with Orrery backend)
**Installation Scripts**: ✅ Verified (Linux, Windows) ⚠️ Unverified (macOS)
**Binary Packaging**: ✅ Complete (mise tasks + GitHub Actions release workflow)
**Operator Documentation**: ✅ Complete (Quickstart, Workflows, Troubleshooting, Metadata, System Requirements)
**Temperature Demo**: ✅ Hardware set up in `../zephyr/cthonica/`
**Production Ready**: ✅ Core functionality ready
**Field Deployment**: ✅ Ready (pending production deployment guide)
