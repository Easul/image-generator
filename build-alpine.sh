#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
OUTPUT_NAME="${OUTPUT_NAME:-ig}"
GOOS_VALUE="${GOOS:-linux}"
GOARCH_VALUE="${GOARCH:-amd64}"
MAIN_PACKAGE="${MAIN_PACKAGE:-./cmd/server}"
OUTPUT_PATH="${DIST_DIR}/${OUTPUT_NAME}"

mkdir -p "${DIST_DIR}"

CGO_ENABLED=0 \
GOOS="${GOOS_VALUE}" \
GOARCH="${GOARCH_VALUE}" \
go build \
  -trimpath \
  -ldflags='-s -w -buildid=' \
  -o "${OUTPUT_PATH}" \
  "${MAIN_PACKAGE}"

chmod +x "${OUTPUT_PATH}"

printf 'Built %s\n' "${OUTPUT_PATH}"
