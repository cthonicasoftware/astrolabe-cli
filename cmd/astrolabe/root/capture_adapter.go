package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
)

// captureAdapter implements tui.CaptureSessionPort.
type captureAdapter struct {
	out       *cliout.Printer
	cacheRoot string
	svc       CaptureService
}

// newCaptureAdapter constructs a captureAdapter.
func newCaptureAdapter(out *cliout.Printer, cacheRoot string) *captureAdapter {
	return &captureAdapter{
		out:       out,
		cacheRoot: cacheRoot,
		svc:       newCaptureService(out, cacheRoot),
	}
}

// captureSession implements tui.CaptureSession.
type captureSession struct {
	feed          chan string
	cancel        context.CancelFunc
	done          chan tui.CaptureSessionResult
	tempRoot      string
	cacheRoot     string
	saveRequested bool
	stopOnce      sync.Once
	result        tui.CaptureSessionResult
}

func (s *captureSession) Feed() <-chan string { return s.feed }
func (s *captureSession) RequestSave()        { s.saveRequested = true }

func (s *captureSession) Stop() tui.CaptureSessionResult {
	s.stopOnce.Do(func() {
		s.cancel()

		result := <-s.done
		if result.Err != nil {
			_ = os.RemoveAll(s.tempRoot)
			s.result = result
			return
		}

		if !s.saveRequested {
			_ = os.RemoveAll(s.tempRoot)
			s.result = result
			return
		}

		run := &core.Run{ID: result.RunID}
		if err := promoteRunArtifacts(run, s.tempRoot, s.cacheRoot); err != nil {
			_ = os.RemoveAll(s.tempRoot)
			s.result = tui.CaptureSessionResult{Err: fmt.Errorf("finalize run artifacts: %w", err)}
			return
		}
		_ = os.RemoveAll(s.tempRoot)
		s.result = tui.CaptureSessionResult{Saved: true, RunID: run.ID}
	})

	return s.result
}

// Start opens the source described by cfg and launches the fan-out goroutines.
// It returns immediately; data flows through Feed() until Stop finalizes it.
func (a *captureAdapter) Start(ctx context.Context, cfg tui.CaptureConfig) (tui.CaptureSession, error) {
	if err := os.MkdirAll(a.cacheRoot, 0o755); err != nil {
		return nil, fmt.Errorf("ensure offline cache: %w", err)
	}

	tempRoot, err := os.MkdirTemp(a.cacheRoot, ".tmp-run-")
	if err != nil {
		return nil, fmt.Errorf("create temp run dir: %w", err)
	}

	req, err := captureRequestFromConfig(cfg)
	if err != nil {
		_ = os.RemoveAll(tempRoot)
		return nil, err
	}

	// Load the operator's saved metadata so TUI-captured runs carry the same
	// operator/device/test context the CLI capture paths apply. Without this the
	// manifest is built from zero values and every run reports "unspecified".
	req.Meta = savedMetadataOptions(a.out)

	components, err := a.svc.BuildComponents(req)
	if err != nil {
		_ = os.RemoveAll(tempRoot)
		return nil, err
	}

	sessionCtx, cancel := context.WithCancel(ctx)

	if err := components.Source.Open(sessionCtx); err != nil {
		cancel()
		_ = os.RemoveAll(tempRoot)
		return nil, fmt.Errorf("open source: %w", err)
	}

	feedCh := make(chan string, 16)
	pipelineFramesCh := make(chan []byte, 16)

	pipelineOpts := capture.Options{
		Source: &frameChannelSource{
			framesCh: pipelineFramesCh,
			meta:     components.Source.Meta(),
		},
		Normalizer: normalize.NewLineJSON(),
		Store:      storage.NewFS(tempRoot),
		Manifest:   components.Manifest,
		Capture:    components.CaptureSettings,
	}

	pipeline, err := capture.NewPipeline(pipelineOpts)
	if err != nil {
		cancel()
		_ = components.Source.Close()
		_ = os.RemoveAll(tempRoot)
		return nil, fmt.Errorf("build pipeline: %w", err)
	}

	sess := &captureSession{
		feed:      feedCh,
		cancel:    cancel,
		done:      make(chan tui.CaptureSessionResult, 1),
		tempRoot:  tempRoot,
		cacheRoot: a.cacheRoot,
	}

	// Fan-out goroutine: forward source frames to both the TUI feed and the
	// pipeline input channel.
	go func() {
		defer close(feedCh)
		defer close(pipelineFramesCh)
		defer func() {
			if err := components.Source.Close(); err != nil {
				a.out.Warning(fmt.Sprintf("Failed to close source: %v", err))
			}
		}()
		for frame := range components.Source.Frames() {
			if len(frame) == 0 {
				continue
			}
			select {
			case feedCh <- string(frame):
			case <-sessionCtx.Done():
				return
			}
			select {
			case pipelineFramesCh <- frame:
			case <-sessionCtx.Done():
				return
			}
		}
	}()

	// Pipeline goroutine: runs until the pipeline finishes, then sends the
	// result on sess.done.
	go func() {
		run, runErr := pipeline.Run(sessionCtx)
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			sess.done <- tui.CaptureSessionResult{Err: fmt.Errorf("capture pipeline: %w", runErr)}
			return
		}
		if run == nil {
			sess.done <- tui.CaptureSessionResult{Err: fmt.Errorf("capture pipeline: run not returned")}
			return
		}
		sess.done <- tui.CaptureSessionResult{RunID: run.ID}
	}()

	return sess, nil
}

// savedMetadataOptions loads the operator's persisted metadata and turns it into
// manifest options for a TUI capture. Load failures degrade to empty options
// (yielding an "unspecified" manifest) rather than aborting the capture.
func savedMetadataOptions(out *cliout.Printer) core.ManifestOptions {
	savedMetadata, err := config.LoadMetadata()
	if err != nil && out != nil {
		out.Warning(fmt.Sprintf("Failed to load saved metadata: %v", err))
	}
	return buildManifestOptions(captureMetadataInput{}, savedMetadata, nil, err == nil)
}

func captureRequestFromConfig(cfg tui.CaptureConfig) (CaptureRequest, error) {
	switch cfg.SourceType {
	case tui.SourceTypeSerial:
		if cfg.SerialConfig == nil {
			return CaptureRequest{}, fmt.Errorf("serial config missing")
		}
		return CaptureRequest{
			SerialConfig: cfg.SerialConfig,
		}, nil

	case tui.SourceTypeTCP:
		portNum, err := strconv.Atoi(cfg.TCPPort)
		if err != nil {
			return CaptureRequest{}, fmt.Errorf("invalid TCP port %q: %w", cfg.TCPPort, err)
		}
		tcpCfg := &sources.TCPConfig{
			Host: cfg.TCPHost,
			Port: portNum,
		}
		return CaptureRequest{
			TCPConfig: tcpCfg,
		}, nil

	default:
		return CaptureRequest{}, fmt.Errorf("unsupported source type: %q", cfg.SourceType)
	}
}
