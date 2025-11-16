# Testing Astrolabe Upload Integration with Orrery Backend

This guide helps you verify that the Astrolabe CLI correctly uploads runs to the Orrery backend using the updated batch format contract.

## Quick Start

### Prerequisites

1. **Orrery backend running** at `http://localhost:8000`
2. **MinIO running** at `http://localhost:9000` (for artifact storage)
3. **Valid API token** from Orrery backend
4. **Project ID** from Orrery backend

### 5-Minute Test

```bash
# 1. Build CLI
go build -o bin/astrolabe ./cmd/astrolabe

# 2. Configure (use the base URL - the CLI adds /api/v1 as needed)
export ASTROLABE_API_URL="http://localhost:8000"
export ASTROLABE_PROJECT_ID="<your-project-id>"
export ASTROLABE_AUTH_TOKEN="<your-api-token>"

# 3. Create a small test run so you know you have data to upload
cat <<'EOF' >/tmp/orrery-upload.log
temperature=22.1,status=warmup
temperature=22.5,status=steady
EOF
./bin/astrolabe capture file /tmp/orrery-upload.log \
  --format raw \
  --device-id bench-01 \
  --test-plan orrery-smoke
RUN_ID=$(ls -t ~/.astrolabe/runs | head -1)

# 4. Upload just that run
./bin/astrolabe upload --run-id "${RUN_ID}"

# 5. Check result
jq . ~/.astrolabe/runs/${RUN_ID}/upload_state.json
```

**Success:** `status` should be `"succeeded"`

---

## Detailed Testing Instructions

### Step 1: Get Backend Credentials

Contact your backend administrator or generate credentials:

1. Get **Project ID**:
   - From Orrery admin UI, or
   - From backend team
   - Example: `01HX9K7M2NVWGA3YF8QRST0VWX`

2. Get **API Token**:
   - Generated via `python manage.py generate_api_token`
   - Example: `orrery_01HX9K7M2NVWGA3YF8QRST0VWX_abc123...`

### Step 2: Configure Astrolabe

#### Option A: Environment Variables (Recommended for Testing)

```bash
export ASTROLABE_API_URL="http://localhost:8000"
export ASTROLABE_PROJECT_ID="01HX9K7M2NVWGA3YF8QRST0VWX"
export ASTROLABE_AUTH_TOKEN="orrery_01HX9K7M2NVWGA3YF8QRST0VWX_abc123..."
```

> ℹ️ **Note:** Provide the base origin (no `/api/v1`). The CLI automatically calls the `/api/v1/...` endpoints when making requests.

#### Option B: Config File (Recommended for Production)

```bash
mkdir -p ~/.astrolabe

cat > ~/.astrolabe/connection.yml <<EOF
api_url: "http://localhost:8000"
project_id: "01HX9K7M2NVWGA3YF8QRST0VWX"
auth_token: "orrery_01HX9K7M2NVWGA3YF8QRST0VWX_abc123..."
offline_cache: "~/.astrolabe/runs"
upload:
  max_retries: 3
EOF
```

### Step 3: Verify Configuration

`config get` requires a key, so check each field individually:

```bash
./bin/astrolabe config get api_url
./bin/astrolabe config get project_id
./bin/astrolabe config get auth_token
```

**Expected output (example):**
```
api_url: http://localhost:8000
project_id: 01HX9K7M2NVWGA3YF8QRST0VWX
auth_token: orrery_01HX9K7M2NVWGA3YF8QRST0VWX_abc123...
```

Values are printed in plain text, so avoid running this command if you cannot protect your terminal history.

### Step 4: Inspect Cached Runs

The current CLI exposes run management in the TUI, so from a shell you can inspect the cache directly:

```bash
ls -1 ~/.astrolabe/runs
RUN_ID=$(ls -t ~/.astrolabe/runs | head -1)
ls -1 ~/.astrolabe/runs/${RUN_ID}
```

Each run directory contains `manifest.json`, `data.jsonl`, and (after an upload attempt) `upload_state.json`. If `upload_state.json` is missing, that run is still pending.

> Tip: If the directory list is empty, ingest a small file with `astrolabe capture file …` to seed a run before continuing.

### Step 5: Upload Single Run

```bash
# Pick a run ID from the list (run IDs look like run-YYYYMMDD-HHMMSS)
RUN_ID="run-20240506-120001"  # replace with your actual ID

# Upload with verbose output
./bin/astrolabe upload --run-id ${RUN_ID}
```

**Expected output:**
```
Uploading run: run-20240506-120001
✓ Created run record (remote ID: 01HXA1B2C3D4E5F6G7H8J9K0M1)
✓ Requested presigned URL for manifest.json (artifact ID: 01HXA2...)
✓ Requested presigned URL for data.jsonl (artifact ID: 01HXA3...)
✓ Uploaded manifest.json (500 bytes)
✓ Uploaded data.jsonl (12.4 KB)
✓ Confirmed upload
✓ Run uploaded successfully
```

### Step 6: Verify Upload State

```bash
cat ~/.astrolabe/runs/${RUN_ID}/upload_state.json | jq
```

**Expected output:**
```json
{
  "status": "succeeded",
  "attempts": 0,
  "last_error": "",
  "completed_at": "2025-11-16T12:00:05Z",
  "remote_run_id": "01HXA1B2C3D4E5F6G7H8J9K0M1"
}
```

`last_attempt` only appears after a failure; successful uploads go straight from `queued → in_flight → succeeded`, so it is normal for that field to be absent.

---

## Understanding the Upload Flow

### What Happens During Upload

```
┌─────────────────────────────────────────────────────────────┐
│ 1. CREATE RUN                                               │
│    POST /api/v1/runs                                        │
│    ↓ Returns: remote_run_id                                 │
├─────────────────────────────────────────────────────────────┤
│ 2. REQUEST PRESIGNED URLS (for each artifact)              │
│    POST /api/v1/runs/{id}/artifacts/presign                │
│    Request: {artifacts: [{filename, role, checksum, ...}]}  │
│    ↓ Returns: {artifacts: [{artifact_id, url, ...}]}       │
│    ↓ CLI stores artifact_id locally                         │
├─────────────────────────────────────────────────────────────┤
│ 3. UPLOAD TO STORAGE (for each artifact)                   │
│    PUT <presigned_url>                                      │
│    ↓ Uploads directly to MinIO/S3 (not through backend)    │
├─────────────────────────────────────────────────────────────┤
│ 4. CONFIRM UPLOAD (for all artifacts)                      │
│    POST /api/v1/runs/{id}/artifacts/confirm                │
│    Request: {artifacts: [{artifact_id, filename, ...}]}    │
│    ↓ Uses artifact_id from step 2                          │
│    ↓ Backend verifies files exist in storage               │
├─────────────────────────────────────────────────────────────┤
│ 5. UPDATE LOCAL STATE                                      │
│    Write upload_state.json                                  │
│    ↓ Status: succeeded                                      │
└─────────────────────────────────────────────────────────────┘
```

### Key Contract Changes (Batch Format)

**OLD (Single Artifact):**
```json
{
  "file_name": "manifest.json",
  "content_type": "application/json",
  "size_bytes": 1024,
  "checksum_sha256": "abc123..."
}
```

**NEW (Batch Format):**
```json
{
  "artifacts": [{
    "filename": "manifest.json",
    "role": "manifest",
    "content_type": "application/json",
    "size_bytes": 1024,
    "checksum_sha256": "abc123...",
    "source": "cli"
  }]
}
```

**Response includes artifact_id:**
```json
{
  "artifacts": [{
    "artifact_id": "01JARTIFACT123",
    "url": "http://localhost:9000/...",
    "method": "PUT",
    "headers": {...},
    "expires_at": "2025-11-16T12:00:00Z"
  }]
}
```

---

## Testing Different Scenarios

### Test 1: Upload All Pending Runs

```bash
# Upload all runs with status "pending"
./bin/astrolabe upload

# Check results
for run in ~/.astrolabe/runs/*/; do
  echo "Run: $(basename $run)"
  jq -r '.status' "$run/upload_state.json"
done
```

### Test 2: Test Deduplication

Upload the same run twice to verify backend deduplication:

```bash
# First upload
./bin/astrolabe upload --run-id ${RUN_ID}

# Reset local state
rm ~/.astrolabe/runs/${RUN_ID}/upload_state.json

# Second upload (backend should detect duplicate artifacts)
./bin/astrolabe upload --run-id ${RUN_ID}
```

**Expected:** Second upload should be faster (files not re-uploaded)

### Test 3: Test Retry Logic

Simulate network failure:

```bash
# Stop backend temporarily
# (In Orrery terminal: Ctrl+C to stop runserver)

# Try upload (should fail and retry)
./bin/astrolabe upload --run-id ${RUN_ID}

# Restart backend
# (In Orrery terminal: restart runserver)

# Retry upload
./bin/astrolabe upload --run-id ${RUN_ID}
```

**Expected:** CLI retries with exponential backoff, succeeds after backend restarts

### Test 4: Test Large Files

Create a run with a large artifact:

```bash
# Create large test file (10MB)
dd if=/dev/urandom of=/tmp/large_data.bin bs=1M count=10

# Capture from file
./bin/astrolabe capture file /tmp/large_data.bin --device-id test-large

# Upload
./bin/astrolabe upload
```

**Expected:** Upload succeeds even with larger files

### Test 5: Test Network Interruption

```bash
# Start upload in background
./bin/astrolabe upload --run-id ${RUN_ID} &
UPLOAD_PID=$!

# Wait 2 seconds, then kill it
sleep 2
kill $UPLOAD_PID

# Check state
cat ~/.astrolabe/runs/${RUN_ID}/upload_state.json
```

**Expected:** `status` should be `"failed"` or `"in_flight"`

**Resume:**
```bash
./bin/astrolabe upload --run-id ${RUN_ID}
```

**Expected:** Upload resumes and completes

---

## Debugging Common Issues

### Issue: "API URL not configured"

**Symptom:**
```
Error: API URL not configured
```

**Fix:**
```bash
export ASTROLABE_API_URL="http://localhost:8000"
```

### Issue: "auth token not configured"

**Symptom:**
```
Error: auth token not configured
```

**Fix:**
```bash
export ASTROLABE_AUTH_TOKEN="<your-token>"
```

### Issue: "project ID not configured"

**Symptom:**
```
Error: project ID not configured
```

**Fix:**
```bash
export ASTROLABE_PROJECT_ID="<your-project-id>"
```

### Issue: "create run failed: status=404"

**Symptom:**
```
Error: create run failed: status=404
```

**Possible causes:**
1. Backend not running
2. Wrong API URL
3. Project doesn't exist

**Fix:**
```bash
# Check backend is running
curl http://localhost:8000/api/v1/

# Verify project exists (ask backend admin)
```

### Issue: "create run failed: status=403"

**Symptom:**
```
Error: create run failed: status=403 body={"error":"forbidden"}
```

**Possible causes:**
1. Invalid API token
2. User not member of project
3. Token expired

**Fix:**
```bash
# Regenerate API token
# Contact backend admin to add you to project
```

### Issue: "presigned url request failed: status=500"

**Symptom:**
```
Error: presigned url request failed: status=500
```

**Possible causes:**
1. MinIO not running
2. Backend storage not configured
3. Backend bug

**Fix:**
```bash
# Check MinIO is running
curl http://localhost:9000/minio/health/live

# Check backend logs for error details
```

### Issue: "artifact missing remote_artifact_id"

**Symptom:**
```
Error: artifact missing remote_artifact_id (must call GetPresignedURL first)
```

**Cause:** Backend didn't return `artifact_id` in presign response

**Fix:** Backend needs to be updated to Phase 3 contract

### Issue: Upload hangs or times out

**Symptom:**
```
Uploading manifest.json...
[hangs]
```

**Possible causes:**
1. Network issue
2. MinIO not accessible
3. Presigned URL expired

**Fix:**
```bash
# Check MinIO connectivity
curl http://localhost:9000/minio/health/live

# Try with verbose logging
export ASTROLABE_LOG_LEVEL=debug
./bin/astrolabe upload --run-id ${RUN_ID}
```

---

## Advanced Debugging

### Enable Debug Logging

```bash
export ASTROLABE_LOG_LEVEL=debug
./bin/astrolabe upload --run-id ${RUN_ID}
```

**Output includes:**
- Full HTTP request/response bodies
- Presigned URLs
- Upload progress
- Retry attempts

### Inspect Network Traffic

Use a proxy to see all HTTP traffic:

```bash
# Install mitmproxy
brew install mitmproxy  # or: pip install mitmproxy

# Run proxy
mitmproxy -p 8888

# Configure CLI to use proxy
export HTTP_PROXY=http://localhost:8888
export HTTPS_PROXY=http://localhost:8888

# Run upload
./bin/astrolabe upload --run-id ${RUN_ID}
```

### Test with curl

Manually test each endpoint:

**1. Create run:**
```bash
curl -X POST http://localhost:8000/api/v1/runs/ \
  -H "Authorization: Bearer ${ASTROLABE_AUTH_TOKEN}" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: test-run-001" \
  -d '{
    "project_id": "'"${ASTROLABE_PROJECT_ID}"'",
    "manifest": {
      "schema_version": "1.0.0",
      "device": {"id": "test", "firmware": "v1.0"},
      "test": {"plan": "smoke"}
    },
    "source": {"kind": "serial", "port": "/dev/ttyUSB0"},
    "capture": {"sample_rate_hz": 1.0},
    "started_at": "2025-11-16T10:00:00Z",
    "records_count": 100
  }' | jq
```

**2. Request presign:**
```bash
RUN_ID="<run-id-from-step-1>"

curl -X POST http://localhost:8000/api/v1/runs/${RUN_ID}/artifacts/presign \
  -H "Authorization: Bearer ${ASTROLABE_AUTH_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "artifacts": [{
      "filename": "test.json",
      "role": "data",
      "content_type": "application/json",
      "size_bytes": 100,
      "checksum_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    }]
  }' | jq
```

**3. Upload file:**
```bash
PRESIGNED_URL="<url-from-step-2>"

echo '{"test": "data"}' | curl -X PUT "${PRESIGNED_URL}" \
  -H "Content-Type: application/json" \
  --data-binary @-
```

**4. Confirm upload:**
```bash
ARTIFACT_ID="<artifact-id-from-step-2>"

curl -X POST http://localhost:8000/api/v1/runs/${RUN_ID}/artifacts/confirm \
  -H "Authorization: Bearer ${ASTROLABE_AUTH_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "artifacts": [{
      "artifact_id": "'"${ARTIFACT_ID}"'",
      "filename": "test.json",
      "checksum_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "uploaded_at": "2025-11-16T12:00:00Z"
    }]
  }'
```

---

## Verification Checklist

After upload completes, verify:

- [ ] `upload_state.json` shows `status: "succeeded"`
- [ ] `remote_run_id` is populated
- [ ] No errors in CLI output
- [ ] Backend shows run with status `PROCESSING`
- [ ] Artifacts visible in MinIO (http://localhost:9001)
- [ ] Backend database shows artifacts with `upload_status='uploaded'`

---

## Performance Benchmarks

Expected upload times (local development):

| Artifact Size | Upload Time | Notes |
|---------------|-------------|-------|
| < 1 KB | < 1 second | Manifest files |
| 1 MB | 1-2 seconds | Small data files |
| 10 MB | 5-10 seconds | Medium data files |
| 100 MB | 30-60 seconds | Large data files |

**Note:** Times vary based on network and storage backend

---

## Integration with CI/CD

### Example GitHub Actions Workflow

```yaml
name: Upload Test Runs

on:
  push:
    branches: [main]

jobs:
  upload:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Build Astrolabe
        run: go build -o bin/astrolabe ./cmd/astrolabe

      - name: Upload Runs
        env:
          ASTROLABE_API_URL: ${{ secrets.ORRERY_API_URL }}  # base origin, no /api/v1
          ASTROLABE_PROJECT_ID: ${{ secrets.ORRERY_PROJECT_ID }}
          ASTROLABE_AUTH_TOKEN: ${{ secrets.ORRERY_AUTH_TOKEN }}
        run: |
          ./bin/astrolabe upload

      - name: Verify Upload
        run: |
          # Check all runs uploaded successfully
          for run in ~/.astrolabe/runs/*/; do
            status=$(jq -r '.status' "$run/upload_state.json")
            if [ "$status" != "succeeded" ]; then
              echo "Upload failed for $(basename $run)"
              exit 1
            fi
          done
```

---

## Next Steps

After successful testing:

1. ✅ Configure production backend URL
2. ✅ Set up automated uploads (cron or systemd timer)
3. ✅ Monitor upload success rates
4. ✅ Set up alerting for failed uploads
5. ✅ Document team upload workflow

---

## Reference

- **Full Integration Guide**: `../../Orrery/INTEGRATION_TEST_GUIDE.md`
- **Upload System Guide**: `UPLOAD_GUIDE.md`
- **Astrolabe Skill**: `ASTROLABE_SKILL.md`
- **Orrery API Contract**: `../../Orrery/API/API_CONTRACT.md`
