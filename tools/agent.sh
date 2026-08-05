#!/bin/bash
# agent.sh — Unified harness for testing the photofield server
#
# Covers server lifecycle, generic HTTP API calls, and tool calls.
#
# USAGE:
#   agent.sh --help                          Print this help
#   agent.sh --verbose                       Verbose output (must precede subcommand; env override: AGT_VERBOSE=1)
#
#   agent.sh server start                    Start server (auto-detect / launch)
#   agent.sh server stop                     Stop via PID file
#   agent.sh server restart                  Stop + start
#   agent.sh server status                   Check if running
#   agent.sh server kill                     Aggressive pkill (photofield + exiftool)
#
#   agent.sh api <method> <url> [body]       Generic HTTP call
#   agent.sh api <method> <url> --key val    Named-arg body
#
#   agent.sh mcp call <tool> <args>          Tool call (JSON or named args)
#   agent.sh mcp call <tool> --key val       Tool call (named args)
#   agent.sh mcp quick [tool args]           Smoke test
#   agent.sh mcp shell                       Interactive REPL
#
# ENV:
#   AGT_PORT        — Server port (default: 8080)
#   AGT_BIN         — Path to photofield binary
#   AGT_DATA_DIR    — Path to data directory
#   AGT_START       — Auto-start server (default: true)
#   AGT_URL         — Full endpoint URL (overrides PORT)
#   AGT_API_BASE    — API base URL (default: http://localhost:$PORT)

set -uo pipefail

# ─── Config ───
PORT="${AGT_PORT:-8080}"
API_BASE="${AGT_API_BASE:-http://localhost:${PORT}}"
ENDPOINT_URL="${AGT_URL:-${API_BASE}/mcp}"
BIN="${AGT_BIN:-$(cd "$(dirname "$0")/.." && pwd)/photofield}"
DATA_DIR="${AGT_DATA_DIR:-$(pwd)/data}"
AUTO_START="${AGT_START:-true}"
VERBOSE=${AGT_VERBOSE:-0}
_SERVER_MANAGED=false

# ─── Paths ───
_pid_file="${DATA_DIR}/agent.pid"
_headers_file="${DATA_DIR}/agent-headers-$$"

# ─── Colors ───
if [[ -t 1 ]]; then
  COL_GREEN=$'\033[0;32m'; COL_RED=$'\033[0;31m'; COL_CYAN=$'\033[0;36m'
  COL_BOLD=$'\033[1m'; COL_RESET=$'\033[0m'
else
  COL_GREEN=''; COL_RED=''; COL_CYAN=''; COL_BOLD=''; COL_RESET=''
fi

log_ok()    { printf '%s\n' "${COL_GREEN}✓${COL_RESET} $*" >&2; }
log_fail()  { printf '%s\n' "${COL_RED}✗${COL_RESET} $*" >&2; }
log_info()  { printf '%s\n' "${COL_CYAN}ℹ${COL_RESET} $*" >&2; }
log_step()  { printf '%s\n' "${COL_CYAN}▶${COL_RESET} $*" >&2; }

# ─── Server Management ───
server_is_running() {
  # Check PID file first
  if [[ -f "$_pid_file" ]]; then
    local pid
    pid=$(cat "$_pid_file" 2>/dev/null)
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    rm -f "$_pid_file"
  fi
  # Fall back to port check
  curl -s --max-time 1 "${ENDPOINT_URL}" &>/dev/null
}

server_start() {
  if server_is_running; then
    log_info "Server already running on port ${PORT}"
    _SERVER_MANAGED=false
    return 0
  fi

  if [[ ! -x "$BIN" ]]; then
    log_fail "Server binary not found: ${BIN}"
    log_info "Set AGT_BIN=/path/to/photofield to override"
    return 1
  fi

  log_step "Starting server..."
  mkdir -p "$DATA_DIR"
  export PHOTOFIELD_ADDRESS=":$(echo "$PORT" | sed 's/.*://')"
  export PHOTOFIELD_DATA_DIR="$DATA_DIR"
  nohup "$BIN" > "${DATA_DIR}/agent.log" 2>&1 &
  _SERVER_MANAGED=true
  local pid=$!
  printf '%s\n' "$pid" > "$_pid_file"
  log_info "PID: ${pid} (log: ${DATA_DIR}/agent.log)"

  local waited=0
  while (( waited < 30 )); do
    if curl -s --max-time 2 "${ENDPOINT_URL}" &>/dev/null; then
      log_ok "Server is ready"
      return 0
    fi
    sleep 0.5
    waited=$((waited + 1))
  done

  log_fail "Server failed to start within 30s"
  tail -20 "${DATA_DIR}/agent.log" >&2
  return 1
}

server_stop() {
  if [[ -f "$_pid_file" ]]; then
    local pid
    pid=$(cat "$_pid_file" 2>/dev/null)
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      log_info "Stopping server (PID ${pid})..."
      kill "$pid" 2>/dev/null || true
      # Wait for it to die
      local w=0
      while (( w < 10 )); do
        kill -0 "$pid" 2>/dev/null || break
        sleep 0.3
        w=$((w + 1))
      done
      kill -0 "$pid" 2>/dev/null && kill -9 "$pid" 2>/dev/null || true
      log_ok "Server stopped"
    fi
    rm -f "$_pid_file"
  else
    log_info "No PID file found"
    # Port may still be in use
    if server_is_running; then
      log_info "Server running but no PID — use 'server kill' for aggressive stop"
      return 1
    fi
    log_ok "Nothing to stop"
  fi
}

server_restart() {
  server_stop
  sleep 1
  server_start
}

server_status() {
  if server_is_running; then
    local pid=""
    if [[ -f "$_pid_file" ]]; then
      pid=$(cat "$_pid_file" 2>/dev/null)
    fi
    log_ok "Server running on port ${PORT}"
    [[ -n "$pid" ]] && log_info "PID: ${pid}"
    # Check which processes are using the port
    local procs
    procs=$(ss -tlnp "sport = :${PORT}" 2>/dev/null | grep -oP 'pid=\K[0-9]+' | head -5 | tr '\n' ',' | sed 's/,$//')
    [[ -n "$procs" ]] && log_info "Port listeners: ${procs}"
    return 0
  else
    log_info "Server not running on port ${PORT}"
    return 1
  fi
}

server_kill() {
  log_step "Killing server processes on port ${PORT}..."
  local count=0
  # Kill by PID file first (most targeted)
  if [[ -f "$_pid_file" ]]; then
    local pid
    pid=$(cat "$_pid_file" 2>/dev/null)
    if [[ -n "$pid" ]]; then
      kill -9 "$pid" 2>/dev/null || true
      count=$((count + 1))
    fi
    rm -f "$_pid_file"
  fi
  # Kill anything listening on our port (catches server children, exiftool, etc.)
  local p
  for p in $(ss -tlnp "sport = :${PORT}" 2>/dev/null | grep -oP 'pid=\K[0-9]+' || true); do
    kill -9 "$p" 2>/dev/null || true
    count=$((count + 1))
  done
  sleep 0.3
  # Double-check (safety net)
  for p in $(ss -tlnp "sport = :${PORT}" 2>/dev/null | grep -oP 'pid=\K[0-9]+' || true); do
    kill -9 "$p" 2>/dev/null || true
    count=$((count + 1))
  done
  log_ok "Killed ${count} process(es) on port ${PORT}"
}

# ─── Generic API Calls ───
api_call() {
  local method="${1^^}"  # uppercase
  local url="$2"
  shift 2

  local body="" hdrs=(-X "$method")
  local named_mode=false

  # Check if remaining args look like named --key val pairs
  if [[ $# -gt 0 && "$1" == "--" ]]; then
    shift
    named_mode=true
  elif [[ $# -gt 0 && "$1" =~ ^--[a-zA-Z] ]]; then
    named_mode=true
  fi

  if [[ "$named_mode" == "true" ]]; then
    body=$(build_args "$@")
  elif [[ $# -gt 0 ]]; then
    body="$*"
  fi

  [[ "$method" == "GET" || "$method" == "HEAD" ]] && body=""

  log_info "→ ${method} ${url}"
  [[ -n "$body" ]] && log_info "Body: ${body:0:200}"

  local status_code tmpfile
  tmpfile=$(mktemp "${DATA_DIR}/agent-raw-XXXXXX")
  status_code=$(curl -s -o "$tmpfile" -w "%{http_code}" \
    "${hdrs[@]}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d "$body" \
    "$url" 2>/dev/null)

  local raw
  raw=$(cat "$tmpfile"; rm -f "$tmpfile")

  log_info "HTTP ${status_code}"

  # Try to pretty-print as JSON
  local pretty
  pretty=$(echo "$raw" | jq '.' 2>/dev/null)
  if [[ -n "$pretty" ]]; then
    echo "$pretty"
  else
    if [[ ${#raw} -lt 500 ]]; then
      echo "$raw"
    else
      printf '%s\n' "${raw:0:500}... [${#raw} chars total]"
    fi
  fi
}

# ─── Session & Requests ───
_SESSION_ID=""
_REQUEST_ID=0

session_init() {
  local resp
  resp=$(curl -s -D "$_headers_file" \
    -X POST "${ENDPOINT_URL}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{
      "protocolVersion":"2024-11-05",
      "capabilities":{},
      "clientInfo":{"name":"agent","version":"0.1"}
    }}' 2>/dev/null)

  _SESSION_ID=$(grep -i "Mcp-Session-Id" "$_headers_file" 2>/dev/null | head -1 | tr -d '\r' | sed 's/.*[Mm]cp-[Ss]ession-[Ii]d:[[:space:]]*//')
  rm -f "$_headers_file"

  if [[ -z "$_SESSION_ID" ]]; then
    log_info "No session ID (server may not require one)"
  else
    log_info "Session: ${_SESSION_ID}"
  fi

  # Send initialized notification
  local hdrs=(-H "Content-Type: application/json")
  [[ -n "$_SESSION_ID" ]] && hdrs+=(-H "Mcp-Session-Id: ${_SESSION_ID}")
  curl -s -X POST "${ENDPOINT_URL}" \
    "${hdrs[@]}" \
    -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' 2>/dev/null || true
}

# Parse SSE response: extract JSON from "data: {...}" lines
_sse_parse() {
  local raw="$1"
  echo "$raw" | sed -n 's/^data: //p' | tail -1
}

# Call a tool, return cleaned JSON on stdout
call_tool() {
  local tool="$1"
  shift
  local args="$*"
  [[ -z "$args" ]] && args="{}"
  _REQUEST_ID=$((_REQUEST_ID + 1))

  local hdrs=(-H "Content-Type: application/json" -H "Accept: application/json, text/event-stream")
  [[ -n "$_SESSION_ID" ]] && hdrs+=(-H "Mcp-Session-Id: ${_SESSION_ID}")

  local raw
  raw=$(curl -s --max-time 30 -X POST "${ENDPOINT_URL}" \
    "${hdrs[@]}" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":${_REQUEST_ID},\"method\":\"tools/call\",\"params\":{\"name\":\"${tool}\",\"arguments\":${args}}}" \
    2>/dev/null)

  _sse_parse "$raw"
}

# Build JSON args from --key val pairs
build_args() {
  local result="{"
  local first=true
  while [[ $# -gt 0 ]]; do
    local key="$1" val="$2"
    shift 2
    key="${key#--}"
    [[ "$first" != "true" ]] && result+=","
    first=false
    if [[ "$val" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
      result+="\"${key}\":${val}"
    elif [[ "$val" == "true" || "$val" == "false" ]]; then
      result+="\"${key}\":${val}"
    elif [[ "$val" == "null" ]]; then
      result+="\"${key}\":null"
    else
      val=$(printf '%s' "$val" | sed 's/\\/\\\\/g; s/"/\\"/g; s/\t/\\t/g')
      result+="\"${key}\":\"${val}\""
    fi
  done
  result+="}"
  echo "$result"
}

# ─── Output formatting ───
print_result() {
  local tool="$1" resp="$2"

  # Check for JSON-RPC error field
  local rpc_error
  rpc_error=$(echo "$resp" | jq -r '.error.message // empty' 2>/dev/null)
  if [[ -n "$rpc_error" ]]; then
    printf '%s\n' "${COL_RED}✗${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — ${rpc_error}"
    [[ "$VERBOSE" == "1" ]] && echo "$resp" | jq '.' 2>/dev/null
    return 1
  fi

  # Check for result.isError (tools can return errors in result)
  local is_error
  is_error=$(echo "$resp" | jq -r '.result.isError // false' 2>/dev/null)

  # Check for content in result
  local content_text
  content_text=$(echo "$resp" | jq -r '.result.content[0].text // empty' 2>/dev/null)

  if [[ -n "$content_text" ]]; then
    if [[ "$is_error" == "true" ]]; then
      printf '%s\n' "${COL_RED}✗${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — ${content_text}"
      [[ "$VERBOSE" == "1" ]] && echo "$resp" | jq '.' 2>/dev/null
      return 1
    fi

    if [[ "$VERBOSE" == "1" ]]; then
      local inner_json
      inner_json=$(echo "$content_text" | jq '.' 2>/dev/null)
      if [[ -n "$inner_json" ]]; then
        echo "$inner_json"
      else
        echo "$content_text"
      fi
      return 0
    fi

    # Normal response — try to parse content as JSON for a clean summary
    local inner_json
    inner_json=$(echo "$content_text" | jq '.' 2>/dev/null)
    if [[ -n "$inner_json" ]]; then
      # Find top-level array keys and report their counts
      local summary_label
      summary_label=$(echo "$inner_json" | jq -r '
        [ to_entries[] | select(.value | type == "array") ] |
        if length > 0 then
          (map("\(.key): \(.value | length)") | join(", "))
        else
          "ok"
        end
      ' 2>/dev/null)
      printf '%s\n' "${COL_GREEN}✓${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — ${summary_label}"
      echo "$inner_json" | jq '.' 2>/dev/null || echo "$inner_json"
    else
      # Plain text response
      if [[ ${#content_text} -lt 500 ]]; then
        printf '%s\n' "${COL_GREEN}✓${COL_RESET} ${COL_BOLD}${tool}${COL_RESET}"
        echo "$content_text"
      else
        printf '%s\n' "${COL_GREEN}✓${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — ${#content_text} chars"
        echo "$content_text" | head -c 500
        echo "..."
      fi
    fi
    return 0
  fi

  # Check for structured content
  local has_structured
  has_structured=$(echo "$resp" | jq -e '.result.structuredContent' &>/dev/null && echo yes || echo no)
  if [[ "$has_structured" == "yes" ]]; then
    local sc_json
    sc_json=$(echo "$resp" | jq -r '.result.structuredContent' 2>/dev/null)
    if [[ -n "$sc_json" ]]; then
      printf '%s\n' "${COL_GREEN}✓${COL_RESET} ${COL_BOLD}${tool}${COL_RESET}"
      echo "$sc_json" | jq '.' 2>/dev/null || echo "$sc_json"
      return 0
    fi
  fi

  # No result at all
  printf '%s\n' "${COL_RED}✗${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — no result"
  [[ "$VERBOSE" == "1" ]] && echo "$resp" | jq '.' 2>/dev/null || echo "$resp"
  return 1
}

# ─── REPL ───
run_repl() {
  echo ""
  printf '%s\n' "${COL_CYAN}Agent Test Shell${COL_RESET}  (type 'help' for commands, 'quit' to exit)"
  echo ""

  session_init

  while true; do
    printf "${COL_CYAN}>${COL_RESET} "
    read -r line
    [[ -z "$line" ]] && continue
    [[ "$line" == "quit" || "$line" == "exit" ]] && break
    [[ "$line" == "help" ]] && {
      echo "  call <tool> <json>          Call a tool with JSON args"
      echo "  call <tool> --key val ...   Call with named args"
      echo "  api <method> <url> [body]    Generic HTTP call"
      echo "  quit                        Exit"
      continue
    }

    local tool args
    if [[ "$line" =~ ^api[[:space:]]+ ]]; then
      local rest="${line#api }"
      local method url body
      if [[ "$rest" =~ ^([A-Z]+)[[:space:]]+([^[:space:]]+)(.*)$ ]]; then
        method="${BASH_REMATCH[1]}"
        url="${BASH_REMATCH[2]}"
        body="${BASH_REMATCH[3]}"
        [[ -z "$body" ]] && body="{}"
        echo "$(api_call "$method" "$url" "$body")"
      else
        echo "Usage: api <METHOD> <URL> [body]"
      fi
    elif [[ "$line" =~ ^call[[:space:]]+ ]]; then
      local rest="${line#call }"
      if [[ "$rest" =~ ^([^[:space:]]+)[[:space:]]+(--.*)$ ]]; then
        tool="${BASH_REMATCH[1]}"
        args=$(build_args ${BASH_REMATCH[2]})
      elif [[ "$rest" =~ ^([^[:space:]]+)[[:space:]]+(.*)$ ]]; then
        tool="${BASH_REMATCH[1]}"
        args="${BASH_REMATCH[2]}"
        [[ -z "$args" ]] && args="{}"
      else
        tool="$rest"
        args="{}"
      fi
      local resp
      resp=$(call_tool "$tool" "$args")
      print_result "$tool" "$resp"
    else
      echo "Unknown command: $line (type 'help')"
    fi
  done
}

# ─── Help ───
print_help() {
  cat <<'EOF'
agent.sh — Unified harness for testing the photofield server

Usage:
  agent.sh [options] <command> [args...]

Options:
  --verbose, -v         Global verbosity flag (must precede subcommand; also AGT_VERBOSE=1)
  --help, -h, --        Print this help

Server commands:
  server start          Start server (auto-detect running or launch)
  server stop           Stop via PID file (graceful)
  server restart        Stop + start
  server status         Show PID and port status
  server kill           Kill PID file + port listeners

API commands:
  api <method> <url> [body]    Generic HTTP call (GET/POST/PUT/DELETE)
  api <method> <url> --key val Named-arg body construction

Tool commands:
  mcp call <tool> <json>       Call a tool with JSON args
  mcp call <tool> --key val    Call a tool with named args
  mcp quick [tool args]        Smoke test (default: list_collections)
  mcp shell                    Interactive REPL

Environment variables:
  AGT_PORT        — Server port (default: 8080)
  AGT_BIN         — Path to photofield binary
  AGT_DATA_DIR    — Path to data directory
  AGT_START       — Auto-start server (default: true)
  AGT_URL         — Full endpoint URL
  AGT_API_BASE    — API base URL (default: http://localhost:$PORT)
  AGT_VERBOSE     — Verbose output (1 = yes)
EOF
}

# ─── CLI ───
# Top-level flags: --verbose/--v before subcommand
cmd="help"
subcmd=""
verbose_override=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --verbose|-v|-V) VERBOSE=1; verbose_override=1; shift ;;
    --help|-h|--) cmd="help"; shift ;;
    server)
      cmd="server"
      shift
      # Detect misplaced --verbose (position 2) and fail loudly
      if [[ $# -gt 0 && "$1" == "--verbose" ]]; then
        echo "Error: --verbose must come before subcommand. Use: agent.sh --verbose server <start|stop|restart|status|kill>" >&2
        exit 1
      fi
      subcmd="${1:-help}"
      shift
      ;;
    api) cmd="api"; shift; break ;;
    mcp)
      cmd="mcp"
      shift
      # Detect misplaced --verbose (position 2) and fail loudly
      if [[ $# -gt 0 && "$1" == "--verbose" ]]; then
        echo "Error: --verbose must come before subcommand. Use: agent.sh --verbose mcp <call|quick|shell>" >&2
        exit 1
      fi
      break
      ;;
    *)
      # If we got here with cmd="help" and no prior match, treat as unknown
      if [[ "$cmd" == "help" ]]; then
        cmd="help"
      fi
      break
      ;;
  esac
done

# ─── Execute ───
# For mcp/server, subcmd is the first remaining arg after the main loop
[[ -z "$subcmd" && $# -gt 0 && "$cmd" != "api" ]] && subcmd="$1" && shift

case "$cmd" in
  help)
    print_help
    exit 0
    ;;

  server)
    case "$subcmd" in
      start)    server_start ;;
      stop)     server_stop ;;
      restart)  server_restart ;;
      status)   server_status ;;
      kill)     server_kill ;;
      *)
        echo "Usage: agent.sh server <start|stop|restart|status|kill>" >&2
        exit 1
        ;;
    esac
    ;;

  api)
    if [[ $# -lt 2 ]]; then
      echo "Usage: agent.sh api <METHOD> <URL> [body]" >&2
      exit 1
    fi
    api_call "$@"
    ;;

  mcp)
    case "$subcmd" in
      call)
        # Parse: call_tool [-- key val ...] or call_tool <json>
        call_tool=""
        named_mode=false
        if [[ $# -eq 0 ]]; then
          echo "Usage: agent.sh mcp call <tool> <args>" >&2
          exit 1
        fi
        call_tool="$1"
        shift
        if [[ $# -gt 0 && "$1" == "--" ]]; then
          shift
          named_mode=true
        elif [[ $# -gt 0 && "$1" =~ ^--[a-zA-Z] ]]; then
          named_mode=true
        fi

        if [[ "$named_mode" == "true" ]]; then
          args_json=$(build_args "$@")
        elif [[ $# -gt 0 ]]; then
          args_json="$*"
        else
          args_json="{}"
        fi

        [[ "$AUTO_START" == "true" ]] && server_start
        session_init
        resp=$(call_tool "$call_tool" "$args_json")
        print_result "$call_tool" "$resp"
        ;;
      quick)
        [[ "$AUTO_START" == "true" ]] && server_start
        session_init
        quick_arg="${1:-list_collections}"
        quick_tool="${quick_arg%% *}"
        quick_args="${quick_arg#* }"
        [[ "$quick_tool" == "$quick_args" ]] && quick_args="{}"
        [[ -z "$quick_tool" ]] && quick_tool="list_collections" && quick_args="{}"
        resp=$(call_tool "$quick_tool" "$quick_args")
        if print_result "$quick_tool" "$resp"; then
          log_ok "Quick test passed"
        else
          log_fail "Quick test failed"
          exit 1
        fi
        ;;
      shell)
        [[ "$AUTO_START" == "true" ]] && server_start
        run_repl
        ;;
      *)
        echo "Usage: agent.sh mcp <call|quick|shell> [args...]" >&2
        exit 1
        ;;
    esac
    ;;

  *)
    echo "Unknown command: $cmd" >&2
    print_help >&2
    exit 1
    ;;
esac
