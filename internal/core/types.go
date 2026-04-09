// Package core defines the shared domain types used across all astrolabe packages.
package core

import "time"

// SourceMeta describes the origin of a capture: the kind of source (serial, tcp, file)
// and the connection parameters relevant to that kind.
type SourceMeta struct {
	Kind string `json:"kind"`
	Port string `json:"port,omitempty"`
	Baud int    `json:"baud,omitempty"`
	Addr string `json:"addr,omitempty"`
	Path string `json:"path,omitempty"`
}

// DeviceInfo identifies the physical device under test.
type DeviceInfo struct {
	ID              string `json:"id"`
	Serial          string `json:"serial,omitempty"`
	Firmware        string `json:"firmware,omitempty"`
	FirmwareHash    string `json:"firmware_hash,omitempty"`
	HardwareVersion string `json:"hardware_version,omitempty"`
}

// TestInfo identifies the test plan and specific variant being executed.
type TestInfo struct {
	Plan    string `json:"plan"`
	Variant string `json:"variant,omitempty"`
	Run     string `json:"run,omitempty"`
}

// Manifest is the human-supplied metadata written into every run directory.
// It records who ran the test, on which device, under which test plan.
type Manifest struct {
	SchemaVersion string            `json:"schema_version"`
	Device        DeviceInfo        `json:"device"`
	Test          TestInfo          `json:"test"`
	Operator      string            `json:"operator,omitempty"`
	Location      string            `json:"location,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// CaptureSettings records the technical parameters of the capture session
// (sample rate, duration, channel list) for reference alongside the data.
type CaptureSettings struct {
	SampleRateHz float64  `json:"sample_rate_hz,omitempty"`
	Duration     float64  `json:"duration_seconds,omitempty"`
	Channels     []string `json:"channels,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

// ManifestOptions holds the user-supplied metadata flags shared across all capture subcommands.
type ManifestOptions struct {
	Operator   string
	Location   string
	Device     DeviceInfo
	Test       TestInfo
	Tags       []string
	Attributes map[string]string
}

// Checksum holds the algorithm name and hex-encoded digest for an artifact.
type Checksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// ArtifactRole classifies the purpose of an artifact within a run directory.
type ArtifactRole string

const (
	// ArtifactRoleManifest identifies the run's manifest.json file.
	ArtifactRoleManifest ArtifactRole = "manifest"
	// ArtifactRoleData identifies the primary NDJSON data file.
	ArtifactRoleData ArtifactRole = "data"
	// ArtifactRoleLog identifies a capture log file.
	ArtifactRoleLog ArtifactRole = "log"
	// ArtifactRoleAttachment identifies any supplementary file attached to the run.
	ArtifactRoleAttachment ArtifactRole = "attachment"
)

// Artifact represents a single file associated with a run, including its
// checksum, upload status, and optional remote reference.
type Artifact struct {
	Name             string       `json:"name"`
	Path             string       `json:"path"`
	MediaType        string       `json:"media_type"`
	Role             ArtifactRole `json:"role"`
	SizeBytes        int64        `json:"size_bytes,omitempty"`
	Checksum         Checksum     `json:"checksum"`
	CreatedAt        time.Time    `json:"created_at"`
	UploadedAt       *time.Time   `json:"uploaded_at,omitempty"`
	RemoteURL        string       `json:"remote_url,omitempty"`
	RemoteArtifactID string       `json:"remote_artifact_id,omitempty"` // Backend-assigned ULID
}

// UploadStatus represents the lifecycle state of a run's upload to the backend.
type UploadStatus string

const (
	// UploadStatusPending means the run has not been queued for upload yet.
	UploadStatusPending UploadStatus = "pending"
	// UploadStatusQueued means the run is waiting in the upload queue.
	UploadStatusQueued UploadStatus = "queued"
	// UploadStatusInFlight means the upload is currently in progress.
	UploadStatusInFlight UploadStatus = "in_flight"
	// UploadStatusSucceeded means all artifacts were uploaded successfully.
	UploadStatusSucceeded UploadStatus = "succeeded"
	// UploadStatusFailed means one or more upload attempts failed.
	UploadStatusFailed UploadStatus = "failed"
)

// UploadState tracks the progress and outcome of uploading a run to the backend.
type UploadState struct {
	Status      UploadStatus `json:"status"`
	Attempts    int          `json:"attempts"`
	LastError   string       `json:"last_error,omitempty"`
	LastAttempt *time.Time   `json:"last_attempt,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	RemoteRunID string       `json:"remote_run_id,omitempty"`
}

// Run is the top-level record for a completed or in-progress capture session.
// It aggregates the source metadata, manifest, capture settings, artifacts,
// and upload state into a single persistent document.
type Run struct {
	ID             string          `json:"run_id"`
	Source         SourceMeta      `json:"source"`
	Manifest       Manifest        `json:"manifest"`
	Capture        CaptureSettings `json:"capture"`
	Started        time.Time       `json:"started"`
	Completed      *time.Time      `json:"completed,omitempty"`
	RecordsCount   uint64          `json:"records_count,omitempty"`
	PrimaryDataURI string          `json:"primary_data_uri,omitempty"`
	Artifacts      []Artifact      `json:"artifacts,omitempty"`
	Upload         UploadState     `json:"upload"`
}

// Record is a single normalized data point emitted by a normalizer.
// Each record has a timestamp, sequence number, type tag, and a free-form payload.
type Record struct {
	TS       time.Time      `json:"ts"`
	Seq      uint64         `json:"seq"`
	Type     string         `json:"type"`
	Payload  map[string]any `json:"payload"`
	Envelope map[string]any `json:"envelope,omitempty"`
}
