# Repository Guidelines

## Project Structure & Module Organization
- Root contains the Go module definition (`go.mod`) and the main TUI entrypoint (`qal_cli.go`).
- Temporary build cache can live in `.gocache/` (created automatically by build commands); clean it with `rm -rf .gocache` when needed.
- Capture artifacts are written under `captures/` at runtime. Treat this directory as disposable output—do not commit generated files.

## Build, Test, and Development Commands
- `GOCACHE=$(pwd)/.gocache go build ./...` — compiles the CLI using a workspace-local cache (required in the sandboxed CLI harness).
- `go run ./...` — launches the Bubble Tea TUI for manual evaluation; add `GOCACHE=$(pwd)/.gocache` if the environment blocks the default cache path.
- `go fmt ./...` — formats all Go sources; run before submitting changes.

## Coding Style & Naming Conventions
- Standard Go style: tabs for indentation, mixedCaps for exported identifiers, lowerCamelCase for locals, and descriptive names for TUI models and commands.
- Always run `gofmt`/`go fmt` after edits; no other formatter is required.
- Keep UI strings concise and actionable so the TUI remains readable within an 80-column terminal.

## Testing Guidelines
- Add Go unit tests beside the code they cover (e.g., `qal_cli_test.go`).
- Use the standard library `testing` package; prefer table-driven tests for config parsing and filename helpers.
- Execute `GOCACHE=$(pwd)/.gocache go test ./...` before opening a PR, and ensure tests pass without relying on connected hardware.

## Commit & Pull Request Guidelines
- Commit messages should follow the pattern `<component>: <imperative summary>` (e.g., `tui: add upload mode selector`).
- Squash unrelated work into separate commits for clarity; keep diffs focused.
- Pull requests must describe the change, list manual/automated verification steps, and include screenshots or terminal recordings when the TUI behavior changes.

## Agent-Specific Instructions
- When simulating captures, avoid committing generated `.jsonl` or `.meta.json` files—use `git clean -fd captures/` after local runs.
- Document any hardware assumptions (serial adapters, baud defaults) directly in the PR so remote reviewers understand the setup.
