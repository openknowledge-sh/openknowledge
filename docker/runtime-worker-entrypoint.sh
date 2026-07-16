#!/bin/sh
set -eu

if [ -n "${OPENAI_API_KEY_FILE:-}" ]; then
  OPENAI_API_KEY="$(tr -d '\r\n' < "${OPENAI_API_KEY_FILE}")"
  export OPENAI_API_KEY
fi

exec openknowledge runtime worker "$@"
