#!/bin/sh
set -eu

read_secret() {
  target="$1"
  source="$2"
  if [ -n "$source" ]; then
    value="$(tr -d '\r\n' < "$source")"
    export "$target=$value"
  fi
}

case "${OPENKNOWLEDGE_AGENT_RUNTIME:-}" in
  codex)
    read_secret CODEX_API_KEY "${CODEX_API_KEY_FILE:-}"
    ;;
  claude)
    read_secret ANTHROPIC_API_KEY "${ANTHROPIC_API_KEY_FILE:-}"
    ;;
  opencode)
    read_secret OPENCODE_API_KEY "${OPENCODE_API_KEY_FILE:-}"
    ;;
esac

exec openknowledge runtime worker "$@"
