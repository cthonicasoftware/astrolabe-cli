// Package version holds build metadata for the Astrolabe CLI.
//
// The values are injected at build time with -ldflags -X. Use
// scripts/ldflags.sh (local/CI builds) or .goreleaser.yml (releases); both
// derive Version from the git tag as a SemVer string without a leading "v".
package version

var (
	// Version is the CLI SemVer (e.g. "1.2.3"). "dev" for unstamped builds.
	Version = "dev"
	// Commit is the short git commit hash.
	Commit = "unknown"
	// BuildDate is the UTC build timestamp (RFC 3339).
	BuildDate = "unknown"
)

// Path is the import path used in -X ldflags.
const Path = "github.com/cthonicasoftware/astrolabe-cli/internal/version"
