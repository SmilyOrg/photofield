---
name: local-dev
description: >-
  Run, test, and debug the photofield server locally. Use when building,
  running, or testing the server, making API calls, calling MCP tools, or
  inspecting runtime state. Covers server lifecycle via `task agent -- server`,
  generic HTTP calls via `task agent -- api`, MCP tool calls, database inspection,
  error debugging, and common fixes.
---

# Local Development — Running and Testing the Server

This skill covers building, running, testing, and debugging the photofield
server on your local machine.

All tool invocation, server management, and API calls go through
`task agent`, which forwards arguments to `tools/agent.sh` — a unified
harness that handles the server lifecycle, generic HTTP calls, and MCP tool
invocation with session management, SSE parsing, and named-arg parsing.

## The `--` Separator — Read This First

**Every command starts with `task agent --`.** The `--` after `agent` is
mandatory — there is no form that omits it. Without it, the `task` runner
tries to find a task named after the next word and fails:

```bash
# ❌ FAILS — task runner looks for a task named "server"
task agent server status
task: Task "server" does not exist

# ✅ WORKS — `--` tells task that "server" is an argument to the `agent` task
task agent -- server status
```

### Two `--` in one command

Some commands end up with two `--` in a row. They serve different roles:

| `--` | Role | Example |
|------|------|--------|
| **First `--`** | Task separator — required for every command | `task agent -- server status` |
| **Second `--`** | (rare) Explicit named-arg boundary | `task agent -- mcp call -- --key val` |

The second `--` is only needed when you want to force `agent.sh` into
named-arg mode even if the first arg doesn't start with `--`. In practice,
`agent.sh` auto-detects named args, so the second `--` is almost never
needed.

### Verbose flag forms

The verbose flag accepts `--verbose`, `-v`, and `-V`. It must come **after**
the task separator `--`:

```bash
task agent -- --verbose mcp call get_photo --file_id 1
task agent -- -v mcp call get_photo --file_id 1
task agent -- -V mcp call get_photo --file_id 1
```

Or avoid the flag entirely via environment variable:
```bash
AGT_VERBOSE=1 task agent -- mcp call get_photo --file_id 1
```

## 1. Build

```bash
go build -o photofield .
```

Kill any old instance before rebuilding:

```bash
task agent -- server kill
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

Use `task agent -- server <command>` to manage the server process. Every
call requires the `--` after `agent`:

```bash
# Start (auto-detects if already running)
task agent -- server start

# Stop gracefully (uses PID file)
task agent -- server stop

# Restart
task agent -- server restart

# Check status (shows PID and port listeners)
task agent -- server status

# Aggressive kill (PID file + all port listeners including exiftool)
task agent -- server kill
```

**How it works:** `server start` launches the binary with `nohup` and writes a
PID file to `/tmp/photofield-agent.pid`. It then polls the server endpoint until
ready (up to 30s). `server stop` reads the PID file and sends SIGTERM.
`server kill` sends SIGKILL to the PID and anything else listening on the port.

**Important:** The server does **not** auto-scan. After starting, run a scan:

```bash
./photofield -scan test
# or from another directory:
AGT_BIN=/path/to/photofield task agent -- server start && ./photofield -scan test
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

Use `task agent -- mcp` to call MCP tools. The harness handles the session
handshake (initialize + initialized notification), session ID extraction from
response headers, SSE response parsing, and named-arg to JSON conversion.

### Tool commands

```bash
# Call a tool with JSON args
task agent -- mcp call list_collections '{}'

# Call with named args (auto-detects --key val pairs)
task agent -- mcp call search_photos --query 'beach' --collection_id 'test' --limit 3

# Verbose mode — shows full raw JSON response
task agent -- --verbose mcp call get_photo --file_id 1 --w 200
# Also accepts -v or -V: task agent -- -v mcp call get_photo --file_id 1
# Or via env (no --verbose flag at all):
# AGT_VERBOSE=1 task agent -- mcp call get_photo --file_id 1 --w 200

# Smoke test (calls list_collections by default)
task agent -- mcp quick

# Smoke test with a specific tool
task agent -- mcp quick get_photo --file_id 1

# Interactive REPL
task agent -- mcp shell
```

### Argument modes

| Mode | Syntax |
|------|--------|
| JSON | `task agent -- mcp call <tool> '<json>'` |
| Named | `task agent -- mcp call <tool> --key val` |

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

Errors show with a red ✗ and the error message. Set `--verbose` (or `-v`
or `-V`) for full raw JSON on every call.

**Output streams:** The `log_*` helpers (`ℹ`, `▶`) go to stderr. Tool result
summaries (`✓`, `✗`) and raw JSON output go to stdout. This lets you pipe tool
results: `task agent -- mcp call list_collections '{}' | jq '.collections'`.

### From another directory

```bash
AGT_BIN=/path/to/photofield task agent -- mcp call list_collections '{}'
AGT_URL=http://remote-host:9000/mcp task agent -- mcp call list_collections '{}'
```

## 5. Generic API Calls

Use `task agent -- api` for arbitrary HTTP calls to any server endpoint. This is
useful for testing non-MCP routes, debugging, or calling endpoints that don't
have a dedicated tool.

```bash
# GET request
task agent -- api GET http://localhost:8080/api/health

# POST with JSON body
task agent -- api POST http://localhost:8080/api/collections \
  '{"name":"my-collection","dirs":["/path/to/photos"]}'

# POST with named args (auto-constructs JSON body)
task agent -- api POST http://localhost:8080/api/collections \
  --name my-collection --dirs /path/to/photos

# PUT / DELETE
task agent -- api DELETE http://localhost:8080/api/collections/test
```

The output shows the HTTP status code, pretty-printed JSON when possible, and
the response body (truncated if over 500 chars). Note: `AGT_VERBOSE=1` does
not currently change the truncation behavior for API calls.

## 6. Test the Server

### Smoke test

```bash
task agent -- mcp quick
```

### Tool tests

```bash
# Basic call
task agent -- mcp call get_photo --file_id 1

# Metadata-only call
task agent -- mcp call get_photo_metadata --file_id 1

# Error handling
task agent -- mcp call get_photo --file_id 999999

# Verbose debugging
task agent -- --verbose mcp call search_photos --query 'test' --collection_id 'test'
```

### API tests

```bash
# Check health
task agent -- api GET http://localhost:8080/api/health

# List collections via API (alternative to mcp call)
task agent -- api GET http://localhost:8080/api/collections
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
| Server not responding | Old binary running | `task agent -- server kill` then rebuild |
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
task agent -- mcp call list_collections '{}'

# Check events for a collection
task agent -- mcp call events --collection_id 'test'

# Search photos
task agent -- mcp call search_photos --query 'faces' --collection_id 'test' --limit 5
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
| `task agent -- server start` | Start the server |
| `task agent -- server stop` | Stop the server |
| `task agent -- server restart` | Restart the server |
| `task agent -- server status` | Show PID/port status |
| `task agent -- server kill` | Kill server processes |
| `task agent -- mcp call <tool> <args>` | Call an MCP tool |
| `task agent -- mcp quick [tool]` | Smoke test |
| `task agent -- mcp shell` | Interactive REPL |
| `task agent -- api <method> <url> [body]` | Generic HTTP call |
| `task agent -- -v <cmd>` | Verbose output (`--verbose`, `-V` also accepted) |
| `sqlite3 data/photofield.cache.db ...` | Inspect the database |
