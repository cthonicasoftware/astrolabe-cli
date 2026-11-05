package capture

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
	"github.com/oklog/ulid/v2"
)

// Source represents an ingest source capable of streaming byte frames along with metadata.
// Record direction values stored under envelope.direction for runs from
// bidirectional sources.
const (
	DirectionRX = "rx"
	DirectionTX = "tx"

	// RecordTypeTX is the record type for data the operator sent.
	RecordTypeTX = "tx"

	// AttrTXRecords is the manifest attribute counting sent records.
	AttrTXRecords = "tx_records"
)

// withDirection returns env with envelope.direction set, without mutating a
// map the normalizer may reuse.
func withDirection(env map[string]any, dir string) map[string]any {
	out := make(map[string]any, len(env)+1)
	for k, v := range env {
		out[k] = v
	}
	out["direction"] = dir
	return out
}

type Source interface {
	Frames() <-chan []byte
	Meta() core.SourceMeta
}

// Frame is a chunk of source bytes with the time it arrived.
type Frame struct {
	Data []byte
	At   time.Time
}

// TimedSource is an optional interface for sources that stamp frames as they
// arrive. The pipeline reads TimedFrames instead of Frames when available, so
// record timestamps match what a live view showed. Frames from a plain
// Source are stamped when the pipeline receives them.
type TimedSource interface {
	TimedFrames() <-chan Frame
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

	// TX carries records for data the operator sent to the device. Optional;
	// a nil channel means the run has no send path. Records bypass the
	// normalizer so they never split a partially received line.
	TX <-chan core.Record

	// Bidirectional marks the source as able to send (serial, TCP). Received
	// records are then tagged envelope.direction="rx", sent ones "tx", and the
	// manifest gets a tx_records attribute.
	Bidirectional bool
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

	// Exactly one of framesCh and timedCh is set; the other stays nil so its
	// select case never fires.
	var (
		seq       uint64
		txCount   uint64
		framesCh  <-chan []byte
		timedCh   <-chan Frame
		txCh      = p.opts.TX
		cancelErr error
	)
	if ts, ok := p.opts.Source.(TimedSource); ok {
		timedCh = ts.TimedFrames()
	} else {
		framesCh = p.opts.Source.Frames()
	}

	timed, _ := p.opts.Normalizer.(normalize.TimedIngester)
	ingest := func(data []byte, at time.Time) ([]core.Record, error) {
		if timed != nil {
			return timed.IngestAt(data, at)
		}
		return p.opts.Normalizer.Ingest(data)
	}

	writeRecords := func(records []core.Record) error {
		if len(records) == 0 {
			return nil
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
			return fmt.Errorf("store write: %w", err)
		}
		run.RecordsCount += uint64(len(records))
		return nil
	}

	writeRX := func(records []core.Record) error {
		if p.opts.Bidirectional {
			for i := range records {
				records[i].Envelope = withDirection(records[i].Envelope, DirectionRX)
			}
		}
		return writeRecords(records)
	}

	writeTX := func(rec core.Record) error {
		rec.Envelope = withDirection(rec.Envelope, DirectionTX)
		if err := writeRecords([]core.Record{rec}); err != nil {
			return err
		}
		txCount++
		return nil
	}

	ingestRX := func(data []byte, at time.Time) error {
		if len(data) == 0 {
			return nil
		}
		records, err := ingest(data, at)
		if err != nil {
			return fmt.Errorf("normalize frame: %w", err)
		}
		return writeRX(records)
	}

	// drainTX records sends still queued when the loop ends; those bytes
	// already reached the device, so they belong in the run.
	drainTX := func() error {
		for txCh != nil {
			select {
			case rec, ok := <-txCh:
				if !ok {
					return nil
				}
				if err := writeTX(rec); err != nil {
					return err
				}
			default:
				return nil
			}
		}
		return nil
	}

loop:
	for {
		select {
		case <-ctx.Done():
			cancelErr = context.Cause(ctx)
			break loop
		case rec, ok := <-txCh:
			if !ok {
				txCh = nil // stop selecting a closed channel
				continue
			}
			if err := writeTX(rec); err != nil {
				return nil, err
			}
		case data, ok := <-framesCh:
			if !ok {
				break loop
			}
			if err := ingestRX(data, time.Now().UTC()); err != nil {
				return nil, err
			}
		case frame, ok := <-timedCh:
			if !ok {
				break loop
			}
			if err := ingestRX(frame.Data, frame.At); err != nil {
				return nil, err
			}
		}
	}

	if err := drainTX(); err != nil {
		return nil, err
	}

	// Emit any partial data the normalizer is still holding (e.g. a final
	// line with no trailing newline).
	if f, ok := p.opts.Normalizer.(normalize.Flusher); ok {
		if err := writeRX(f.Flush()); err != nil {
			return nil, err
		}
	}

	if p.opts.Bidirectional {
		// Copy so the caller's Options.Manifest map is never mutated.
		attrs := make(map[string]string, len(run.Manifest.Attributes)+1)
		for k, v := range run.Manifest.Attributes {
			attrs[k] = v
		}
		attrs[AttrTXRecords] = strconv.FormatUint(txCount, 10)
		run.Manifest.Attributes = attrs
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
