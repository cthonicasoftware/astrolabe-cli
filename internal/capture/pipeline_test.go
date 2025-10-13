package capture

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/normalize"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/storage"
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
