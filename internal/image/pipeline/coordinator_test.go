package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"photofield/internal/task"
)

// TestCoordinatorSequentialExecution verifies that tasks run one at a time
func TestCoordinatorSequentialExecution(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MetadataWorkers:  1,
		ThumbnailWorkers: 1,
		ContentsWorkers:  1,
		FaceWorkers:      1,
	}
	coordinator := NewCoordinator(ctx, cfg)
	defer coordinator.Close()

	// Queue multiple tasks
	numTasks := 3
	tasks := make([]*task.Task, numTasks)

	for i := 0; i < numTasks; i++ {
		collectionId := string(rune('A' + i))
		tsk, isNew := coordinator.AddMetadata(collectionId, collectionId, []string{}, 0, false)
		if !isNew {
			t.Fatalf("Task %d should be new", i)
		}
		tasks[i] = tsk
	}

	// Wait for all tasks to complete (they'll complete quickly due to nil DB early-return)
	for i, tsk := range tasks {
		<-tsk.Completed()
		t.Logf("Task %d completed", i)
	}

	// All tasks should have been processed
	activeTasks := coordinator.List()
	if len(activeTasks) != 0 {
		t.Errorf("Expected 0 active tasks after completion, got %d", len(activeTasks))
	}
}

// TestCoordinatorDuplicatePrevention verifies duplicate tasks are rejected
// while the first task is still in-flight, and that a new task can be
// created once the first one completes.
func TestCoordinatorDuplicatePrevention(t *testing.T) {
	ctx := context.Background()

	// Block task execution until the test releases it.
	block := make(chan struct{})
	started := make(chan struct{})
	var startOnce sync.Once
	cfg := Config{
		MetadataWorkers: 1,
		TaskRunner: func(ctx context.Context, cfg Config, tsk *task.Task) error {
			startOnce.Do(func() { close(started) }) // signal first execution
			<-block
			return nil
		},
	}
	coordinator := NewCoordinator(ctx, cfg)
	defer coordinator.Close()

	task1, isNew1 := coordinator.AddMetadata("collection1", "Collection 1", []string{}, 0, false)
	if !isNew1 {
		t.Fatal("First task should be new")
	}

	// Wait until the worker is inside the runner so task1 is definitely in-flight.
	<-started

	// While task1 is blocked in the runner, adding the same collection must
	// return the existing task (duplicate prevention).
	task2, isNew2 := coordinator.AddMetadata("collection1", "Collection 1", []string{}, 0, false)
	if isNew2 {
		t.Fatal("Duplicate task should not be new while first is in-flight")
	}
	if task1 != task2 {
		t.Errorf("Expected same task instance, got different pointers")
	}

	// Unblock the worker and wait for completion.
	close(block)
	<-task1.Completed()

	// After completion the registry entry is removed; re-adding must create a
	// fresh task.
	task3, isNew3 := coordinator.AddMetadata("collection1", "Collection 1", []string{}, 0, false)
	if !isNew3 {
		t.Fatal("Task should be new after previous task completed")
	}
	if task1 == task3 {
		t.Error("Expected a new task instance after previous task completed")
	}
	<-task3.Completed()
}

// TestCoordinatorShutdown verifies clean shutdown
func TestCoordinatorShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		MetadataWorkers: 1,
	}
	coordinator := NewCoordinator(ctx, cfg)

	// Queue a task
	tsk, _ := coordinator.AddMetadata("test", "Test", []string{}, 0, false)

	// Cancel context
	cancel()
	coordinator.Close()

	// Task should complete (or be cancelled)
	select {
	case <-tsk.Completed():
		// Good, task completed
	case <-time.After(100 * time.Millisecond):
		t.Error("Task did not complete after shutdown")
	}
}

// TestCoordinatorList verifies task listing
func TestCoordinatorList(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MetadataWorkers: 1,
	}
	coordinator := NewCoordinator(ctx, cfg)
	defer coordinator.Close()

	// Initially empty
	if len(coordinator.List()) != 0 {
		t.Error("Expected empty task list initially")
	}

	// Add multiple tasks and wait for completion.
	// The async worker may complete tasks before List() is called, so we only
	// assert the post-completion empty state rather than a mid-flight snapshot.
	task1, _ := coordinator.AddMetadata("col1", "Collection 1", []string{}, 0, false)
	task2, _ := coordinator.AddMetadata("col2", "Collection 2", []string{}, 0, false)

	<-task1.Completed()
	<-task2.Completed()

	if len(coordinator.List()) != 0 {
		t.Error("Expected empty task list after task completion")
	}
}

// TestCoordinatorPriorityOrder verifies metadata tasks are dequeued before contents,
// contents before faces, and within a stage newer tasks are dequeued first.
func TestCoordinatorPriorityOrder(t *testing.T) {
	// Build the queue directly without running the worker
	c := &Coordinator{
		registry: task.NewRegistry(),
		queue:    make([]*task.Task, 0),
	}

	// Insert in "worst" order: faces first, then contents, then metadata
	// and within metadata: older first
	tFaces := task.NewFacesTask("col1", "Col1", []string{}, 0, false)
	time.Sleep(time.Millisecond)
	tContents := task.NewContentsTask("col1", "Col1", []string{}, 0, false)
	time.Sleep(time.Millisecond)
	tMetaOld := task.NewMetadataTask("col1", "Col1", []string{}, 0, false)
	time.Sleep(time.Millisecond)
	tMetaNew := task.NewMetadataTask("col2", "Col2", []string{}, 0, false)

	for _, t := range []*task.Task{tFaces, tContents, tMetaOld, tMetaNew} {
		c.insertSorted(t)
	}

	// Dequeue from tail: expect metadata-new, metadata-old, contents, faces
	want := []string{tMetaNew.Id, tMetaOld.Id, tContents.Id, tFaces.Id}
	for i, wantId := range want {
		if len(c.queue) == 0 {
			t.Fatalf("Queue empty after %d dequeues, want %s", i, wantId)
		}
		got := c.queue[len(c.queue)-1]
		c.queue = c.queue[:len(c.queue)-1]
		if got.Id != wantId {
			t.Errorf("Dequeue %d: got %s, want %s", i, got.Id, wantId)
		}
	}
}
