# Split Plan: `index-refactor` → Multiple PRs

The `index-refactor` branch contains two interleaved feature streams that are split into smaller PRs for easier review:

1. **Indexing pipeline refactor** — replaces the old `queue.Queue`-based system with a typed sequential coordinator
2. **Face detection** — new feature: detect faces, cluster them into people, display as a layout

## Branch Overview

| Branch | Base | Status | Description |
|--------|------|--------|-------------|
| `split/pipeline-core` | `main` | ✅ done | Core pipeline coordinator + `clip`→`ai` rename |
| `split/faces-db` | `split/pipeline-core` | | Face database schema and operations |
| `split/faces-detection` | `split/faces-db` | | Face detection pipeline stage + API wiring |
| `split/faces-clustering` | `split/faces-detection` | | Face clustering algorithm and pipeline stage |
| `split/faces-render` | `split/faces-clustering` | | Face layout and photo overlay rendering |
| `split/faces-ui` | `split/faces-render` | | Frontend UI for faces + e2e tests |
| `split/tooling` | `main` | | Dev tooling: Taskfile, VS Code, API scripts |

---

## 1. `split/pipeline-core` ✅

**Base**: `main`

Replaces the old `queue.Queue`-based indexing with a sequential coordinator. Renames `internal/clip` → `internal/ai`.

**Files**:
- `internal/ai/client.go`, `internal/ai/embedding.go` — renamed from `internal/clip/`
- `internal/image/pipeline/` — new package: coordinator, files, metadata, contents, thumbnail, source, progress stages + tests + README
- `internal/task/task.go` — typed task constants (`INDEX_FILES`, `INDEX_METADATA`, `INDEX_CONTENTS`) and registry
- `internal/image/indexContents.go`, `indexFiles.go`, `indexMetadata.go` — **deleted** (replaced by pipeline)
- `internal/image/source.go`, `database.go`, `search.go` — clip→ai rename, queue fields removed, accessor methods added
- `internal/collection/collection.go`, `internal/render/scene.go`, `internal/scene/sceneSource.go`, `config.go` — clip→ai rename
- `api.yaml` — `POST /tasks` with `force` field, `DELETE /tasks/{id}`, `INDEX_ALL` task type
- `internal/openapi/api.gen.go` — regenerated
- `PIPELINE_DESIGN.md` — **deleted** (content merged into pipeline README)

---

## 2. `split/faces-db`

**Base**: `split/pipeline-core`

Self-contained database layer for face data. No behavior change visible to users yet.

**Files**:
- `db/migrations/000016_faces.{up,down}.sql` — `face` table: `file_id`, `x/y/w/h`, `confidence`, `embedding`, `person_id` + `face_count` count column in `infos`
- `internal/image/source.go` — `ListFaces()` accessor on `Source`

---

## 3. `split/faces-detection`

**Base**: `split/faces-db`

Adds the `INDEX_FACES` pipeline stage. After indexing contents, faces can be detected from original files using the AI service.

**Files**:
- `internal/image/pipeline/faces.go` — `RunFaces()` stage: sources files missing face data, calls `FaceDetector.DetectFaces()`, writes to DB
- `internal/task/task.go` — add `TypeIndexFaces`, `NewFacesTask()`
- `internal/image/pipeline/coordinator.go` — add `FaceDetector`/`MaxFaceFileSize`/`FaceWorkers` to `Config`; add `AddFaces()`, `stagePriority` case; `AddFiles` auto-cascade includes faces
- `api.yaml` — add `INDEX_FACES` to `TaskType` enum; regenerate `internal/openapi/api.gen.go`
- `main.go` — `INDEX_FACES` case in `PostTasks`, face fields in `applyConfig`, face task in `--scan` startup
- `config.go` — face detection config fields
- `defaults.yaml` — face detection defaults (`max_face_file_size`, `face_workers`)

---

## 4. `split/faces-clustering`

**Base**: `split/faces-detection`

Adds the `CLUSTER_FACES` task that groups detected faces into person identities using k-NN + Chinese Whispers.

**Files**:
- `internal/face/types.go` — `Face`, `FaceEmbedding` types
- `internal/face/similarity.go` — `BuildKNNGraph()` using cosine distance (k=10 neighbors, threshold 0.6)
- `internal/face/cluster.go` — `ChineseWhispers()` clustering algorithm
- `internal/image/pipeline/clusterfaces.go` — `RunClusterFaces()` stage
- `internal/task/task.go` — add `TypeClusterFaces`, `NewClusterFacesTask()`
- `internal/image/pipeline/coordinator.go` — add `FaceClusterer` to `Config`; add `AddClusterFaces()`, coordinator case
- `api.yaml` — add `CLUSTER_FACES` to `TaskType` enum; regenerate `internal/openapi/api.gen.go`
- `main.go` — `CLUSTER_FACES` case in `PostTasks` and `taskDisplayOrder`

---

## 5. `split/faces-render`

**Base**: `split/faces-clustering`

Renders face boxes on photos and adds a Faces layout that groups photos by detected person.

**Files**:
- `internal/render/rect.go` — `Rect` utility helpers used for face bounding boxes
- `internal/render/bitmap.go` — face box drawing on photo tiles
- `internal/render/photo.go` — face overlay integration in `Draw()`
- `internal/render/scene.go` — face data on `Scene` struct
- `internal/scene/sceneSource.go` — load person collections as face-grouped scenes
- `internal/layout/common.go` — `Faces` layout type constant
- `internal/layout/faces.go` — `LayoutFaces()`: groups photos by `person_id`, renders face crops as thumbnails
- `internal/layout/highlights.go` — small tweaks needed for face layout
- `internal/io/io.go` — small interface addition needed by face render pipeline
- `docs/features/layouts.md` — Faces layout documentation

---

## 6. `split/faces-ui`

**Base**: `split/faces-render`

Frontend UI for triggering face indexing, browsing people, and viewing face detection status.

**Files**:
- `ui/src/components/CollectionDebug.vue` — "Index Faces" and "Cluster Faces" buttons
- `ui/src/components/CollectionPanel.vue` — face collection display in side panel
- `ui/src/components/DisplaySettings.vue` — face display toggle
- `ui/src/components/TaskList.vue` — face task display improvements
- `ui/src/api.js` — face API calls (`postTask` for faces/clustering)
- `ui/src/App.vue` — face panel integration
- `e2e/tests/tasks.feature` — e2e scenarios for `INDEX_FACES` and `CLUSTER_FACES` tasks
- `e2e/src/fixtures.ts`, `e2e/src/steps.ts` — e2e test support

---

## 7. `split/tooling` (independent)

**Base**: `main`  
**No prerequisites** — can be merged at any point.

Dev tooling, IDE config, and changelog entries. No functional changes to the application.

**Files**:
- `Taskfile.yml` — new task targets (face-related build/run tasks)
- `.vscode/settings.json` — IDE configuration
- `.github/copilot-instructions.md` — updated project instructions
- `api.sh` — API testing/exploration script
- `.changes/unreleased/Added-20260314-004311.yaml` — changelog: pipeline refactor
- `.changes/unreleased/Added-20260314-004317.yaml` — changelog: face detection
- `.changes/unreleased/Added-20260314-004324.yaml` — changelog: face clustering

---

## Dependency Graph

```
main
 ├── split/pipeline-core ✅
 │    └── split/faces-db
 │         └── split/faces-detection
 │              └── split/faces-clustering
 │                   └── split/faces-render
 │                        └── split/faces-ui
 └── split/tooling (independent)
```
