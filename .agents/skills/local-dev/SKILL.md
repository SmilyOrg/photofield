---
name: local-dev
description: >-
  Run, test, and debug the photofield server locally. Use when building,
  running, or testing the server, making API calls, calling MCP tools, or
  inspecting runtime state. Covers server lifecycle via `./tools/agent.sh server`,
  generic HTTP calls via `./tools/agent.sh api`, MCP tool calls, database inspection,
  error debugging, and common fixes.
---

# Local Development — Running and Testing the Server

This skill covers building, running, testing, and debugging the photofield
server on your local machine.

All tool invocation, server management, and API calls go through
`./tools/agent.sh` — a unified harness that handles the server lifecycle,
generic HTTP calls, and MCP tool invocation with session management, SSE
parsing, and named-arg parsing.

## Verbose Flag

The harness accepts `--verbose`, `-v`, and `-V`. Any of these can be placed
directly after `./tools/agent.sh`:

```bash
./tools/agent.sh --verbose mcp call get_photo --file_id 1
./tools/agent.sh -v mcp call get_photo --file_id 1
./tools/agent.sh -V mcp call get_photo --file_id 1
```

Or avoid the flag entirely via environment variable:
```bash
AGT_VERBOSE=1 ./tools/agent.sh mcp call get_photo --file_id 1
```

## 1. Build

```bash
go build -o photofield .
```

Kill any old instance before rebuilding:

```bash
./tools/agent.sh server kill
```

## 2. Configuration

The server reads `data/configuration.yaml`. Without it, collections point to
empty directories.

**Minimal setup:**

```bash
mkdir -p data
cat > data/configuration.yaml <<EOF
collections:
  - name: test
    dirs:
      - docs/assets
EOF
```

## 3. Server Lifecycle

Use `./tools/agent.sh server <command>` to manage the server process:

```bash
# Start (auto-detects if already running)
./tools/agent.sh server start

# Stop gracefully (uses PID file)
./tools/agent.sh server stop

# Restart
./tools/agent.sh server restart

# Check status (shows PID and port listeners)
./tools/agent.sh server status

# Aggressive kill (PID file + all port listeners including exiftool)
./tools/agent.sh server kill
```

**How it works:** `server start` launches the binary with `nohup` and writes a
PID file to `/tmp/photofield-agent.pid`. It then polls the server endpoint until
ready (up to 30s). `server stop` reads the PID file and sends SIGTERM.
`server kill` sends SIGKILL to the PID and anything else listening on the port.

**Important:** The server does **not** auto-scan. After starting, run a scan:

```bash
./photofield -scan test
# or from another directory:
AGT_BIN=/path/to/photofield ./tools/agent.sh server start && ./photofield -scan test
```

The server listens on port `8080` by default (override with `AGT_PORT`).

**Note:** With `AGT_START=true` (the default), `mcp call`, `mcp quick`, and
`mcp shell` will auto-start the server if it is not already running. The
`server start`, `server stop`, `server restart`, `server status`, and
`server kill` commands manage the process regardless of `AGT_START`.

### Stale PID cleanup

If the server crashes without a clean shutdown, the PID file may become stale.
The harness auto-detects this: `server start` will clean up a dead PID file
and launch a new instance. `server status` also removes stale entries.

## 4. MCP Tool Calls

Use `./tools/agent.sh mcp` to call MCP tools. The harness handles the session
handshake (initialize + initialized notification), session ID extraction from
response headers, SSE response parsing, and named-arg to JSON conversion.

### Tool commands

```bash
# Call a tool with JSON args
./tools/agent.sh mcp call list_collections '{}'

# Call with named args (auto-detects --key val pairs)
./tools/agent.sh mcp call search_photos --query 'beach' --collection_id 'test' --limit 3

# Verbose mode — shows full raw JSON response
./tools/agent.sh --verbose mcp call get_photo --file_id 1 --w 200

# Smoke test (calls list_collections by default)
./tools/agent.sh mcp quick

# Smoke test with a specific tool
./tools/agent.sh mcp quick get_photo --file_id 1

# Interactive REPL
./tools/agent.sh mcp shell
```

### Argument modes

| Mode | Syntax |
|------|--------|
| JSON | `./tools/agent.sh mcp call <tool> '<json>'` |
| Named | `./tools/agent.sh mcp call <tool> --key val` |

Named args are auto-converted to JSON: numbers stay numeric, `true`/`false`
become booleans, `null` stays null, everything else is quoted as strings.
`agent.sh` auto-detects the named-arg mode when the first token starts with
`--`, so no explicit `-- --` boundary is ever needed.

### Output

Non-verbose mode shows a clean summary:
```
✓ list_collections — 22 items
✓ events — 19 events
✓ get_photo
```

Errors show with a red ✗ and the error message. Set `--verbose` (see Verbose Flag above) for full raw JSON on every call.

**Output streams:** The `log_*` helpers (`ℹ`, `▶`) go to stderr. Tool result
summaries (`✓`, `✗`) and raw JSON output go to stdout. This lets you pipe tool
results: `./tools/agent.sh mcp call list_collections '{}' | jq '.collections'`.

### From another directory

```bash
AGT_BIN=/path/to/photofield ./tools/agent.sh mcp call list_collections '{}'
AGT_URL=http://remote-host:9000/mcp ./tools/agent.sh mcp call list_collections '{}'
```

## 5. Generic API Calls

Use `./tools/agent.sh api` for arbitrary HTTP calls to any server endpoint. This is
useful for testing non-MCP routes, debugging, or calling endpoints that don't
have a dedicated tool.

```bash
# GET request
./tools/agent.sh api GET http://localhost:8080/api/health

# POST with JSON body
./tools/agent.sh api POST http://localhost:8080/api/collections \
  '{"name":"my-collection","dirs":["/path/to/photos"]}'

# POST with named args (auto-constructs JSON body)
./tools/agent.sh api POST http://localhost:8080/api/collections \
  --name my-collection --dirs /path/to/photos

# PUT / DELETE
./tools/agent.sh api DELETE http://localhost:8080/api/collections/test
```

The output shows the HTTP status code, pretty-printed JSON when possible, and
the response body (truncated if over 500 chars). Note: `AGT_VERBOSE=1` does
not currently change the truncation behavior for API calls.

## 6. Test the Server

### Tool tests

```bash
# Basic call
./tools/agent.sh mcp call get_photo --file_id 1

# Metadata-only call
./tools/agent.sh mcp call get_photo_metadata --file_id 1

# Error handling
./tools/agent.sh mcp call get_photo --file_id 999999

# Verbose debugging
./tools/agent.sh --verbose mcp call search_photos --query 'test' --collection_id 'test'
```

### API tests

```bash
# Check health
./tools/agent.sh api GET http://localhost:8080/api/health

# List collections via API (alternative to mcp call)
./tools/agent.sh api GET http://localhost:8080/api/collections
```

## 7. Inspect Errors and Crashes

The harness captures the server's **entire stdout and stderr** to
`/tmp/photofield-agent.log` via `nohup`. Panics and errors appear in this log:

```bash
tail -100 /tmp/photofield-agent.log
```

### Session warnings

If the server does not return a `Mcp-Session-Id` header, the harness continues
without one (some MCP servers don't require it). You will see a benign info
message: `No session ID (server may not require one)`.

| Symptom | Cause | Fix |
|---------|-------|-----|
| `cannot create context from nil parent` | Nil context passed to `WithTimeout` | Add `if ctx == nil { ctx = context.Background() }` in handler |
| `file not found: N` | Photo ID doesn't exist | Scan collection or check DB |
| Empty response data | Rendering panic | Check server log |
| Schema says all fields required | SDK infers from Go struct pointers | Use explicit `InputSchema` in tool registration |
| Server not responding | Old binary running | `./tools/agent.sh server kill` then rebuild |
| No photos found | Config points to empty dirs | Create `data/configuration.yaml` |

## 8. Inspect Runtime State

### Database

```bash
sqlite3 data/photofield.cache.db "SELECT id, width, height FROM infos ORDER BY id;"
sqlite3 data/photofield.cache.db ".tables"
```

### Via the harness

```bash
# List collections
./tools/agent.sh mcp call list_collections '{}'

# Check events for a collection
./tools/agent.sh mcp call events --collection_id 'test'

# Search photos
./tools/agent.sh mcp call search_photos --query 'faces' --collection_id 'test' --limit 5
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `AGT_PORT` | `8080` | Server port |
| `AGT_BIN` | `./photofield` | Path to binary |
| `AGT_DATA_DIR` | `./data` | Data directory |
| `AGT_START` | `true` | Auto-start server if not running |
| `AGT_URL` | (derived) | Full MCP endpoint URL |
| `AGT_API_BASE` | `http://localhost:$PORT` | Base URL for API calls |
| `AGT_VERBOSE` | `0` | Verbose output |

## Quick Reference

| Command | Purpose |
|---------|---------|
| `go build -o photofield .` | Build the server |
| `./photofield -scan <name>` | Scan a collection |
| `./tools/agent.sh server start` | Start the server |
| `./tools/agent.sh server stop` | Stop the server |
| `./tools/agent.sh server restart` | Restart the server |
| `./tools/agent.sh server status` | Show PID/port status |
| `./tools/agent.sh server kill` | Kill server processes |
| `./tools/agent.sh mcp call <tool> <args>` | Call an MCP tool |
| `./tools/agent.sh mcp quick [tool]` | Smoke test |
| `./tools/agent.sh mcp shell` | Interactive REPL |
| `./tools/agent.sh api <method> <url> [body]` | Generic HTTP call |
| `./tools/agent.sh --verbose <cmd>` | Verbose output (see Verbose Flag above for `-v`/`-V`/env var) |
| `sqlite3 data/photofield.cache.db ...` | Inspect the database |
