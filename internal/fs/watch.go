package fs

import (
	"path/filepath"
	"time"

	"github.com/rjeczalik/notify"
)

type Event struct {
	Op      Op
	Path    string
	OldPath string
}

type Op uint32

const (
	Update Op = 1 << iota
	Remove
	Rename
)

type Watcher struct {
	Events   chan Event
	filename string
	c        chan notify.EventInfo
}

func NewFileWatcher(path string) (*Watcher, error) {
	w := &Watcher{
		Events: make(chan Event, 100),
		c:      make(chan notify.EventInfo, 100),
	}
	dir := filepath.Dir(path)
	w.filename = filepath.Base(path)
	err := notify.Watch(
		dir,
		w.c,
		notify.Remove,
		notify.Rename,
		notify.Create,
		notify.Write,
	)
	if err != nil {
		w.Close()
		return nil, err
	}
	go w.run()
	return w, nil
}

func NewPathsWatcher(paths []string) (*Watcher, error) {
	w := &Watcher{
		Events: make(chan Event, 100),
		c:      make(chan notify.EventInfo, 100),
	}
	for _, path := range paths {
		err := notify.Watch(
			path,
			w.c,
			notify.Remove,
			notify.Rename,
			notify.Create,
			notify.Write,
		)
		if err != nil {
			w.Close()
			return nil, err
		}
	}
	go w.run()
	return w, nil
}

func NewRecursiveWatcher(dirs []string) (*Watcher, error) {
	w := &Watcher{
		Events: make(chan Event, 100),
		c:      make(chan notify.EventInfo, 100),
	}
	for _, dir := range dirs {
		err := notify.Watch(
			dir+"/...",
			w.c,
			notify.Remove,
			notify.Rename,
			notify.Create,
			notify.Write,
		)
		if err != nil {
			w.Close()
			return nil, err
		}
	}
	go w.run()
	return w, nil
}

func removePathFromEvents(events []Event, path string) (found bool) {
	for i := range events {
		e := &events[i]
		if e.Path == path {
			e.Path = ""
			found = true
		}
	}
	return
}

func (w *Watcher) run() {
	// Update events are delayed by 1x - 2x this interval to avoid multiple
	// updates for the same file and out of order remove and update events.
	interval := 200 * time.Millisecond
	ticker := &time.Ticker{
		C: make(chan time.Time),
	}
	tickerRunning := false
	next := make([]Event, 0, 100)
	pending := make([]Event, 0, 100)
	pendingRenamePath := ""
	for {
		select {
		case <-ticker.C:
			// println("tick")
			// ticker.Stop()
			for i, e := range next {
				if e.Path == "" {
					continue
				}
				if i > 0 && next[i-1].Path == e.Path {
					continue
				}
				// println("update", e.Path)
				w.Events <- e
			}
			next = next[:0]
			next, pending = pending, next
			if len(next) == 0 {
				ticker.Stop()
				tickerRunning = false
			}
		case e := <-w.c:
			if e == nil {
				return
			}
			// println("event", e.Path(), e.Event())
			if w.filename != "" && filepath.Base(e.Path()) != w.filename {
				// println("skip", e.Path())
				continue
			}
			switch e.Event() {
			case notify.Rename:
				if pendingRenamePath != "" {
					// println("rename", pendingRenamePath, e.Path())
					ev := Event{
						Op:      Rename,
						Path:    pendingRenamePath,
						OldPath: e.Path(),
					}
					pendingRenamePath = ""
					w.Events <- ev

					// Remove the previous path (reported second)
					// ev := Event{
					// 	Path: e.Path(),
					// 	Op:   Remove,
					// }
					// if removePathFromEvents(next, ev.Path) {
					// 	continue
					// }
					// if removePathFromEvents(pending, ev.Path) {
					// 	continue
					// }

					// // Add the new path (reported first)
					// w.Events <- ev
					// ev = Event{
					// 	Path: pendingRenamePath,
					// 	Op:   Update,
					// }
					// pendingRenamePath = ""
					// pending = append(pending, ev)
					// if !tickerRunning {
					// 	ticker = time.NewTicker(interval)
					// }
				} else {
					pendingRenamePath = e.Path()
				}

			case notify.Create,
				notify.Write:
				pending = append(pending, Event{
					Op:   Update,
					Path: e.Path(),
				})
				if !tickerRunning {
					ticker = time.NewTicker(interval)
				}
			case notify.Remove:
				ev := Event{
					Op:   Remove,
					Path: e.Path(),
				}
				// println("remove", ev.Path)
				if removePathFromEvents(next, ev.Path) {
					continue
				}
				if removePathFromEvents(pending, ev.Path) {
					continue
				}
				w.Events <- ev
			}
		}
	}
}

func (w *Watcher) Close() {
	if w == nil {
		return
	}
	if w.c != nil {
		notify.Stop(w.c)
		close(w.c)
	}
	if w.Events != nil {
		close(w.Events)
	}
}
