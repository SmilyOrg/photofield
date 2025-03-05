package bench

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"photofield/io"
	"testing"
)

type Sample struct {
	Id   io.ImageId
	Path string
	Size io.Size
}

type sourceWithSamples struct {
	source  io.Source
	samples []Sample
}

func BenchmarkSources(seed int64, sources io.Sources, samples []Sample, count int) {
	log.Printf("benchmark build samples")
	workingSources := make([]sourceWithSamples, 0, len(sources))
	for _, source := range sources {
		workingSources = append(workingSources, sourceWithSamples{
			source,
			workingSamples(source, samples),
		})
	}
	log.Printf("benchmark run")
	maxLen := 20
	for i := 0; i < count; i++ {
		for _, s := range workingSources {
			if len(s.samples) == 0 {
				fmt.Printf("# BenchmarkSourceGet/%-*s\t%s\n", maxLen, s.source.Name(), "no samples")
				continue
			}
			r := testing.Benchmark(func(b *testing.B) {
				BenchmarkSource(b, seed, s.source, s.samples)
			})
			if r.N == 0 {
				fmt.Printf("# BenchmarkSourceGet/%-*s\t%s\n", maxLen, s.source.Name(), "error")
				continue
			}
			fmt.Printf("BenchmarkSourceGet/%-*s\t%s\t%s\n", maxLen, s.source.Name(), r.String(), r.MemString())
		}
	}
}

func workingSamples(source io.Source, samples []Sample) []Sample {
	working := make([]Sample, 0, len(samples))
	for _, sample := range samples {
		if source.Exists(context.Background(), sample.Id, sample.Path) {
			working = append(working, sample)
		}
	}
	return working
}

func BenchmarkSource(b *testing.B, seed int64, source io.Source, samples []Sample) {
	b.StopTimer()
	ctx := context.Background()
	b.ReportMetric(float64(len(samples)), "samples")

	rnd := rand.New(rand.NewSource(seed))

	for i := 0; i < b.N; i++ {
		sample := samples[rnd.Intn(len(samples))]
		resized := source.Size(sample.Size)
		b.StartTimer()
		r := source.Get(ctx, sample.Id, sample.Path, sample.Size)
		b.StopTimer()
		if r.Error != nil {
			b.Fatal(r.Error)
		}
		ns := float64(b.Elapsed().Nanoseconds())
		origmp := float64(sample.Size.Area()) / 1e6
		b.ReportMetric(ns/origmp/float64(b.N), "ns/origmp/op")
		resmp := float64(resized.Area()) / 1e6
		b.ReportMetric(ns/resmp/float64(b.N), "ns/resmp/op")
		gotsize := io.Size{X: r.Image.Bounds().Dx(), Y: r.Image.Bounds().Dy()}
		gotmp := float64(gotsize.Area()) / 1e6
		b.ReportMetric(ns/gotmp/float64(b.N), "ns/gotmp/op")
	}
}
