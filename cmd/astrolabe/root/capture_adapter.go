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
}

// newCaptureAdapter constructs a captureAdapter.
func newCaptureAdapter(out *cliout.Printer, cacheRoot string) *captureAdapter {
	return &captureAdapter{out: out, cacheRoot: cacheRoot}
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

	// Build the managed source from the capture configuration.
	src, manifest, captureSettings, err := a.buildSource(cfg)
	if err != nil {
		_ = os.RemoveAll(tempRoot)
		return nil, err
	}

	sessionCtx, cancel := context.WithCancel(ctx)

	if err := src.Open(sessionCtx); err != nil {
		cancel()
		_ = os.RemoveAll(tempRoot)
		return nil, fmt.Errorf("open source: %w", err)
	}

	feedCh := make(chan string, 16)
	pipelineFramesCh := make(chan []byte, 16)

	pipelineOpts := capture.Options{
		Source: &frameChannelSource{
			framesCh: pipelineFramesCh,
			meta:     src.Meta(),
		},
		Normalizer: normalize.NewLineJSON(),
		Store:      storage.NewFS(tempRoot),
		Manifest:   manifest,
		Capture:    captureSettings,
	}

	pipeline, err := capture.NewPipeline(pipelineOpts)
	if err != nil {
		cancel()
		_ = src.Close()
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
			if err := src.Close(); err != nil {
				a.out.Warning(fmt.Sprintf("Failed to close source: %v", err))
			}
		}()
		for frame := range src.Frames() {
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

// buildSource constructs the concrete capture source, manifest, and settings
// from the TUI CaptureConfig.
func (a *captureAdapter) buildSource(cfg tui.CaptureConfig) (captureManagedSource, core.Manifest, core.CaptureSettings, error) {
	switch cfg.SourceType {
	case tui.SourceTypeSerial:
		if cfg.SerialConfig == nil {
			return nil, core.Manifest{}, core.CaptureSettings{}, fmt.Errorf("serial config missing")
		}
		src := sources.NewSerialWithConfig(*cfg.SerialConfig)
		manifest := buildSerialManifest(*cfg.SerialConfig, "", core.ManifestOptions{})
		settings := buildSerialCaptureSettings(*cfg.SerialConfig)
		return src, manifest, settings, nil

	case tui.SourceTypeTCP:
		portNum, err := strconv.Atoi(cfg.TCPPort)
		if err != nil {
			return nil, core.Manifest{}, core.CaptureSettings{}, fmt.Errorf("invalid TCP port %q: %w", cfg.TCPPort, err)
		}
		tcpCfg := sources.TCPConfig{
			Host: cfg.TCPHost,
			Port: portNum,
		}
		src, err := sources.NewTCPWithConfig(tcpCfg)
		if err != nil {
			return nil, core.Manifest{}, core.CaptureSettings{}, fmt.Errorf("create TCP source: %w", err)
		}
		manifest := buildTCPManifest(tcpCfg, "", core.ManifestOptions{})
		settings := buildTCPCaptureSettings(tcpCfg)
		return src, manifest, settings, nil

	default:
		return nil, core.Manifest{}, core.CaptureSettings{}, fmt.Errorf("unsupported source type: %q", cfg.SourceType)
	}
}
