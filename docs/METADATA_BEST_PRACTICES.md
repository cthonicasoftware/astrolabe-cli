# Metadata Best Practices

Effective metadata makes your QA data searchable, traceable, and valuable. This guide explains how to structure metadata for maximum benefit.

---

## Why Metadata Matters

Good metadata enables:
- **Traceability**: Link test results to specific devices, firmware, and test plans
- **Searchability**: Find specific runs in the backend
- **Analysis**: Compare results across firmware versions, test stations, or time periods
- **Compliance**: Maintain audit trails for quality standards

Poor metadata leads to:
- Orphaned data that cannot be linked to products
- Difficulty reproducing or investigating issues
- Lost context when reviewing historical data

---

## Core Metadata Fields

### Operator

**Purpose:** Track who performed the capture for accountability and follow-up.

```json
"operator": "jane.smith"
```

**Best Practices:**
- Use consistent identifiers across your organization (username, email, or display name)
- For automated captures, use system/service identifiers (e.g., `ci-runner-01`, `jenkins`)

### Location

**Purpose:** Identify where the test was performed for troubleshooting and equipment tracking.

```json
"location": "HQ / Lab 101 / Bench A3"
```

**Best Practices:**
- Use a consistent format across your organization
- Include enough specificity to uniquely identify the test station
- Useful for correlating environmental factors with test results

### Device Information

**Purpose:** Identify the device under test for traceability and analysis.

```json
"device": {
  "id": "DUT-2024-001",
  "serial": "SN12345678",
  "hardware_version": "v2.1",
  "firmware": "1.4.2",
  "firmware_hash": "abc123def456"
}
```

**Fields:**
- **`id`**: Use a consistent identifier format (e.g., `DUT-YYYY-NNN`)
- **`serial`**: Physical serial number from the device label
- **`hardware_version`**: PCB or hardware revision
- **`firmware`**: Use semantic versioning (e.g., `1.4.2`)
- **`firmware_hash`**: Short git hash or build hash for exact traceability

**Best Practices:**
- Keep hardware and firmware versions separate for independent tracking
- Always include `firmware_hash` for builds — version strings alone aren't unique enough

### Test Information

**Purpose:** Link captures to test plans and execution context.

```json
"test": {
  "plan": "functional-validation",
  "variant": "extended",
  "run": "42"
}
```

**Fields:**
- **`plan`**: Use kebab-case names (e.g., `power-cycle-stress`, `calibration-full`)
- **`variant`**: Distinguish test configurations (e.g., `quick`, `standard`, `extended`)
- **`run`**: Sequential run number or identifier within a plan

### Tags

**Purpose:** Flexible categorization for filtering and grouping.

```json
"tags": ["calibration", "production-line", "batch-2024-Q1"]
```

**Best Practices:**
- Use lowercase with hyphens
- Create a taxonomy for your organization
- Common tag categories:
  - **Test type**: `calibration`, `functional`, `stress`, `regression`
  - **Stage**: `prototype`, `pilot`, `production`
  - **Priority**: `critical`, `standard`, `debug`
  - **Batch**: `batch-2024-Q1`, `lot-12345`

### Custom Attributes

**Purpose:** Capture domain-specific context not covered by standard fields.

```json
"attributes": {
  "temperature_c": "25",
  "humidity_percent": "45",
  "supply_voltage": "3.3",
  "test_fixture": "JIG-A",
  "operator_shift": "day"
}
```

**Best Practices:**
- Use snake_case keys
- Include units in key names when relevant (`temperature_c`, `voltage_mv`)
- Keep values as strings for consistency
- Document your attribute schema for team consistency

---

## Example Configurations

### Production Line Testing

```json
{
  "operator": "station-3-auto",
  "location": "Factory-SZ / Production / Line 3 / Station 3",
  "device": {
    "hardware_version": "v2.1",
    "firmware": "1.4.2-release"
  },
  "test": {
    "plan": "production-functional",
    "variant": "standard"
  },
  "tags": ["production", "functional", "automated"],
  "attributes": {
    "line_number": "3",
    "shift": "day",
    "batch_id": "B2024-0142"
  }
}
```

### R&D Prototyping

```json
{
  "operator": "alex.chen",
  "location": "HQ / R&D / Lab 201 / Proto Bench 1",
  "device": {
    "id": "PROTO-007",
    "hardware_version": "v3.0-proto",
    "firmware": "2.0.0-dev.45",
    "firmware_hash": "abc123d"
  },
  "test": {
    "plan": "power-consumption",
    "variant": "sleep-modes"
  },
  "tags": ["prototype", "power", "investigation"],
  "attributes": {
    "experiment_id": "PWR-2024-015",
    "supply_voltage": "3.3",
    "temperature_c": "25",
    "notes": "Testing new sleep mode implementation"
  }
}
```

### Regression Testing (CI/CD)

```json
{
  "operator": "jenkins-main",
  "location": "cloud / ci-runner-pool",
  "device": {
    "id": "HIL-SIM-01",
    "firmware": "${GIT_TAG}",
    "firmware_hash": "${GIT_COMMIT}"
  },
  "test": {
    "plan": "regression-suite",
    "run": "${BUILD_NUMBER}"
  },
  "tags": ["regression", "automated", "ci"],
  "attributes": {
    "build_url": "${BUILD_URL}",
    "branch": "${GIT_BRANCH}",
    "pr_number": "${PR_NUMBER}"
  }
}
```

---

## Common Mistakes to Avoid

### 1. Inconsistent Naming

**Bad:**
```
firmware: "1.4.2"
firmware: "v1.4.2"
firmware: "Version 1.4.2"
```

**Good:** Pick one format and use it everywhere:
```
firmware: "1.4.2"
```

### 2. Missing Device Identity

**Bad:** No way to trace back to physical device
```json
"device": {}
```

**Good:** Include unique identifiers
```json
"device": {
  "id": "DUT-2024-042",
  "serial": "SN12345678"
}
```

### 3. Overloading Tags

**Bad:** Using tags for structured data
```json
"tags": ["temp-25", "humidity-45", "voltage-3.3"]
```

**Good:** Use attributes for key-value data
```json
"attributes": {
  "temperature_c": "25",
  "humidity_percent": "45",
  "supply_voltage": "3.3"
}
```

### 4. Vague Test Plans

**Bad:**
```json
"test": {
  "plan": "test1"
}
```

**Good:**
```json
"test": {
  "plan": "thermal-stress-cycle",
  "variant": "extended"
}
```

### 5. Not Versioning Firmware

**Bad:** Cannot distinguish between builds
```json
"device": {
  "firmware": "latest"
}
```

**Good:** Include exact version and hash
```json
"device": {
  "firmware": "1.4.2",
  "firmware_hash": "abc123d"
}
```

---

## Setting Defaults

Configure defaults in the TUI to avoid repetitive entry:

1. Launch: `astrolabe tui`
2. Select **Configure Metadata**
3. Set common values that rarely change:
   - Operator identifier
   - Location
   - Device hardware version
4. Press `Ctrl+S` to save

Override per-capture via CLI flags when values differ.

---

## Related Documentation

- [Operator Quickstart](OPERATOR_QUICKSTART.md) - Getting started
- [Configuration Guide](CONFIG_TUI.md) - Setting up metadata in TUI
- [Directory Structure](DIRECTORY_STRUCTURE.md) - Where metadata is stored
