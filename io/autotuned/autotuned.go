package autotuned

import (
	"context"
	goio "io"
	"math"
	"sync"
	"time"

	"photofield/io"
	"photofield/io/configured"
)

const (
	defaultLearningRate      = 0.001
	defaultRegularization    = 0.01
	defaultEpsilon           = 0.0001
	minSamplesForPrediction  = 6
	acceleratedLearningCount = 50
)

type parameter struct {
	value float64
	grad  float64
}

func (p *parameter) update(error, input, lr, lambda float64) {
	p.grad = error * input
	p.value += lr * (p.grad - lambda*math.Copysign(1, p.value))
}

// Update Model struct to include normalizers
type Model struct {
	mu          sync.RWMutex
	c           parameter
	kOrig       parameter
	kThumb      parameter
	lambda      float64
	epsilon     float64
	sampleCount int
	source      *configured.Configured
}

// Update predict method to use normalized values
func (m *Model) predict(orig, thumb io.Size) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	origPixels := float64(orig.Area()) / 1e6
	thumbPixels := float64(thumb.Area()) / 1e6

	latencyMs := m.c.value + m.kOrig.value*origPixels + m.kThumb.value*thumbPixels
	return time.Duration(latencyMs * float64(time.Millisecond))
}

// Update the update method to use normalized values
func (m *Model) update(orig, thumb io.Size, observed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sampleCount++
	lr := defaultLearningRate

	origPixels := float64(orig.Area()) / 1e6
	thumbPixels := float64(thumb.Area()) / 1e6

	predicted := m.c.value + m.kOrig.value*origPixels + m.kThumb.value*thumbPixels
	observed_ms := float64(observed) / float64(time.Millisecond)
	error := observed_ms - predicted

	// Update coefficients with normalized features
	m.c.update(error, 1, lr, m.lambda)
	m.kOrig.update(error, origPixels, lr, m.lambda)
	m.kThumb.update(error, thumbPixels, lr, m.lambda)

	// fmt.Printf(
	// 	"source %10s, sample %6d, observed % 8.2f ms, predicted % 8.2f ms, error % 8.2f ms, c % 6.2f, kOrig % 6.2f, kThumb % 6.2f\n",
	// 	m.source.Name(), m.sampleCount, observed_ms, predicted, error, m.c.value, m.kOrig.value, m.kThumb.value,
	// )
	// fmt.Printf(
	// 	"source %10s, sample %4d, orig raw % 6.2f norm % 6.2f mean % 4.2f +- % 4.2f MP, thumb raw % 6.2f norm % 6.2f +- % 4.2f MP\n",
	// 	m.source.Name(), m.sampleCount, origPixels, origNorm, m.origNormalizer.mean, math.Sqrt(m.origNormalizer.variance), thumbPixels, thumbNorm, math.Sqrt(m.thumbNormalizer.variance),
	// )
}

type Autotuned struct {
	source *configured.Configured
	model  *Model
}

// Update New function to initialize normalizers
func New(source *configured.Configured) *Autotuned {
	return &Autotuned{
		source: source,
		model: &Model{
			lambda:  defaultRegularization,
			epsilon: defaultEpsilon,
			source:  source,
		},
	}
}

// Implement io.Source interface
func (a *Autotuned) Get(ctx context.Context, id io.ImageId, path string, original io.Size) io.Result {
	start := time.Now()
	result := a.source.Get(ctx, id, path, original)
	if result.Error == nil && !result.FromCache {
		thumb := a.source.Size(original)
		a.model.update(original, thumb, time.Since(start))
	}
	return result
}

func (a *Autotuned) GetDurationEstimate(original io.Size) time.Duration {
	if a.model.sampleCount < minSamplesForPrediction {
		return a.source.GetDurationEstimate(original)
	}
	resized := a.source.Size(original)
	return a.model.predict(original, resized)
}

// Forward remaining io.Source interface methods
func (a *Autotuned) Name() string              { return a.source.Name() }
func (a *Autotuned) DisplayName() string       { return a.source.DisplayName() }
func (a *Autotuned) Ext() string               { return a.source.Ext() }
func (a *Autotuned) Size(size io.Size) io.Size { return a.source.Size(size) }
func (a *Autotuned) Rotate() bool              { return a.source.Rotate() }
func (a *Autotuned) Close() error              { return a.source.Close() }
func (a *Autotuned) Exists(ctx context.Context, id io.ImageId, path string) bool {
	return a.source.Exists(ctx, id, path)
}
func (a *Autotuned) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	a.source.Reader(ctx, id, path, fn)
}
func (a *Autotuned) Decode(ctx context.Context, r goio.Reader) io.Result {
	return a.source.Decode(ctx, r)
}
