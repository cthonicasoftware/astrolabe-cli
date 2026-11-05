package capture

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
	"github.com/oklog/ulid/v2"
)

type sliceSource struct {
	meta   core.SourceMeta
	frames [][]byte
}

func (s *sliceSource) Frames() <-chan []byte {
	ch := make(chan []byte, len(s.frames))
	go func() {
		for _, frame := range s.frames {
			ch <- frame
		}
		close(ch)
	}()
	return ch
}

func (s *sliceSource) Meta() core.SourceMeta {
	return s.meta
}

type stubNormalizer struct{}

func (stubNormalizer) Init(*core.Run) error { return nil }
func (stubNormalizer) Close() error         { return nil }
func (stubNormalizer) Ingest(_ []byte) ([]core.Record, error) {
	return []core.Record{{Type: "sample"}}, nil
}

type spyStore struct {
	cancel     context.CancelFunc
	started    bool
	finalized  bool
	writeCalls int
}

func (s *spyStore) Start(*core.Run) error {
	s.started = true
	return nil
}

func (s *spyStore) Write(records []core.Record) error {
	s.writeCalls += len(records)
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	return nil
}

func (s *spyStore) Finalize(*core.Run) error {
	s.finalized = true
	return nil
}

func (s *spyStore) Artifacts() []core.Artifact { return nil }

type chanSource struct {
	meta   core.SourceMeta
	frames <-chan []byte
}

func (c *chanSource) Frames() <-chan []byte { return c.frames }
func (c *chanSource) Meta() core.SourceMeta { return c.meta }

func TestPipelineRun(t *testing.T) {
	tmp := t.TempDir()
	src := &sliceSource{
		meta: core.SourceMeta{
			Kind: "serial",
			Port: "/dev/ttyUSB0",
			Baud: 115200,
		},
		frames: [][]byte{
			[]byte("hello\n"),
			[]byte("world\n"),
		},
	}

	opts := Options{
		Source:     src,
		Normalizer: normalize.NewLineJSON(),
		Store:      storage.NewFS(tmp),
		Manifest: core.Manifest{
			SchemaVersion: "v1alpha1",
			Test: core.TestInfo{
				Plan: "unit-test",
			},
		},
	}

	pipeline, err := NewPipeline(opts)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	run, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if run == nil {
		t.Fatalf("run is nil")
	}
	if run.ID == "" {
		t.Fatalf("run ID is empty")
	}
	if run.RecordsCount != 2 {
		t.Fatalf("expected 2 records, got %d", run.RecordsCount)
	}
	if run.Completed == nil {
		t.Fatalf("completed timestamp missing")
	}
	if len(run.Artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(run.Artifacts))
	}
	if run.PrimaryDataURI == "" {
		t.Fatalf("primary data uri not set")
	}

	for _, artifact := range run.Artifacts {
		if _, err := os.Stat(artifact.Path); err != nil {
			t.Fatalf("artifact file missing: %s", artifact.Path)
		}
		if artifact.Checksum.Value == "" {
			t.Fatalf("artifact missing checksum: %s", artifact.Name)
		}
	}
}

func TestPipelineRunReassemblesChunkedLines(t *testing.T) {
	src := &sliceSource{
		meta: core.SourceMeta{Kind: "serial"},
		frames: [][]byte{
			[]byte("h"),
			[]byte("ello\r\nwor"),
			[]byte("ld\r\ntail"), // "tail" has no newline; must be flushed
		},
	}
	store := &spyStore{}

	pipeline, err := NewPipeline(Options{
		Source:     src,
		Normalizer: normalize.NewLineJSON(),
		Store:      store,
		Manifest:   core.Manifest{SchemaVersion: "v1alpha1"},
	})
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	run, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if run.RecordsCount != 3 {
		t.Fatalf("expected 3 records (hello, world, tail), got %d", run.RecordsCount)
	}
	if store.writeCalls != 3 {
		t.Fatalf("expected 3 records written, got %d", store.writeCalls)
	}
}

func TestPipelineFinalizeOnContextCancel(t *testing.T) {
	frames := make(chan []byte)
	done := make(chan struct{})
	defer close(done)

	go func() {
		frames <- []byte("frame-1")
		<-done
	}()

	ctx, cancel := context.WithCancel(context.Background())
	store := &spyStore{cancel: cancel}

	source := &chanSource{
		meta: core.SourceMeta{
			Kind: "serial",
			Port: "/dev/ttyACM0",
			Baud: 115200,
		},
		frames: frames,
	}

	opts := Options{
		Source:     source,
		Normalizer: stubNormalizer{},
		Store:      store,
		Manifest: core.Manifest{
			SchemaVersion: "v1alpha1",
			Test:          core.TestInfo{Plan: "cancel-path"},
		},
	}

	pipeline, err := NewPipeline(opts)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	run, err := pipeline.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if run == nil {
		t.Fatalf("run is nil")
	}
	if !store.finalized {
		t.Fatalf("store finalize not called on cancellation")
	}
}

// TestDefaultRunID_ULID_Format verifies that generated run IDs are valid ULIDs
func TestDefaultRunID_ULID_Format(t *testing.T) {
	id := defaultRunID()

	// ULID must be exactly 26 characters
	if len(id) != 26 {
		t.Errorf("ULID length: got %d, want 26 (got: %s)", len(id), id)
	}

	// ULID must be valid Crockford Base32
	// Valid characters: 0-9, A-Z (excluding I, L, O, U)
	validChars := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	for i, ch := range id {
		if !strings.ContainsRune(validChars, ch) {
			t.Errorf("invalid ULID character at position %d: %c", i, ch)
		}
	}

	// Parse as ULID to verify it's valid
	parsed, err := ulid.Parse(id)
	if err != nil {
		t.Fatalf("failed to parse as ULID: %v", err)
	}

	// Verify timestamp is reasonable (within last minute)
	timestamp := ulid.Time(parsed.Time())
	now := time.Now()
	if timestamp.After(now) || timestamp.Before(now.Add(-1*time.Minute)) {
		t.Errorf("ULID timestamp out of reasonable range: %v", timestamp)
	}
}

// TestDefaultRunID_Uniqueness verifies that multiple calls generate unique IDs
func TestDefaultRunID_Uniqueness(t *testing.T) {
	const iterations = 100
	ids := make(map[string]bool, iterations)

	for i := 0; i < iterations; i++ {
		id := defaultRunID()
		if ids[id] {
			t.Fatalf("duplicate ULID generated: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != iterations {
		t.Errorf("expected %d unique IDs, got %d", iterations, len(ids))
	}
}

// TestDefaultRunID_Sortability verifies that ULIDs are lexicographically sortable by time
func TestDefaultRunID_Sortability(t *testing.T) {
	// Generate IDs with small delays to ensure different timestamps
	id1 := defaultRunID()
	time.Sleep(2 * time.Millisecond)
	id2 := defaultRunID()
	time.Sleep(2 * time.Millisecond)
	id3 := defaultRunID()

	// Verify lexicographic ordering matches time ordering
	if !(id1 < id2 && id2 < id3) {
		t.Errorf("ULIDs not sorted by time: %s, %s, %s", id1, id2, id3)
	}

	// Parse and verify timestamps are actually increasing
	t1, _ := ulid.Parse(id1)
	t2, _ := ulid.Parse(id2)
	t3, _ := ulid.Parse(id3)

	if !(t1.Time() < t2.Time() && t2.Time() < t3.Time()) {
		t.Errorf("ULID timestamps not increasing: %d, %d, %d",
			t1.Time(), t2.Time(), t3.Time())
	}
}

type timedSource struct {
	meta   core.SourceMeta
	frames []Frame
}

func (s *timedSource) Frames() <-chan []byte { panic("pipeline should read TimedFrames") }
func (s *timedSource) Meta() core.SourceMeta { return s.meta }
func (s *timedSource) TimedFrames() <-chan Frame {
	ch := make(chan Frame, len(s.frames))
	for _, f := range s.frames {
		ch <- f
	}
	close(ch)
	return ch
}

// Records from a timed source carry the frame's arrival time, not the time
// the pipeline got around to processing it.
func TestPipelineUsesFrameArrivalTime(t *testing.T) {
	t0 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1 := t0.Add(time.Second)
	store := &recordingStore{}
	pipeline, err := NewPipeline(Options{
		Source: &timedSource{frames: []Frame{
			{Data: []byte("hel"), At: t0},
			{Data: []byte("lo\nworld\n"), At: t1},
		}},
		Normalizer: normalize.NewLineJSON(),
		Store:      store,
		Manifest:   core.Manifest{SchemaVersion: "v1alpha1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.records) != 2 {
		t.Fatalf("got %d records, want 2", len(store.records))
	}
	if !store.records[0].TS.Equal(t0) || !store.records[1].TS.Equal(t1) {
		t.Errorf("ts = %v, %v; want %v, %v", store.records[0].TS, store.records[1].TS, t0, t1)
	}
}
