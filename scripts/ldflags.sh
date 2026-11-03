#!/usr/bin/env bash
# Prints the -ldflags string that stamps version metadata into the binary.
# Single source for mise and CI builds (GoReleaser uses its own templates for
# the same three variables in .goreleaser.yml).
#
# Version resolution:
#   1. $ASTROLABE_VERSION, if set
#   2. `git describe --tags --always --dirty`
#   3. "dev"
# A leading "v" is stripped so the value is plain SemVer (1.2.3).
set -euo pipefail

PKG="github.com/cthonicasoftware/astrolabe-cli/internal/version"

VERSION="${ASTROLABE_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
VERSION="${VERSION#v}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_DATE="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "-s -w -X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.BuildDate=${BUILD_DATE}"
