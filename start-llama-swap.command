#!/bin/zsh
set -u

cd "$(dirname "$0")" || exit 1

BINARY="./build/llama-swap-darwin-arm64"
CONFIG="./config.local.yaml"
LISTEN="127.0.0.1:9292"

echo "Starting llama-swap..."
echo "Directory: $(pwd)"
echo "Config: ${CONFIG}"
echo "UI: http://${LISTEN}/ui/"
echo

if [[ ! -x "$BINARY" ]]; then
  echo "Error: missing executable $BINARY"
  echo "Run this first:"
  echo "  make mac"
  echo
  read -r "?Press Enter to close..."
  exit 1
fi

if [[ ! -f "$CONFIG" ]]; then
  echo "Error: missing config $CONFIG"
  echo
  read -r "?Press Enter to close..."
  exit 1
fi

if lsof -nP -iTCP:9292 -sTCP:LISTEN >/dev/null 2>&1; then
  echo "Port 9292 is already in use. Existing listener:"
  lsof -nP -iTCP:9292 -sTCP:LISTEN
  echo
  read -r "?Press Enter to close..."
  exit 1
fi

"$BINARY" --config "$CONFIG" --listen "$LISTEN" --watch-config --allow-config-edit

echo
read -r "?llama-swap stopped. Press Enter to close..."
