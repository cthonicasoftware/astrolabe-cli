package capture

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/normalize"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/storage"
	"github.com/oklog/ulid/v2"
)

// Source represents an ingest source capable of streaming byte frames along with metadata.
type Source interface {
	Frames() <-chan []byte
	Meta() core.SourceMeta
}

// RunIDGenerator allows callers to supply deterministic run identifiers (useful in tests).
type RunIDGenerator func() string

// Options define the dependencies and metadata required to execute a capture pipeline.
type Options struct {
	Source     Source
	Normalizer normalize.Normalizer
	Store      storage.Store
	Manifest   core.Manifest
	Capture    core.CaptureSettings
	RunID      string
	RunIDGenerator
}

// Validate ensures the options contain the minimum required pieces to execute a capture.
func (o Options) Validate() error {
	if o.Source == nil {
		return errors.New("capture: source is required")
	}
	if o.Normalizer == nil {
		return errors.New("capture: normalizer is required")
	}
	if o.Store == nil {
		return errors.New("capture: store is required")
	}
	if o.Manifest.SchemaVersion == "" {
		return errors.New("capture: manifest schema_version is required")
	}
	return nil
}

// Pipeline coordinates a capture session across a source, normalizer, and storage backend.
type Pipeline struct {
	opts Options
}

// NewPipeline constructs a pipeline with validated options, wiring default run ID generators when absent.
func NewPipeline(opts Options) (*Pipeline, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	if opts.RunID == "" {
		gen := opts.RunIDGenerator
		if gen == nil {
			gen = defaultRunID
		}
		opts.RunID = gen()
	}
	return &Pipeline{opts: opts}, nil
}

// Run executes the capture until the source channel closes or the context is cancelled, returning the finalized run.
func (p *Pipeline) Run(ctx context.Context) (*core.Run, error) {
	run := &core.Run{
		ID:        p.opts.RunID,
		Source:    p.opts.Source.Meta(),
		Manifest:  p.opts.Manifest,
		Capture:   p.opts.Capture,
		Started:   time.Now().UTC(),
		Upload:    core.UploadState{Status: core.UploadStatusPending},
		Artifacts: nil,
	}

	if err := p.opts.Normalizer.Init(run); err != nil {
		return nil, fmt.Errorf("normalizer init: %w", err)
	}
	defer p.opts.Normalizer.Close()

	if err := p.opts.Store.Start(run); err != nil {
		return nil, fmt.Errorf("store start: %w", err)
	}

	var (
		seq       uint64
		framesCh  = p.opts.Source.Frames()
		cancelErr error
	)

loop:
	for {
		select {
		case <-ctx.Done():
			cancelErr = context.Cause(ctx)
			break loop
		case frame, ok := <-framesCh:
			if !ok {
				break loop
			}
			if len(frame) == 0 {
				continue
			}

			records, err := p.opts.Normalizer.Ingest(frame)
			if err != nil {
				return nil, fmt.Errorf("normalize frame: %w", err)
			}
			if len(records) == 0 {
				continue
			}

			for i := range records {
				if records[i].Seq == 0 {
					seq++
					records[i].Seq = seq
				} else if records[i].Seq > seq {
					seq = records[i].Seq
				}
				if records[i].TS.IsZero() {
					records[i].TS = time.Now().UTC()
				}
			}

			if err := p.opts.Store.Write(records); err != nil {
				return nil, fmt.Errorf("store write: %w", err)
			}

			run.RecordsCount += uint64(len(records))
		}
	}

	completed := time.Now().UTC()
	run.Completed = &completed

	if err := p.opts.Store.Finalize(run); err != nil {
		return nil, fmt.Errorf("store finalize: %w", err)
	}

	run.Artifacts = p.opts.Store.Artifacts()
	if run.PrimaryDataURI == "" {
		if data := primaryDataArtifact(run.Artifacts); data != nil {
			run.PrimaryDataURI = data.Path
		}
	}

	return run, cancelErr
}

func primaryDataArtifact(artifacts []core.Artifact) *core.Artifact {
	for i := range artifacts {
		if artifacts[i].Role == core.ArtifactRoleData {
			return &artifacts[i]
		}
	}
	return nil
}

func defaultRunID() string {
	// Generate a monotonic ULID with current timestamp
	// ULIDs are:
	// - 26 characters (Crockford Base32)
	// - Lexicographically sortable by time
	// - Globally unique
	// - Compatible with backend's ULID expectations
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
