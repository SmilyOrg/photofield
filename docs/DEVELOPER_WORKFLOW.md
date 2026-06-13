# Developer Workflow Guide

This document summarizes common pitfalls and best practices for running, testing,
debugging, and iterating on the photofield MCP server.

## 1. Starting the Server

### Prerequisites

The server reads configuration from `data/configuration.yaml`. If it doesn't
exist, the server runs with defaults and the default collection config points
to every subdirectory of the current working directory (which indexes nothing
useful).

**Quick setup:**

```bash
# Create a minimal config pointing to a test photo directory
mkdir -p data
cat > data/configuration.yaml <<EOF
collections:
  - name: test
    dirs:
      - docs/assets
EOF
```

### Starting

```bash
# Build first
go build -o photofield .

# Start in background, capture output to a log file
./photofield > /tmp/photofield.log 2>&1 &

# Wait for startup
sleep 5

# Verify it's running
curl -s http://localhost:8080/mcp -X POST \
  -H "Content-Type: application/json" \
  -d '{"test":true}'
# Returns: "malformed payload: invalid message version tag..."
# This is expected - the MCP endpoint requires proper JSON-RPC protocol.
# A successful startup just means the server is listening.
```

### Important notes

- The server does **not** automatically scan photos on startup. Run the scan
  first (or use the UI):

  ```bash
  ./photofield -scan test 2>&1
  ```

- The server listens on port `8080` by default.

- **Kill the server before rebuilding:** `pkill -f photofield`

## 2. Sending MCP Requests

The MCP server uses JSON-RPC 2.0 over HTTP with a streamable transport. The
protocol requires a **session lifecycle**:

### Step-by-step

```bash
BASE="http://localhost:8080/mcp"

# Step 1: Initialize - gets a Session-Id back in the headers
curl -v -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize",
       "params":{"protocolVersion":"2024-11-05",
                 "capabilities":{},
                 "clientInfo":{"name":"test","version":"1.0"}}}' \
  2>&1

# Extract the Session-Id from the response headers (e.g., Mcp-Session-Id: ABC123)

# Step 2: Send the "initialized" notification (no ID)
curl -s -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: ABC123" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# Step 3: Call any tool using the same Session-Id
curl -s -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: ABC123" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call",
       "params":{"name":"list_collections","arguments":{}}}'
```

### Tool call parameters

- The MCP SDK infers the JSON Schema from Go struct types. If a struct field
  is a **pointer** (`*int`, `*string`), the SDK may still mark it as required
  in the generated schema. For tools with optional fields, use an **explicit
  `InputSchema`** in the `Tool` registration (see `internal/mcp/mcp.go`),
  which lets you control the `required` array precisely.

- When calling tools, **always include all parameters that the schema marks as
  required** (check via `tools/list`).

## 3. Inspecting Errors and Crashes

### Stack traces

When the server panics, it logs a full Go stack trace to **stderr**. Since the
server runs in the background, redirect stderr to a log file:

```bash
./photofield > /tmp/photofield.log 2>&1 &

# After an error:
tail -100 /tmp/photofield.log
```

The stack trace will show the panic chain. A typical error pattern:

```
get_photo handler recovered from panic: cannot create context from nil parent

runtime/debug.Stack()
  ...
context.WithTimeout({0x0, 0x0}, ...)    ← nil context!
photofield/internal/io/djpeg.Djpeg.Get(...)  ← called with nil ctx
photofield/internal/mcp/photo.go:251     ← photo.Draw(nil, ...)
```

### What to look for

1. **"cannot create context from nil parent"** → A handler is passing `nil` as
   the context to code that calls `context.WithTimeout/WithDeadline`. Fix: pass
   `context.Background()` as fallback, or ensure the caller provides a valid
   context.

2. **"file not found: N"** → The file ID doesn't exist in the database. Check
   `sqlite3 data/photofield.cache.db "SELECT * FROM infos;"` to find valid IDs.

3. **Empty response data** → The image rendering succeeded but produced empty
   output. Check if the photo exists and the dimensions are valid.

## 4. Checking Runtime State

### Database inspection

```bash
# List all indexed photos with IDs
sqlite3 data/photofield.cache.db "SELECT id, width, height FROM infos ORDER BY id;"

# Count photos
sqlite3 data/photofield.cache.db "SELECT COUNT(*) FROM infos;"

# See tables
sqlite3 data/photofield.cache.db ".tables"
```

### Collection status via MCP

```bash
# List all collections and their indexed count
curl -s -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call",
       "params":{"name":"list_collections","arguments":{}}}'
```

## 5. Quick Test Script

Save this as `test_mcp.sh` for fast iteration:

```bash
#!/bin/bash
set -e
BASE="http://localhost:8080/mcp"

# Initialize and get session ID
INIT=$(curl -v -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize",
       "params":{"protocolVersion":"2024-11-05",
                 "capabilities":{},
                 "clientInfo":{"name":"test","version":"1.0"}}}' 2>&1)
SESSION=$(echo "$INIT" | grep "Mcp-Session-Id" | awk '{print $2}' | tr -d '\r')
echo "Session: $SESSION"

# Send initialized notification
curl -s -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' > /dev/null

# Call tools
echo ""
echo "=== list_collections ==="
curl -s -X POST "$BASE" \
  -H "Mcp-Session-Id: $SESSION" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call",
       "params":{"name":"list_collections","arguments":{}}}'
echo ""
echo ""

echo "=== get_photo (file_id=1) ==="
curl -s -X POST "$BASE" \
  -H "Mcp-Session-Id: $SESSION" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call",
       "params":{"name":"get_photo","arguments":
         {"file_id":1,"w":400,"h":225,"format":"jpeg",
          "crop_x":0,"crop_y":0,"crop_w":1000,"crop_h":1000}}}'
echo ""
```

## 6. Common Fixes Checklist

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| `file not found: N` | Photo ID doesn't exist | Scan collection or check DB |
| Empty response data | Rendering panics silently | Check server log for panic |
| `cannot create context from nil parent` | Nil context passed to WithTimeout/Deadline | Add `if ctx == nil { ctx = context.Background() }` |
| Schema says all fields required | SDK infers schema from Go struct pointers | Override with explicit `InputSchema` |
| Server not responding | Old binary still running | `pkill -f photofield` then rebuild |
| No photos found | Default config points to empty dirs | Create `data/configuration.yaml` |

## 7. Recommended Improvements

The following changes would make the workflow significantly easier:

### a) Add an MCP health/status endpoint
A simple HTTP endpoint (e.g., `GET /mcp/health`) that returns whether the
server is up, how many collections are indexed, and last scan time. This
avoids needing to send a full MCP session just to check if the server is
running.

### b) Add a `GET /mcp/tools` endpoint
Expose the list of available tools as a simple HTTP JSON endpoint. Currently
you must go through the full MCP session lifecycle to discover available
tools and their schemas.

### c) Make the server rebuild-aware
Add a Makefile or Taskfile target that handles `pkill`, `go build`, and
`./photofield` in sequence. This prevents the common mistake of calling
tools on an outdated binary.

### d) Improve nil context handling globally
Instead of patching each handler, add a middleware or wrapper in the MCP
tool handler registration that ensures a non-nil context is always passed.
This would prevent the most common crash pattern.

### e) Add structured error responses to panics
Currently panics are caught and logged, but the MCP response is empty or
contains only an error message. Including the panic message and a hint about
what to check would make debugging much faster.

### f) Provide a CLI test harness
A small Go test binary or subcommand (e.g., `photofield test-mcp`) that
connects to the running server and runs a battery of tool calls with
assertions. This would make it easy to verify end-to-end functionality
without manual curl commands.

### g) Improve schema generation for optional fields
The MCP SDK's automatic schema generation marks pointer fields as required,
which is incorrect. Either:
- Fix the SDK to use `nullable: true` for pointer types and exclude them
  from the `required` array, or
- Document the pattern of using explicit `InputSchema` for tools with
  optional fields.

### h) Add a `--watch` flag for automatic reload
A flag that watches for binary changes and restarts the server, so you
can iterate without manually killing and restarting.
