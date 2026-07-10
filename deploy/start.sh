#!/usr/bin/env bash
set -euo pipefail

if [ -f "$HOME/.bashrc" ]; then
  # shellcheck source=/dev/null
  source "$HOME/.bashrc"
fi

ENV_FILE="${BWS_ENV_FILE:-.env}"
if [ ! -f "$ENV_FILE" ]; then
  echo "missing env file: $ENV_FILE" >&2
  exit 1
fi

set -a
# shellcheck source=/dev/null
source <(sed 's/\r$//' "$ENV_FILE")
set +a

APP_BIN="${BWS_BIN:-./bws-checkin}"
exec "$APP_BIN" "$@"
