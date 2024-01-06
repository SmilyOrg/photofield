package fs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher(t *testing.T) {
	dir, err := os.MkdirTemp("", "watcher-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	w, err := NewWatcher([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// Create a file
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for the event
	select {
	case e := <-w.Events:
		if e.Path != file || e.Op != Update {
			t.Fatalf("unexpected event: %+v", e)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Modify the file
	if err := os.WriteFile(file, []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for the event
	select {
	case e := <-w.Events:
		if e.Path != file || e.Op != Update {
			t.Fatalf("unexpected event: %+v", e)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Rename the file
	newFile := filepath.Join(dir, "test2.txt")
	if err := os.Rename(file, newFile); err != nil {
		t.Fatal(err)
	}

	// Wait for the event
	select {
	case e := <-w.Events:
		if e.Path != newFile || e.OldPath != file || e.Op != Rename {
			t.Fatalf("unexpected event: %+v", e)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Wait for the events
	// select {
	// case e := <-w.Events:
	// 	if e.Path != file || e.Op != Remove {
	// 		t.Fatalf("unexpected event: %+v", e)
	// 	}
	// case <-time.After(1 * time.Second):
	// 	t.Fatal("timeout waiting for event")
	// }
	// select {
	// case e := <-w.Events:
	// 	if e.Path != newFile || e.Op != Update {
	// 		t.Fatalf("unexpected event: %+v", e)
	// 	}
	// case <-time.After(1 * time.Second):
	// 	t.Fatal("timeout waiting for event")
	// }

	// Create a directory
	dir2 := filepath.Join(dir, "testdir")
	if err := os.Mkdir(dir2, 0755); err != nil {
		t.Fatal(err)
	}

	// Wait for the event
	select {
	case e := <-w.Events:
		if e.Path != dir2 || e.Op != Update {
			t.Fatalf("unexpected event: %+v", e)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Rename the directory
	newDir := filepath.Join(dir, "testdir2")
	if err := os.Rename(dir2, newDir); err != nil {
		t.Fatal(err)
	}

	// Wait for the event
	select {
	case e := <-w.Events:
		if e.Path != newDir || e.OldPath != dir2 || e.Op != Rename {
			t.Fatalf("unexpected event: %+v", e)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	// select {
	// case e := <-w.Events:
	// 	if e.Path != dir2 || e.Op != Remove {
	// 		t.Fatalf("unexpected event: %+v", e)
	// 	}
	// case <-time.After(1 * time.Second):
	// 	t.Fatal("timeout waiting for event")
	// }
	// select {
	// case e := <-w.Events:
	// 	if e.Path != newDir || e.Op != Update {
	// 		t.Fatalf("unexpected event: %+v", e)
	// 	}
	// case <-time.After(1 * time.Second):
	// 	t.Fatal("timeout waiting for event")
	// }
}
