# Adding a Capture Source

This guide covers adding a new capture source (RTT, SCPI, CAN, ...) to the CLI.

## The contract

A capture source is any type that implements `sources.Spec`
(`internal/sources/spec.go`):

```go
type Spec interface {
	Kind() string                            // "serial", "tcp", "file", ...
	NewSource() (ManagedSource, error)       // construct + validate
	ManifestAttrs() map[string]string        // provenance, must include source_kind
	CaptureSettings() core.CaptureSettings   // channels + human-readable notes
}
```

`CaptureRequest.Source` holds a single `Spec`, so "exactly one source per
capture" is enforced by the type system. There is no dispatch table and no
per-source branch in the capture service — adding a source does not require
editing `capture_service.go`.

## Steps

### 1. Write the source: `internal/sources/<kind>.go`

Define a config struct, a `DefaultXConfig()`, and the source type implementing
`ManagedSource` (`Open`, `Frames`, `Meta`, `Close`). Follow `tcp.go` — it is the
smallest complete example.

`Meta()` returns `core.SourceMeta{Kind: "<kind>", ...}`. `Kind` must match
`Spec.Kind()` and the `source_kind` manifest attribute.

### 2. Implement `Spec` on the config type

Put the four methods in the same file, on the **config** type (value receiver),
not the source type. See the bottom of `tcp.go`.

`ManifestAttrs()` must return a freshly allocated map on every call — the
capture service mutates it to add `run_label` and `source_format`.

### 3. Register the compile-time assertion

Add one line to the `var _ Spec = ...` block in `internal/sources/spec.go`, and
one entry to the table in `internal/sources/spec_test.go`. The conformance test
then covers your source automatically.

### 4. Add the CLI subcommand: `cmd/astrolabe/root/capture_<kind>.go`

Copy `capture_tcp.go`. Bind flags to your config struct, then:

```go
result, err := svc.Run(cmd.Context(), CaptureRequest{
	Source:   cfg,          // your Spec
	RunLabel: flags.name,
	Meta:     metaOpts,
})
```

Register it in `capture.go`'s `AddCommand` block.

### 5. Pick a normalizer (optional)

`CaptureRequest.Normalizer` defaults to `normalize.NewLineJSON()`. Set it when
the source carries CSV or opaque bytes. Format is a property of the normalizer,
not the transport: any normalizer exposing `Format() string` gets recorded as
the `source_format` manifest attribute automatically.

### 6. Manifest validation

Add a `case "<kind>":` to the source-kind switch in
`internal/validate/validate.go` if your kind has required `SourceMeta` fields.
This is validating stored manifests, so it is keyed by string, not by `Spec`.

### 7. TUI tab (optional)

`internal/tui/capture_tabs.go` — add a tab index, field constants, edit-mode
handling, and a `buildCaptureConfig` case, then map it to your `Spec` in
`captureRequestFromConfig` (`cmd/astrolabe/root/capture_adapter.go`).

The TUI and cobra layers still need hand-written per-source code because each
source has genuinely different parameters to present. That is the irreducible
part; everything below it is not.

## Checklist

- [ ] `internal/sources/<kind>.go` — source + `Spec` impl
- [ ] `internal/sources/spec.go` — compile-time assertion
- [ ] `internal/sources/spec_test.go` — conformance table entry
- [ ] `cmd/astrolabe/root/capture_<kind>.go` — subcommand
- [ ] `cmd/astrolabe/root/capture.go` — `AddCommand`
- [ ] `internal/validate/validate.go` — required-field case, if any
- [ ] TUI tab, if the source should be operator-selectable
- [ ] `docs/` — an operator-facing guide for the source
