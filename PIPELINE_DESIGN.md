# Image Indexing Pipeline - Final Design

## Implementation Summary

A new **`internal/image/pipeline`** submodule implements a clean, typed, **sequential batch architecture** for photo indexing.

## Key Improvements

### 1. **Sequential Stage Execution**
- Pipeline runs in **three sequential stages**
- Each stage completes ALL files before next stage begins
- Prevents race conditions and ensures data consistency

**Flow**:
```
Stage 1: Metadata → (complete all) → 
Stage 2: Contents → (complete all) → 
Stage 3: Faces   → (complete)
```

### 2. **Streaming from Database**
- All candidates query from DB using channels (`<-chan FileRef`)
- No materialized lists in memory (handles 40k+ photos efficiently)
- File indexing remains separate (can be integrated later)

### 3. **Strongly Typed Stages**
Each stage has explicit input/output types:
```go
FileRef          → Basic file reference (id + path)
FileWithMeta     → File + EXIF metadata
FileWithThumb    → File + thumbnail reader + orientation
FileWithContents → File after color/embedding extraction
```

### 4. **Functions Return Channels**
Cleaner signatures make dependencies explicit:
```go
// Before (unclear)
func metadataWorkers(in <-chan X, out chan<- Y, errs chan<- error)

// After (clear, no error channels)
func MetadataWorkers(in <-chan FileRef) <-chan FileWithMeta
```

**Note**: Errors are now logged immediately via `log.Printf()` instead of being returned via error channels. This prevents race conditions and simplifies error handling.

### 5. **Database-Sourced Metadata**
**Key change**: Thumbnail and contents stages read metadata from DB, not from metadata channel:

```go
// Stage 1: Extract metadata, write to DB
metaOut := MetadataExtractWorkers(...)
drainAndWaitStage("Metadata", metaOut)  // Wait for completion

// Stage 2: Re-source files, load metadata from DB
files := FileSource(...)
metaOut := MetadataLoadWorkers(ctx, db, files, workers)  // From DB!
```

This ensures metadata is persisted before thumbnails/contents begin processing.

### 6. **Selective Reindexing**
Force flags control **what** gets reprocessed:
- `ForceColor=true` → Reindex colors, reuse existing thumbnails and embeddings
- Stages check DB for existing data unless force flag set
- Thumb stage always tries to load before generating

### 7. **Faces After Contents**
Face detection waits for **ALL contents to complete**:

```go
// Stage 2: Contents
drainAndWaitStage("Contents", contentsOut)  // Wait!

// Stage 3: Faces (only starts after contents done)
if intent.NeedsFaces() {
    files := FileSource(...)
    FaceWorkers(...)
}
```

### 8. **No EXIF Thumbnail Extraction**
Thumbnails come from:
1. SQLite cache (fast path)
2. Generated from original (using metadata orientation)

EXIF thumbnails removed (unreliable quality/dimensions).

### 9. **Database Method Calls**
No raw SQL generation—pipeline calls database interface methods:
```go
type Database interface {
    ListMissingMetadata(dirs []string, max int) <-chan MissingInfo
    ListMissingContents(dirs []string, max int) <-chan MissingInfo
    HasColor(id ImageId) bool
    HasEmbedding(id ImageId) bool
    // ...
}
```
