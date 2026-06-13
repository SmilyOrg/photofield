# MCP Tools for Photofield

Four MCP tools expose photofield's photo library to AI agents:
`list_collections`, `events`, `search_photos`, `get_photo`.

## Tool Reference

### `list_collections`

List all photo collections with indexed counts and timestamps.

**Input:** `{}` (no parameters)

**Output:** Array of collections, each with `id`, `name`, `indexed_count`, `indexed_at`.

**Use this first** — the collection ID is required for all other tools.

---

### `events`

Split a collection's photos into time-bounded events. Photos on different
calendar days, or more than 1 hour apart (same day), form separate events.
Returns metadata only (counts, date ranges, locations) — not images.

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `collection_id` | string | yes | From `list_collections` |

**Output:** Array of `EventSummary` objects with `index`, `created_after`, `created_before`, `photo_count`, `location_count`, `locations`.

**Workflow:** `list_collections` → pick a collection → `events` → get high-level context → `search_photos` for details.

---

### `search_photos`

Search photos by natural language, image similarity (`img:N`), face similarity
(`face:N`), or structured qualifiers. Returns metadata summaries — not images.

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `collection_id` | string | yes | From `list_collections` |
| `query` | string | yes | Search query (see syntax below) |
| `sort` | string | no | `-date` (default, newest first), `+date`, `-similarity`, or `-similarity,+date` |
| `limit` | int | no | Max results (default 50). Use 10–20 for previews, 100–200 for full sets. |

**Query syntax:**
| Query | Meaning |
|-------|---------|
| `sunset beach` | Natural language search via CLIP embeddings |
| `created:2024-06` | Photos from June 2024 |
| `tag:vacation` | Photos tagged "vacation" |
| `filename:IMG_` | Files matching glob (supports `*` and `?`) |
| `img:123` | Visually similar to photo ID 123 |
| `face:456` | Similar to face ID 456 |
| `t:0.3` | Minimum similarity threshold (0.15–0.30) |
| `dedup:0.9` | Remove near-duplicates (<90% similarity) |

Qualifiers are combinable: `'created:2023-06..2023-08 tag:vacation'`.

**Output:** Array of `SearchResult` objects with `id`, `file_name`, `datetime`, `width`, `height`, `color`, `location`, `similarity`, `tags`.

**Workflow:** `search_photos` → examine results → `get_photo(file_id)` to see images.

---

### `get_photo`

Retrieve a photo as an embedded image with rich metadata. This is the **only**
tool that returns actual image data.

**Parameters:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file_id` | integer | yes | From `search_photos` results |
| `w` | int | no | Target width (1–4096). Omit for default 256px thumbnail. |
| `h` | int | no | Target height (1–4096). Omit for default 256px thumbnail. |
| `format` | string | no | `jpeg` (default), `png`, or `webp` |
| `crop_x` | int | no | Crop left edge in original image pixels |
| `crop_y` | int | no | Crop top edge in original image pixels |
| `crop_w` | int | no | Crop width in original image pixels |
| `crop_h` | int | no | Crop height in original image pixels |

**Default behavior:** Returns a 256×256 JPEG thumbnail. This is the recommended
default for browsing — fast and token-efficient. Only add `w`/`h` when you need
to inspect details (e.g., read text in a sign).

**Cropping:** All crop coordinates are in the **original** image's pixel space.
All four crop params must be specified together. The crop is applied before
resizing.

**Output:** The image is returned as an MCP `ImageContent` block. Structured
metadata includes `width`, `height`, `orig_width`, `orig_height`, `path`,
`filename`, `extension`, `video`, `created_at`, `tags`, `faces`, `latlng`,
`location`, `thumbnails`, `image_url`.

**Workflow:** `list_collections` → `search_photos` → `get_photo(file_id)` for
thumbnails → `get_photo(file_id, w=800, h=600)` only when you need details.

## Full Workflow Example

```
1. list_collections({})                              → pick "vacation"
2. events({collection_id: "vacation"})               → 12 events, "Paris, France"
3. search_photos({collection_id: "vacation", query: "eiffel tower"})  → 8 results
4. get_photo({file_id: 123})                         → thumbnail JPEG
5. get_photo({file_id: 123, w: 800, h: 600})        → larger preview
```
