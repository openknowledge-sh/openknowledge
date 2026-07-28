#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
unformatted="$(
  find "$root/packages/cli" -type f -name '*.go' -print0 \
    | sort -z \
    | xargs -0 gofmt -l
)"

if [ -n "$unformatted" ]; then
  echo "Go formatting check failed:" >&2
  while IFS= read -r path; do
    printf -- '- %s\n' "${path#"$root/"}" >&2
  done <<< "$unformatted"
  exit 1
fi

echo "Go sources are formatted"
