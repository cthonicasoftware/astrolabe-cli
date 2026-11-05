package capture

import (
	"context"
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
)

// recordingStore keeps every written record and the finalized run.
type recordingStore struct {
	records   []core.Record
	finalized *core.Run
}

func (s *recordingStore) Start(*core.Run) error { return nil }
func (s *recordingStore) Write(records []core.Record) error {
	s.records = append(s.records, records...)
	return nil
}
func (s *recordingStore) Finalize(run *core.Run) error {
	s.finalized = run
	return nil
}
func (s *recordingStore) Artifacts() []core.Artifact { return nil }

func TestPipelineRecordsTXWithoutSplittingPartialRX(t *testing.T) {
	frames := make(chan []byte)
	tx := make(chan core.Record)
	store := &recordingStore{}
	callerAttrs := map[string]string{"source_kind": "serial"}

	pipeline, err := NewPipeline(Options{
		Source:        &chanSource{meta: core.SourceMeta{Kind: "serial"}, frames: frames},
		Normalizer:    normalize.NewLineJSON(),
		Store:         store,
		Manifest:      core.Manifest{SchemaVersion: "v1alpha1", Attributes: callerAttrs},
		TX:            tx,
		Bidirectional: true,
	})
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}

	done := make(chan *core.Run, 1)
	go func() {
		run, err := pipeline.Run(context.Background())
		if err != nil {
			t.Errorf("run: %v", err)
		}
		done <- run
	}()

	// Unbuffered channels + one select loop make this order deterministic.
	frames <- []byte("par")
	tx <- core.Record{Type: RecordTypeTX, Payload: map[string]any{"line": "AT"}}
	frames <- []byte("tial\n")
	close(tx)
	frames <- []byte("next\n")
	close(frames)

	run := <-done
	if len(store.records) != 3 {
		t.Fatalf("got %d records, want 3: %+v", len(store.records), store.records)
	}

	want := []struct {
		typ, dir, line string
	}{
		{RecordTypeTX, DirectionTX, "AT"},
		{"sample", DirectionRX, "partial"},
		{"sample", DirectionRX, "next"},
	}
	for i, w := range want {
		rec := store.records[i]
		if rec.Seq != uint64(i+1) {
			t.Errorf("record %d seq = %d, want %d", i, rec.Seq, i+1)
		}
		if rec.Type != w.typ {
			t.Errorf("record %d type = %q, want %q", i, rec.Type, w.typ)
		}
		if got := rec.Envelope["direction"]; got != w.dir {
			t.Errorf("record %d direction = %v, want %q", i, got, w.dir)
		}
		if got := rec.Payload["line"]; got != w.line {
			t.Errorf("record %d line = %v, want %q", i, got, w.line)
		}
		if rec.TS.IsZero() {
			t.Errorf("record %d has zero timestamp", i)
		}
	}

	if got := run.Manifest.Attributes[AttrTXRecords]; got != "1" {
		t.Errorf("tx_records = %q, want \"1\"", got)
	}
	if run.Manifest.Attributes["source_kind"] != "serial" {
		t.Error("existing manifest attributes were dropped")
	}
	if _, leaked := callerAttrs[AttrTXRecords]; leaked {
		t.Error("pipeline mutated the caller's manifest attributes map")
	}
}

func TestPipelineUnidirectionalLeavesRecordsUntagged(t *testing.T) {
	store := &recordingStore{}
	pipeline, err := NewPipeline(Options{
		Source:     &sliceSource{meta: core.SourceMeta{Kind: "file"}, frames: [][]byte{[]byte("a\nb\n")}},
		Normalizer: normalize.NewLineJSON(),
		Store:      store,
		Manifest:   core.Manifest{SchemaVersion: "v1alpha1"},
	})
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	run, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for i, rec := range store.records {
		if rec.Envelope != nil {
			t.Errorf("record %d envelope = %v, want nil", i, rec.Envelope)
		}
	}
	if _, ok := run.Manifest.Attributes[AttrTXRecords]; ok {
		t.Error("unidirectional run should not get tx_records")
	}
}

func TestPipelineDrainsQueuedTXOnSourceClose(t *testing.T) {
	tx := make(chan core.Record, 2)
	tx <- core.Record{Type: RecordTypeTX, Payload: map[string]any{"line": "one"}}
	tx <- core.Record{Type: RecordTypeTX, Payload: map[string]any{"line": "two"}}
	frames := make(chan []byte)
	close(frames) // source gone before the loop sees the queued sends

	store := &recordingStore{}
	pipeline, err := NewPipeline(Options{
		Source:        &chanSource{meta: core.SourceMeta{Kind: "serial"}, frames: frames},
		Normalizer:    normalize.NewLineJSON(),
		Store:         store,
		Manifest:      core.Manifest{SchemaVersion: "v1alpha1"},
		TX:            tx,
		Bidirectional: true,
	})
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	run, err := pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := run.Manifest.Attributes[AttrTXRecords]; got != "2" {
		t.Errorf("tx_records = %q, want \"2\"", got)
	}
}
