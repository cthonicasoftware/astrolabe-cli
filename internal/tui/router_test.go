package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
)

type stubModel struct{}

func (stubModel) Init() tea.Cmd                          { return nil }
func (m stubModel) Update(msg tea.Msg) (Screen, tea.Cmd) { return m, nil }
func (stubModel) View() string                           { return "" }

type stubCapturePort struct {
	session CaptureSession
	err     error
}

func (s stubCapturePort) Start(ctx context.Context, cfg CaptureConfig) (CaptureSession, error) {
	return s.session, s.err
}

type stubCaptureSession struct {
	feed      chan FeedChunk
	reconnect *[]bool // records SetReconnect calls
}

func (s stubCaptureSession) Feed() <-chan FeedChunk     { return s.feed }
func (s stubCaptureSession) Stop() CaptureSessionResult { return CaptureSessionResult{} }
func (s stubCaptureSession) RequestSave()               {}
func (s stubCaptureSession) CanSend() bool              { return false }
func (s stubCaptureSession) Send(string, sources.LineEnding) (time.Time, error) {
	return time.Time{}, nil
}
func (s stubCaptureSession) SendBytes([]byte) (time.Time, error) { return time.Time{}, nil }
func (s stubCaptureSession) SetReconnect(on bool) {
	if s.reconnect != nil {
		*s.reconnect = append(*s.reconnect, on)
	}
}

// cfgCapturePort records the config Start was given.
type cfgCapturePort struct {
	session CaptureSession
	got     *CaptureConfig
}

func (p cfgCapturePort) Start(_ context.Context, cfg CaptureConfig) (CaptureSession, error) {
	*p.got = cfg
	return p.session, nil
}

type stubPrefsPort struct{ prefs LiveViewPrefs }

func (p stubPrefsPort) Load() (LiveViewPrefs, error) { return p.prefs, nil }
func (p stubPrefsPort) Save(LiveViewPrefs) error     { return nil }

type stubMetadataPort struct {
	values  MetadataValues
	loadErr error
	path    string
	saveErr error
}

func (s *stubMetadataPort) Load() (MetadataValues, error) { return s.values, s.loadErr }
func (s *stubMetadataPort) Save(v MetadataValues) (string, error) {
	s.values = v
	return s.path, s.saveErr
}
func (s *stubMetadataPort) Path() (string, error) { return s.path, nil }

type stubUploadPort struct {
	err error
}

func (s stubUploadPort) UploadRun(ctx context.Context, runID string) error { return s.err }

func TestRouterCtrlCTriggersCleanup(t *testing.T) {
	cleaned := false
	r := &router{
		factories: map[ScreenID]ScreenFactory{
			ScreenWelcome: func(ctx ScreenContext) (Screen, func(), error) {
				return stubModel{}, func() { cleaned = true }, nil
			},
		},
		current: stubModel{},
		cleanup: func() { cleaned = true },
	}

	model, cmd := r.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if model != r {
		t.Fatalf("Update() returned unexpected model")
	}
	if cmd == nil {
		t.Fatalf("Update() cmd = nil, want quit command")
	}
	if !cleaned {
		t.Fatalf("cleanup was not called before quit")
	}
}

func TestBuildFactoriesCaptureLiveRequiresPort(t *testing.T) {
	factory := buildFactories(RouterConfig{})[ScreenCaptureLive]
	_, _, err := factory(ScreenContext{Args: CaptureConfig{SourceType: SourceTypeTCP}})
	if err == nil || err.Error() != "capture not configured" {
		t.Fatalf("expected capture not configured error, got %v", err)
	}
}

func TestBuildFactoriesCaptureLivePropagatesStartError(t *testing.T) {
	factory := buildFactories(RouterConfig{
		CapturePort: stubCapturePort{err: errors.New("boom")},
	})[ScreenCaptureLive]
	_, _, err := factory(ScreenContext{
		Args: CaptureConfig{SourceType: SourceTypeTCP, TCPHost: "127.0.0.1", TCPPort: "9000"},
	})
	if err == nil || err.Error() != "start capture: boom" {
		t.Fatalf("expected wrapped start error, got %v", err)
	}
}

func TestBuildFactoriesCaptureLiveAppliesReconnectPref(t *testing.T) {
	prefs := DefaultLiveViewPrefs()
	prefs.Reconnect = false
	var calls []bool
	var got CaptureConfig
	factory := buildFactories(RouterConfig{
		CapturePort:       cfgCapturePort{session: stubCaptureSession{feed: make(chan FeedChunk), reconnect: &calls}, got: &got},
		LiveViewPrefsPort: stubPrefsPort{prefs: prefs},
	})[ScreenCaptureLive]
	model, _, err := factory(ScreenContext{
		Args: CaptureConfig{SourceType: SourceTypeTCP, TCPHost: "127.0.0.1", TCPPort: "9000", Reconnect: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconnect {
		t.Error("Start got Reconnect on, want the saved pref (off)")
	}
	model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if len(calls) != 1 || !calls[0] {
		t.Errorf("SetReconnect calls = %v, want [true]", calls)
	}
}

func TestBuildFactoriesMetadataReturnsModelOnLoadError(t *testing.T) {
	factory := buildFactories(RouterConfig{
		MetadataPort: &stubMetadataPort{
			loadErr: errors.New("load failed"),
			path:    "/tmp/meta.json",
		},
	})[ScreenMetadata]
	model, cleanup, err := factory(ScreenContext{})
	if err != nil {
		t.Fatalf("factory returned unexpected error: %v", err)
	}
	if cleanup != nil {
		t.Fatalf("metadata factory should not return cleanup")
	}
	metaModel, ok := model.(*metadataModel)
	if !ok {
		t.Fatalf("expected *metadataModel, got %T", model)
	}
	if metaModel.loadErr == nil || metaModel.loadErr.Error() != "load failed" {
		t.Fatalf("expected loadErr to be preserved, got %v", metaModel.loadErr)
	}
}

func TestBuildFactoriesUploadRequiresPort(t *testing.T) {
	factory := buildFactories(RouterConfig{RunsCacheRoot: t.TempDir()})[ScreenUpload]
	_, _, err := factory(ScreenContext{})
	if err == nil || err.Error() != "upload not configured" {
		t.Fatalf("expected upload not configured error, got %v", err)
	}
}

func TestBuildFactoriesUploadFailsWhenNoPendingRuns(t *testing.T) {
	factory := buildFactories(RouterConfig{
		UploadPort:    stubUploadPort{},
		RunsCacheRoot: t.TempDir(),
	})[ScreenUpload]
	_, _, err := factory(ScreenContext{})
	if err == nil || err.Error() != "no pending runs: all runs have been uploaded" {
		t.Fatalf("expected no pending runs error, got %v", err)
	}
}

func TestBuildFactoriesUploadBuildsModelWithPendingRuns(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, "run-123")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "manifest.json"), []byte(`{"run_id":"run-123","schema":"v1alpha1"}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	factory := buildFactories(RouterConfig{
		UploadPort:    stubUploadPort{},
		RunsCacheRoot: root,
	})[ScreenUpload]
	model, cleanup, err := factory(ScreenContext{})
	if err != nil {
		t.Fatalf("factory returned unexpected error: %v", err)
	}
	if cleanup != nil {
		t.Fatalf("upload factory should not return cleanup")
	}
	uploadModel, ok := model.(uploadModel)
	if !ok {
		t.Fatalf("expected uploadModel, got %T", model)
	}
	if len(uploadModel.runIDs) != 1 || uploadModel.runIDs[0] != "run-123" {
		t.Fatalf("unexpected pending runs: %#v", uploadModel.runIDs)
	}
}

func TestBuildFactoriesRunsViewerHandlesEmptyRoot(t *testing.T) {
	factory := buildFactories(RouterConfig{RunsCacheRoot: t.TempDir()})[ScreenRuns]
	model, cleanup, err := factory(ScreenContext{})
	if err != nil {
		t.Fatalf("factory returned unexpected error: %v", err)
	}
	if cleanup != nil {
		t.Fatalf("runs factory should not return cleanup")
	}
	if _, ok := model.(*runsViewModel); !ok {
		t.Fatalf("expected runsViewerModel, got %T", model)
	}
}

func TestMetadataModelSaveUsesPortValues(t *testing.T) {
	port := &stubMetadataPort{path: "/tmp/meta.json"}
	model := NewMetadataEditor(port, MetadataValues{
		Device: core.DeviceInfo{},
		Test:   core.TestInfo{},
	}, "/tmp/meta.json", nil).(*metadataModel)

	model.inputs[fieldOperator].SetValue("alice")
	model.inputs[fieldTestPlan].SetValue("")
	model.attributes.SetValue("site=bench")

	if err := model.save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if port.values.Operator != "alice" {
		t.Fatalf("expected saved operator, got %q", port.values.Operator)
	}
	if port.values.Test.Plan != "unspecified" {
		t.Fatalf("expected default plan, got %q", port.values.Test.Plan)
	}
	if got := port.values.Attributes["site"]; got != "bench" {
		t.Fatalf("expected attribute to round-trip, got %q", got)
	}
}
