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

## 2. Calling MCP Tools

Use `tools/mcp-test.sh` for all MCP tool calls. It handles the session
handshake, SSE parsing, and session ID management automatically.

### Basic usage

```bash
# Call a tool with JSON args
./tools/mcp-test.sh call list_collections '{}'

# Call with named args (auto-detects --key val pairs)
./tools/mcp-test.sh call search_photos --query 'beach' --collection_id 'test' --limit 3

# Verbose mode — always shows full JSON
./tools/mcp-test.sh --verbose call get_photo --file_id 1 --w 200

# Quick smoke test (list_collections only)
./tools/mcp-test.sh quick

# Interactive REPL
./tools/mcp-test.sh shell
```

### Arguments

- **JSON mode**: `./tools/mcp-test.sh call <tool> '<json_args>'`
- **Named args**: `./tools/mcp-test.sh call <tool> --key val` (auto-detected)
- **Explicit named**: `./tools/mcp-test.sh call <tool> -- --key val` (forces mode)

### Environment

| Variable | Default | Purpose |
|----------|---------|---------|
| `MCPT_PORT` | `8080` | Server port |
| `MCPT_BIN` | `./photofield` | Path to binary |
| `MCPT_DATA_DIR` | `./data` | Data directory |
| `MCPT_START` | `true` | Auto-start if not running |
| `MCPT_URL` | (derived) | Full URL (overrides PORT) |

### Output

Non-verbose mode shows a clean summary:
```
✓ list_collections — 22 items
✓ events — 19 events
✓ get_photo
```

Errors show with a red ✗ and the error message. Set `--verbose` for full JSON
on every call, or use it with `quick` for the full response.

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
# List all collections
./tools/mcp-test.sh call list_collections '{}'

# Check a specific collection's events
./tools/mcp-test.sh call events --collection_id 'test'

# Search photos
./tools/mcp-test.sh call search_photos --query 'faces' --collection_id 'test' --limit 5
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

## 6. Testing

### Quick smoke test

```bash
./tools/mcp-test.sh quick
```

### Manual tool testing

```bash
# Test a specific tool with arguments
./tools/mcp-test.sh call get_photo --file_id 1

# Test error handling
./tools/mcp-test.sh call get_photo --file_id 999999

# Verbose output for debugging
./tools/mcp-test.sh --verbose call search_photos --query 'test' --collection_id 'test'
```

### From another directory

The harness auto-detects the `photofield` binary relative to the repo root.
To call it from elsewhere:

```bash
MCPT_BIN=/path/to/photofield ./tools/mcp-test.sh call list_collections '{}'
```

Or use a custom URL:

```bash
MCPT_URL=http://remote-host:9000/mcp ./tools/mcp-test.sh call list_collections '{}'
```
