# Photofield AI Coding Agent Instructions

## Project Overview
Photofield is a high-performance photo viewer that emphasizes speed and handling massive photo collections. It's a Go backend serving tiled image data via HTTP API to a Vue.js frontend using OpenLayers for seamless zooming. The architecture prioritizes read-only filesystem operations and SQLite caching for metadata.

## Architecture & Core Components

### Backend (Go)
- **`main.go`**: Monolithic HTTP server handling all API routes, tile rendering, and static file serving
- **`internal/collection/`**: Photo collection management and filesystem scanning
- **`internal/render/`**: Server-side tile rendering using Canvas library for progressive loading
- **`internal/layout/`**: Different photo arrangement algorithms (album, timeline, wall, map)
- **`io/`**: Pluggable I/O abstraction layer for thumbnails, caching, and image processing
- **SQLite databases**: Two separate DBs for metadata (`photofield.cache.db`) and thumbnails (`photofield.thumbs.db`)

### Frontend (Vue.js)
- **`ui/src/components/CollectionView.vue`**: Main photo grid component using OpenLayers for tiled rendering
- **`ui/src/api.js`**: API client for backend communication
- **OpenLayers integration**: Handles progressive multi-resolution loading and seamless zoom

## Key Development Patterns

### Configuration System
- `defaults.yaml` defines default settings merged with `configuration.yaml`
- Collections are the primary abstraction - each represents a set of directories with layout and metadata
- Configuration supports dynamic collection expansion from subdirectories
- Hot-reload via filesystem watchers in `config.go`

### I/O Abstraction Layer
The `io/` package provides a powerful abstraction for image processing:
```go
// Common pattern: chain I/O modules for different processing steps
reader := io.NewCached(io.NewThumb(io.NewGoImage()))
```
Key modules: `cached/`, `thumb/`, `djpeg/`, `exiftool/`, `ffmpeg/`, `sqlite/`

### Database Migrations
- Use numbered migration files in `db/migrations/` (up/down pattern)
- Separate thumbnail database with its own migrations in `db/migrations-thumbs/`
- Key tables: `infos` (photo metadata), `tag` (tagging system), `infos_tag` (many-to-many)

## Development Workflow

### Build Commands
- **Development**: `task dev` (runs both API and UI in watch mode)
- **API only**: `task watch` (auto-reload Go server)
- **UI only**: `task ui` or `cd ui && npm run dev`
- **Production build**: `task build` (includes embedded UI)

### Testing & Debugging
- End-to-end tests in `e2e/` using Playwright
- Profiling tools included: `tools/profile-*.ps1` scripts
- Built-in pprof endpoint at `/debug/pprof/`
- SQLite databases are human-readable for debugging

### Build Tags & Embedding
- `//go:embed` directives conditionally compile features:
  - `-tags embedui`: Embeds Vue.js UI build
  - `-tags embedgeo`: Embeds geolocation data
  - `-tags embeddocs`: Embeds documentation
- Use `task assets` to download required geo data before building with embedgeo

## Project-Specific Conventions

### File Organization
- No subdirectories in main package - keep `main.go` focused on HTTP routing
- Internal packages should be focused and avoid circular dependencies
- UI components mirror the layout types: album, timeline, wall, map views

### Performance Considerations
- Tile-based rendering requires careful coordinate system handling
- SQLite WAL mode for better concurrent access
- Prefer readonly filesystem operations - Photofield never modifies source photos
- Use `godirwalk` for fast directory scanning (1000-10000 files/sec)

### API Design
- RESTful endpoints with collection-scoped operations
- Tile coordinates follow standard web mercator pattern `/tiles/{z}/{x}/{y}`
- Support for progressive image quality via tile pyramids
- Search integration with optional AI backend for semantic search

## Common Tasks
- **Adding new layout**: Implement in `internal/layout/`, add to config options, create corresponding Vue component
- **New I/O module**: Follow the `io.Reader` interface pattern in the `io/` package
- **Database schema changes**: Create numbered migration files with both up and down SQL
- **Configuration options**: Add to struct in `config.go` and document in `defaults.yaml`

## External Dependencies
- **Required**: `exiftool` for metadata extraction, optionally `djpeg` for optimized JPEG processing
- **AI features**: Requires separate `photofield-ai` service for semantic search
- **Geo features**: Downloads boundary data from `tinygpkg-data` releases
