#!/usr/bin/env bash
set -euo pipefail

# Stable releases only. Pass inputs as arguments, never interpolate them into shell code.
version="${1:-}"
version="${version#v}"
if [[ ! "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "Expected a stable version such as 0.1.0 or v0.1.0" >&2
  exit 1
fi
printf '%s\n' "$version"
