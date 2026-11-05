package capture

import (
	"context"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
)

func runLinkPipeline(t *testing.T, opts Options, frames []core.Frame) *recordingStore {
	t.Helper()
	store := &recordingStore{}
	opts.Source = &timedSource{frames: frames}
	opts.Normalizer = normalize.NewLineJSON()
	opts.Store = store
	opts.Manifest = core.Manifest{SchemaVersion: "v1alpha1"}
	p, err := NewPipeline(opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	return store
}

func linkFrames(t0 time.Time) []core.Frame {
	return []core.Frame{
		{Data: []byte("abc\nde"), At: t0},
		{At: t0.Add(time.Second), Link: &core.LinkEvent{State: core.LinkDown, Err: "read: EOF"}},
		{Data: []byte("f\n"), At: t0.Add(2 * time.Second)},
		{At: t0.Add(3 * time.Second), Link: &core.LinkEvent{State: core.LinkUp, Attempts: 2, Gap: 1500 * time.Millisecond}},
	}
}

func TestPipeline_LinkDownFlushesPartialLineAsTruncated(t *testing.T) {
	store := runLinkPipeline(t, Options{}, linkFrames(time.Now().UTC()))
	r := store.records
	if len(r) != 5 {
		t.Fatalf("got %d records, want 5: %+v", len(r), r)
	}
	wantTypes := []string{"sample", "sample", RecordTypeLinkDown, "sample", RecordTypeLinkUp}
	for i, w := range wantTypes {
		if r[i].Type != w {
			t.Errorf("record %d type = %q, want %q", i, r[i].Type, w)
		}
	}
	if r[0].Payload["line"] != "abc" {
		t.Errorf("rec0 = %v", r[0].Payload)
	}
	if r[0].Envelope[EnvelopeTruncated] != nil {
		t.Errorf("rec0 should not be truncated")
	}
	if r[1].Payload["line"] != "de" || r[1].Envelope[EnvelopeTruncated] != true {
		t.Errorf("rec1 = %v / %v", r[1].Payload, r[1].Envelope)
	}
	if r[2].Payload["error"] != "read: EOF" {
		t.Errorf("link_down payload = %v", r[2].Payload)
	}
	if _, ok := r[2].Envelope["direction"]; ok {
		t.Errorf("link_down has direction")
	}
	if r[3].Payload["line"] != "f" {
		t.Errorf("rec3 = %v, want f", r[3].Payload)
	}
	if r[4].Payload["attempts"] != 2 || r[4].Payload["gap_ms"] != int64(1500) {
		t.Errorf("link_up payload = %#v", r[4].Payload)
	}
	if _, ok := r[4].Payload["port"]; ok {
		t.Errorf("link_up has port")
	}
}

func TestPipeline_LinkRecordTimestampsAreEventTimes(t *testing.T) {
	t0 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	store := runLinkPipeline(t, Options{}, linkFrames(t0))
	if !store.records[2].TS.Equal(t0.Add(time.Second)) {
		t.Errorf("link_down ts = %v", store.records[2].TS)
	}
	if !store.records[4].TS.Equal(t0.Add(3 * time.Second)) {
		t.Errorf("link_up ts = %v", store.records[4].TS)
	}
}

func TestPipeline_ReconnectsAttribute(t *testing.T) {
	t0 := time.Now().UTC()
	down := core.Frame{At: t0, Link: &core.LinkEvent{State: core.LinkDown}}
	up := core.Frame{At: t0, Link: &core.LinkEvent{State: core.LinkUp, Attempts: 1}}

	s := runLinkPipeline(t, Options{Reconnecting: true}, []core.Frame{down, up, down, up})
	if got := s.finalized.Manifest.Attributes[AttrReconnects]; got != "2" {
		t.Errorf("reconnects = %q, want 2", got)
	}
	s = runLinkPipeline(t, Options{Reconnecting: true}, nil)
	if got := s.finalized.Manifest.Attributes[AttrReconnects]; got != "0" {
		t.Errorf("reconnects = %q, want 0", got)
	}
	s = runLinkPipeline(t, Options{}, nil)
	if _, ok := s.finalized.Manifest.Attributes[AttrReconnects]; ok {
		t.Errorf("reconnects present without Reconnecting")
	}
}

func TestPipeline_UnmatchedLinkDownAtEnd(t *testing.T) {
	t0 := time.Now().UTC()
	store := runLinkPipeline(t, Options{Reconnecting: true}, []core.Frame{
		{Data: []byte("x\n"), At: t0},
		{At: t0, Link: &core.LinkEvent{State: core.LinkDown, Err: "gone"}},
	})
	last := store.records[len(store.records)-1]
	if last.Type != RecordTypeLinkDown {
		t.Errorf("last = %q", last.Type)
	}
	if store.finalized == nil {
		t.Error("not finalized")
	}
	if got := store.finalized.Manifest.Attributes[AttrReconnects]; got != "0" {
		t.Errorf("reconnects = %q, want 0", got)
	}
}

func TestPipeline_LinkUpIncludesPortWhenSet(t *testing.T) {
	store := runLinkPipeline(t, Options{}, []core.Frame{
		{At: time.Now().UTC(), Link: &core.LinkEvent{State: core.LinkUp, Port: "/dev/ttyACM1"}},
	})
	if got := store.records[0].Payload["port"]; got != "/dev/ttyACM1" {
		t.Errorf("port = %v", got)
	}
}

func TestPipeline_LinkBidirectionalDirections(t *testing.T) {
	store := runLinkPipeline(t, Options{Bidirectional: true}, linkFrames(time.Now().UTC()))
	r := store.records
	if r[1].Envelope["direction"] != DirectionRX || r[1].Envelope[EnvelopeTruncated] != true {
		t.Errorf("truncated env = %v", r[1].Envelope)
	}
	for _, i := range []int{2, 4} {
		if _, ok := r[i].Envelope["direction"]; ok {
			t.Errorf("record %d (%s) has direction", i, r[i].Type)
		}
	}
}
