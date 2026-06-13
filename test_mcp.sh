#!/bin/bash
set -e

BASE_URL="http://localhost:8080/mcp"

# Step 1: Initialize
echo "=== Step 1: Initialize ==="
INITIALIZE_RESP=$(curl -s -D- -X POST "$BASE_URL" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}')
echo "$INITIALIZE_RESP" | head -10
echo ""

# Extract session ID
SESSION_ID=$(echo "$INITIALIZE_RESP" | grep "Mcp-Session-Id" | tr -d '\r' | sed 's/.*Mcp-Session-Id: //')
echo "Session ID: $SESSION_ID"

# Step 2: Send initialized notification
echo ""
echo "=== Step 2: Send initialized notification ==="
curl -s -D- -X POST "$BASE_URL" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' 2>&1 | head -5
echo ""

# Step 3: Call get_photo
echo ""
echo "=== Step 3: Call get_photo ==="
curl -s -D- -X POST "$BASE_URL" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_photo","arguments":{"file_id":1,"w":400,"h":225,"format":"jpeg","crop_x":0,"crop_y":0,"crop_w":4000,"crop_h":2250}}}' 2>&1
