# Astrolabe Project TODOs

This TODO list is organized by priority based on the Astrolabe MVP completion roadmap.

---

## 🟡 MEDIUM PRIORITY - Deployment & Usability

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

### Performance Testing with High-Throughput Sources
**Status**: Not Started
**Required for**: Production confidence with fast devices

- [x] Test serial capture at high baud rates (921600+)
- [x] Test TCP capture with high-frequency data streams
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
- [ ] Create video walkthrough of TUI usage (asciinema?)

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
