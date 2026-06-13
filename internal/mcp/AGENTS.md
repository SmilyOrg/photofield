# Agent Developer Workflow Guide

This guide covers running, testing, debugging, and iterating on the photofield MCP server.

## 1. Starting the Server

### Prerequisites

The server reads configuration from `data/configuration.yaml`. If it doesn't
exist, the server runs with defaults and the default collection config points
to every subdirectory of the current working directory (which indexes nothing
useful).

**Quick setup:**

```bash
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
go build -o photofield .
./photofield > /tmp/photofield.log 2>&1 &
sleep 5
```

**Important:** The server does **not** auto-scan photos. Run a scan first:

```bash
./photofield -scan test
```

The server listens on port `8080` by default. Kill with `pkill -f photofield`
before rebuilding.

## 2. Sending MCP Requests

The MCP server uses JSON-RPC 2.0 over HTTP with streamable transport. You need
a **session lifecycle**:

```bash
BASE="http://localhost:8080/mcp"

# Step 1: Initialize — gets a Session-Id back in headers
curl -v -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize",
       "params":{"protocolVersion":"2024-11-05",
                 "capabilities":{},
                 "clientInfo":{"name":"test","version":"1.0"}}}' \
  2>&1

# Step 2: Send initialized notification (no ID)
curl -s -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: ABC123" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# Step 3: Call any tool
curl -s -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: ABC123" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call",
       "params":{"name":"list_collections","arguments":{}}}'
```

### Optional fields

When a struct field is a **pointer** (`*int`, `*string`), the MCP SDK may still
mark it as required in the generated schema. Use the explicit `InputSchema` in
the tool registration (see `mcp.go`) to control the `required` array precisely.
When calling tools, include only the parameters the schema marks as required.

## 3. Inspecting Errors and Crashes

### Stack traces

Panics are caught and logged to **stderr**. Redirect stderr to a log file:

```bash
./photofield > /tmp/photofield.log 2>&1 &
tail -100 /tmp/photofield.log
```

Common patterns:

1. **"cannot create context from nil parent"** → Handler passes `nil` context.
   Fix: add `if ctx == nil { ctx = context.Background() }` in the handler.

2. **"file not found: N"** → File ID doesn't exist. Check `sqlite3 data/photofield.cache.db "SELECT id FROM infos;"`.

3. **Empty response data** → Rendering panicked silently. Check server log.

## 4. Checking Runtime State

### Database inspection

```bash
sqlite3 data/photofield.cache.db "SELECT id, width, height FROM infos ORDER BY id;"
sqlite3 data/photofield.cache.db ".tables"
```

### Collection status via MCP

```bash
curl -s -X POST "$BASE" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call",
       "params":{"name":"list_collections","arguments":{}}}'
```

## 5. Common Fixes

| Symptom | Cause | Fix |
|---------|-------|-----|
| `file not found: N` | Photo ID doesn't exist | Scan collection or check DB |
| Empty response data | Rendering panic | Check server log for panic |
| `cannot create context from nil parent` | Nil context passed to `WithTimeout` | Add `if ctx == nil { ctx = context.Background() }` |
| Schema says all fields required | SDK infers schema from Go struct pointers | Use explicit `InputSchema` |
| Server not responding | Old binary running | `pkill -f photofield` then rebuild |
| No photos found | Default config points to empty dirs | Create `data/configuration.yaml` |

## 6. Test Script

See `test_mcp.sh` in the repo root for a quick end-to-end test.
