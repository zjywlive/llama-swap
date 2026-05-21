#!/bin/zsh
set -u

cd "$(dirname "$0")" || exit 1

BINARY="./build/llama-swap-darwin-arm64"
CONFIG="./config.local.yaml"
HOST="127.0.0.1"
DEFAULT_PORT=9292
MAX_PORT=9392
LISTEN="${HOST}:${DEFAULT_PORT}"

echo "Starting llama-swap..."
echo "Directory: $(pwd)"
echo "Config: ${CONFIG}"
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

port_in_use() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
}

show_listener() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN 2>/dev/null || true
}

listener_pids() {
  lsof -nP -tiTCP:"$1" -sTCP:LISTEN 2>/dev/null | tr '\n' ' ' | sed 's/[[:space:]]*$//'
}

find_free_port() {
  local port="$1"
  while (( port <= MAX_PORT )); do
    if ! port_in_use "$port"; then
      echo "$port"
      return 0
    fi
    (( port++ ))
  done
  return 1
}

wait_for_port_free() {
  local port="$1"
  local attempt
  for attempt in {1..20}; do
    if ! port_in_use "$port"; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

choose_listen_port() {
  local port="$DEFAULT_PORT"
  local choice
  local pids
  local free_port

  if ! port_in_use "$port"; then
    LISTEN="${HOST}:${port}"
    return 0
  fi

  while true; do
    echo "Port ${port} is already in use. Existing listener:"
    show_listener "$port"
    echo
    echo "Choose an action:"
    echo "  k - kill the existing listener and use port ${port}"
    echo "  a - automatically use the next free port"
    echo "  q - quit"
    printf "Selection [k/a/q]: "
    read -r choice
    echo

    case "${choice:l}" in
      k|kill)
        pids="$(listener_pids "$port")"
        if [[ -z "$pids" ]]; then
          LISTEN="${HOST}:${port}"
          return 0
        fi
        echo "Stopping listener process(es): ${pids}"
        kill $=pids 2>/dev/null || true
        if wait_for_port_free "$port"; then
          LISTEN="${HOST}:${port}"
          return 0
        fi
        echo "Port ${port} is still in use after waiting."
        echo
        ;;
      a|auto|"")
        free_port="$(find_free_port $(( port + 1 )))"
        if [[ -z "$free_port" ]]; then
          echo "Error: no free port found between $(( port + 1 )) and ${MAX_PORT}."
          return 1
        fi
        LISTEN="${HOST}:${free_port}"
        echo "Using alternate port ${free_port}."
        return 0
        ;;
      q|quit)
        return 1
        ;;
      *)
        echo "Please type k, a, or q."
        echo
        ;;
    esac
  done
}

if ! choose_listen_port; then
  echo
  read -r "?Press Enter to close..."
  exit 1
fi

echo "UI: http://${LISTEN}/ui/"
echo

"$BINARY" --config "$CONFIG" --listen "$LISTEN" --watch-config --allow-config-edit

echo
read -r "?llama-swap stopped. Press Enter to close..."
