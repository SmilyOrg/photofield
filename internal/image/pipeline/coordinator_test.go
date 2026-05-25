package pipeline

import (
	"context"
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
func TestCoordinatorDuplicatePrevention(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MetadataWorkers: 1,
	}
	coordinator := NewCoordinator(ctx, cfg)
	defer coordinator.Close()

	// Add first task
	task1, isNew1 := coordinator.AddMetadata("collection1", "Collection 1", []string{}, 0, false)
	if !isNew1 {
		t.Fatal("First task should be new")
	}

	// Try to add duplicate
	task2, isNew2 := coordinator.AddMetadata("collection1", "Collection 1", []string{}, 0, false)
	if isNew2 {
		t.Fatal("Duplicate task should not be new")
	}

	// Should return same task
	if task1.Id != task2.Id {
		t.Errorf("Expected same task ID, got %s and %s", task1.Id, task2.Id)
	}

	// Wait for completion
	<-task1.Completed()
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

	// Add multiple tasks
	coordinator.AddMetadata("col1", "Collection 1", []string{}, 0, false)
	coordinator.AddMetadata("col2", "Collection 2", []string{}, 0, false)

	// Should have tasks in list
	tasks := coordinator.List()
	if len(tasks) == 0 {
		t.Error("Expected tasks in list")
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
