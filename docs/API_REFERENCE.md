# Astrolabe Backend API Reference

This document describes the backend API that `astrolabe upload` expects.

Astrolabe does not upload captured data directly to your application server. The backend creates a run record, returns presigned artifact upload URLs, and then accepts confirmation that the upload completed.

## Scope

This is the current integration contract implemented by the CLI upload client in `internal/upload/`.

- Base backend URL: configured as `api_url`
- API prefix added by the CLI: `/api/v1`
- Authentication: `Authorization: Bearer <token>` on backend requests
- Idempotency: `POST /api/v1/runs/` includes `Idempotency-Key: <local-run-id>`

Provide the base origin in configuration, for example `https://qa.example.com`. Do not include `/api/v1` in `api_url`.

## Upload Flow

1. Create a remote run record with `POST /api/v1/runs/`
2. Request a presigned URL for each artifact with `POST /api/v1/runs/{run_id}/artifacts/presign`
3. Upload each file to object storage using the returned URL
4. Confirm each uploaded artifact with `POST /api/v1/runs/{run_id}/artifacts/confirm`

The CLI currently uploads these artifacts from the local run cache:

- `manifest.json` with role `manifest` and content type `application/json`
- `data.jsonl` with role `data` and content type `application/x-ndjson`, when the file exists

## Endpoint: Create Run

Creates the remote run record before any artifact uploads happen.

### Request

`POST /api/v1/runs/`

Headers:

- `Authorization: Bearer <token>`
- `Content-Type: application/json`
- `Idempotency-Key: <local-run-id>`

Example request body:

```json
{
  "project_id": "project-123",
  "manifest": {
    "schema_version": "v1alpha1",
    "device": {
      "id": "bench-01",
      "firmware": "1.2.3"
    },
    "test": {
      "plan": "smoke"
    },
    "operator": "Jane Doe",
    "location": "Bench A",
    "tags": ["nightly"],
    "attributes": {
      "station": "west"
    }
  },
  "source": {
    "kind": "serial",
    "port": "/dev/ttyUSB0",
    "baud": 115200
  },
  "capture": {
    "sample_rate_hz": 1,
    "channels": ["serial"]
  },
  "started_at": "2026-03-18T23:30:00Z",
  "completed_at": "2026-03-18T23:45:00Z",
  "records_count": 100
}
```

### Response

Success response body:

```json
{
  "run_id": "remote-run-67890"
}
```

Accepted success status codes:

- `200 OK`
- `201 Created`

### Field Notes

- `project_id` is required.
- `completed_at` is omitted if the run has not been marked complete.
- `manifest`, `source`, and `capture` are forwarded from the local run metadata without reshaping.
- The CLI uses the local run ID as the idempotency key so retried create requests can be safely deduplicated by the backend.

## Endpoint: Request Artifact Upload URL

Returns a backend-assigned artifact ID plus a presigned upload URL for one artifact. The CLI currently sends one artifact per request, but the request and response format are batch-shaped.

### Request

`POST /api/v1/runs/{run_id}/artifacts/presign`

Headers:

- `Authorization: Bearer <token>`
- `Content-Type: application/json`

Example request body:

```json
{
  "artifacts": [
    {
      "filename": "manifest.json",
      "role": "manifest",
      "content_type": "application/json",
      "size_bytes": 1024,
      "checksum_sha256": "4d5e6f7a8b9c00112233445566778899aabbccddeeff00112233445566778899",
      "source": "cli"
    }
  ]
}
```

### Response

Example response body for a new upload:

```json
{
  "artifacts": [
    {
      "artifact_id": "01JARTIFACT123",
      "url": "https://storage.example.com/upload/01JARTIFACT123?...",
      "method": "PUT",
      "headers": {
        "Content-Type": "application/json"
      },
      "expires_at": "2026-03-18T23:55:00Z"
    }
  ]
}
```

Example response body when the backend detects the artifact already exists:

```json
{
  "artifacts": [
    {
      "artifact_id": "01JARTIFACT123",
      "status": "existing"
    }
  ]
}
```

Accepted success status codes:

- `200 OK`
- `201 Created`

### Field Notes

- `artifact_id` is required. The CLI stores it and sends it back during confirmation.
- The CLI sends exactly one artifact inside the `artifacts` array for each request.
- The CLI expects at least one item in the response `artifacts` array and uses the first item returned.
- `status: "existing"` tells the CLI to skip the object storage upload and proceed directly to confirmation.
- `method` is respected when present. If omitted, the CLI defaults to `PUT`.
- `headers` may contain required storage headers and are forwarded onto the object storage upload request.
- `source` is currently sent as `"cli"` by Astrolabe.
- The CLI currently emits these artifact roles in upload requests: `manifest` and `data`.

## Request: Upload Artifact to Object Storage

This is not a backend API endpoint, but it is part of the contract the backend returns.

The CLI performs an HTTP request to the presigned URL using:

- Method: value from `method`, or `PUT` if omitted
- `Content-Type`: the local artifact media type
- Any additional headers returned in `headers`
- Raw file bytes as the request body

Accepted success status codes:

- `200 OK`
- `201 Created`
- `204 No Content`

The CLI retries only retryable object storage upload failures with exponential backoff.

Retryable cases are:

- Network errors returned by the HTTP client
- `429 Too Many Requests`
- Any `5xx` response

Non-retryable cases include:

- Context cancellation or deadline expiry
- Local file access failures
- Other `4xx` responses from the upload target

## Endpoint: Confirm Artifact Upload

Confirms to the backend that the artifact was uploaded successfully or already existed remotely.

### Request

`POST /api/v1/runs/{run_id}/artifacts/confirm`

Headers:

- `Authorization: Bearer <token>`
- `Content-Type: application/json`

Example request body:

```json
{
  "artifacts": [
    {
      "artifact_id": "01JARTIFACT123",
      "filename": "manifest.json",
      "checksum_sha256": "4d5e6f7a8b9c00112233445566778899aabbccddeeff00112233445566778899",
      "uploaded_at": "2026-03-18T23:50:00Z"
    }
  ]
}
```

### Response

The CLI accepts any of these success responses:

- `200 OK`
- `201 Created`
- `204 No Content`

No response body is required.

### Field Notes

- `artifact_id` is required.
- `filename` must match the artifact name used during presign.
- `uploaded_at` is generated by the CLI at confirmation time.
- Confirmation is still sent when presign returned `status: "existing"`.

## Error Handling Expectations

The CLI treats backend responses outside the accepted success codes as failures and includes the response body in the surfaced error when available.

Upload retries apply only to the object storage upload request. Backend API requests for create, presign, and confirm are not retried inside `internal/upload`.

## Minimal Backend Checklist

A compatible backend should:

- Accept a base URL configured without `/api/v1`
- Implement `POST /api/v1/runs/`
- Implement `POST /api/v1/runs/{run_id}/artifacts/presign`
- Implement `POST /api/v1/runs/{run_id}/artifacts/confirm`
- Return an `artifact_id` for every presign response item
- Return a usable presigned upload URL unless the artifact already exists
- Accept confirmation for both newly uploaded and deduplicated artifacts

## Related Docs

- `docs/UPLOAD_GUIDE.md` for operator-facing upload workflow
- `docs/TESTING_ORRERY_INTEGRATION.md` for end-to-end integration testing
