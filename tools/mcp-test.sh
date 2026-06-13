#!/bin/bash
# mcp-test.sh — MCP server test harness for AI agents
#
# Call MCP tools directly without boilerplate. The harness handles:
#   - Server lifecycle (auto-start / auto-detect / auto-stop)
#   - MCP session handshake (initialize → notified → call)
#   - JSON-RPC requests with proper session headers
#   - SSE response parsing → clean JSON output
#
# USAGE:
#   mcp-test.sh call <tool> <json_args>          Call a tool with JSON args
#   mcp-test.sh call <tool> --key val [--key2 v] Call with named args
#   mcp-test.sh quick                             Smoke test (list_collections)
#   mcp-test.sh shell                             Interactive REPL
#   mcp-test.sh --                                Print this help
#
# ENV:
#   MCPT_PORT      — Server port (default: 8080)
#   MCPT_BIN       — Path to photofield binary
#   MCPT_DATA_DIR  — Path to data directory
#   MCPT_START     — Auto-start server (default: true)
#   MCPT_URL       — Full server URL (overrides PORT)

set -uo pipefail

# ─── Config ───
PORT="${MCPT_PORT:-8080}"
URL="${MCPT_URL:-http://localhost:${PORT}/mcp}"
BIN="${MCPT_BIN:-$(cd "$(dirname "$0")/../.." && pwd)/photofield}"
DATA_DIR="${MCPT_DATA_DIR:-$(pwd)/data}"
AUTO_START="${MCPT_START:-true}"
VERBOSE=0

# ─── Color helpers ───
if [[ -t 1 ]]; then
  COL_GREEN='\033[0;32m'; COL_RED='\033[0;31m'; COL_CYAN='\033[0;36m'
  COL_BOLD='\033[1m'; COL_RESET='\033[0m'
else
  COL_GREEN=''; COL_RED=''; COL_CYAN=''; COL_BOLD=''; COL_RESET=''
fi

log_ok()    { echo "${COL_GREEN}✓${COL_RESET} $*"; }
log_fail()  { echo "${COL_RED}✗${COL_RESET} $*" >&2; }
log_info()  { echo "${COL_CYAN}ℹ${COL_RESET} $*" >&2; }
log_step()  { echo "${COL_BOLD}--- $*${COL_RESET}" >&2; }

# ─── Server Management ───
_server_pid_file="/tmp/photofield-mcp-test.pid"

server_start() {
  if curl -s --max-time 2 "${URL}" &>/dev/null; then
    log_info "Server already running on port ${PORT}"
    _ServerManaged=false
    return 0
  fi

  if [[ ! -x "$BIN" ]]; then
    log_fail "Server binary not found: ${BIN}"
    echo "  Set MCPT_BIN=/path/to/photofield to override" >&2
    return 1
  fi

  log_step "Starting MCP server..."
  nohup "$BIN" > /tmp/photofield-mcp-test.log 2>&1 &
  _ServerManaged=true
  echo $! > "$_server_pid_file"
  log_info "PID: $! (log: /tmp/photofield-mcp-test.log)"

  local waited=0
  while (( waited < 30 )); do
    if curl -s --max-time 2 "${URL}" &>/dev/null; then
      log_ok "Server is ready"
      return 0
    fi
    sleep 0.5
    waited=$((waited + 1))
  done

  log_fail "Server failed to start within 30s"
  tail -20 /tmp/photofield-mcp-test.log >&2
  return 1
}

# ─── MCP Session & Requests ───
_SESSION_ID=""
_REQUEST_ID=0

session_init() {
  # Send initialize request and capture headers to get session ID
  local resp
  resp=$(curl -s -D /tmp/mcp-headers-$$ \
    -X POST "${URL}" \
    -H "Content-Type: application/json" \
    -H "Accept: application/json, text/event-stream" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{
      "protocolVersion":"2024-11-05",
      "capabilities":{},
      "clientInfo":{"name":"mcp-test","version":"0.1"}
    }}' 2>/dev/null)

  # Extract session ID from HTTP headers
  _SESSION_ID=$(grep -i "Mcp-Session-Id" /tmp/mcp-headers-$$ 2>/dev/null | head -1 | tr -d '\r' | sed 's/.*[Mm]cp-[Ss]ession-[Ii]d:[[:space:]]*//')
  rm -f /tmp/mcp-headers-$$

  if [[ -z "$_SESSION_ID" ]]; then
    log_info "No session ID (server may not require one)"
  else
    log_info "Session: ${_SESSION_ID}"
  fi

  # Send initialized notification
  local hdrs=(-H "Content-Type: application/json")
  [[ -n "$_SESSION_ID" ]] && hdrs+=(-H "Mcp-Session-Id: ${_SESSION_ID}")
  curl -s -X POST "${URL}" \
    "${hdrs[@]}" \
    -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' 2>/dev/null || true
}

# Parse SSE response: extract JSON from "data: {...}" lines
_sse_parse() {
  local raw="$1"
  # Extract the last JSON data line from SSE
  echo "$raw" | sed -n 's/^data: //p' | tail -1
}

# Call a tool, return cleaned JSON on stdout
mcp_call() {
  local tool="$1"
  shift
  local args="$*"
  [[ -z "$args" ]] && args="{}"
  _REQUEST_ID=$((_REQUEST_ID + 1))

  local hdrs=(-H "Content-Type: application/json" -H "Accept: application/json, text/event-stream")
  [[ -n "$_SESSION_ID" ]] && hdrs+=(-H "Mcp-Session-Id: ${_SESSION_ID}")

  local raw
  raw=$(curl -s --max-time 30 -X POST "${URL}" \
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
    # Strip leading -- from key name
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
  local rpc_error rpc_error_msg
  rpc_error=$(echo "$resp" | jq -r '.error.message // empty' 2>/dev/null)
  if [[ -n "$rpc_error" ]]; then
    echo "${COL_RED}✗${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — ${rpc_error}"
    [[ "$VERBOSE" == "1" ]] && echo "$resp" | jq '.' 2>/dev/null
    return 1
  fi

  # Check for result.isError (MCP tools can return errors in result)
  local is_error
  is_error=$(echo "$resp" | jq -r '.result.isError // false' 2>/dev/null)

  # Check for content in result (MCP uses content: [{type:"text",text:"..."}])
  local content_type content_text
  content_type=$(echo "$resp" | jq -r '.result.content[0].type // empty' 2>/dev/null)
  content_text=$(echo "$resp" | jq -r '.result.content[0].text // empty' 2>/dev/null)

  if [[ -n "$content_text" ]]; then
    # If isError flag is set, this is an error response
    if [[ "$is_error" == "true" ]]; then
      echo "${COL_RED}✗${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — ${content_text}"
      [[ "$VERBOSE" == "1" ]] && echo "$resp" | jq '.' 2>/dev/null
      return 1
    fi

    if [[ "$VERBOSE" == "1" ]]; then
      # Full JSON output
      # Try to parse the text as JSON for pretty-printing
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
      # Content is JSON — show a summary
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
      echo "${COL_GREEN}✓${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — ${summary_label}"
      # Print the parsed JSON
      echo "$inner_json" | jq '.' 2>/dev/null || echo "$inner_json"
    else
      # Plain text response
      if [[ ${#content_text} -lt 500 ]]; then
        echo "${COL_GREEN}✓${COL_RESET} ${COL_BOLD}${tool}${COL_RESET}"
        echo "$content_text"
      else
        echo "${COL_GREEN}✓${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — ${#content_text} chars"
        echo "$content_text" | head -c 500
        echo "..."
      fi
    fi
    return 0
  fi

  # No content — check for structured content
  local has_structured
  has_structured=$(echo "$resp" | jq -e '.result.structuredContent' &>/dev/null && echo yes || echo no)
  if [[ "$has_structured" == "yes" ]]; then
    local sc_json
    sc_json=$(echo "$resp" | jq -r '.result.structuredContent' 2>/dev/null)
    if [[ -n "$sc_json" ]]; then
      echo "${COL_GREEN}✓${COL_RESET} ${COL_BOLD}${tool}${COL_RESET}"
      echo "$sc_json" | jq '.' 2>/dev/null || echo "$sc_json"
      return 0
    fi
  fi

  # No result at all
  echo "${COL_RED}✗${COL_RESET} ${COL_BOLD}${tool}${COL_RESET} — no result"
  [[ "$VERBOSE" == "1" ]] && echo "$resp" | jq '.' 2>/dev/null || echo "$resp"
  return 1
}

# ─── REPL ───
run_repl() {
  echo ""
  echo "${COL_BOLD}MCP Test Shell${COL_RESET}  (type 'help' for commands, 'quit' to exit)"
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
      echo "  quit                        Exit"
      continue
    }

    if [[ "$line" =~ ^call[[:space:]]+ ]]; then
      local rest="${line#call }"
      local tool args
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
      resp=$(mcp_call "$tool" "$args")
      print_result "$tool" "$resp"
    else
      echo "Unknown command: $line (type 'help')"
    fi
  done
}

# ─── CLI ───
# Parse args: --verbose/--v must come before subcommand
while [[ $# -gt 0 ]]; do
  case "$1" in
    --verbose|-v) VERBOSE=1; shift ;;
    --help|-h|--) cmd="help"; shift ;;
    --quick|quick) cmd="quick"; QUICK_ARG="${2:-}"; shift; [[ -n "$QUICK_ARG" ]] && shift ;;
    --shell|shell) cmd="shell"; shift ;;
    call)
      cmd="call"
      shift
      break  # rest are call args
      ;;
    *) cmd="unknown"; break ;;
  esac
done

# Parse call args: tool [-- key val ...] or tool <json>
call_tool="" call_json="" named_mode=false
if [[ "$cmd" == "call" ]]; then
  if [[ $# -eq 0 ]]; then
    cmd="help"
  elif [[ "$1" == "--" ]]; then
    shift
    named_mode=true
    call_tool=""
  else
    call_tool="$1"
    shift
    if [[ $# -gt 0 && "$1" == "--" ]]; then
      shift
      named_mode=true
    elif [[ $# -gt 0 && "$1" =~ ^--[a-zA-Z] ]]; then
      # Auto-detect named args
      named_mode=true
    fi
  fi
fi

# ─── Execute ───
case "$cmd" in
  help)
    cat <<EOF
mcp-test.sh — MCP server test harness for AI agents

  Call MCP tools directly without boilerplate.

  USAGE:
    mcp-test.sh call <tool> <json_args>          Call a tool with JSON args
    mcp-test.sh call <tool> --key val [--key2 v] Call with named args
    mcp-test.sh quick [tool args]                 Smoke test (default: list_collections)
    mcp-test.sh shell                             Interactive REPL
    mcp-test.sh --                                Print this help

  ENV:
    MCPT_PORT      — Server port (default: 8080)
    MCPT_BIN       — Path to photofield binary
    MCPT_DATA_DIR  — Path to data directory
    MCPT_START     — Auto-start server (default: true)
    MCPT_URL       — Full server URL (overrides PORT)
EOF
    exit 0
    ;;
  quick)
    [[ "$AUTO_START" == "true" ]] && server_start
    session_init
    # Quick arg format: "tool_name args_json" (default: list_collections '{}')
    quick_tool="${QUICK_ARG%% *}"
    quick_args="${QUICK_ARG#* }"
    [[ "$quick_tool" == "$quick_args" ]] && quick_args="{}"  # no space = no args
    [[ -z "$quick_tool" ]] && quick_tool="list_collections" && quick_args="{}"
    resp=$(mcp_call "$quick_tool" "$quick_args")
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
  call)
    if [[ -z "$call_tool" ]]; then
      echo "Usage: $0 call <tool> <args>" >&2
      exit 1
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
    resp=$(mcp_call "$call_tool" "$args_json")
    print_result "$call_tool" "$resp"
    ;;
  *)
    echo "Unknown command: $cmd" >&2
    exit 1
    ;;
esac
