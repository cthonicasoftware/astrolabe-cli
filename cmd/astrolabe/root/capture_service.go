package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
)

type CaptureRequest struct {
	// Source is the configured capture source. Exactly one source per request
	// is guaranteed by the type: a Spec value cannot describe two sources.
	Source sources.Spec
	// Normalizer converts source frames into records. Nil selects LineJSON.
	Normalizer normalize.Normalizer
	RunLabel   string
	Meta       core.ManifestOptions
}

type CaptureComponents struct {
	Source          captureManagedSource
	Normalizer      normalize.Normalizer
	Manifest        core.Manifest
	CaptureSettings core.CaptureSettings
}

type CaptureResult struct {
	Run         *core.Run
	Interrupted bool
}

type CaptureService interface {
	Run(ctx context.Context, req CaptureRequest) (CaptureResult, error)
	BuildComponents(req CaptureRequest) (CaptureComponents, error)
}

type captureService struct {
	storeRoot string
	out       *cliout.Printer
}

func newCaptureService(out *cliout.Printer, storeRoot string) CaptureService {
	return &captureService{
		storeRoot: storeRoot,
		out:       out,
	}
}

func (s *captureService) Run(ctx context.Context, req CaptureRequest) (CaptureResult, error) {
	components, err := s.BuildComponents(req)
	if err != nil {
		return CaptureResult{}, err
	}

	store := storage.NewFS(s.storeRoot)
	pipeline, err := capture.NewPipeline(capture.Options{
		Source:     components.Source,
		Normalizer: components.Normalizer,
		Store:      store,
		Manifest:   components.Manifest,
		Capture:    components.CaptureSettings,
	})
	if err != nil {
		return CaptureResult{}, fmt.Errorf("build pipeline: %w", err)
	}

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := components.Source.Open(signalCtx); err != nil {
		return CaptureResult{}, fmt.Errorf("open source: %w", err)
	}
	defer func() {
		if err := components.Source.Close(); err != nil && s.out != nil {
			s.out.Warning(fmt.Sprintf("Failed to close source: %v", err))
		}
	}()

	run, runErr := pipeline.Run(signalCtx)
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return CaptureResult{}, fmt.Errorf("capture pipeline: %w", runErr)
	}
	if run == nil {
		return CaptureResult{}, fmt.Errorf("capture pipeline: run not returned")
	}

	return CaptureResult{
		Run:         run,
		Interrupted: runErr != nil,
	}, nil
}

func (s *captureService) BuildComponents(req CaptureRequest) (CaptureComponents, error) {
	if req.Source == nil {
		return CaptureComponents{}, errors.New("capture request must specify a source")
	}

	source, err := req.Source.NewSource()
	if err != nil {
		return CaptureComponents{}, fmt.Errorf("create %s source: %w", req.Source.Kind(), err)
	}

	normalizer := req.Normalizer
	if normalizer == nil {
		normalizer = normalize.NewLineJSON()
	}

	attrs := req.Source.ManifestAttrs()
	if attrs == nil {
		attrs = make(map[string]string)
	}
	if req.RunLabel != "" {
		attrs["run_label"] = req.RunLabel
	}
	// Record format is a property of the normalizer, not the transport, so any
	// source paired with a format-aware normalizer gets the attribute.
	if describer, ok := normalizer.(interface{ Format() string }); ok {
		attrs["source_format"] = describer.Format()
	}

	return CaptureComponents{
		Source:          source,
		Normalizer:      normalizer,
		Manifest:        buildCaptureManifest(req.Meta, attrs),
		CaptureSettings: req.Source.CaptureSettings(),
	}, nil
}
