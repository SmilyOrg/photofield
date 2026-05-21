# Image Indexing Pipeline

Clean, typed, sequential batch architecture for indexing photo metadata, thumbnails, contents (color/AI), and faces.

## Architecture

### Design Principles

1. **Strongly typed stages**: Each stage has explicit input/output types
2. **Streaming from database**: All data flows through channels from DB queries
3. **Sequential batch processing**: Each stage completes fully before next begins
4. **Selective reindexing**: Force flags control what gets reprocessed
5. **Database-sourced metadata**: Stages read metadata from DB, not passed through channels

### Pipeline Flow (Sequential Stages)

```
Stage 1: Metadata Extraction
┌─────────────┐
│ FileSource  │ Query DB for candidates
└──────┬──────┘
       │ <-chan FileRef
       ▼
┌─────────────┐
│  Metadata   │ Extract EXIF OR load from DB
│   Workers   │ Writes to DB immediately
└─────────────┘
       │ (Wait for ALL files to complete)
       │
       ▼

Stage 2: Thumbnails & Contents
┌─────────────┐
│ FileSource  │ Re-query DB for same candidates
└──────┬──────┘
       │ <-chan FileRef
       ▼
┌─────────────┐
│  Metadata   │ Load metadata from DB
│   Loader    │ (orientation needed for thumbnails)
└──────┬──────┘
       │ <-chan FileWithMeta
       ▼
┌─────────────┐
│  Thumbnail  │ Load cached OR generate new
│   Workers   │ (uses metadata for orientation)
└──────┬──────┘
       │ <-chan FileWithThumb
       ▼
┌─────────────┐
│  Contents   │ Extract color + AI embedding
│   Workers   │ Writes to DB immediately
└─────────────┘
       │ (Wait for ALL files to complete)
       │
       ▼

Stage 3: Face Detection
┌─────────────┐
│ FileSource  │ Re-query DB for same candidates
└──────┬──────┘
       │ <-chan FileRef
       ▼
┌─────────────┐
│    Face     │ Detect faces from original files
│   Workers   │ (only starts after ALL contents done)
└─────────────┘
```

### Key Behavior

**Sequential Stages**: Each stage waits for all items to complete:
1. **Metadata** → processes all files → writes to DB → completes (skipped if not forcing)
2. **Contents** → waits for metadata → loads from DB via batch query → processes all files → writes to DB → completes
3. **Faces** → waits for contents → processes all files → writes to DB → completes

**Metadata Sourcing**: Stage 2 uses `FileSourceWithMetadata` which batches metadata loading via `GetBatch()` for efficiency, avoiding N individual queries.

**Stage 1 Optimization**: When `ForceMetadata=false`, Stage 1 is skipped entirely—metadata is assumed to already exist in DB from previous runs.

## Types

### Stage Data Types

```go
type FileRef struct {
    ID   ImageId
    Path string
}

type FileWithMeta struct {
    FileRef
    Info img.Info      // EXIF metadata
    Tags []tag.Tag
}

type FileWithThumb struct {
    FileRef
    Thumb       io.ReadSeeker
    Orientation Orientation
}

type FileWithContents struct {
    FileRef  // Contents written to DB, no data carried
}
```

### Intent Configuration

```go
type IndexIntent struct {
    ForceMetadata  bool  // Reextract EXIF even if exists
    ForceColor     bool  // Reextract color even if exists
    ForceEmbedding bool  // Reextract AI embedding even if exists
    ForceFaces     bool  // Redetect faces even if exists
}
```

## Usage

### Basic Indexing (Missing Data Only)

```go
import (
    "photofield/internal/image/pipeline"
    "photofield/internal/task"
)

// Create config
cfg := pipeline.Config{
    DB:                  source.database,
    MetadataExtractor:   source.decoder,
    ThumbnailSources:    source.thumbnailSources,
    ThumbnailGenerators: source.thumbnailGenerators,
    ThumbnailSink:       source.thumbnailSink,
    AIService:           source.Clip,
    ImageDecoder:        thumbnailDecoder,
    FaceDetector:        source.Clip,
    MaxFaceFileSize:     50_000_000,
    MetadataWorkers:     4,
    ThumbnailWorkers:    8,
    ContentsWorkers:     4,
    FaceWorkers:         2,
}

// Create task
t := task.NewIndexTask("my-collection", "My Collection", 
    []string{"/photos"}, 0, pipeline.IntentMissingOnly())

err := pipeline.Run(ctx, cfg, t)
```

### Force Reindex Colors Only

```go
intent := pipeline.IndexIntent{
    ForceColor: true,  // All others false
}

// Result:
// - Loads existing thumbnails (not regenerating)
// - Reextracts color from thumbnails
// - Skips embedding (not forced, already exists)
```

### Force Reindex Everything

```go
intent := pipeline.IntentForceAll()

// Result:
// - Reextracts all metadata
// - Regenerates all thumbnails
// - Reextracts all colors
// - Reextracts all embeddings
// - Redetects all faces
```

## Task Coordination

The `Coordinator` manages pipeline execution with a background worker that processes tasks sequentially. **Only one pipeline runs at a time across all collections.**

### Basic Usage

```go
// Create coordinator (typically one per application)
ctx := context.Background()
coordinator := pipeline.NewCoordinator(ctx)
defer coordinator.Close()

// Queue a pipeline task
task, isNew := coordinator.Add(
    ctx,
    cfg,
    "vacation-2024",           // collectionId
    "Vacation Photos 2024",    // collectionName
    []string{"/photos/vacation"},
    0,                         // maxPhotos (0 = no limit)
    pipeline.IntentMissingOnly(),
)

if !isNew {
    // Task already queued or running for this collection
    fmt.Println("Already indexing:", task.Id)
}

// Wait for completion
<-task.Completed()
```

### Sequential Processing

The coordinator uses a background worker goroutine to process tasks one at a time:

```go
// Queue multiple collections
task1, _ := coordinator.Add(ctx, cfg, "photos-2023", "Photos 2023", dirs1, 0, intent)
task2, _ := coordinator.Add(ctx, cfg, "photos-2024", "Photos 2024", dirs2, 0, intent)
task3, _ := coordinator.Add(ctx, cfg, "videos", "Videos", dirs3, 0, intent)

// Execution order:
// 1. photos-2023 runs (task2 and task3 queued)
// 2. photos-2024 runs after photos-2023 completes (task3 queued)
// 3. videos runs after photos-2024 completes
```

### Preventing Duplicate Tasks

The coordinator prevents duplicate tasks for the same collection:

```go
// First call queues the task
task1, isNew1 := coordinator.Add(ctx, cfg, "photos", "Photos", dirs, 0, intent)
// isNew1 == true, task queued

// Second call returns existing task
task2, isNew2 := coordinator.Add(ctx, cfg, "photos", "Photos", dirs, 0, intent)
// isNew2 == false, task2 == task1, no duplicate queued

// Both wait on the same completion channel
<-task1.Completed()
<-task2.Completed() // Already closed
```

### Listing Active Tasks

```go
activeTasks := coordinator.List()
for _, task := range activeTasks {
    fmt.Printf("%s: %s (%d done)\n", task.Id, task.Name, task.Done)
}
```

## Stage Details

### FileSource

**Input**: Database query based on intent  
**Output**: `<-chan FileRef`

Queries database for candidates:
- `ForceAll` → SELECT all files
- `ForceMeta` → SELECT files missing metadata
- `ForceContents` → SELECT files missing color OR embedding

### MetadataLoadWorkers

**Input**: `<-chan FileRef`  
**Output**: `<-chan FileWithMeta`

Loads existing metadata from database (fast path when metadata exists).

### MetadataExtractWorkers

**Input**: `<-chan FileRef`  
**Output**: `<-chan FileWithMeta`

Extracts metadata from files using EXIF decoder, writes to database immediately.

### ThumbnailWorkers

**Input**: `<-chan FileWithMeta`  
**Output**: `<-chan FileWithThumb`

Try in order:
1. Load from SQLite cache
2. Generate from original using metadata orientation

Uses metadata for correct thumbnail generation (orientation).

### ContentsWorkers

**Input**: `<-chan FileWithThumb`  
**Output**: `<-chan FileWithContents`

Extracts:
- **Color**: Prominent color using k-means clustering
- **Embedding**: AI vector embedding via CLIP

Checks database for existing data unless force flag set.

### FaceWorkers

**Input**: `<-chan FileWithContents`  
**Output**: None (writes directly to database)

Detects faces from original files (not thumbnails) using AI model.  
**Important**: Runs AFTER contents to ensure proper ordering.

**Note**: All pipeline functions log errors immediately via `log.Printf()` instead of returning error channels. This prevents race conditions and simplifies error handling.

## Key Behaviors

### Sequential Stage Execution

The pipeline runs in **three sequential stages**, each completing fully before the next begins:

1. **Stage 1: Metadata**
   - Sources files from DB
   - Extracts or loads metadata
   - Writes metadata to DB
   - **Waits for ALL files to complete**

2. **Stage 2: Contents (Thumbnails + Color/AI)**
   - Re-sources same files from DB
   - Loads metadata from DB (for orientation)
   - Generates thumbnails
   - Extracts color and embeddings
   - Writes contents to DB
   - **Waits for ALL files to complete**

3. **Stage 3: Faces**
   - Re-sources same files from DB
   - Detects faces from original files
   - Writes faces to DB
   - Completes

### Database-Sourced Metadata

**Important**: Thumbnail and contents workers query metadata from DB rather than receiving it through channels:

```go
// Stage 2 always loads metadata from DB
metaOut, _ := MetadataLoadWorkers(ctx, db, files, workers)
// This ensures stage 1 metadata is available
```

This design ensures:
- Stage 1 metadata is persisted before stage 2 begins
- Orientation data is available for thumbnail generation
- No race conditions between metadata write and thumbnail read

### Selective Reindexing

```go
// Force color only
intent := IndexIntent{ForceColor: true}

// What happens:
// 1. Query selects ALL files (not just missing color)
// 2. Metadata stage: loads from DB (not extracting)
// 3. Thumb stage: loads cached thumbs (not generating)
// 4. Contents stage: extracts color, skips embedding
```

### Smart Thumbnail Loading

```go
// Thumb worker logic:
thumb := loadFromCache(id, path)
if thumb == nil {
    thumb = generate(id, path, metadata.Orientation)
}
```

Always tries to reuse cached thumbnails, even during force reindex.

### Error Handling

All stages return error channels. Coordinator collects all errors:
- Logs first 10 errors
- Continues processing remaining items
- Returns error if any occurred

### Worker Pools

Each stage spawns N workers processing in parallel:
- Metadata: 4 workers (EXIF extraction is I/O bound)
- Thumbnails: 8 workers (decoding is CPU intensive)
- Contents: 4 workers (AI calls are network/GPU bound)
- Faces: 2 workers (heavy operation, large files)

## Dependencies

### Database Interface

```go
type Database interface {
    ListMissingMetadata(dirs []string, max int) <-chan MissingInfo
    ListMissingContents(dirs []string, max int) <-chan MissingInfo
    ListAllWithPaths(dirs []string, max int) <-chan IdPath
    
    GetInfo(id ImageId) Info
    HasColor(id ImageId) bool
    HasEmbedding(id ImageId) bool
    HasFaces(id ImageId) bool
    
    Write(path string, info Info, flags UpdateFlags)
    WriteTags(id ImageId, tags []Tag)
    WriteAI(id ImageId, embedding Embedding)
    WriteFaces(id ImageId, faces []Face)
}
```

### External Services

- **MetadataExtractor**: EXIF decoder (exiftool or Go decoder)
- **ThumbnailSource**: SQLite cache
- **ThumbnailGenerator**: Image decoders (djpeg, ffmpeg, Go image)
- **AIService**: CLIP service for embeddings
- **FaceDetector**: Face detection AI model

## Performance

### Memory

- Fixed buffer sizes (100 items per channel)
- Streaming from database (no materialized candidate list)
- Workers process items immediately (no accumulation)

### Throughput

Typical rates (depends on hardware):
- Metadata extraction: ~50-100 files/sec (exiftool)
- Thumbnail generation: ~20-50 files/sec (decoding)
- Color extraction: ~100-200 files/sec (from thumbs)
- Embedding extraction: ~10-30 files/sec (AI service)
- Face detection: ~5-15 files/sec (AI model, large files)

### Backpressure

Bounded channels naturally control backpressure:
- Slow AI service → backs up thumbnail stage
- Slow thumbnail generation → backs up metadata stage
- Database query pauses when channels full

## Testing

Each stage can be tested independently by mocking channels:

```go
func TestMetadataExtract(t *testing.T) {
    in := make(chan FileRef, 10)
    in <- FileRef{ID: 1, Path: "/test.jpg"}
    close(in)
    
    out := MetadataExtractWorkers(ctx, mockDB, mockDecoder, in, 1, false, nil)
    
    result := <-out
    assert.NotNil(t, result.Info)
}
```

## Migration Path

Replace existing `Queue` system:

### Before (Queue-based)
```go
source.metadataQueue.AppendItems(...)
source.contentsQueue.AppendItems(...)
```

### After (Pipeline-based)
```go
task := task.NewIndexTask(collectionId, collectionName, dirs, maxPhotos, intent)
pipeline.Run(ctx, config, task)
```

All stages, workers, and coordination handled by pipeline package. Progress tracked via task.
