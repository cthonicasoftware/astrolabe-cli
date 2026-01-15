# Metadata Best Practices

Effective metadata makes your QA data searchable, traceable, and valuable. This guide explains how to structure metadata for maximum benefit.

---

## Why Metadata Matters

Good metadata enables:
- **Traceability**: Link test results to specific devices, firmware, and test plans
- **Searchability**: Find specific runs in the Orrery backend
- **Analysis**: Compare results across firmware versions, test stations, or time periods
- **Compliance**: Maintain audit trails for quality standards

Poor metadata leads to:
- Orphaned data that cannot be linked to products
- Difficulty reproducing or investigating issues
- Lost context when reviewing historical data

---

## Core Metadata Fields

### Operator Information

**Purpose:** Track who performed the capture for accountability and follow-up.

```json
"operator": {
  "name": "Jane Smith",
  "email": "jane.smith@company.com",
  "id": "jsmith"
}
```

**Best Practices:**
- Use consistent IDs across your organization
- For automated captures, use system/service identifiers (e.g., `ci-runner-01`)
- Email enables notification workflows

### Location Information

**Purpose:** Identify where the test was performed for troubleshooting and equipment tracking.

```json
"location": {
  "site": "HQ",
  "building": "Building A",
  "room": "Lab 101",
  "bench_id": "BENCH-A3"
}
```

**Best Practices:**
- Use consistent naming conventions across sites
- `bench_id` should uniquely identify the test station
- Useful for correlating environmental factors with test results

### Device Information

**Purpose:** Identify the device under test for traceability and analysis.

```json
"device": {
  "id": "DUT-2024-001",
  "serial_number": "SN12345678",
  "model": "Widget Pro",
  "hardware_version": "v2.1",
  "firmware_version": "1.4.2",
  "firmware_hash": "abc123def456"
}
```

**Best Practices:**
- **`id`**: Use a consistent identifier format (e.g., `DUT-YYYY-NNN`)
- **`serial_number`**: Physical label from the device
- **`firmware_version`**: Use semantic versioning (e.g., `1.4.2`)
- **`firmware_hash`**: Short git hash or build hash for exact traceability
- Keep hardware and firmware versions separate for independent tracking

### Test Information

**Purpose:** Link captures to test plans and execution context.

```json
"test": {
  "plan": "functional-validation",
  "plan_version": "2.0",
  "variant": "extended",
  "run_number": "42",
  "environment": "production"
}
```

**Best Practices:**
- **`plan`**: Use kebab-case names (e.g., `power-cycle-stress`, `calibration-full`)
- **`plan_version`**: Version your test plans to track methodology changes
- **`variant`**: Distinguish test configurations (e.g., `quick`, `standard`, `extended`)
- **`environment`**: Indicate test stage (`development`, `staging`, `production`)

### Tags

**Purpose:** Flexible categorization for filtering and grouping.

```json
"tags": {
  "tags": ["calibration", "production-line", "batch-2024-Q1"]
}
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
  "operator": {
    "name": "Line Operator",
    "id": "station-3-auto"
  },
  "location": {
    "site": "Factory-SZ",
    "building": "Production",
    "room": "Line 3",
    "bench_id": "STATION-3"
  },
  "device": {
    "model": "Widget Pro",
    "hardware_version": "v2.1",
    "firmware_version": "1.4.2-release"
  },
  "test": {
    "plan": "production-functional",
    "plan_version": "3.0",
    "environment": "production"
  },
  "tags": {
    "tags": ["production", "functional", "automated"]
  },
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
  "operator": {
    "name": "Alex Chen",
    "email": "alex.chen@company.com",
    "id": "achen"
  },
  "location": {
    "site": "HQ",
    "building": "R&D",
    "room": "Lab 201",
    "bench_id": "PROTO-BENCH-1"
  },
  "device": {
    "id": "PROTO-007",
    "model": "Widget Pro",
    "hardware_version": "v3.0-proto",
    "firmware_version": "2.0.0-dev.45",
    "firmware_hash": "abc123d"
  },
  "test": {
    "plan": "power-consumption",
    "variant": "sleep-modes",
    "environment": "development"
  },
  "tags": {
    "tags": ["prototype", "power", "investigation"]
  },
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
  "operator": {
    "name": "CI Pipeline",
    "id": "jenkins-main"
  },
  "location": {
    "site": "Cloud",
    "bench_id": "ci-runner-pool"
  },
  "device": {
    "id": "HIL-SIM-01",
    "model": "Widget Pro Simulator",
    "firmware_version": "${GIT_TAG}",
    "firmware_hash": "${GIT_COMMIT}"
  },
  "test": {
    "plan": "regression-suite",
    "plan_version": "1.0",
    "run_number": "${BUILD_NUMBER}",
    "environment": "ci"
  },
  "tags": {
    "tags": ["regression", "automated", "ci"]
  },
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
firmware_version: "1.4.2"
firmware_version: "v1.4.2"
firmware_version: "Version 1.4.2"
```

**Good:** Pick one format and use it everywhere:
```
firmware_version: "1.4.2"
```

### 2. Missing Device Identity

**Bad:** No way to trace back to physical device
```json
"device": {
  "model": "Widget"
}
```

**Good:** Include unique identifiers
```json
"device": {
  "id": "DUT-2024-042",
  "serial_number": "SN12345678",
  "model": "Widget Pro"
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
  "plan_version": "2.1",
  "variant": "extended"
}
```

### 5. Not Versioning Firmware

**Bad:** Cannot distinguish between builds
```json
"device": {
  "firmware_version": "latest"
}
```

**Good:** Include exact version and hash
```json
"device": {
  "firmware_version": "1.4.2",
  "firmware_hash": "abc123d"
}
```

---

## Setting Defaults

Configure defaults in the TUI to avoid repetitive entry:

1. Launch: `astrolabe tui`
2. Select **Configure Metadata**
3. Set common values that rarely change:
   - Operator name/ID
   - Location (site, building, room, bench)
   - Device model and hardware version
4. Press `Ctrl+S` to save

Override per-capture via CLI flags or TUI prompts when values differ.

---

## Integration with Orrery Backend

Metadata flows to Orrery where it enables:

- **Filtering**: Find all runs for a specific device or firmware version
- **Grouping**: Aggregate results by test plan or location
- **Trending**: Track metrics over time by firmware version
- **Alerts**: Trigger notifications based on metadata conditions

Ensure your metadata schema aligns with your Orrery dashboards and queries.

---

## Related Documentation

- [Operator Quickstart](OPERATOR_QUICKSTART.md) - Getting started
- [Configuration Guide](CONFIG_TUI.md) - Setting up metadata in TUI
- [Directory Structure](DIRECTORY_STRUCTURE.md) - Where metadata is stored
