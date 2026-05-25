package pipeline

import (
	"context"
	"log"
	"sort"
	"sync"

	img "photofield/internal/image"
	"photofield/internal/task"
)

// Config contains all dependencies for the pipeline
type Config struct {
	// Database operations
	DB *img.Database

	// Metadata extraction
	MetadataExtractor MetadataExtractor
	EnableTags        bool

	// Thumbnail operations
	ThumbnailSources    []ThumbnailSource
	ThumbnailGenerators []ThumbnailGenerator
	ThumbnailSink       ThumbnailSink

	// Contents extraction
	AIService    AIService
	ImageDecoder ImageDecoder

	// Face detection
	FaceDetector    FaceDetector
	MaxFaceFileSize int64 // Max file size for face detection (bytes)

	// File scanning
	Extensions []string

	// Worker counts
	MetadataWorkers  int
	ThumbnailWorkers int
	ContentsWorkers  int
	FaceWorkers      int
}

// stagePriority returns the queue priority for a task type.
// Higher value = higher priority = closer to tail = dequeued first.
func stagePriority(taskType string) int {
	switch taskType {
	case task.TypeIndexFiles:
		return 3
	case task.TypeIndexMetadata:
		return 2
	case task.TypeIndexContents:
		return 1
	case task.TypeIndexFaces:
		return 0
	default:
		return -2
	}
}

// RunMetadata executes Stage 1: extract EXIF/metadata for files in the collection.
func RunMetadata(ctx context.Context, cfg Config, t *task.Task) error {
	if cfg.DB == nil {
		return nil
	}
	dirs := t.Dirs
	maxPhotos := t.MaxPhotos
	force := t.Force

	counter := t.Counter()
	defer close(counter)

	if force {
		if count, ok := cfg.DB.GetDirsCount(dirs); ok {
			if maxPhotos > 0 && count > maxPhotos {
				count = maxPhotos
			}
			t.Total = count
			log.Printf("index metadata extract %d files\n", count)
		}
	} else {
		if count, ok := cfg.DB.CountMissing(dirs, img.Missing{Metadata: true}); ok {
			if maxPhotos > 0 && count > maxPhotos {
				count = maxPhotos
			}
			t.Total = count
			if count > 0 {
				log.Printf("index metadata extract %d files\n", count)
			}
		}
	}

	files := fileSource(ctx, cfg.DB, dirs, maxPhotos, force, img.Missing{Metadata: true})

	metaOut := processMetadata(ctx, cfg.DB, cfg.MetadataExtractor,
		files, cfg.MetadataWorkers, cfg.EnableTags, counter)

	for range metaOut {
	}

	return nil
}

// RunContents executes Stage 2: generate thumbnails and extract color + AI embeddings.
func RunContents(ctx context.Context, cfg Config, t *task.Task) error {
	if cfg.DB == nil {
		return nil
	}
	dirs := t.Dirs
	maxPhotos := t.MaxPhotos
	force := t.Force

	counter := t.Counter()
	defer close(counter)

	if force {
		if count, ok := cfg.DB.GetDirsCount(dirs); ok {
			if maxPhotos > 0 && count > maxPhotos {
				count = maxPhotos
			}
			t.Total = count
			log.Printf("index contents extract %d files\n", count)
		}
	} else {
		if count, ok := cfg.DB.CountMissing(dirs, img.Missing{Color: true, Embedding: true}); ok {
			if maxPhotos > 0 && count > maxPhotos {
				count = maxPhotos
			}
			t.Total = count
			if count > 0 {
				log.Printf("index contents extract %d files\n", count)
			}
		}
	}

	metaOut := fileSourceWithMetadata(ctx, cfg.DB, dirs, maxPhotos, force)

	contents := newContentsProcessor(cfg.DB, cfg.AIService, cfg.ImageDecoder, force)
	processThumbnails(ctx, cfg.ThumbnailSources, cfg.ThumbnailGenerators,
		cfg.ThumbnailSink, metaOut, cfg.ThumbnailWorkers, counter, contents.Process)
	contents.Done()

	return nil
}

// RunFaces executes Stage 3: detect faces in files.
func RunFaces(ctx context.Context, cfg Config, t *task.Task) error {
	if cfg.DB == nil {
		return nil
	}
	if cfg.FaceDetector == nil {
		log.Println("index faces skipped: no face detector configured")
		return nil
	}
	dirs := t.Dirs
	maxPhotos := t.MaxPhotos
	force := t.Force

	counter := t.Counter()
	defer close(counter)

	if force {
		if count, ok := cfg.DB.GetDirsCount(dirs); ok {
			if maxPhotos > 0 && count > maxPhotos {
				count = maxPhotos
			}
			t.Total = count
			log.Printf("index faces starting %d files\n", count)
		}
	} else {
		if count, ok := cfg.DB.CountMissing(dirs, img.Missing{Faces: true}); ok {
			if maxPhotos > 0 && count > maxPhotos {
				count = maxPhotos
			}
			t.Total = count
			if count > 0 {
				log.Printf("index faces starting %d files\n", count)
			}
		}
	}

	files := fileSource(ctx, cfg.DB, dirs, maxPhotos, force, img.Missing{Faces: true})

	contentsFiles := make(chan fileWithContents, 100)
	go func() {
		defer close(contentsFiles)
		for file := range files {
			contentsFiles <- fileWithContents{fileRef: file}
		}
	}()

	processFaces(ctx, cfg.DB, cfg.FaceDetector,
		contentsFiles, cfg.FaceWorkers, cfg.MaxFaceFileSize, counter)
	log.Println("index faces completed")

	return nil
}

// Coordinator manages pipeline execution and prevents concurrent runs
type Coordinator struct {
	registry *task.Registry
	cfg      Config
	queue    []*task.Task
	queueMu  sync.Mutex
	signal   chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewCoordinator creates a new pipeline coordinator
func NewCoordinator(ctx context.Context, cfg Config) *Coordinator {
	ctx, cancel := context.WithCancel(ctx)
	c := &Coordinator{
		registry: task.NewRegistry(),
		cfg:      cfg,
		queue:    make([]*task.Task, 0),
		signal:   make(chan struct{}, 1),
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start background worker
	go c.worker()

	return c
}

// insertSorted inserts t into the queue maintaining ascending (stagePriority, EnqueuedAt) order.
// The tail of the slice therefore holds the highest-priority, most-recently-enqueued task,
// which is what worker() dequeues so the backing array never grows unboundedly.
func (c *Coordinator) insertSorted(t *task.Task) {
	pos := sort.Search(len(c.queue), func(i int) bool {
		q := c.queue[i]
		qp := stagePriority(q.Type)
		tp := stagePriority(t.Type)
		if qp != tp {
			return qp > tp
		}
		return q.EnqueuedAt.After(t.EnqueuedAt)
	})
	c.queue = append(c.queue, nil)
	copy(c.queue[pos+1:], c.queue[pos:])
	c.queue[pos] = t
}

// worker processes tasks from the queue sequentially
func (c *Coordinator) worker() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.signal:
			// Dequeue from tail (highest priority, most recent within stage)
			c.queueMu.Lock()
			if len(c.queue) == 0 {
				c.queueMu.Unlock()
				continue
			}
			t := c.queue[len(c.queue)-1]
			c.queue = c.queue[:len(c.queue)-1]
			c.queueMu.Unlock()

			c.processTask(t)

			// Re-signal to process next task (non-blocking)
			select {
			case c.signal <- struct{}{}:
			default:
			}
		}
	}
}

// processTask executes a single pipeline task by dispatching on its type
func (c *Coordinator) processTask(t *task.Task) {
	defer t.Close()
	defer c.registry.Delete(t.Id)

	log.Printf("index task start %s\n", t.Id)

	var err error
	switch t.Type {
	case task.TypeIndexFiles:
		err = RunFiles(t.Context(), c.cfg, t)
		if err == nil {
			// Auto-enqueue follow-up pipeline stages after file scan
			c.AddMetadata(t.CollectionId, t.CollectionName, t.Dirs, t.MaxPhotos, false)
			c.AddContents(t.CollectionId, t.CollectionName, t.Dirs, t.MaxPhotos, false)
			c.AddFaces(t.CollectionId, t.CollectionName, t.Dirs, t.MaxPhotos, false)
		}
	case task.TypeIndexMetadata:
		err = RunMetadata(t.Context(), c.cfg, t)
	case task.TypeIndexContents:
		err = RunContents(t.Context(), c.cfg, t)
	case task.TypeIndexFaces:
		err = RunFaces(t.Context(), c.cfg, t)
	default:
		log.Printf("index task error: unknown type: %q: %s\n", t.Id, t.Type)
		return
	}

	if err != nil {
		log.Printf("index task error: %s: %v\n", t.Id, err)
	} else {
		log.Printf("index task done %s\n", t.Id)
	}
}

// StopTask cancels and removes a task by ID. Returns true if the task was found
// and stopped, false if no task with that ID exists.
func (c *Coordinator) StopTask(id string) bool {
	t, ok := c.registry.Load(id)
	if !ok {
		return false
	}

	// Remove from queue if it hasn't been dequeued yet.
	c.queueMu.Lock()
	for i, q := range c.queue {
		if q.Id == id {
			c.queue = append(c.queue[:i], c.queue[i+1:]...)
			break
		}
	}
	c.queueMu.Unlock()

	// Cancel context (no-op if processTask already called t.Close via defer).
	t.Close()
	c.registry.Delete(id)
	log.Printf("index task stop %s\n", id)
	return true
}

// Close stops the coordinator and cancels all pending tasks
func (c *Coordinator) Close() {
	c.cancel()
	// Do NOT close c.signal – the worker exits via ctx.Done().
	// Closing a buffered channel while the worker may be sending to it causes a panic.

	c.queueMu.Lock()
	for _, t := range c.queue {
		t.Close()
		c.registry.Delete(t.Id)
	}
	c.queue = nil
	c.queueMu.Unlock()
}

// addTask registers and enqueues a task. Returns the task and true if newly added,
// or the existing task and false if already queued or running.
func (c *Coordinator) addTask(t *task.Task) (*task.Task, bool) {
	existing, loaded := c.registry.LoadOrStore(t)
	if loaded {
		log.Printf("index collection %s %s already queued\n", t.CollectionId, t.Type)
		return existing, false
	}

	c.queueMu.Lock()
	c.insertSorted(t)
	c.queueMu.Unlock()

	select {
	case c.signal <- struct{}{}:
	default:
	}

	log.Printf("index task add %s for %s\n", t.Type, t.CollectionId)
	return t, true
}

// AddFiles queues a file scan task for the given collection.
// On success the coordinator automatically enqueues metadata, contents, and
// face detection tasks so callers only need to trigger this single stage.
func (c *Coordinator) AddFiles(collectionId, collectionName string, dirs []string, maxPhotos int) (*task.Task, bool) {
	return c.addTask(task.NewFilesTask(collectionId, collectionName, dirs, maxPhotos))
}

// AddMetadata queues a metadata extraction task for the given collection.
func (c *Coordinator) AddMetadata(collectionId, collectionName string, dirs []string, maxPhotos int, force bool) (*task.Task, bool) {
	return c.addTask(task.NewMetadataTask(collectionId, collectionName, dirs, maxPhotos, force))
}

// AddContents queues a thumbnail + contents extraction task for the given collection.
func (c *Coordinator) AddContents(collectionId, collectionName string, dirs []string, maxPhotos int, force bool) (*task.Task, bool) {
	return c.addTask(task.NewContentsTask(collectionId, collectionName, dirs, maxPhotos, force))
}

// AddFaces queues a face detection task for the given collection.
func (c *Coordinator) AddFaces(collectionId, collectionName string, dirs []string, maxPhotos int, force bool) (*task.Task, bool) {
	return c.addTask(task.NewFacesTask(collectionId, collectionName, dirs, maxPhotos, force))
}

// AddAll queues all three stages (metadata, contents, faces) for the given collection.
// Returns one entry per stage with a bool indicating whether it was newly added.
func (c *Coordinator) AddAll(collectionId, collectionName string, dirs []string, maxPhotos int, force bool) ([3]*task.Task, [3]bool) {
	var tasks [3]*task.Task
	var isNew [3]bool
	tasks[0], isNew[0] = c.AddMetadata(collectionId, collectionName, dirs, maxPhotos, force)
	tasks[1], isNew[1] = c.AddContents(collectionId, collectionName, dirs, maxPhotos, force)
	tasks[2], isNew[2] = c.AddFaces(collectionId, collectionName, dirs, maxPhotos, force)
	return tasks, isNew
}

// Get retrieves a task by ID
func (c *Coordinator) Get(id string) (*task.Task, bool) {
	return c.registry.Load(id)
}

// List returns all active tasks
func (c *Coordinator) List() []*task.Task {
	tasks := make([]*task.Task, 0)
	c.registry.Range(func(key string, t *task.Task) bool {
		tasks = append(tasks, t)
		return true
	})
	return tasks
}
