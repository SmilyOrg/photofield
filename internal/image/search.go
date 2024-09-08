package image

import (
	"container/heap"
	"log"
	"math"
	"photofield/internal/clip"
	"photofield/internal/metrics"
	"photofield/rangetree"
	"photofield/tag"
	"sort"

	"github.com/kelindar/intmap"
)

type similar struct {
	id         ImageId
	similarity float32
}

func (source *Source) getSimilarityInfos(list []similar) []SimilarityInfo {
	size := len(list)
	idToIndex := intmap.New(size*4, 0.25)
	infos := make([]SimilarityInfo, size)
	ids := make([]ImageId, size)
	for i := 0; i < size; i++ {
		similar := list[i]
		ids[i] = similar.id
		idToIndex.Store(uint32(similar.id), uint32(i))
	}
	for info := range source.database.GetBatch(ids) {
		index, ok := idToIndex.Load(uint32(info.Id))
		if !ok {
			log.Printf("Unable to look up similarity index for %d\n", info.Id)
			continue
		}
		infos[index] = SimilarityInfo{
			SourcedInfo: info.SourcedInfo,
			Similarity:  list[int(index)].similarity,
		}
	}
	return infos
}

type similarityBatchWorker struct {
	source *Source
	input  chan []similar
	output chan []SimilarityInfo
}

func (w similarityBatchWorker) run() {
	for batch := range w.input {
		w.output <- w.source.getSimilarityInfos(batch)
	}
	close(w.output)
}

func (w similarityBatchWorker) close() {
	close(w.input)
}

func (source *Source) ListSimilar(dirs []string, embedding clip.Embedding, options ListOptions) <-chan SimilarityInfo {
	out := make(chan SimilarityInfo, 1000)
	go func() {
		defer metrics.Elapsed("list similar")()
		defer close(out)

		if embedding == nil {
			return
		}

		// Prepare search term embedding
		similars := make([]similar, 0, 1000)
		search := embedding.Float32()
		searchInvNorm := embedding.InvNormFloat32()

		// List all related embeddings and compute their similarity
		done := metrics.Elapsed("list similar embeddings")
		embeddings := source.database.ListEmbeddings(dirs, options)
		for emb := range embeddings {
			dot, err := clip.DotProductFloat32Float(search, emb.Float())
			if err != nil {
				log.Printf("Unable to compute dot product for %d: %s", emb.Id, err.Error())
				continue
			}

			similarity := dot * searchInvNorm * emb.InvNormFloat32()
			similars = append(similars, similar{
				id:         emb.Id,
				similarity: similarity,
			})
		}
		done()

		// Sort embeddings by similarity
		done = metrics.Elapsed("list similar sort")
		sort.Slice(similars, func(i, j int) bool {
			return similars[i].similarity > similars[j].similarity
		})
		done()

		// Get image info for the sorted embeddings in batches
		done = metrics.Elapsed("list similar batches")
		list := similars
		length := len(list)
		batch := 1000

		const workerCount = 10
		var workers [workerCount]similarityBatchWorker
		for i := 0; i < workerCount; i++ {
			workers[i] = similarityBatchWorker{
				source: source,
				input:  make(chan []similar, 1),
				output: make(chan []SimilarityInfo, 1),
			}
			go workers[i].run()
		}

		batchCount := length / batch
		i := 0
		bi := 0
		for {
			// Needs to be done in two parts to maintain ordered output
			wsent := 0
			for ; wsent < workerCount && bi < batchCount; wsent++ {
				workers[wsent].input <- list[i : i+batch]
				bi++
				i += batch
			}
			for wrecv := 0; wrecv < wsent; wrecv++ {
				infos := <-workers[wrecv].output
				for _, info := range infos {
					out <- info
				}
			}
			if bi >= batchCount {
				break
			}
		}
		for _, w := range workers {
			w.close()
		}
		done()

		done = metrics.Elapsed("list similar remainder")
		for ; i < length; i++ {
			similar := list[i]
			info, ok := source.database.Get(similar.id)
			if ok {
				out <- SimilarityInfo{
					SourcedInfo: SourcedInfo{
						Id:   similar.id,
						Info: info.Info,
					},
					Similarity: similar.similarity,
				}
			}
		}
		done()
	}()
	return out
}

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

func (source *Source) classifyEmbedding(timgs []taggedImage, emb clip.Embedding, k int) (tag.Id, float32, error) {
	topk := make(knnImageHeap, 0, k)
	for _, timg := range timgs {
		dot, err := clip.DotProductFloat32Float(timg.Emb, emb.Float())
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
		for _, term := range options.Query.Terms {
			if term.Qualifier != nil && term.Qualifier.Key == "tag" {
				name := term.Qualifier.Value
				id, ok := source.database.GetTagId(name)
				if !ok {
					continue
				}
				tags = append(tags, knnTag{
					id:  id,
					not: term.Not,
				})
				if !term.Not {
					searchTags[id] = struct{}{}
				}
				println("tag", name, id, term.Not)
			}
		}

		bias, err := options.Query.QualifierFloat32("bias")
		if err != nil {
			bias = 0
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

		k, err := options.Query.QualifierInt("k")
		if err != nil {
			k = 5
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
