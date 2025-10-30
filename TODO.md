# Astrolabe Project TODOs

This TODO list is organized by priority based on the Astrolabe MVP completion roadmap.

---

## 🔴 HIGH PRIORITY - Production Readiness Blockers

### Verify Upload and Validation to API endpoint
**Status**: In Progress (Backend API work started in Orrery)
**Blocks**: Production deployment, end-to-end validation

- [ ] Generate random capture files for testing
- [ ] Test upload flow with real Django backend
- [ ] Verify presigned URL upload mechanism
- [ ] Test retry logic with simulated failures
- [ ] Validate checksum verification on backend
- [ ] Test offline cache and reconnection flow

### End-to-End Integration Tests with Real Backend
**Status**: Pending (requires backend API completion)
**Blocks**: Production confidence

- [ ] Set up test backend instance
- [ ] Test full capture → upload → retrieval flow
- [ ] Test error scenarios (network failures, auth errors, corrupted data)
- [ ] Verify manifest and artifact integrity end-to-end
- [ ] Test batch upload functionality

---

## 🟡 MEDIUM PRIORITY - Deployment & Usability

### Create Install & Uninstall Script
**Status**: Unix Complete (Linux/macOS) ✅
**Required for**: Field deployment

- [x] Create installation script that adds astrolabe to PATH
- [x] During installation, offer to add shell completion for user's shell
- [x] Support Linux, macOS installation paths
- [x] Create uninstall script with cleanup of config/cache (optional)
- [x] Test installation on fresh systems (dry-run validated)
- [ ] Create Windows PowerShell install script (install.ps1)
- [ ] Create Windows PowerShell uninstall script (uninstall.ps1)
- [ ] Test on fresh Windows system

**Files created:**
- `scripts/install.sh` - Bash installer for Linux/macOS
- `scripts/uninstall.sh` - Bash uninstaller for Linux/macOS
- `config/connection.yml.example` - Sample connection config
- `config/metadata.json.example` - Sample metadata config

### Binary Packaging for Distribution
**Status**: Not Started
**Required for**: Easy deployment to test benches

- [ ] Linux: Static binary (CGO_ENABLED=0)
- [ ] macOS: Universal binary (amd64 + arm64)
- [ ] Windows: Signed executable
- [ ] Create `.tar.gz` archives (Linux, macOS)
- [ ] Create `.zip` archives (Windows)
- [ ] Optional: `.deb` and `.rpm` packages (Linux)
- [ ] Optional: Homebrew formula (macOS)

### Performance Testing with High-Throughput Sources
**Status**: Not Started
**Required for**: Production confidence with fast devices

- [ ] Test serial capture at high baud rates (921600+)
- [ ] Test TCP capture with high-frequency data streams
- [ ] Profile memory usage during long-running captures
- [ ] Test large file ingestion (multi-GB CSV/JSONL files)
- [ ] Identify and fix performance bottlenecks

---

## 🟢 MEDIUM PRIORITY - Documentation & Training

### Operator Training Materials
**Status**: Not Started
**Required for**: Field deployment success

- [ ] Create operator quickstart guide
- [ ] Document common workflows (serial capture, file ingestion, uploads)
- [ ] Create troubleshooting guide (serial port permissions, network issues)
- [ ] Document metadata configuration best practices
- [ ] Create video walkthrough of TUI usage

### Installation Documentation
**Status**: Not Started
**Required for**: Field deployment

- [ ] Document system requirements (OS, permissions, dependencies)
- [ ] Create step-by-step installation guide
- [ ] Document configuration file setup
- [ ] Explain serial port permissions (Linux/macOS)
- [ ] Document API authentication setup

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

- [ ] Add scroll wrapping in TUI menus
- [ ] Test with keyboard navigation
- [ ] Ensure consistent behavior across all TUI screens

### Create Temperature Calibration Demo
**Status**: Not Started
**Purpose**: MVP demonstration with real hardware
**Goal**: Demo Astrolabe MVP once backend API is online

- [ ] Set up MCP9808 with [Zephyr Driver](https://docs.zephyrproject.org/latest/boards/shields/adafruit_mcp9808/doc/index.html)
- [ ] Use [NRF9160DK](https://docs.zephyrproject.org/latest/boards/nordic/nrf9160dk/doc/index.html) or [Adafruit Board](https://docs.zephyrproject.org/latest/boards/adafruit/index.html)
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
**Status**: Not Started
**Purpose**: Help operators understand common patterns

- [ ] Document batch file ingestion workflow
- [ ] Document continuous monitoring setup
- [ ] Document multi-device capture scenarios
- [ ] Create example scripts for common tasks

---

## Completion Status

**MVP Core**: ✅ Complete (Serial, File, TCP sources; Upload system; TUI)
**Backend Integration**: 🚧 In Progress (Orrery Django backend)
**Production Ready**: ❌ Pending (requires HIGH priority items)
**Field Deployment**: ❌ Pending (requires MEDIUM priority items)
