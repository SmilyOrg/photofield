package image

import (
	"container/heap"
	"log"
	"math"
	"photofield/internal/ai"
	"photofield/internal/metrics"
	"photofield/internal/rangetree"
	"photofield/internal/tag"
)
type taggedImage struct {
	Id         ImageId
	TagId      tag.Id
	Emb        []float32
	EmbInvNorm float32
	Weight     float32
}

type knnTag struct {
	id  tag.Id
	not bool
}

type distTag struct {
	id   tag.Id
	dist float32
}

type knnImageHeap []distTag

func (h knnImageHeap) Len() int           { return len(h) }
func (h knnImageHeap) Less(i, j int) bool { return h[i].dist > h[j].dist }
func (h knnImageHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *knnImageHeap) Push(x interface{}) {
	*h = append(*h, x.(distTag))
}
func (h *knnImageHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func (source *Source) classifyEmbedding(timgs []taggedImage, emb ai.Embedding, k int) (tag.Id, float32, error) {
	topk := make(knnImageHeap, 0, k)
	for _, timg := range timgs {
		dot, err := ai.DotProductFloat32Float(timg.Emb, emb.Float())
		if err != nil {
			log.Printf("Unable to compute dot product for %d: %s", timg.Id, err.Error())
			continue
		}
		cosine := dot * timg.EmbInvNorm * emb.InvNormFloat32()
		dist := float32(math.Max(0, float64(1-cosine*timg.Weight)))

		if len(topk) < k {
			heap.Push(&topk, distTag{
				id:   timg.TagId,
				dist: dist,
			})
		} else if dist < topk[0].dist {
			heap.Pop(&topk)
			heap.Push(&topk, distTag{
				id:   timg.TagId,
				dist: dist,
			})
		}
	}
	topTagId := tag.Id(0)
	topTagCount := 0
	meanDist := float32(0)
	for _, t := range topk {
		meanDist += t.dist
		if t.id == topTagId {
			topTagCount++
		} else {
			topTagId = t.id
			topTagCount = 1
		}
	}
	if len(topk) > 0 {
		meanDist /= float32(len(topk))
	}
	return topTagId, meanDist, nil
}

func (source *Source) ListKnn(dirs []string, options ListOptions) <-chan SourcedInfo {
	out := make(chan SourcedInfo, 1000)
	go func() {
		defer metrics.Elapsed("list knn")()

		// Parse tags from query
		tags := make([]knnTag, 0, 10)
		searchTags := make(map[tag.Id]struct{})
		for _, tag := range options.Expression.Tags {
			id, ok := source.database.GetTagId(tag.Value)
			if !ok {
				continue
			}
			tags = append(tags, knnTag{
				id:  id,
				not: tag.Token.Not,
			})
			if !tag.Token.Not {
				searchTags[id] = struct{}{}
			}
			println("tag", tag.Value, id, tag.Token.Not)
		}

		bias := 0.0
		if options.Expression.Bias.Present {
			bias = float64(options.Expression.Bias.Value)
		}

		// Get embeddings for all images with the specified tags
		timgs := make([]taggedImage, 0, 1000)
		for _, tag := range tags {
			log.Printf("Tag %d: %v", tag.id, tag.not)
			for id := range source.rangesToImageIds(source.database.ListTagRanges(tag.id)) {
				emb, err := source.database.GetImageEmbedding(id)
				if err != nil {
					log.Printf("Unable to get embedding for %d: %s", id, err.Error())
					continue
				}
				weight := float32(1)
				if !tag.not {
					weight = float32(1 + bias)
				}
				timgs = append(timgs, taggedImage{
					Id:         id,
					TagId:      tag.id,
					Emb:        emb.Float32(),
					EmbInvNorm: emb.InvNormFloat32(),
					Weight:     weight,
				})
			}
		}

		k := 5
		if options.Expression.K.Present {
			k = int(options.Expression.K.Value)
		}

		done := metrics.Elapsed("list knn embeddings")
		embeddings := source.database.ListEmbeddings(dirs, options)
		for emb := range embeddings {
			topTagId, _, err := source.classifyEmbedding(timgs, emb, k)
			if err != nil {
				log.Printf("Unable to classify embedding for %d: %s", emb.Id, err.Error())
				continue
			}
			_, hit := searchTags[topTagId]
			if hit {
				info := <-source.database.GetBatch([]ImageId{emb.Id})
				out <- SourcedInfo{
					Id:   emb.Id,
					Info: info.Info,
				}
			}
			// log.Printf("Image %d: %d, %f, %v", emb.Id, topTagId, meanDist, hit)
		}
		close(out)
		done()
	}()
	return out
}

func (source *Source) rangesToImageIds(ranges <-chan rangetree.Range) <-chan ImageId {
	out := make(chan ImageId, 10)
	go func() {
		for r := range ranges {
			for i := range r.Chan() {
				out <- ImageId(i)
			}
		}
		close(out)
	}()
	return out
}
