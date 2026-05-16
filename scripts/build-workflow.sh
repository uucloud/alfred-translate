#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKFLOW_DIR="$ROOT/workflow"
DIST_DIR="$ROOT/dist"
OUTPUT="$DIST_DIR/AlfredTranslate.alfredworkflow"
ARCH="${GOARCH:-arm64}"

mkdir -p "$DIST_DIR"
export GOCACHE="${GOCACHE:-$ROOT/.cache/go-build}"
mkdir -p "$GOCACHE"

GOOS=darwin GOARCH="$ARCH" go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "$WORKFLOW_DIR/alfred-translate" \
  "$ROOT/cmd/alfred-translate"

(
  cd "$WORKFLOW_DIR"
  zip -qr "$OUTPUT" .
)

echo "$OUTPUT"
