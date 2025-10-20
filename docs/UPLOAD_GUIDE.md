# Upload System Guide

This guide explains how the Astrolabe upload system works and how to configure it.

## Overview

The upload system follows a three-step process:

1. **Create Run Record** - Register the run with the QA backend server
2. **Upload Artifacts** - Upload files (manifest, data) to cloud storage using presigned URLs
3. **Confirm Uploads** - Notify the server that uploads completed successfully

## Configuration

### Interactive Configuration (Recommended)

Launch the interactive configuration editor:

```bash
astrolabe config edit
```

This TUI allows you to configure:
- API URL (your backend server)
- Project ID
- Authentication token (with masked input)
- Offline cache location
- Max retry attempts

Press `Ctrl+T` to toggle token visibility, `Ctrl+S` to save, and `Esc` to exit.

### Manual Configuration

You can also manually edit `~/.astrolabe/connection.yml`:

```yaml
# ~/.astrolabe/connection.yml
api_url: "https://qa.yourcompany.com"
project_id: "project-123"
auth_token: "your-api-token-here"
offline_cache: "~/.astrolabe/runs"

# Optional upload settings
upload:
  max_retries: 3        # Number of retry attempts (default: 3)
  batch_bytes: 10485760 # 10MB batch size (not yet used)
```

Or set these via environment variables:
- `ASTROLABE_API_URL`
- `ASTROLABE_PROJECT_ID`
- `ASTROLABE_AUTH_TOKEN`

## Usage

### Upload a specific run

```bash
astrolabe upload --run-id <run-id>
```

### Upload all pending runs

```bash
astrolabe upload
```

This will scan `~/.astrolabe/runs` and upload any runs that haven't been uploaded yet.

### Upload UI

**Interactive mode** (terminal):
- Shows a progress bar with spinner
- Displays current run being uploaded
- Shows checkmarks (✓) for successful uploads
- Shows X marks (✗) for failures
- Live count: X/Y runs uploaded

**Non-interactive mode** (CI/scripts):
- Plain text output
- Line-by-line progress
- Suitable for logging and automation

## How Presigned URLs Work

**What are presigned URLs?**

A presigned URL is a temporary, secure URL that grants permission to upload a file to cloud storage (like AWS S3) without needing permanent credentials.

**The Flow:**

```
1. CLI → Server: "I need to upload manifest.json (500 bytes)"
2. Server → CLI: "Here's a secure URL, valid for 10 minutes"
3. CLI → Cloud Storage: *uploads file directly*
4. CLI → Server: "Upload complete, checksum: abc123"
5. Server: *verifies and marks artifact as uploaded*
```

**Benefits:**
- Files upload directly to cloud storage (faster, no server bottleneck)
- Server doesn't handle large file transfers (saves bandwidth)
- URLs expire automatically (security)
- CLI doesn't need cloud storage credentials

## Retry Logic and Resilience

The upload system is designed to handle network failures gracefully:

### Exponential Backoff

When a network error occurs, the system retries with increasing delays:
- Attempt 1: Immediate
- Attempt 2: Wait 1 second
- Attempt 3: Wait 2 seconds
- Attempt 4: Wait 4 seconds
- Attempt 5: Wait 8 seconds

This prevents hammering a server that might be temporarily overloaded.

### Upload State Persistence

Each run's upload status is saved to `upload_state.json`:

```json
{
  "status": "succeeded",
  "attempts": 2,
  "last_error": "",
  "last_attempt": "2024-01-15T10:30:00Z",
  "completed_at": "2024-01-15T10:30:05Z",
  "remote_run_id": "run-67890"
}
```

This allows the system to:
- Skip runs that are already uploaded
- Resume failed uploads (future enhancement)
- Track error history for debugging

### Status Values

- `pending` - Never uploaded
- `queued` - Scheduled for upload
- `in_flight` - Currently uploading
- `succeeded` - Upload complete
- `failed` - Upload failed after retries

## Backend API Endpoints

The upload client expects these endpoints on your backend server:

### 1. Create Run

```
POST /api/v1/runs
Authorization: Bearer <token>
Content-Type: application/json

{
  "project_id": "project-123",
  "manifest": {...},
  "source": {...},
  "capture": {...},
  "started_at": "2024-01-15T10:00:00Z",
  "records_count": 1000
}

Response:
{
  "run_id": "remote-run-67890"
}
```

### 2. Get Presigned URL

```
POST /api/v1/runs/<run_id>/artifacts/presign
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "manifest.json",
  "content_type": "application/json",
  "size_bytes": 1024,
  "checksum_sha256": "abc123..."
}

Response:
{
  "url": "https://s3.amazonaws.com/bucket/path?credentials...",
  "method": "PUT",
  "headers": {
    "Content-MD5": "..."
  }
}
```

### 3. Confirm Upload

```
POST /api/v1/runs/<run_id>/artifacts/confirm
Authorization: Bearer <token>
Content-Type: application/json

{
  "file_name": "manifest.json",
  "checksum_sha256": "abc123...",
  "uploaded_at": "2024-01-15T10:30:00Z"
}

Response: 200 OK or 204 No Content
```

## File Structure

After a capture, the offline cache contains:

```
~/.astrolabe/runs/
  └── run-abc123/
      ├── manifest.json       # Run metadata
      ├── data.jsonl          # Captured data
      └── upload_state.json   # Upload tracking
```

## Code Architecture

The upload system has three main components:

### 1. `APIClient` (api_client.go)

Handles HTTP communication with the QA backend:
- Creates run records
- Requests presigned URLs
- Confirms successful uploads

### 2. `Uploader` (uploader.go)

Handles actual file uploads:
- Uploads to presigned URLs
- Implements retry logic
- Tracks upload attempts

### 3. `Client` (client.go)

Orchestrates the full process:
- Loads run metadata from disk
- Manages upload state
- Coordinates API calls and file uploads
- Persists state for resume capability

## Testing

Run the integration test:

```bash
go test ./internal/upload -v
```

This test creates a mock server and simulates the full upload flow.

## Troubleshooting

### "API URL not configured"

Set your backend URL:
```bash
export ASTROLABE_API_URL="https://qa.yourcompany.com"
```

### "auth token not configured"

Generate an API token from your QA app and set:
```bash
export ASTROLABE_AUTH_TOKEN="your-token"
```

### "project ID not configured"

Set the project you're uploading to:
```bash
export ASTROLABE_PROJECT_ID="project-123"
```

### Upload fails with network errors

Check your internet connection and verify the API URL is correct. The system will retry automatically, but if the backend is down, uploads will fail after max retries.

## Next Steps

Future enhancements:
- Retry failed uploads (check upload_state.json and retry if status=failed)
- Progress bars for large files
- Parallel upload of multiple artifacts
- Compression before upload
- Partial upload resume (for very large files)
