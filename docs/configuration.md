# Configuration

You can configure the app via `configuration.yaml`.

The location of the file depends on your installation method, see
[Quick Start](/quick-start).

## Minimal Example

The following is a minimal `configuration.yaml` example, see [Defaults](#defaults) for all options.

::: code-group
```yaml [configuration.yaml]
collections:
  # Normal Album-type collection
  - name: Vacation Photos
    dirs:
      - /photo/vacation-photos

  # Timeline collection (similar to Google Photos)
  - name: My Timeline
    layout: timeline
    dirs:
      - /photo/myphotos
      - /exampleuser

  # Shuffle collection that changes daily
  - name: Daily Shuffle
    sort: +shuffle-daily
    dirs:
      - /photo/all-photos

  # Create collections from sub-directories based on their name
  - expand_subdirs: true
    expand_sort: desc
    dirs:
      - /photo
```

:::

## Environment Variables

Some settings can only be configured via environment variables, not through
`configuration.yaml`.

| Variable | Default | Description |
|---|---|---|
| `PHOTOFIELD_DATA_DIR` | `.` | Directory where `configuration.yaml` and the cache databases are stored |
| `PHOTOFIELD_ADDRESS` | `:8080` | Server listen address in `host:port` format. Set to e.g. `:1200` to listen on a different port |
| `PHOTOFIELD_API_PREFIX` | `/api` | HTTP path prefix for the API |
| `PHOTOFIELD_CORS_ALLOWED_ORIGINS` | _(none)_ | Comma-separated list of origins allowed via CORS, e.g. `http://localhost:5173` |
| `PHOTOFIELD_DOCS_URL` | `/docs/usage` | URL for the docs link shown in the UI |
| `PHOTOFIELD_DOCS_PATH` | _(none)_ | Rewrites internal `/docs/` links to the given path (useful when hosting docs and the app at different paths) |

### Examples

Change the port to `1200`:

::: code-group
```sh [Linux / macOS]
PHOTOFIELD_ADDRESS=:1200 ./photofield
```
```bat [Windows]
set PHOTOFIELD_ADDRESS=:1200
photofield.exe
```
```yaml [docker-compose.yaml]
services:
  photofield:
    image: ghcr.io/smilyorg/photofield:latest
    ports:
      - 1200:1200
    environment:
      - PHOTOFIELD_ADDRESS=:1200
```
:::

Change the data directory:

::: code-group
```sh [Linux / macOS]
PHOTOFIELD_DATA_DIR=/path/to/data ./photofield
```
```bat [Windows]
set PHOTOFIELD_DATA_DIR=C:\path\to\data
photofield.exe
```
```yaml [docker-compose.yaml]
services:
  photofield:
    image: ghcr.io/smilyorg/photofield:latest
    environment:
      - PHOTOFIELD_DATA_DIR=/app/data
    volumes:
      - /path/to/data:/app/data
```
:::

## Defaults

<<< @/../defaults.yaml

