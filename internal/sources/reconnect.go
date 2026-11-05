package sources

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// ErrLinkDown is returned by Reconnecting.Write while the link is down.
var ErrLinkDown = errors.New("link down: reconnecting")

// errReconnectingClosed is returned by Open after Close.
var errReconnectingClosed = errors.New("reconnecting source is closed")

// errReconnectingOpen is returned by an Open while another Open is running
// or has succeeded.
var errReconnectingOpen = errors.New("reconnecting source already open")

const (
	defaultReconnectInitial = 250 * time.Millisecond
	defaultReconnectMax     = 5 * time.Second
)

// ReconnectPolicy controls how a Reconnecting source retries after a drop.
type ReconnectPolicy struct {
	Initial time.Duration        // 0 → 250ms
	Max     time.Duration        // 0 → 5s
	Timeout time.Duration        // per outage; 0 → retry forever
	OnEvent func(core.LinkEvent) // optional; called from the wrapper goroutine, must not block
}

// SupportsReconnect reports whether spec's Kind is "serial" or "tcp".
func SupportsReconnect(spec Spec) bool {
	switch spec.Kind() {
	case "serial", "tcp":
		return true
	}
	return false
}

// Reconnecting wraps the source a Spec builds. When the inner source drops it
// rebuilds and reopens it with backoff, and it reports data and link events
// in order on one channel (TimedFrames).
type Reconnecting struct {
	spec    Spec
	policy  ReconnectPolicy
	enabled atomic.Bool
	wake    chan struct{} // nudged by SetEnabled(false) to cut a backoff short
	out     chan core.Frame

	mu      sync.Mutex
	cur     ManagedSource // current inner source; nil before Open
	up      bool          // link is up: Write goes to cur
	meta    core.SourceMeta
	ctx     context.Context
	cancel  context.CancelFunc
	opening bool // an Open is running or has succeeded
	started bool // the forwarding goroutine owns (and will close) out
	closed  bool
}

// NewReconnecting returns a wrapper around the source spec builds. With
// enabled false a drop ends the stream as an unwrapped source would, but
// still reports the LinkDown event.
func NewReconnecting(spec Spec, policy ReconnectPolicy, enabled bool) *Reconnecting {
	if policy.Initial <= 0 {
		policy.Initial = defaultReconnectInitial
	}
	if policy.Max <= 0 {
		policy.Max = defaultReconnectMax
	}
	if policy.Max < policy.Initial {
		policy.Max = policy.Initial
	}
	r := &Reconnecting{
		spec:   spec,
		policy: policy,
		wake:   make(chan struct{}, 1),
		out:    make(chan core.Frame, 16),
	}
	r.enabled.Store(enabled)
	return r
}

// Open builds and opens the first inner source. Its errors are returned as-is;
// the first open is never retried.
func (r *Reconnecting) Open(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return errReconnectingClosed
	}
	if r.opening {
		r.mu.Unlock()
		return errReconnectingOpen
	}
	r.opening = true
	r.ctx, r.cancel = context.WithCancel(ctx)
	runCtx := r.ctx
	r.mu.Unlock()

	src, err := r.openNext(runCtx)
	if err != nil {
		r.mu.Lock()
		r.cancel()
		r.opening = false // a failed Open may be tried again
		r.mu.Unlock()
		return err
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		_ = src.Close()
		return errReconnectingClosed
	}
	r.cur, r.up, r.started = src, true, true
	r.meta = src.Meta()
	r.mu.Unlock()

	go r.run(runCtx, src)
	return nil
}

// openNext builds and opens the next inner source, closing it again if Open
// fails.
func (r *Reconnecting) openNext(ctx context.Context) (ManagedSource, error) {
	src, err := r.spec.NewSource()
	if err != nil {
		return nil, err
	}
	if err := src.Open(ctx); err != nil {
		_ = src.Close()
		return nil, err
	}
	return src, nil
}

// run forwards the inner source's frames and handles drops. It is the only
// sender on out and closes it on return.
func (r *Reconnecting) run(ctx context.Context, src ManagedSource) {
	defer close(r.out)
	for {
		if !r.forward(ctx, src) || ctx.Err() != nil {
			return
		}

		r.mu.Lock()
		r.up = false
		r.mu.Unlock()

		downAt := time.Now()
		if !r.emit(ctx, downAt, core.LinkEvent{State: core.LinkDown, Err: dropReason(src)}) {
			return
		}
		if !r.Enabled() {
			_ = src.Close()
			return
		}

		next, attempts := r.reconnect(ctx, src)
		if next == nil {
			return
		}
		if !r.setCurrent(next) {
			return // Close won the race; setCurrent closed next
		}
		src = next
		upAt := time.Now()
		if !r.emit(ctx, upAt, core.LinkEvent{State: core.LinkUp, Attempts: attempts, Gap: upAt.Sub(downAt)}) {
			return
		}
	}
}

// forward copies src's frames to out until src's channel closes (true) or
// ctx is done (false).
func (r *Reconnecting) forward(ctx context.Context, src ManagedSource) bool {
	in := src.Frames()
	for {
		select {
		case b, ok := <-in:
			if !ok {
				return true
			}
			select {
			case r.out <- core.Frame{Data: b, At: time.Now().UTC()}:
			case <-ctx.Done():
				return false
			}
		case <-ctx.Done():
			return false
		}
	}
}

// emit reports ev via OnEvent and on out. It returns false if ctx ended first.
func (r *Reconnecting) emit(ctx context.Context, at time.Time, ev core.LinkEvent) bool {
	if r.policy.OnEvent != nil {
		r.policy.OnEvent(ev)
	}
	select {
	case r.out <- core.Frame{At: at.UTC(), Link: &ev}:
		return true
	case <-ctx.Done():
		return false
	}
}

// reconnect retries until a new inner source opens, returning it and the
// number of attempts. It returns nil when ctx ends, reconnect is disabled, or
// the outage outlasts policy.Timeout.
func (r *Reconnecting) reconnect(ctx context.Context, dead ManagedSource) (ManagedSource, int) {
	// A held handle (serial especially) can block reopening the same port.
	_ = dead.Close()

	var deadline <-chan time.Time
	if r.policy.Timeout > 0 {
		t := time.NewTimer(r.policy.Timeout)
		defer t.Stop()
		deadline = t.C
	}

	for attempt := 1; ; attempt++ {
		delay := time.NewTimer(backoffDelay(attempt, r.policy.Initial, r.policy.Max, rand.Float64()))
		select {
		case <-delay.C:
		case <-ctx.Done():
			delay.Stop()
			return nil, 0
		case <-deadline:
			delay.Stop()
			return nil, 0
		case <-r.wake:
			delay.Stop()
		}
		if !r.Enabled() || ctx.Err() != nil {
			return nil, 0
		}
		select {
		case <-deadline:
			return nil, 0
		default:
		}
		if src, err := r.openNext(ctx); err == nil {
			return src, attempt
		}
	}
}

// setCurrent installs src as the live inner source. If Close already ran it
// closes src instead and returns false.
func (r *Reconnecting) setCurrent(src ManagedSource) bool {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		_ = src.Close()
		return false
	}
	r.cur, r.up = src, true
	r.mu.Unlock()
	return true
}

// dropReason describes why src's stream ended.
func dropReason(src ManagedSource) string {
	if e, ok := src.(interface{ Err() error }); ok {
		if err := e.Err(); err != nil {
			return err.Error()
		}
	}
	return "source closed"
}

// backoffDelay is initial doubled per attempt after the first, capped at
// max, then scaled by a jitter factor in [0.8, 1.2) chosen by rnd in [0, 1).
func backoffDelay(attempt int, initial, max time.Duration, rnd float64) time.Duration {
	d := initial
	for i := 1; i < attempt && d < max; i++ {
		d *= 2
	}
	if d > max {
		d = max
	}
	return time.Duration(float64(d) * (0.8 + 0.4*rnd))
}

// TimedFrames returns the ordered stream of data frames and link events. It
// closes when the run ends: Close, ctx cancel, a drop with reconnect
// disabled, or an outage longer than policy.Timeout.
func (r *Reconnecting) TimedFrames() <-chan core.Frame { return r.out }

// Frames returns nil; consumers use TimedFrames.
func (r *Reconnecting) Frames() <-chan []byte { return nil }

// Meta returns the first inner source's metadata, or the zero value before a
// successful Open.
func (r *Reconnecting) Meta() core.SourceMeta {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.meta
}

// Write sends p to the current inner source, or returns ErrLinkDown while
// the link is down.
func (r *Reconnecting) Write(p []byte) (int, error) {
	r.mu.Lock()
	cur, up := r.cur, r.up
	r.mu.Unlock()
	if !up || cur == nil {
		return 0, ErrLinkDown
	}
	w, ok := cur.(interface {
		Write([]byte) (int, error)
	})
	if !ok {
		return 0, errors.New("source is not writable")
	}
	return w.Write(p)
}

// Close stops reconnecting (aborting any backoff), closes the current inner
// source, and ends TimedFrames. It is safe to call more than once.
func (r *Reconnecting) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	cur, started, cancel := r.cur, r.started, r.cancel
	r.up = false
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if !started {
		close(r.out) // no forwarding goroutine will
	}
	if cur != nil {
		return cur.Close()
	}
	return nil
}

// SetEnabled turns reconnecting on or off. Turning it off during an outage
// ends the run at the next check.
func (r *Reconnecting) SetEnabled(on bool) {
	r.enabled.Store(on)
	if !on {
		select {
		case r.wake <- struct{}{}:
		default:
		}
	}
}

// Enabled reports whether reconnecting is on.
func (r *Reconnecting) Enabled() bool { return r.enabled.Load() }
