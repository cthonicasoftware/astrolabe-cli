package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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
	// Reconnect, when non-nil, wraps serial and TCP sources so a dropped link
	// is retried instead of ending the run. Nil leaves the source unwrapped.
	Reconnect *sources.ReconnectPolicy
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
	lastLink := s.trackLinkState(&req)
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
		// Headless runs never send, but tag RX records from serial/TCP so
		// the record schema is the same as interactive runs.
		Bidirectional: sources.IsWritable(components.Source),
		Reconnecting:  req.Reconnect != nil && sources.SupportsReconnect(req.Source),
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

	interrupted := runErr != nil
	if !interrupted && s.out != nil && req.Reconnect != nil && req.Reconnect.Timeout > 0 &&
		lastLink() == core.LinkDown {
		s.out.Warning(fmt.Sprintf("Reconnect timed out after %s; ending capture", req.Reconnect.Timeout))
	}

	return CaptureResult{
		Run:         run,
		Interrupted: interrupted,
	}, nil
}

// trackLinkState wraps the request's reconnect OnEvent (on a copy of the
// policy) and returns a func reporting the most recent link state.
func (s *captureService) trackLinkState(req *CaptureRequest) func() core.LinkState {
	var mu sync.Mutex
	var last core.LinkState
	if req.Reconnect != nil {
		policy := *req.Reconnect
		inner := policy.OnEvent
		policy.OnEvent = func(ev core.LinkEvent) {
			mu.Lock()
			last = ev.State
			mu.Unlock()
			if inner != nil {
				inner(ev)
			}
		}
		req.Reconnect = &policy
	}
	return func() core.LinkState {
		mu.Lock()
		defer mu.Unlock()
		return last
	}
}

func (s *captureService) BuildComponents(req CaptureRequest) (CaptureComponents, error) {
	if req.Source == nil {
		return CaptureComponents{}, errors.New("capture request must specify a source")
	}

	var source captureManagedSource
	if req.Reconnect != nil && sources.SupportsReconnect(req.Source) {
		source = sources.NewReconnecting(req.Source, *req.Reconnect, true)
	} else {
		built, err := req.Source.NewSource()
		if err != nil {
			return CaptureComponents{}, fmt.Errorf("create %s source: %w", req.Source.Kind(), err)
		}
		source = built
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

// reconnectPolicyFromFlags returns nil when reconnect is off. Initial and Max
// stay zero so the wrapper applies its defaults.
func reconnectPolicyFromFlags(enabled bool, timeout time.Duration) *sources.ReconnectPolicy {
	if !enabled {
		return nil
	}
	return &sources.ReconnectPolicy{Timeout: timeout}
}

// reconnectPolicyWithOutput is reconnectPolicyFromFlags plus link-event
// reporting through out.
func reconnectPolicyWithOutput(out *cliout.Printer, enabled bool, timeout time.Duration) *sources.ReconnectPolicy {
	policy := reconnectPolicyFromFlags(enabled, timeout)
	if policy == nil || out == nil {
		return policy
	}
	policy.OnEvent = func(ev core.LinkEvent) {
		switch ev.State {
		case core.LinkDown:
			out.Warning(fmt.Sprintf("Link down: %s; reconnecting", ev.Err))
		case core.LinkUp:
			out.Step(fmt.Sprintf("Link up after %s", ev.Gap.Round(100*time.Millisecond)))
		}
	}
	return policy
}
