package core

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestRunJSONRoundTrip(t *testing.T) {
	started := time.Date(2024, time.October, 15, 10, 30, 0, 0, time.UTC)
	completed := started.Add(5 * time.Minute)
	lastAttempt := completed.Add(2 * time.Minute)
	completedAt := lastAttempt.Add(30 * time.Second)

	original := Run{
		ID: "run-123",
		Source: SourceMeta{
			Kind: "serial",
			Port: "/dev/ttyUSB0",
			Baud: 115200,
		},
		Manifest: Manifest{
			SchemaVersion: "v1",
			Device: DeviceInfo{
				ID:              "fixture-42",
				Serial:          "SN-1001",
				Firmware:        "1.2.3",
				FirmwareHash:    "deadbeefcafebabe",
				HardwareVersion: "rev-b",
			},
			Test: TestInfo{
				Plan:    "burn-in",
				Variant: "optical",
				Run:     "13",
			},
			Operator:   "alice",
			Location:   "lab-bay-7",
			Tags:       []string{"smoke", "release"},
			Attributes: map[string]string{"fixture": "bench-3"},
		},
		Capture: CaptureSettings{
			SampleRateHz: 9600,
			Duration:     300,
			Channels:     []string{"ch1", "ch2"},
			Notes:        "stability sweep",
		},
		Started:        started,
		Completed:      &completed,
		RecordsCount:   42,
		PrimaryDataURI: "file:///tmp/qa-agent/run-123/data.jsonl",
		Artifacts: []Artifact{
			{
				Name:      "manifest.json",
				Path:      "/tmp/qa-agent/run-123/manifest.json",
				MediaType: "application/json",
				Role:      ArtifactRoleManifest,
				SizeBytes: 512,
				Checksum: Checksum{
					Algorithm: "sha256",
					Value:     "abc123",
				},
				CreatedAt:  started,
				UploadedAt: &completed,
				RemoteURL:  "https://qa.example.test/run-123/manifest",
			},
			{
				Name:      "data.jsonl",
				Path:      "/tmp/qa-agent/run-123/data.jsonl",
				MediaType: "application/x-ndjson",
				Role:      ArtifactRoleData,
				SizeBytes: 10240,
				Checksum: Checksum{
					Algorithm: "sha256",
					Value:     "def456",
				},
				CreatedAt: started,
			},
		},
		Upload: UploadState{
			Status:      UploadStatusSucceeded,
			Attempts:    2,
			LastAttempt: &lastAttempt,
			CompletedAt: &completedAt,
			RemoteRunID: "remote-456",
		},
	}

	payload, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}

	var decoded Run
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal run: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round-trip mismatch\nexpected: %#v\nactual:   %#v", original, decoded)
	}
}

func TestRecordJSONRoundTrip(t *testing.T) {
	ts := time.Date(2024, time.November, 2, 9, 15, 0, 0, time.UTC)
	record := Record{
		TS:   ts,
		Seq:  21,
		Type: "measurement",
		Payload: map[string]any{
			"channel": "ch1",
			"value":   "12.3",
			"units":   "mA",
		},
		Envelope: map[string]any{
			"raw": "0xffeedd",
		},
	}

	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}

	var decoded Record
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}

	if !reflect.DeepEqual(record, decoded) {
		t.Fatalf("round-trip mismatch\nexpected: %#v\nactual:   %#v", record, decoded)
	}
}
