package core

import "time"

type SourceMeta struct {
	Kind string `json:"kind"`
	Port string `json:"port,omitempty"`
	Baud int    `json:"baud,omitempty"`
	Addr string `json:"addr,omitempty"`
	Path string `json:"path,omitempty"`
}

type DeviceInfo struct {
	ID              string `json:"id"`
	Serial          string `json:"serial,omitempty"`
	Firmware        string `json:"firmware,omitempty"`
	FirmwareHash    string `json:"firmware_hash,omitempty"`
	HardwareVersion string `json:"hardware_version,omitempty"`
}

type TestInfo struct {
	Plan    string `json:"plan"`
	Variant string `json:"variant,omitempty"`
	Run     string `json:"run,omitempty"`
}

type Manifest struct {
	SchemaVersion string            `json:"schema_version"`
	Device        DeviceInfo        `json:"device"`
	Test          TestInfo          `json:"test"`
	Operator      string            `json:"operator,omitempty"`
	Location      string            `json:"location,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type CaptureSettings struct {
	SampleRateHz float64  `json:"sample_rate_hz,omitempty"`
	Duration     float64  `json:"duration_seconds,omitempty"`
	Channels     []string `json:"channels,omitempty"`
	Notes        string   `json:"notes,omitempty"`
}

type Checksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

type ArtifactRole string

const (
	ArtifactRoleManifest   ArtifactRole = "manifest"
	ArtifactRoleData       ArtifactRole = "data"
	ArtifactRoleLog        ArtifactRole = "log"
	ArtifactRoleAttachment ArtifactRole = "attachment"
)

type Artifact struct {
	Name       string       `json:"name"`
	Path       string       `json:"path"`
	MediaType  string       `json:"media_type"`
	Role       ArtifactRole `json:"role"`
	SizeBytes  int64        `json:"size_bytes,omitempty"`
	Checksum   Checksum     `json:"checksum"`
	CreatedAt  time.Time    `json:"created_at"`
	UploadedAt *time.Time   `json:"uploaded_at,omitempty"`
	RemoteURL  string       `json:"remote_url,omitempty"`
}

type UploadStatus string

const (
	UploadStatusPending   UploadStatus = "pending"
	UploadStatusQueued    UploadStatus = "queued"
	UploadStatusInFlight  UploadStatus = "in_flight"
	UploadStatusSucceeded UploadStatus = "succeeded"
	UploadStatusFailed    UploadStatus = "failed"
)

type UploadState struct {
	Status      UploadStatus `json:"status"`
	Attempts    int          `json:"attempts"`
	LastError   string       `json:"last_error,omitempty"`
	LastAttempt *time.Time   `json:"last_attempt,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	RemoteRunID string       `json:"remote_run_id,omitempty"`
}

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

type Record struct {
	TS       time.Time      `json:"ts"`
	Seq      uint64         `json:"seq"`
	Type     string         `json:"type"`
	Payload  map[string]any `json:"payload"`
	Envelope map[string]any `json:"envelope,omitempty"`
}
