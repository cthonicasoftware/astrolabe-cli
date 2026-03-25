package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
)

type CaptureRequest struct {
	SerialConfig *sources.Config
	TCPConfig    *sources.TCPConfig
	RunLabel     string
	Meta         core.ManifestOptions
}

type CaptureComponents struct {
	Source          captureManagedSource
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
		Normalizer: normalize.NewLineJSON(),
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
	switch {
	case req.SerialConfig != nil && req.TCPConfig != nil:
		return CaptureComponents{}, fmt.Errorf("capture request must specify exactly one source")
	case req.SerialConfig == nil && req.TCPConfig == nil:
		return CaptureComponents{}, fmt.Errorf("capture request must specify a source")
	case req.SerialConfig != nil:
		cfg := *req.SerialConfig
		return CaptureComponents{
			Source:          sources.NewSerialWithConfig(cfg),
			Manifest:        s.buildSerialManifest(cfg, req.RunLabel, req.Meta),
			CaptureSettings: s.buildSerialCaptureSettings(cfg),
		}, nil
	case req.TCPConfig != nil:
		cfg := *req.TCPConfig
		source, err := sources.NewTCPWithConfig(cfg)
		if err != nil {
			return CaptureComponents{}, fmt.Errorf("create TCP source: %w", err)
		}
		return CaptureComponents{
			Source:          source,
			Manifest:        s.buildTCPManifest(cfg, req.RunLabel, req.Meta),
			CaptureSettings: s.buildTCPCaptureSettings(cfg),
		}, nil
	default:
		return CaptureComponents{}, fmt.Errorf("unsupported capture request")
	}
}

func (s *captureService) buildSerialManifest(cfg sources.Config, name string, opts core.ManifestOptions) core.Manifest {
	attrs := map[string]string{
		"source_kind": "serial",
		"port":        cfg.Port,
		"baud":        strconv.Itoa(cfg.Baud),
	}
	if name != "" {
		attrs["run_label"] = name
	}
	return buildCaptureManifest(opts, attrs)
}

func (s *captureService) buildSerialCaptureSettings(cfg sources.Config) core.CaptureSettings {
	return core.CaptureSettings{
		Channels: []string{"serial"},
		Notes:    fmt.Sprintf("serial capture from %s @ %d baud", cfg.Port, cfg.Baud),
	}
}

func (s *captureService) buildTCPManifest(cfg sources.TCPConfig, name string, opts core.ManifestOptions) core.Manifest {
	attrs := map[string]string{
		"source_kind":     "tcp",
		"host":            cfg.Host,
		"port":            strconv.Itoa(cfg.Port),
		"connect_timeout": cfg.ConnectTimeout.String(),
		"read_timeout":    cfg.ReadTimeout.String(),
		"buffer_size":     strconv.Itoa(cfg.BufferSize),
	}
	if name != "" {
		attrs["run_label"] = name
	}
	return buildCaptureManifest(opts, attrs)
}

func (s *captureService) buildTCPCaptureSettings(cfg sources.TCPConfig) core.CaptureSettings {
	return core.CaptureSettings{
		Channels: []string{"tcp"},
		Notes:    fmt.Sprintf("tcp capture from %s:%d", cfg.Host, cfg.Port),
	}
}
