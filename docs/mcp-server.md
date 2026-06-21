# MCP Server

The Photofield MCP server lets LLM agents search and retrieve photos from your Photofield instance through a simple, agent-friendly interface that avoids manual browser navigation and raw API calls. It runs on the same port as the main server at `/mcp` by default.

> **Experimental.** The MCP integration is early and not yet fully explored. It is a read-only interface, so feel free to experiment without worry.

## Use Cases

- **Find specific moments**: "Find the selfie I made in front of a brachiosaurus at the zoo" or "Show me photos where I'm wearing the red hat." The agent uses semantic search to locate exactly what you're thinking of.
- **Trip exploration**: "What did I do in Barcelona on the 14th of July?" The agent groups photos into chronological events, extracts location names, and walks you through your day.
- **Photo retrieval**: "Show me that picture of the sunset behind the lighthouse from the Sicily trip." The agent finds and displays the actual image.
- **Collection overview**: "How many photos do I have in total?" or "What collections are available?" The agent inspects collection metadata without user intervention.

## What It Does

Five tools are available:

| Tool | Purpose |
|---|---|
| `list_collections` | List all photo collections, their IDs, and indexed photo counts |
| `events` | Split a collection into chronological events (by day and location) |
| `search_photos` | Search photos using natural language, image similarity, or face similarity |
| `get_photo_metadata` | Retrieve structured metadata for a single photo: tags, faces, GPS coordinates, dimensions |
| `get_photo` | Retrieve an actual photo image as a base64-encoded image, with options for resizing and cropping |

## Client Configuration

Point your MCP client at the Photofield MCP endpoint:

```json
{
  "mcpServers": {
    "photofield": {
      "url": "http://localhost:8080/mcp",
      "transport": "http",
      "directTools": true
    }
  }
}
```

## AI / Semantic Search

Text-based semantic search requires the [photofield-ai](https://github.com/SmilyOrg/photofield-ai) server to be running and configured in `configuration.yaml`:

```yaml
ai:
  textual:
    host: http://localhost:8081
  visual:
    host: http://localhost:8081
  faces:
    host: http://localhost:8081
```

Without the AI server, `search_photos` still works for tag, date, and filename filters, but text-based semantic search and face/image similarity searches will return no results.

## Health Check

The server exposes a health check endpoint:

| Path | Method | Description |
