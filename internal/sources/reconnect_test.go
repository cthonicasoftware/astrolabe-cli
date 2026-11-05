package sources

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// fakeSource is a ManagedSource whose frames channel the test closes to
// simulate a drop.
type fakeSource struct {
	frames  chan []byte
	openErr error
	err     error // returned by Err()

	mu       sync.Mutex
	writes   [][]byte
	closes   int
	dropOnce sync.Once
}

func newFakeSource() *fakeSource { return &fakeSource{frames: make(chan []byte, 8)} }

func (f *fakeSource) Frames() <-chan []byte { return f.frames }
func (f *fakeSource) Meta() core.SourceMeta {
	return core.SourceMeta{Kind: "serial", Port: "COM_FAKE"}
}
func (f *fakeSource) Open(context.Context) error { return f.openErr }
func (f *fakeSource) Close() error {
	f.mu.Lock()
	f.closes++
	f.mu.Unlock()
	return nil
}
func (f *fakeSource) Err() error { return f.err }
func (f *fakeSource) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, append([]byte(nil), p...))
	return len(p), nil
}
func (f *fakeSource) drop() { f.dropOnce.Do(func() { close(f.frames) }) }
func (f *fakeSource) closeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closes
}
func (f *fakeSource) written() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.writes...)
}

// fakeStep is one scripted NewSource result: an error, or a source.
type fakeStep struct {
	src *fakeSource
	err error
}

// fakeSpec hands out scripted sources; once the script runs out it fails.
type fakeSpec struct {
	kind string

	mu    sync.Mutex
	steps []fakeStep
	calls int
}

func (s *fakeSpec) Kind() string { return s.kind }
func (s *fakeSpec) NewSource() (ManagedSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if len(s.steps) == 0 {
		return nil, errors.New("no more scripted sources")
	}
	st := s.steps[0]
	s.steps = s.steps[1:]
	if st.err != nil {
		return nil, st.err
	}
	return st.src, nil
}
func (s *fakeSpec) ManifestAttrs() map[string]string      { return map[string]string{"source_kind": s.kind} }
func (s *fakeSpec) CaptureSettings() core.CaptureSettings { return core.CaptureSettings{} }
func (s *fakeSpec) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func fastPolicy() ReconnectPolicy {
	return ReconnectPolicy{Initial: time.Millisecond, Max: 4 * time.Millisecond}
}

func recv(t *testing.T, ch <-chan core.Frame) core.Frame {
	t.Helper()
	select {
	case f, ok := <-ch:
		if !ok {
			t.Fatal("channel closed, wanted a frame")
		}
		return f
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a frame")
	}
	return core.Frame{}
}

func expectClosed(t *testing.T, ch <-chan core.Frame, within time.Duration) {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case f, ok := <-ch:
			if !ok {
				return
			}
			t.Fatalf("got frame %+v (link %+v), wanted channel close", f, f.Link)
		case <-deadline:
			t.Fatalf("channel not closed within %s", within)
		}
	}
}

func expectData(t *testing.T, f core.Frame, want string) {
	t.Helper()
	if f.Link != nil || string(f.Data) != want {
		t.Fatalf("got frame data=%q link=%+v, want data %q", f.Data, f.Link, want)
	}
	if f.At.IsZero() || f.At.Location() != time.UTC {
		t.Errorf("frame At = %v, want non-zero UTC", f.At)
	}
}

func expectLink(t *testing.T, f core.Frame, state core.LinkState) *core.LinkEvent {
	t.Helper()
	if f.Link == nil || f.Link.State != state {
		t.Fatalf("got frame data=%q link=%+v, want link %s", f.Data, f.Link, state)
	}
	return f.Link
}

func TestReconnecting_ReopensAndOrdersEvents(t *testing.T) {
	first, second := newFakeSource(), newFakeSource()
	first.err = errors.New("device disconnected")
	badOpen := newFakeSource()
	badOpen.openErr = errors.New("busy")
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{
		{src: first},
		{err: errors.New("no such port")},
		{src: badOpen},
		{src: second},
	}}
	r := NewReconnecting(spec, fastPolicy(), true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()
	if got := r.Meta(); got.Port != "COM_FAKE" {
		t.Errorf("Meta = %+v", got)
	}
	out := r.TimedFrames()

	// "b" is waiting before second is even opened: the wrapper must still
	// emit Up before forwarding it.
	second.frames <- []byte("b")
	first.frames <- []byte("a")
	expectData(t, recv(t, out), "a")
	first.drop()
	down := expectLink(t, recv(t, out), core.LinkDown)
	if down.Err != "device disconnected" {
		t.Errorf("Down.Err = %q", down.Err)
	}
	up := expectLink(t, recv(t, out), core.LinkUp)
	if up.Attempts != 3 {
		t.Errorf("Up.Attempts = %d, want 3", up.Attempts)
	}
	if up.Gap <= 0 {
		t.Errorf("Up.Gap = %v, want > 0", up.Gap)
	}
	expectData(t, recv(t, out), "b")

	if first.closeCount() == 0 {
		t.Error("dead inner source was not closed before reopening")
	}
	if badOpen.closeCount() == 0 {
		t.Error("source that failed to open was not closed")
	}
}

func TestReconnecting_DownErrDefaultsToSourceClosed(t *testing.T) {
	first := newFakeSource()
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{{src: first}}}
	r := NewReconnecting(spec, fastPolicy(), false)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	first.drop()
	if down := expectLink(t, recv(t, r.TimedFrames()), core.LinkDown); down.Err != "source closed" {
		t.Errorf("Down.Err = %q, want %q", down.Err, "source closed")
	}
}

func TestReconnecting_FirstOpenErrorNotRetried(t *testing.T) {
	bad := newFakeSource()
	bad.openErr = errors.New("permission denied")
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{{src: bad}, {src: newFakeSource()}}}
	r := NewReconnecting(spec, fastPolicy(), true)
	err := r.Open(context.Background())
	if !errors.Is(err, bad.openErr) {
		t.Fatalf("Open err = %v, want %v", err, bad.openErr)
	}
	time.Sleep(20 * time.Millisecond)
	if n := spec.callCount(); n != 1 {
		t.Errorf("NewSource called %d times, want 1", n)
	}
	if got := r.Meta(); got != (core.SourceMeta{}) {
		t.Errorf("Meta after failed Open = %+v, want zero", got)
	}
	_ = r.Close()
}

func TestReconnecting_DisabledClosesOnDrop(t *testing.T) {
	first := newFakeSource()
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{{src: first}, {src: newFakeSource()}}}
	r := NewReconnecting(spec, fastPolicy(), false)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := r.TimedFrames()
	first.drop()
	expectLink(t, recv(t, out), core.LinkDown)
	expectClosed(t, out, 200*time.Millisecond)
	if n := spec.callCount(); n != 1 {
		t.Errorf("NewSource called %d times, want 1", n)
	}
}

func TestReconnecting_DisableDuringBackoffStops(t *testing.T) {
	first := newFakeSource()
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{{src: first}}} // every reopen fails
	r := NewReconnecting(spec, fastPolicy(), true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := r.TimedFrames()
	first.drop()
	expectLink(t, recv(t, out), core.LinkDown)

	deadline := time.Now().Add(time.Second)
	for spec.callCount() < 2 { // first failed reopen
		if time.Now().After(deadline) {
			t.Fatal("no reopen attempt")
		}
		time.Sleep(time.Millisecond)
	}
	r.SetEnabled(false)
	if r.Enabled() {
		t.Fatal("Enabled() = true after SetEnabled(false)")
	}
	expectClosed(t, out, 50*time.Millisecond)
}

func TestReconnecting_CloseDuringBackoffReturnsPromptly(t *testing.T) {
	first := newFakeSource()
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{{src: first}}}
	r := NewReconnecting(spec, ReconnectPolicy{Initial: 5 * time.Second, Max: 5 * time.Second}, true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := r.TimedFrames()
	first.drop()
	expectLink(t, recv(t, out), core.LinkDown)

	start := time.Now()
	done := make(chan struct{})
	go func() { _ = r.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Close did not return within 200ms")
	}
	expectClosed(t, out, 200*time.Millisecond-time.Since(start))
	_ = r.Close() // idempotent
}

func TestReconnecting_CloseConcurrentWithForwarding(t *testing.T) {
	first := newFakeSource()
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{{src: first}}}
	r := NewReconnecting(spec, fastPolicy(), true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	go func() {
		for i := 0; i < 100; i++ {
			select {
			case first.frames <- []byte("x"):
			case <-time.After(10 * time.Millisecond):
				return
			}
		}
	}()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = r.Close() }()
	}
	wg.Wait()
	expectClosedDraining(t, r.TimedFrames(), 200*time.Millisecond)
	if first.closeCount() == 0 {
		t.Error("inner source not closed")
	}
}

// expectClosedDraining discards data frames until the channel closes.
func expectClosedDraining(t *testing.T, ch <-chan core.Frame, within time.Duration) {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatalf("channel not closed within %s", within)
		}
	}
}

func TestReconnecting_TimeoutEndsRun(t *testing.T) {
	first := newFakeSource()
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{{src: first}}}
	p := fastPolicy()
	p.Timeout = 20 * time.Millisecond
	r := NewReconnecting(spec, p, true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := r.TimedFrames()
	first.drop()
	expectLink(t, recv(t, out), core.LinkDown)
	expectClosed(t, out, 200*time.Millisecond)
}

func TestReconnecting_RepeatedDropsResetAttempts(t *testing.T) {
	a, b, c := newFakeSource(), newFakeSource(), newFakeSource()
	// The first outage fails twice before recovering; the second recovers
	// on its first try, so its count only reads 1 if attempts reset.
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{
		{src: a},
		{err: errors.New("gone")},
		{err: errors.New("gone")},
		{src: b},
		{src: c},
	}}
	r := NewReconnecting(spec, fastPolicy(), true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := r.TimedFrames()
	for i, tc := range []struct {
		src  *fakeSource
		want int
	}{{a, 3}, {b, 1}} {
		tc.src.drop()
		expectLink(t, recv(t, out), core.LinkDown)
		if up := expectLink(t, recv(t, out), core.LinkUp); up.Attempts != tc.want {
			t.Errorf("outage %d: Up.Attempts = %d, want %d", i+1, up.Attempts, tc.want)
		}
	}
	c.frames <- []byte("c")
	expectData(t, recv(t, out), "c")
}

func TestReconnecting_WriteWhileDownReturnsErrLinkDown(t *testing.T) {
	first, second := newFakeSource(), newFakeSource()
	// The reopen succeeds only after the test has written while down.
	gate := make(chan struct{})
	spec := &gatedSpec{fakeSpec: fakeSpec{kind: "serial", steps: []fakeStep{{src: first}, {src: second}}}, gate: gate}
	r := NewReconnecting(spec, fastPolicy(), true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := r.TimedFrames()

	if _, err := r.Write([]byte("up1")); err != nil {
		t.Fatalf("Write while up: %v", err)
	}
	first.drop()
	expectLink(t, recv(t, out), core.LinkDown)
	if _, err := r.Write([]byte("down")); !errors.Is(err, ErrLinkDown) {
		t.Fatalf("Write while down err = %v, want ErrLinkDown", err)
	}
	close(gate)
	expectLink(t, recv(t, out), core.LinkUp)
	if _, err := r.Write([]byte("up2")); err != nil {
		t.Fatalf("Write after Up: %v", err)
	}
	if got := first.written(); len(got) != 1 || string(got[0]) != "up1" {
		t.Errorf("first writes = %q", got)
	}
	if got := second.written(); len(got) != 1 || string(got[0]) != "up2" {
		t.Errorf("second writes = %q", got)
	}
}

// gatedSpec blocks every NewSource after the first until gate is closed.
type gatedSpec struct {
	fakeSpec
	gate  chan struct{}
	first atomic.Bool
}

func (g *gatedSpec) NewSource() (ManagedSource, error) {
	if g.first.Swap(true) {
		<-g.gate
	}
	return g.fakeSpec.NewSource()
}

func TestReconnecting_OnEventCalled(t *testing.T) {
	first, second := newFakeSource(), newFakeSource()
	spec := &fakeSpec{kind: "tcp", steps: []fakeStep{{src: first}, {src: second}}}
	var mu sync.Mutex
	var events []core.LinkState
	p := fastPolicy()
	p.OnEvent = func(ev core.LinkEvent) {
		mu.Lock()
		events = append(events, ev.State)
		mu.Unlock()
	}
	r := NewReconnecting(spec, p, true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := r.TimedFrames()
	first.drop()
	expectLink(t, recv(t, out), core.LinkDown)
	expectLink(t, recv(t, out), core.LinkUp)
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 || events[0] != core.LinkDown || events[1] != core.LinkUp {
		t.Errorf("OnEvent got %v, want [down up]", events)
	}
}

func TestReconnecting_SecondOpenErrors(t *testing.T) {
	first, extra := newFakeSource(), newFakeSource()
	spec := &fakeSpec{kind: "serial", steps: []fakeStep{{src: first}, {src: extra}}}
	r := NewReconnecting(spec, fastPolicy(), true)
	if err := r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := r.Open(context.Background()); err == nil {
		t.Fatal("second Open succeeded, want error")
	}
	if n := spec.callCount(); n != 1 {
		t.Errorf("NewSource called %d times, want 1", n)
	}
	out := r.TimedFrames()
	first.frames <- []byte("a")
	expectData(t, recv(t, out), "a")
	_ = r.Close()
	expectClosed(t, out, 200*time.Millisecond) // a double close would panic
}

func TestReconnecting_FramesIsNil(t *testing.T) {
	r := NewReconnecting(&fakeSpec{kind: "serial"}, fastPolicy(), true)
	if r.Frames() != nil {
		t.Error("Frames() should be nil; consumers use TimedFrames")
	}
}

func TestSupportsReconnect(t *testing.T) {
	cases := []struct {
		spec Spec
		want bool
	}{
		{Config{}, true},
		{TCPConfig{}, true},
		{FileConfig{}, false},
	}
	for _, c := range cases {
		if got := SupportsReconnect(c.spec); got != c.want {
			t.Errorf("SupportsReconnect(%T) = %v, want %v", c.spec, got, c.want)
		}
	}
}

func TestBackoffDelay(t *testing.T) {
	const initial, max = 250 * time.Millisecond, 5 * time.Second
	cases := []struct {
		attempt int
		base    time.Duration
	}{
		{1, 250 * time.Millisecond},
		{3, time.Second},
		{10, 5 * time.Second},
		{100, 5 * time.Second}, // no overflow
	}
	for _, c := range cases {
		lo := time.Duration(float64(c.base) * 0.8)
		hi := time.Duration(float64(c.base) * 1.2)
		if got := backoffDelay(c.attempt, initial, max, 0); got != lo {
			t.Errorf("attempt %d rnd 0 = %v, want %v", c.attempt, got, lo)
		}
		if got := backoffDelay(c.attempt, initial, max, 0.5); got != c.base {
			t.Errorf("attempt %d rnd 0.5 = %v, want %v", c.attempt, got, c.base)
		}
		if got := backoffDelay(c.attempt, initial, max, 0.999999); got < lo || got > hi {
			t.Errorf("attempt %d rnd ~1 = %v, want in [%v, %v]", c.attempt, got, lo, hi)
		}
	}
}
