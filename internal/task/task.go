package task

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Pipeline task type constants
const (
	TypeIndexFiles    = "INDEX_FILES"
	TypeIndexMetadata = "INDEX_METADATA"
	TypeIndexContents = "INDEX_CONTENTS"
	TypeIndexFaces    = "INDEX_FACES"
)

// Task represents a long-running operation that can be tracked
type Task struct {
	Id           string `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	CollectionId string `json:"collection_id"`
	Done         int    `json:"done"`
	Pending      int    `json:"pending,omitempty"`
	Total        int    `json:"total,omitempty"`
	Offset       int    `json:"-"`
	Queue        string `json:"-"`

	// Pipeline-specific fields
	CollectionName string    `json:"-"` // Original collection name for follow-up task naming
	Dirs           []string  `json:"-"`
	MaxPhotos      int       `json:"-"`
	Force          bool      `json:"-"` // Force reprocessing even if data already exists
	EnqueuedAt     time.Time `json:"-"` // Used for priority ordering within a stage

	// Context for cancellation and completion signaling
	ctx    context.Context    `json:"-"`
	cancel context.CancelFunc `json:"-"`
	mu     sync.RWMutex       `json:"-"`
}

// Counter returns a channel for incrementing the task's Done counter
func (t *Task) Counter() chan<- int {
	counter := make(chan int, 10)
	go func() {
		for add := range counter {
			t.AddDone(add)
		}
	}()
	return counter
}

// AddDone adds to the Done count in a thread-safe way.
func (t *Task) AddDone(add int) {
	t.mu.Lock()
	t.Done += add
	t.mu.Unlock()
}

// SetTotal sets the Total count in a thread-safe way.
func (t *Task) SetTotal(total int) {
	t.mu.Lock()
	t.Total = total
	t.mu.Unlock()
}

// Progress returns a consistent snapshot of Done and Total.
func (t *Task) Progress() (done int, total int) {
	t.mu.RLock()
	done, total = t.Done, t.Total
	t.mu.RUnlock()
	return
}

// Completed returns a channel that closes when the task is complete
func (t *Task) Completed() <-chan struct{} {
	return t.ctx.Done()
}

// Context returns the task's context
func (t *Task) Context() context.Context {
	return t.ctx
}

// Close marks the task as completed by canceling its context
func (t *Task) Close() {
	if t.cancel != nil {
		t.cancel()
	}
}

// Registry manages a collection of tasks
type Registry struct {
	tasks sync.Map
}

// NewRegistry creates a new task registry
func NewRegistry() *Registry {
	return &Registry{}
}

// Store adds a task to the registry
func (r *Registry) Store(id string, task *Task) {
	r.tasks.Store(id, task)
}

// Load retrieves a task from the registry
func (r *Registry) Load(id string) (*Task, bool) {
	value, ok := r.tasks.Load(id)
	if !ok {
		return nil, false
	}
	return value.(*Task), true
}

// LoadOrStore atomically loads or stores a task
func (r *Registry) LoadOrStore(task *Task) (*Task, bool) {
	actual, loaded := r.tasks.LoadOrStore(task.Id, task)
	return actual.(*Task), loaded
}

// Delete removes a task from the registry
func (r *Registry) Delete(id string) {
	r.tasks.Delete(id)
}

// Range iterates over all tasks
func (r *Registry) Range(f func(key string, task *Task) bool) {
	r.tasks.Range(func(key, value interface{}) bool {
		return f(key.(string), value.(*Task))
	})
}

// New creates a new task with the given parameters
func New(taskType, id, name, collectionId string) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	return &Task{
		Type:         taskType,
		Id:           id,
		Name:         name,
		CollectionId: collectionId,
		Done:         0,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// NewIndexTask creates a task for indexing a collection
//
// Deprecated: use NewMetadataTask, NewContentsTask, or NewFacesTask instead.
func NewIndexTask(collectionId, collectionName string, dirs []string, maxPhotos int, intent interface{}) *Task {
	t := New(
		"INDEX",
		fmt.Sprintf("index-%s", collectionId),
		fmt.Sprintf("Indexing %s", collectionName),
		collectionId,
	)
	t.Dirs = dirs
	t.MaxPhotos = maxPhotos
	return t
}

// newStageTask is a shared constructor for per-stage tasks
func newStageTask(taskType, collectionId, collectionName string, dirs []string, maxPhotos int, force bool) *Task {
	typeSlug := map[string]string{
		TypeIndexFiles:    "files",
		TypeIndexMetadata: "metadata",
		TypeIndexContents: "contents",
		TypeIndexFaces:    "faces",
	}[taskType]
	t := New(
		taskType,
		fmt.Sprintf("index-%s-%s", typeSlug, collectionId),
		fmt.Sprintf("Indexing %s %s", typeSlug, collectionName),
		collectionId,
	)
	t.Dirs = dirs
	t.MaxPhotos = maxPhotos
	t.Force = force
	t.CollectionName = collectionName
	t.EnqueuedAt = time.Now()
	return t
}

// NewMetadataTask creates a task for metadata extraction
func NewMetadataTask(collectionId, collectionName string, dirs []string, maxPhotos int, force bool) *Task {
	return newStageTask(TypeIndexMetadata, collectionId, collectionName, dirs, maxPhotos, force)
}

// NewContentsTask creates a task for thumbnail generation and feature extraction
func NewContentsTask(collectionId, collectionName string, dirs []string, maxPhotos int, force bool) *Task {
	return newStageTask(TypeIndexContents, collectionId, collectionName, dirs, maxPhotos, force)
}

// NewFilesTask creates a task for file scanning
func NewFilesTask(collectionId, collectionName string, dirs []string, maxPhotos int) *Task {
	return newStageTask(TypeIndexFiles, collectionId, collectionName, dirs, maxPhotos, false)
}

// NewFacesTask creates a task for face detection
func NewFacesTask(collectionId, collectionName string, dirs []string, maxPhotos int, force bool) *Task {
	return newStageTask(TypeIndexFaces, collectionId, collectionName, dirs, maxPhotos, force)
}


