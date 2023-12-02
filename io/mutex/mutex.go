package mutex

import (
	"context"
	"fmt"
	"photofield/io"
	"sync"
	"time"
)

type Mutex struct {
	Source  io.Source
	loading sync.Map
}

func (m Mutex) Close() error {
	return m.Source.Close()
}

type loadingResult struct {
	result io.Result
	loaded chan struct{}
}

func (m Mutex) Name() string {
	return fmt.Sprintf("%s (mutex)", m.Source.Name())
}

func (m Mutex) Size(size io.Size) io.Size {
	return m.Source.Size(size)
}

func (m Mutex) GetDurationEstimate(size io.Size) time.Duration {
	return m.Source.GetDurationEstimate(size)
}

func (m Mutex) Rotate() bool {
	return false
}

func (m Mutex) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	loading := &loadingResult{}
	loading.loaded = make(chan struct{})
	key := id
	stored, loaded := m.loading.LoadOrStore(key, loading)
	if loaded {
		loading = stored.(*loadingResult)
		fmt.Printf("%v blocking on channel\n", key)
		<-loading.loaded
		fmt.Printf("%v channel unblocked\n", key)
		return loading.result
	}

	fmt.Printf("%v not found, loading, mutex locked\n", key)
	loading.result = m.Source.Get(ctx, id, path)
	fmt.Printf("%v loaded, closing channel\n", key)
	close(loading.loaded)
	return loading.result
}

func (m Mutex) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	return false
}
