#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-0.1.0}"
echo "Building bxw release ${VERSION}..."

TARGETS=("amd64:x64" "arm64:arm64")

for pair in "${TARGETS[@]}"; do
  GOARCH="${pair%%:*}"
  NPMARCH="${pair##*:}"
  PKG_DIR="npm/bxw-win32-${NPMARCH}"
  BIN_DIR="${PKG_DIR}/bin"
  mkdir -p "${BIN_DIR}"
  
  echo "Building Windows ${GOARCH} -> ${PKG_DIR}/bin/bxw.exe..."
  GOOS=windows GOARCH="${GOARCH}" CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X bxw/internal/app.Version=${VERSION}" \
    -o "${BIN_DIR}/bxw.exe" main.go
    
  cp LICENSE "${PKG_DIR}/"
done

echo "Done building npm binaries for version ${VERSION}."
