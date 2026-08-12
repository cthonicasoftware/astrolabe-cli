package sources

import (
	"context"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// ManagedSource is a capture source whose lifetime the caller controls: Open
// starts the underlying stream, Frames delivers it, and Close releases it.
// It matches capture.Source structurally, with lifecycle added.
type ManagedSource interface {
	Frames() <-chan []byte
	Meta() core.SourceMeta
	Open(context.Context) error
	Close() error
}

// Spec describes a configured capture source completely enough for the capture
// layer to run it without knowing which kind of source it is. Each source
// config type (Config, TCPConfig, FileConfig, ...) implements Spec in its own
// file, so adding a source requires no changes to the capture plumbing.
type Spec interface {
	// Kind is the stable source identifier recorded in manifests ("serial",
	// "tcp", "file"). It must match the "source_kind" manifest attribute.
	Kind() string

	// NewSource constructs the runnable source, validating configuration and
	// returning an error if the config cannot produce a usable source.
	NewSource() (ManagedSource, error)

	// ManifestAttrs returns provenance attributes describing this source, to be
	// merged into the run manifest. It must include "source_kind".
	ManifestAttrs() map[string]string

	// CaptureSettings returns the capture-level settings for this source.
	CaptureSettings() core.CaptureSettings
}

// Compile-time proof that every source config implements Spec. Add new sources
// here so a missing method fails the build rather than a call site.
var (
	_ Spec = Config{}
	_ Spec = TCPConfig{}
	_ Spec = FileConfig{}
)
