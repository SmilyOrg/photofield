# Photofield Search Architecture — Research Report

Research into the search feature used by the MCP tools `photos_search_photos` and
`photos_get_photo_metadata`, focused on the `created:` date qualifier, index
building, result matching, EXIF date/timezone handling, and dedup.

## 1. Query parsing

The parser lives in `internal/search/`:

- **`query.go`** — grammar. A `participle`-based parser (lexer rules at
  lines 33–46, `Parse` at lines 69–71). A term is either a quoted String, a
  `Word:Word...` Qualifier, or a bare Word. `Qualifier` struct at lines 27–30.
  `Words()` (lines 107–117) collects the bare words — that's the free text that
  later becomes a CLIP query.
- **`expression.go`** — typed/validated expression. `Expression` struct at
  lines 26–40 (`Created DateRange`, `Deduplicate Float32` for `dedup`,
  `Filenames Strings` for `filename`, `Text`, `Threshold` for `t`, etc.).
  `validQualifiers` at lines 42–53: `created, t, dedup, bias, k, filter, tag,
  filename, img, face`. `Expression()` at lines 74–133 does the mapping:
  `expr.Created = q.ExpressionDateRange("created")` (line 78),
  `expr.Filenames = q.ExpressionStrings("filename")` (line 103).
- **`date.go`** — the `created:` parser. `ExpressionDateRange` (lines 99–181)
  handles operators `>=`, `<=`, `>`, `<` (lines 120–146), `..` ranges
  (lines 149–164), and single dates. `parseFlexibleDate` (lines 184–260)
  handles `YYYY`, `YYYY-MM`, `YYYY-MM-DD` and `*` wildcards
  (`parseInRange` lines 263–283, `MIN_YEAR=10`, `MAX_YEAR=9000`).
  `normalizeDay` (lines 286–292) clamps e.g. `2023-04-31` → Apr 30.

Critical semantic: **ranges are half-open `[From, To)`** — for a non-start
date, `d = d.AddDate(0,0,1)` (e.g. line 231 for full dates), and **all range
bounds are built with `time.UTC`** (e.g. line 231:
`time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)`). Example:
`created:2025-12-10` → From `2025-12-10T00:00Z`, To `2025-12-11T00:00Z`
(see `date_test.go` line 63).

Wildcards (`*-12-25`, `2025-*-01`, etc.) are not resolved into a unix range at
parse time; they set `DateWildcard` flags and are resolved **per-row at match
time** by `DateWildcard.Apply` / `DateRange.Match` (`date.go` lines 14–90).

`filename:` is just a list of strings (`string.go`, `ExpressionStrings`
lines 37–60); the `*`/`?` → `%`/`_` conversion happens later in the DB layer
(`filenameToLikePattern`, `database.go` lines 2355–2380).

## 2. Index building and storage

- **Schema**: `db/migrations/000009_prefix_and_unix.up.sql` — `infos` table
  has `created_at_unix INTEGER` and `created_at_tz_offset INTEGER` (offset in
  **minutes**), plus a generated `created_at` text column
  (`datetime(created_at_unix + created_at_tz_offset*60, 'unixepoch')` +
  formatted offset) used for display. **`clip_emb`** table
  (`db/migrations/000011_clip_emb.up.sql`): `file_id`, `inv_norm INTEGER`,
  `embedding BLOB` — a flat BLOB table, **no vector index**; similarity is
  brute-force in Go.
- **Write path**: `internal/image/database.go`, `writePendingInfosSqlite`.
  The `updateMeta` prepared statement at lines 293–315 inserts/updates
  `created_at_unix` / `created_at_tz_offset`. Binding in the `UpdateMeta` case
  at lines 636–667:

  ```go
  // database.go:636
  case UpdateMeta:
      dir, file := filepath.Split(imageInfo.Path)
      _, timezoneOffsetSeconds := imageInfo.DateTime.Zone()
      ...
      updateMeta.BindInt64(5, imageInfo.DateTime.Unix())
      updateMeta.BindInt64(6, int64(timezoneOffsetSeconds/60))
  ```

  So the stored value is `DateTime.Unix()` (absolute instant) plus the zone
  offset attached at decode time.
- **Source of `DateTime`**: the indexing pipeline
  (`internal/image/pipeline/metadata.go` lines 27–44) calls
  `decoder.DecodeInfo(path, &info)` and immediately
  `db.Write(file.Path, info, img.UpdateMeta)`. The decoder chain is in
  `internal/image/decoder.go` (`NewDecoder` lines 32–45): exiftool if
  available, else go-exif.
  - `internal/image/exiftool-mostlygeek.go` — tag list at lines 52–62, in
    priority order: `SubSecDateTimeOriginal`, `DateTimeOriginal`,
    `EXIF:DateTimeOriginal`, `CreateDate` (XMP/EXIF),
    `XMP:DateTimeOriginal`, `GPSDateTime`, `TimeStamp`, `FileModifyDate`,
    `FileCreateDate`. The first-parsed date wins
    (`if info.DateTime.IsZero()` at line 121); date-handling switch at
    lines 117–145 (see §4).
  - `internal/image/goexif-rwcarlsen.go:44` —
    `info.DateTime, _ = x.DateTime()` (go-exif returns UTC for offsetless
    EXIF strings).
- **CLIP embeddings**: computed from thumbnails in
  `internal/image/pipeline/contents.go` (`Process` lines 55–90:
  `p.aiService.EmbedImageReader(thumb.Thumb)` then `db.WriteAI`). Stored via
  the `UpdateAI` case at `database.go` lines 679–694
  (`Embedding.InvNormUint16()` + `Byte()` blob → `clip_emb`).

## 3. Result matching / fetch — where the date filter is applied

All search paths (MCP `search_photos` → `collection.Search` →
`source.ListInfos` → `database.List` → `listWithPrefixIds`) funnel into
`internal/image/database.go`, `listWithPrefixIds` (starts line 1963):

- **Date filter is in SQL, on unix seconds** (lines 2090–2098):

  ```go
  if !options.Expression.Created.From.IsZero() {
      sql += `AND created_at_unix >= :created_from`
  }
  if !options.Expression.Created.To.IsZero() {
      sql += `AND created_at_unix < :created_to`
  }
  ```

  Bound at lines 2195–2202 with `options.Expression.Created.From.Unix()` /
  `.To.Unix()` — i.e. **UTC-midnight bounds** compared against the stored
  unix instant.
- **Wildcard `created:` values get a second, in-Go post-filter** per row
  (lines 2256–2262):

  ```go
  unix := stmt.ColumnInt64(5)
  timezoneOffset := stmt.ColumnInt(6)
  info.DateTime = time.Unix(unix, 0).In(time.FixedZone("", timezoneOffset*60))

  // Search post-query expression filtering
  if !options.Expression.Created.Match(info.DateTime) {
      continue
  }
  ```

  `DateRange.Match` (`date.go:82-96`) only does work when wildcards are
  present; the bounds are still resolved in UTC (`Apply` uses
  `reference.Location()`, and From/To are UTC).
- **Text + `created:` combination**: free text is embedded via CLIP
  (`internal/collection/search.go` `Search` lines 96–170):
  `search.Parse` (line 107) → `q.Expression()` (line 112) →
  `source.Clip.EmbedText(expr.Text)` (line 136) → passed as `ImageEmbedding`
  in `ListOptions`. In `listWithPrefixIds`, each row's `clip_emb` embedding is
  cosine-compared in Go (`database.go` lines 2273–2299,
  `ai.CosineSimilarityFloat32Float32` at 2287; `t:` threshold at 2293).
  **No vector index** — it's a full scan of the date/tag/filename-filtered
  set. Note the interaction: when sort is *not* similarity, `LIMIT` is applied
  in SQL (lines 2156–2163), so threshold-filtered rows are lost from the
  limited page ("silently discarded", matching the MCP description). When sort
  *is* `-similarity`, `opts.Limit = 0` (line 2415) so all rows in the date
  window are fetched, scored, sorted in Go (lines 2351–2367 / 2411–2428) and
  then limited.
- MCP handlers: `internal/mcp/mcp.go` — `search_photos` tool registered around
  line 70 (description incl. `created:`/`dedup:` docs at lines 75–88), handler
  `searchPhotosHandler`; `get_photo_metadata` handler in
  `internal/mcp/photo.go` line 164, `CreatedAt` formatted at line 535:
  `info.DateTime.Format("2006-01-02T15:04:05Z07:00")` where `info.DateTime`
  comes from `GetInfo` (`database.go:1112`:
  `time.Unix(unix,0).In(FixedZone(offset*60))`).
- The HTTP API path (`main.go:1325` `loadScene` and
  `internal/scene/sceneSource.go:104–137`) uses the same parser/DB path.

## 4. EXIF DateTimeOriginal parsing and timezone handling

**`internal/image/decoder.go:48-73` — `parseDateTime`:**

```go
func parseDateTime(value string) (t time.Time, hasTimezone bool, hasSubsec bool, err error) {
    t, err = time.Parse("2006:01:02 15:04:05Z07:00", value)   // with offset, e.g. XMP
    ...
    t, err = time.Parse("2006:01:02 15:04:05", value)          // NO offset → Go returns UTC
    ...
```

This is the crux: **a bare EXIF `DateTimeOriginal` like
`2023:06:01 12:00:00` (no offset — the norm for cameras) is parsed as *UTC***,
because Go's `time.Parse` with a zone-less layout returns a UTC time. So a
photo taken at 12:00 JST is stored as the instant 12:00 **UTC** — 9 hours
later than its true instant, and its `created_at_tz_offset` is 0 (UTC).
Consequence for search: since `created:` bounds are also UTC wall-clock, the
search "date" for these photos equals the camera's *local wall-clock date*
(the +9h offset is baked in on both sides) — internally consistent, but any
mixing with offset-carrying dates breaks it.

**`internal/image/exiftool-mostlygeek.go:117-145` — multi-date reconciliation:**

```go
if strings.Contains(name, "Date") || strings.Contains(name, "Time") {
    if info.DateTime.IsZero() {
        t, _, _, err := parseDateTime(value)
        if err == nil {
            info.DateTime = t
        }
    } else if name != "GPSDateTime" && name != "FileModifyDate" && name != "FileCreateDate" {
        t, hasTimezone, _, err := parseDateTime(value)
        if err == nil && info.DateTime.Location() == time.UTC {
            if hasTimezone {
                // Prefer time with timezone if available
                info.DateTime = t
            } else {
                // If there are two times that are more than 10 minutes apart and
                // the first one doesn't have a timezone, it's likely that the
                // first one is local time and the second one is UTC, so we add
                // the timezone to the first one.
                d := info.DateTime.Sub(t)
                if d.Abs() > 10*time.Minute {
                    d = d.Truncate(time.Minute)
                    info.DateTime = info.DateTime.Add(-d).In(time.FixedZone("", int(d.Seconds())))
                }
            }
        }
    }
}
```

This is a second skew source: if the *first* date found is the bare EXIF
`DateTimeOriginal` (UTC-misinterpreted local time) and a *later* tag carries a
real offset (e.g. XMP `CreateDate ... +09:00`), **the offset-carrying value
replaces it** (lines 130–131). Then `DateTime.Unix()` is the *true* UTC
instant, while the `created:` search bounds remain UTC wall-clock — for a JST
camera that means photos taken 00:00–08:59 JST fall on the *previous* UTC
day (a ±9h/day-boundary skew). Conversely, if only the offsetless EXIF tag
exists, the stored instant is "local time as UTC" (offset 0), which is
self-consistent with UTC-bounded search but wrong as an absolute time
(displayed with a `+00:00` offset).

The 10-minute heuristic (lines 132–143) attempts to attach the diff between
two offsetless times (e.g. EXIF local vs a UTC-stamped tag) as a fixed zone —
`info.DateTime.Add(-d).In(time.FixedZone("", int(d.Seconds())))`.

Display side: everywhere the row is read back, `created_at_unix` +
`created_at_tz_offset` are recombined: `database.go` lines 1112, 2258, 2577 —
`time.Unix(unix, 0).In(time.FixedZone("", timezoneOffset*60))` — so
`get_photo_metadata`'s `CreatedAt` (photo.go:535) shows whatever offset was
stored (often `+00:00` even for JST photos).

## 5. Dedup (`dedup:` qualifier)

- Parsed as a **float threshold** in `internal/search/expression.go`:
  `Deduplicate Float32` (line 30),
  `expr.Deduplicate = q.ExpressionFloat32("dedup")` (line 84). Documented in
  `internal/mcp/mcp.go:85` ("keeps only photos <90% similar to each other"
  for `dedup:0.9`).
- Implemented in `internal/image/database.go` inside `listWithPrefixIds`:
  - Embeddings are force-joined when dedup is present (line 2020:
    `if options.ImageEmbedding != nil || options.Expression.Deduplicate.Present { joinEmbeddings = true }`).
  - The actual dedup is sequential, comparing each row only against the
    **immediately preceding kept row's embedding** (lines 2300–2313):

    ```go
    if options.Expression.Deduplicate.Present {
        if lastEmb != nil {
            sim, err := ai.CosineSimilarityFloat32Float32(lastEmb, lastEmbInvNorm, ee, einv)
            ...
            if sim >= options.Expression.Deduplicate.Value {
                continue
            }
        }
        lastEmb = ee
        lastEmbInvNorm = einv
    }
    ```

  So it's a 1-pass, adjacent-in-result-order filter, run *after* the SQL LIMIT
  when sort isn't similarity — dedup quality depends on the row order (default:
  date order), and a near-duplicate separated from its neighbor by another
  photo will not be suppressed.

## Summary of the likely ~9h skew

1. `parseDateTime` (`internal/image/decoder.go:48`) treats offsetless EXIF
   dates as UTC → `created_at_unix` = camera-local wall time (0 offset).
2. The exiftool reconciliation (`exiftool-mostlygeek.go:126-143`) can swap in
   an offset-carrying date (e.g. XMP with `+09:00`), converting the stored
   instant to true UTC — after which the same camera's photos sit 9h away
   from the no-offset ones.
3. Search bounds are always UTC-midnight over `created_at_unix`
   (`database.go:2090-2098`, `date.go` builds ranges in `time.UTC`), so the
   effective "date" of a photo depends entirely on which representation got
   stored: for offset-0 rows it's the camera's local date; for
   offset-corrected rows it's the UTC date (skewed by +9h for JST), putting
   the first 9 hours of each JST day into the previous UTC day.
