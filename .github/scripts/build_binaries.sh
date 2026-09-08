#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 2 ]]; then
  echo "Usage: $0 VERSION OUTPUT_DIRECTORY" >&2
  exit 1
fi
cd "$(dirname "$0")/../.."
version=$(bash .github/scripts/release-version.sh "$1")
mkdir -p "$2"
output=$(cd "$2" && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
archives=()

for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os=${platform%/*}
  arch=${platform#*/}
  binary=dh
  [[ "$os" != windows ]] || binary=dh.exe
  target="$stage/$os-$arch"
  mkdir -p "$target"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$target/$binary" ./cmd/dh
  if [[ "$os" == windows ]]; then
    archive="dh-$os-$arch.zip"
    # Build a fresh ZIP rather than updating a previous archive in place.
    (cd "$target" && zip -q "$stage/$archive" "$binary")
  else
    archive="dh-$os-$arch.tar.gz"
    tar -C "$target" -czf "$stage/$archive" "$binary"
  fi
  cp "$stage/$archive" "$output/$archive"
  archives+=("$archive")
done
(cd "$output" && sha256sum "${archives[@]}" > checksums.txt)
