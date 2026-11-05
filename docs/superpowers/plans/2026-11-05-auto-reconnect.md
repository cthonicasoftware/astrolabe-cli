# Automatic Reconnect for Serial/TCP Capture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a serial port or TCP connection drops mid-capture, retry opening it with backoff and keep appending to the same Run, recording the gap as `link_down`/`link_up` records (GitHub issue #121).

**Architecture:** A new `sources.Reconnecting` wrapper owns the inner Source and rebuilds it with `Spec.NewSource()` on each attempt. It emits one ordered `<-chan core.Frame` carrying data frames and link events, so the gap marker always sits between the last byte before the drop and the first byte after. The pipeline turns link events into records; the TUI fan-out forwards them to the live view. Headless and TUI captures both use the wrapper.

**Tech Stack:** Go 1.26, bubbletea/lipgloss v2, `go.bug.st/serial` (+ `enumerator`), cobra.

**Spec:** GitHub issue #121 plus the decisions below (from the 2026-11-05 grilling session). There is no separate spec file; this section is the spec.

### Decisions (spec)

1. **Scope:** wrapper used by both TUI and headless capture, for serial and TCP Sources only (file Sources are never wrapped).
2. **Signalling:** a single ordered channel of `core.Frame`; a frame is either data or a link event. `capture.Frame` becomes an alias of `core.Frame`.
3. **Run records:** on link down, flush the normalizer's partial line with `envelope.truncated = true`, then write `type: "link_down"` (payload `error`). On link up write `type: "link_up"` (payload `attempts`, `gap_ms`, and `port` only when the serial port name changed). Link records carry no `envelope.direction`. The Manifest gets a `reconnects` attribute (count) on every reconnect-capable Run.
4. **Retry:** exponential backoff starting at 250ms, doubling, capped at 5s, ±20% jitter. Optional overall timeout per outage (`--reconnect-timeout`, 0 = forever); on expiry the Run ends normally with its final `link_down`. The first `Open` failure is returned immediately (no retry). Context cancellation (Stop, Ctrl-C) aborts the backoff immediately.
5. **Renamed USB ports:** retry the original name first; if that fails and the device had a USB serial number at first open, open the port that now reports that serial number. Never match on VID/PID alone. Manifest `port` stays the original name.
6. **Toggle:** a live-view pref toggled with `r`, persisted with the other live-view prefs, checked at drop time. Default **on** in the TUI, **off** headless (`--reconnect` turns it on). While down: sending disabled; status bar ` RECONNECTING ` (warning style), then ` RECONNECTED ` for 3s; muted marker lines in the buffer. ` DISCONNECTED ` remains the terminal state.
7. **TCP keepalive:** `net.KeepAliveConfig{Enable: true, Idle: 5s, Interval: 5s, Count: 3}` in `TCP.Open`; constants, no flags.
8. **Delivery:** one PR.

## Global Constraints

- Go 1.26.0 (`go.mod`); no new third-party dependencies.
- `internal/sources` must not import `internal/capture` (dependency direction). Shared frame/event types live in `internal/core`.
- Record type strings, exactly: `"link_down"`, `"link_up"`. Payload keys, exactly: `error`, `attempts`, `gap_ms`, `port`. Envelope key: `truncated`. Manifest attribute: `reconnects`.
- Backoff values, exactly: initial `250 * time.Millisecond`, max `5 * time.Second`, jitter ±20%.
- Keepalive values, exactly: idle `5s`, interval `5s`, count `3`.
- Status copy, exactly: `" RECONNECTING"`, `" RECONNECTED"`, `" DISCONNECTED"` (existing). Key: `r`, help text `"toggle reconnect"`.
- Test: `mise run test` (`go test ./...`). Lint: `mise run lint`. Both must pass before each commit.
- `docs/API_REFERENCE.md` is the Orrery contract source of truth; run the `verify-orrery-contract` skill after Task 1.

## Review Focus

1. **Existing settings file with no `reconnect` key** → reconnect must load as **on** (a plain `bool` would silently turn it off for every current user). Test in Task 5.
2. **Stop pressed mid-backoff (5s cap)** → `Close`/cancel returns in well under 1s and the saved Run ends with an unmatched `link_down`. Tests in Task 2 and Task 1.
3. **Repeated drops in one Run** → attempts counter resets per outage, `reconnects` counts every successful reconnect, every `link_down` is followed by its own `link_up`. Test in Task 2.
4. **Toggle turned off while already reconnecting** → retries stop at the next attempt, channel closes, live view shows DISCONNECTED. Test in Task 2.
5. **Send racing a drop** → `Write` returns `ErrLinkDown` (or the inner write error) and no `tx` record is written. Test in Task 2 (wrapper) and Task 5 (`captureSession.Send` already skips the record on error; pin it).

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/core/frame.go` (new) | `Frame`, `LinkEvent`, `LinkState` |
| `internal/capture/pipeline.go` | alias `Frame`; link records; truncated flush; `reconnects` attr |
| `internal/sources/reconnect.go` (new) | `Reconnecting` wrapper, `ReconnectPolicy`, `ErrLinkDown`, `SupportsReconnect` |
| `internal/sources/relocate.go` (new) | `Relocator` interface; `Config` implementation via USB serial number |
| `internal/sources/tcp.go` | keepalive dialer |
| `cmd/astrolabe/root/capture_service.go`, `capture_serial.go`, `capture_tcp.go` | headless wiring + flags |
| `cmd/astrolabe/root/capture_adapter.go` | TUI fan-out reads `TimedFrames`, forwards link events, `SetReconnect` |
| `internal/tui/capture_port.go`, `app.go`, `router.go`, `buffer.go` | TUI states, key, markers |
| `internal/config/settings.go`, `cmd/astrolabe/root/live_view_prefs_adapter.go` | persisted pref |
| `docs/API_REFERENCE.md`, `docs/OPERATOR_TROUBLESHOOTING.md` | record types; operator notes |

---

### Task 1: Core frame types and pipeline link records

**Files:**
- Create: `internal/core/frame.go`
- Modify: `internal/capture/pipeline.go`
- Modify: `docs/API_REFERENCE.md` (line ~103, next to the rx/tx note)
- Test: `internal/capture/pipeline_link_test.go` (new)

**Interfaces:**
- Produces (in `core`):
  ```go
  type LinkState string
  const (
      LinkDown LinkState = "down"
      LinkUp   LinkState = "up"
  )
  type LinkEvent struct {
      State    LinkState
      Err      string        // LinkDown only
      Attempts int           // LinkUp only
      Gap      time.Duration // LinkUp only
      Port     string        // LinkUp only; set when the serial port name changed
  }
  // Frame is source data or, when Link is non-nil, a link event. At is the
  // arrival (or event) time.
  type Frame struct {
      Data []byte
      At   time.Time
      Link *LinkEvent
  }
  ```
- Produces (in `capture`): `type Frame = core.Frame`; constants `RecordTypeLinkDown = "link_down"`, `RecordTypeLinkUp = "link_up"`, `AttrReconnects = "reconnects"`, `EnvelopeTruncated = "truncated"`; new field `Options.Reconnecting bool` (when true, the manifest always gets `reconnects`, even `"0"`).

- [ ] **Step 1: Write the failing tests** in `pipeline_link_test.go`, using a `TimedSource` fake fed from a slice of `core.Frame` and the existing test store/normalizer helpers from `pipeline_test.go`:
  - `TestPipeline_LinkDownFlushesPartialLineAsTruncated`: frames `Data("abc\nde")`, `Link{Down, Err:"read: EOF"}`, `Data("f\n")`, `Link{Up, Attempts:2, Gap:1500ms}`. Assert records in order: `sample "abc"`; `sample "de"` with `Envelope["truncated"] == true`; `link_down` with `Payload["error"] == "read: EOF"` and `Envelope` has no `"direction"`; `sample "f"` (not `"def"`); `link_up` with `Payload["attempts"] == 2`, `Payload["gap_ms"] == int64(1500)`, no `"port"` key.
  - `TestPipeline_LinkRecordTimestampsAreEventTimes`: link record `TS` equals the frame's `At`.
  - `TestPipeline_ReconnectsAttribute`: `Options.Reconnecting: true`, two Down/Up pairs → `Manifest.Attributes["reconnects"] == "2"`; with zero events → `"0"`; with `Reconnecting: false` → key absent.
  - `TestPipeline_UnmatchedLinkDownAtEnd`: frames end with `Link{Down}` then channel closes → last record is `link_down`, run finalizes without error.
  - `TestPipeline_LinkUpIncludesPortWhenSet`: `Link{Up, Port:"/dev/ttyACM1"}` → `Payload["port"] == "/dev/ttyACM1"`.
  - With `Bidirectional: true`, the truncated sample still has `direction: "rx"` and the link records still have none.

- [ ] **Step 2: Run** `go test ./internal/capture/ -run Link -v` — Expected: FAIL (undefined `core.Frame` / `Link`).

- [ ] **Step 3: Implement.** Add `internal/core/frame.go`. In `pipeline.go` replace the `Frame` struct with the alias, add the constants and `Options.Reconnecting`. In the `timedCh` case, branch on `frame.Link != nil`: for Down → flush (`normalize.Flusher`), tag each flushed record's envelope with `truncated: true` (copy the map, as `withDirection` does), `writeRX` them, then `writeRecords` the `link_down`; for Up → `writeRecords` the `link_up` and increment a `reconnects` counter. Write `AttrReconnects` alongside `AttrTXRecords` when `Options.Reconnecting`. Confirm `LineJSON.Flush` clears its buffer (the `"f"` assertion proves it); fix it if not.

- [ ] **Step 4: Run** `go test ./internal/capture/ ./internal/normalize/ -v` — Expected: PASS, including existing tests.

- [ ] **Step 5: Document** in `docs/API_REFERENCE.md`: one bullet after the rx/tx note describing `link_down`/`link_up`, their payload keys, `envelope.truncated`, and the `reconnects` attribute. Run the `verify-orrery-contract` skill; it must report no discrepancy (backend treats `data.jsonl` as opaque).

- [ ] **Step 6: Commit**
  ```bash
  git add internal/core/frame.go internal/capture/ docs/API_REFERENCE.md
  git commit -m "feat(capture): record link_down/link_up gaps in runs"
  ```

---

### Task 2: Reconnecting wrapper (same-name retry)

**Files:**
- Create: `internal/sources/reconnect.go`
- Modify: `internal/sources/spec.go` (compile-time assertion)
- Modify: `internal/sources/serial.go` (`readLoop`), `internal/sources/tcp.go` (`readLoop`): store the error that ended the loop and expose it as `func (s *Serial) Err() error` / `func (t *TCP) Err() error` (nil after a clean `Close`)
- Test: `internal/sources/reconnect_test.go` (new), `internal/sources/write_test.go` (`TestSerial_ReadLoopStopsOnPortError` also asserts `Err()` returns the port error)

**Interfaces:**
- Consumes: `core.Frame`, `core.LinkEvent` (Task 1); `Spec`, `ManagedSource` (existing).
- Produces:
  ```go
  var ErrLinkDown = errors.New("link down: reconnecting")

  type ReconnectPolicy struct {
      Initial time.Duration         // 0 → 250ms
      Max     time.Duration         // 0 → 5s
      Timeout time.Duration         // per outage; 0 → retry forever
      OnEvent func(core.LinkEvent)  // optional; called from the wrapper goroutine, must not block
  }

  // SupportsReconnect reports whether spec's Kind is "serial" or "tcp".
  func SupportsReconnect(spec Spec) bool

  func NewReconnecting(spec Spec, policy ReconnectPolicy, enabled bool) *Reconnecting
  func (r *Reconnecting) Open(ctx context.Context) error        // first open; errors returned as-is, no retry
  func (r *Reconnecting) TimedFrames() <-chan core.Frame
  func (r *Reconnecting) Frames() <-chan []byte                 // returns nil; consumers use TimedFrames
  func (r *Reconnecting) Meta() core.SourceMeta                 // first inner source's Meta
  func (r *Reconnecting) Write(p []byte) (int, error)           // ErrLinkDown while down
  func (r *Reconnecting) Close() error                          // idempotent; aborts backoff
  func (r *Reconnecting) SetEnabled(on bool)                    // atomic
  func (r *Reconnecting) Enabled() bool
  ```
  `*Reconnecting` satisfies `WritableSource` and (structurally) `capture.TimedSource`.

Behavior the tests pin: one goroutine forwards inner `Frames()` as `core.Frame{Data, At: time.Now().UTC()}`. When the inner channel closes and ctx is not done: emit `Link{Down, Err}` (`Err` is the inner source's `Err().Error()` when it implements `interface{ Err() error }` and returns non-nil, else `"source closed"`); if `!Enabled()` close the output channel. Otherwise loop: sleep backoff (select on ctx), re-check `Enabled()`, `spec.NewSource()` + `Open(ctx)`; on success emit `Link{Up, Attempts, Gap}` **before** forwarding any new data; on `Timeout` expiry close the output channel. Attempts and backoff reset per outage.

- [ ] **Step 1: Write the failing tests** with a `fakeSpec` (`Kind` configurable, `NewSource` returns the next scripted `fakeSource` or an error) and `fakeSource` (frames chan you close to simulate a drop, records writes). Use `Initial: time.Millisecond, Max: 4*time.Millisecond`.
  - `TestReconnecting_ReopensAndOrdersEvents`: data `"a"`, drop, two failed `NewSource` opens, success, data `"b"` → output exactly `Data a`, `Down`, `Up{Attempts:3}`, `Data b`.
  - `TestReconnecting_FirstOpenErrorNotRetried`: first `Open` fails → `Open` returns that error, `NewSource` called once.
  - `TestReconnecting_DisabledClosesOnDrop`: `enabled=false`, drop → `Down` then channel closed, no further `NewSource`.
  - `TestReconnecting_DisableDuringBackoffStops` (Review Focus 4): always-failing reopen, `SetEnabled(false)` after first failure → channel closes within 50ms.
  - `TestReconnecting_CloseDuringBackoffReturnsPromptly` (Review Focus 2): `Initial: 5*time.Second`, drop, then `Close()` → returns and channel closes within 200ms.
  - `TestReconnecting_TimeoutEndsRun`: `Timeout: 20ms`, always failing → `Down` then channel closed within 200ms.
  - `TestReconnecting_RepeatedDropsResetAttempts` (Review Focus 3): drop/recover twice, each recovering on its first try → both `Up` events have `Attempts == 1`.
  - `TestReconnecting_WriteWhileDownReturnsErrLinkDown` (Review Focus 5): after `Down` and before `Up`, `Write` → `errors.Is(err, ErrLinkDown)`; after `Up`, writes reach the new inner source.
  - `TestReconnecting_OnEventCalled`: `OnEvent` receives `Down` then `Up`.
  - `TestSupportsReconnect`: `Config{}` and `TCPConfig{}` → true, `FileConfig{}` → false.
  - `TestBackoffDelay`: table test of the unexported `backoffDelay(attempt int, initial, max time.Duration, rnd float64) time.Duration` — attempt 1 → 250ms·(0.8..1.2), attempt 3 → 1s·(0.8..1.2), attempt 10 → capped to 5s·(0.8..1.2).

- [ ] **Step 2: Run** `go test ./internal/sources/ -run 'Reconnect|Backoff' -v` — Expected: FAIL (undefined `NewReconnecting`).

- [ ] **Step 3: Implement** `reconnect.go` per the Interfaces block and behavior above. Add `_ WritableSource = (*Reconnecting)(nil)` to `spec.go`.

- [ ] **Step 4: Run** `go test -race ./internal/sources/ -v` — Expected: PASS, no races.

- [ ] **Step 5: Commit**
  ```bash
  git add internal/sources/reconnect.go internal/sources/reconnect_test.go internal/sources/spec.go internal/sources/serial.go internal/sources/tcp.go internal/sources/write_test.go
  git commit -m "feat(sources): reconnecting wrapper with backoff"
  ```

---

### Task 3: Headless `--reconnect` and `--reconnect-timeout`

**Files:**
- Modify: `cmd/astrolabe/root/capture_service.go`, `capture_serial.go`, `capture_tcp.go`
- Modify: `docs/OPERATOR_TROUBLESHOOTING.md` (new "Device resets mid-capture" section)
- Test: `cmd/astrolabe/root/capture_service_test.go`, `capture_test.go`

**Interfaces:**
- Consumes: `sources.NewReconnecting`, `sources.SupportsReconnect`, `sources.ReconnectPolicy` (Task 2); `capture.Options.Reconnecting` (Task 1).
- Produces: `CaptureRequest.Reconnect *sources.ReconnectPolicy` (nil = off). `BuildComponents` wraps the source in `NewReconnecting(req.Source, *req.Reconnect, true)` when `req.Reconnect != nil && SupportsReconnect(req.Source)`, and `Run` sets `Options.Reconnecting` accordingly. Flags on both `capture serial` and `capture tcp`: `--reconnect` (bool, default false, "reconnect if the device drops") and `--reconnect-timeout` (duration, default 0, "give up after this long disconnected (0 = never)"; ignored without `--reconnect`).

- [ ] **Step 1: Write the failing tests:**
  - `TestBuildComponents_WrapsWhenReconnectSet`: TCP spec + `Reconnect: &sources.ReconnectPolicy{}` → `components.Source` is `*sources.Reconnecting`; with nil → not wrapped; `FileConfig` + policy → not wrapped.
  - `TestCaptureSerialCmd_ReconnectFlags` / `TestCaptureTCPCmd_ReconnectFlags`: `--reconnect --reconnect-timeout 10m` yields a request with `Reconnect.Timeout == 10*time.Minute`; no `--reconnect` → `Reconnect == nil` even when the timeout is given.
  - `TestCaptureService_ReconnectsAcrossTCPServerRestart`: local `net.Listener` sends `"one\n"` then closes the connection; accepts again and sends `"two\n"`; cancel ctx after the second line. Run's `data.jsonl` contains `one`, `link_down`, `link_up`, `two`; manifest `reconnects == "1"`.

- [ ] **Step 2: Run** `go test ./cmd/astrolabe/root/ -run 'Reconnect' -v` — Expected: FAIL.

- [ ] **Step 3: Implement** the field, wrapping, and flags. Set `ReconnectPolicy.OnEvent` in the serial/TCP commands to print through `out`: `out.Warning("Link down: <err>; reconnecting")` and `out.Step("Link up after <gap>")` (gap rounded to 0.1s).

- [ ] **Step 4: Run** `mise run test` — Expected: PASS.

- [ ] **Step 5: Document** the flags and the `link_down`/`link_up` records in `docs/OPERATOR_TROUBLESHOOTING.md`.

- [ ] **Step 6: Commit**
  ```bash
  git add cmd/astrolabe/root/ docs/OPERATOR_TROUBLESHOOTING.md
  git commit -m "feat(capture): --reconnect for headless serial/tcp capture"
  ```

---

### Task 4: TCP keepalive

**Files:**
- Modify: `internal/sources/tcp.go:86-111`
- Test: `internal/sources/tcp_test.go`

**Interfaces:**
- Produces: unexported `func (t *TCP) dialer() *net.Dialer` used by `Open`; package constants `tcpKeepAliveIdle = 5 * time.Second`, `tcpKeepAliveInterval = 5 * time.Second`, `tcpKeepAliveCount = 3`.

- [ ] **Step 1: Write the failing test** `TestTCP_DialerUsesKeepAlive`: `d := tcp.dialer()`; assert `d.Timeout == ConnectTimeout` and `d.KeepAliveConfig == net.KeepAliveConfig{Enable: true, Idle: 5*time.Second, Interval: 5*time.Second, Count: 3}`.
- [ ] **Step 2: Run** `go test ./internal/sources/ -run KeepAlive -v` — Expected: FAIL.
- [ ] **Step 3: Implement** `dialer()` and use it in `Open`.
- [ ] **Step 4: Run** `go test ./internal/sources/ -v` — Expected: PASS.
- [ ] **Step 5: Commit**
  ```bash
  git add internal/sources/tcp.go internal/sources/tcp_test.go
  git commit -m "feat(sources): tcp keepalive detects dead peers in ~20s"
  ```

---

### Task 5: TUI reconnect states, toggle, and persisted pref

**Files:**
- Modify: `internal/config/settings.go`, `cmd/astrolabe/root/live_view_prefs_adapter.go`
- Modify: `internal/tui/capture_port.go`, `capture_tabs.go` (`CaptureConfig`), `router.go:209-224`, `app.go`, `buffer.go`
- Modify: `cmd/astrolabe/root/capture_adapter.go`
- Test: `internal/config/settings_test.go`, `internal/tui/app_reconnect_test.go` (new), `cmd/astrolabe/root/capture_adapter_test.go`

**Interfaces:**
- Consumes: `sources.Reconnecting` (`TimedFrames`, `SetEnabled`), `core.LinkEvent` (Tasks 1–2).
- Produces:
  - `config.LiveViewPrefs.Reconnect *bool` with tag `json:"reconnect,omitempty"`; nil means on.
  - `tui.LiveViewPrefs.Reconnect bool`; `DefaultLiveViewPrefs()` sets it `true`.
  - `tui.CaptureConfig.Reconnect bool` (initial toggle state).
  - `tui.FeedChunk.Link *core.LinkEvent` (nil for data).
  - `tui.CaptureSession` gains `SetReconnect(on bool)`.
  - `App.EnableReconnect(set func(bool))`, mirroring `EnableSend`; the `r` binding is active only after it.
  - `LineBuffer.AddMarker(text string, at time.Time)` — a muted, non-echo line.
- Adapter: `Start` always wraps serial/TCP with `NewReconnecting(spec, ReconnectPolicy{}, cfg.Reconnect)` and sets `Options.Reconnecting: true`. The fan-out loop ranges over `TimedFrames()` (no re-stamping) and forwards link events as `FeedChunk{At, Link}` to the feed only (the pipeline gets the same frame on its channel). `closeSource` and `writer` point at the wrapper.
- Router: load prefs **before** `Start`, set `captureCfg.Reconnect = prefs.Reconnect`, then `appModel.EnableReconnect(session.SetReconnect)`.
- App state: `reconnecting bool`, `reconnectAttempt int`, `reconnectedUntil time.Time`. On `Link{Down}`: `reconnecting = true`, leave input mode but keep the draft, add marker `── link down 15:04:05 (<err>) ──`. On `Link{Up}`: clear `reconnecting`, set `reconnectedUntil = now+3s` (and a `tea.Tick` to redraw), add marker `── link up after 5.2s ──` (append ` on <port>` when `Port != ""`). `canSend()` also requires `!a.reconnecting`. Status precedence: DISCONNECTED > RECONNECTING > RECONNECTED > PAUSED. Input line while reconnecting: `Reconnecting… (attempt N)`. Status bar shows `Reconnect on`/`Reconnect off` next to the timestamps indicator. Pressing `r` flips `prefs.Reconnect`, calls the setter, and saves prefs like `toggleTimestamps`.

- [ ] **Step 1: Write the failing tests:**
  - `TestLoadSettings_MissingReconnectDefaultsOn` (Review Focus 1): settings JSON `{"live_view":{"line_ending":"LF","echo":true}}` → adapter's `Load()` returns `Reconnect == true`; `{"live_view":{"reconnect":false}}` → false; Save→Load round-trips false.
  - `TestApp_LinkDownShowsReconnectingAndBlocksSend`: feed `FeedChunk{Link: Down}` → status contains `RECONNECTING`, `canSend() == false`, draft text preserved, buffer's last line contains `link down`.
  - `TestApp_LinkUpShowsReconnectedThenClears`: after Up, status contains `RECONNECTED`; after advancing past `reconnectedUntil`, it doesn't; `canSend() == true`.
  - `TestApp_FeedClosedWhileReconnectingShowsDisconnected`: Down, then feed closed → `DISCONNECTED`, not `RECONNECTING`.
  - `TestApp_ReconnectKeyTogglesAndSaves`: `r` flips the setter's argument and calls the prefs save func; without `EnableReconnect`, `r` does nothing.
  - `TestCaptureSession_LinkEventsReachFeed`: adapter test with a TCP listener that drops and re-accepts → feed yields data, `Link{Down}`, `Link{Up}`, data, in that order.
  - `TestCaptureSession_SendWhileDownRecordsNothing` (Review Focus 5): Send during an outage returns an error wrapping `sources.ErrLinkDown`, and the saved Run has no `tx` record for it.

- [ ] **Step 2: Run** `go test ./internal/tui/ ./internal/config/ ./cmd/astrolabe/root/ -run 'Reconnect|Link' -v` — Expected: FAIL.

- [ ] **Step 3: Implement** per the Interfaces block. Add `Reconnect` to `keys` (`key.WithKeys("r")`, help `"toggle reconnect"`) and to the help section that lists `Time`.

- [ ] **Step 4: Run** `mise run test && mise run lint` — Expected: PASS; `screens_fit_test.go` still passes at its widths.

- [ ] **Step 5: Manual check** (`mise run tui`): capture from a USB serial board, unplug for ~3s, replug → RECONNECTING, then RECONNECTED, marker lines, data resumes; press `r` and unplug → DISCONNECTED.

- [ ] **Step 6: Commit**
  ```bash
  git add internal/config/ internal/tui/ cmd/astrolabe/root/
  git commit -m "feat(tui): live reconnect states and r toggle"
  ```

---

### Task 6: Follow renamed USB serial ports

**Files:**
- Create: `internal/sources/relocate.go`
- Modify: `internal/sources/reconnect.go`
- Test: `internal/sources/relocate_test.go` (new), `internal/sources/reconnect_test.go`

**Interfaces:**
- Produces:
  ```go
  // Relocator is implemented by specs whose device can return under a new name.
  type Relocator interface {
      // Identity returns a stable device ID for the current port, or "" if none.
      Identity() string
      // Relocate returns a spec for whichever port now reports id.
      Relocate(id string) (Spec, bool)
  }
  ```
  `Config` implements it using the USB `SerialNumber` from a package var `listPorts = enumerator.GetDetailedPortsList` (replaced in tests). Empty serial numbers never match.
- Wrapper change: after the first successful `Open`, if the spec is a `Relocator`, store `id := Identity()`. On each attempt, try the current spec; if it fails and `id != ""`, try `Relocate(id)`; on success, make the relocated spec current (so later drops try the new name first) and set `LinkEvent.Port` to its `ManifestAttrs()["port"]`. `Meta()` and the Manifest keep the original port.

- [ ] **Step 1: Write the failing tests:**
  - `TestConfig_IdentityFromSerialNumber`: fake list `[{Name:"/dev/ttyACM0", IsUSB:true, SerialNumber:"E66138"}]`, `Config{Port:"/dev/ttyACM0"}` → `"E66138"`; port not listed or no serial → `""`.
  - `TestConfig_RelocateFindsNewName`: list has `ACM1` with serial `E66138` → returns `Config` with `Port:"/dev/ttyACM1"` and the same baud/parity/etc.; returns `false` when no port has the serial.
  - `TestConfig_RelocateIgnoresVIDPIDOnlyMatch`: a port with the same VID/PID but a different or empty serial → `false`.
  - `TestReconnecting_FollowsRelocatedPort`: a `fakeSpec` implementing `Relocator` whose original name always fails and whose relocated spec opens → `Up.Port == "/dev/ttyACM1"`; on a second drop, the relocated spec is tried first.
  - `TestReconnecting_NoIdentitySameNameOnly`: `Identity() == ""` → `Relocate` never called.

- [ ] **Step 2: Run** `go test ./internal/sources/ -run 'Relocat|Identity' -v` — Expected: FAIL.

- [ ] **Step 3: Implement** `relocate.go` and the wrapper change.

- [ ] **Step 4: Run** `go test -race ./internal/sources/ && mise run test` — Expected: PASS.

- [ ] **Step 5: Manual check** on Linux: hold the port open, reset the board so it re-enumerates as `ttyACM1` → `link_up` has `port: "/dev/ttyACM1"`. Repeat on macOS and Windows if hardware is available; note results in the PR.

- [ ] **Step 6: Commit**
  ```bash
  git add internal/sources/
  git commit -m "feat(sources): follow USB serial number when a port is renamed"
  ```

---

## Final verification (before opening the PR)

- [ ] `mise run lint && go test -race ./...` — Expected: PASS.
- [ ] `verify-orrery-contract` skill reports no discrepancies.
- [ ] PR body links #121 and #117 and lists the manual hardware checks from Tasks 5 and 6 with their results.
