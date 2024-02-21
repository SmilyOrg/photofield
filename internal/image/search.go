package image

import (
	"container/heap"
	"log"
	"math"
	"path/filepath"
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
	for i := range dirs {
		dirs[i] = filepath.FromSlash(dirs[i])
	}
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
		dot, err := clip.DotProductFloat32Float32(timg.Emb, emb.Float32())
		if err != nil {
			log.Printf("Unable to compute dot product for %d: %s", timg.Id, err.Error())
			continue
		}
		cosine := dot * timg.EmbInvNorm * emb.InvNormFloat32()
		dist := float32(math.Max(0, float64(1-cosine)))

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
	for i := range dirs {
		dirs[i] = filepath.FromSlash(dirs[i])
	}
	out := make(chan SourcedInfo, 1000)
	go func() {
		defer metrics.Elapsed("list knn")()

		// Parse tags from query
		tags := make([]knnTag, 0, 10)
		searchTags := make(map[tag.Id]struct{})
		tagNames := make([]string, 0, 10)
		for _, term := range options.Query.Terms {
			if term.Qualifier != nil && term.Qualifier.Key == "tagi" {
				tags = append(tags, knnTag{
					not: term.Not,
				})
				tagNames = append(tagNames, term.Qualifier.Value)
			}
		}

		// Get tag ids
		tagIds, ok := source.database.GetTagIds(tagNames)
		if !ok {
			log.Printf("Unable to get tag ids for %v", tagNames)
			close(out)
			return
		}
		for i, id := range tagIds {
			tags[i].id = id
			if !tags[i].not {
				searchTags[id] = struct{}{}
			}
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
				timgs = append(timgs, taggedImage{
					Id:         id,
					TagId:      tag.id,
					Emb:        emb.Float32(),
					EmbInvNorm: emb.InvNormFloat32(),
				})
			}
		}

		k, err := options.Query.QualifierInt("k")
		if err != nil {
			k = 5
		}

		// topk := make(knnImageHeap, 0, k)

		done := metrics.Elapsed("list knn embeddings")
		embeddings := source.database.ListEmbeddings(dirs, options)
		for emb := range embeddings {
			topTagId, meanDist, err := source.classifyEmbedding(timgs, emb, k)
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
			log.Printf("Image %d: %d, %f, %v", emb.Id, topTagId, meanDist, hit)
		}

		done()

		// infos := source.database.List(dirs, options)
		// for info := range infos {
		// 	// if info.NeedsMeta() || info.NeedsColor() {
		// 	// 	info.Info = source.GetInfo(info.Id)
		// 	// }
		// 	out <- info.SourcedInfo
		// }
		close(out)
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

type taggedImage2 struct {
	Id     ImageId
	TagId  tag.Id
	Emb    clip.Embedding
	Weight float32
}

func cosineSimilarity(a, b clip.Embedding) (float32, error) {
	dot, err := clip.DotProductFloat32Float32(a.Float32(), b.Float32())
	if err != nil {
		return 0, err
	}
	invNormA := a.InvNormFloat32()
	invNormB := b.InvNormFloat32()
	return dot * invNormA * invNormB, nil
}

func cosineSimilarity64(a, b clip.Embedding) (float64, error) {
	dot, err := clip.DotProductFloat32Float32(a.Float32(), b.Float32())
	if err != nil {
		return 0, err
	}
	invNormA := float64(a.InvNormFloat32())
	invNormB := float64(b.InvNormFloat32())
	return float64(dot) * invNormA * invNormB, nil
}

func negativeLogSimilarity(a, b clip.Embedding) (float64, error) {
	d, err := cosineSimilarity64(a, b)
	if err != nil {
		return 0, err
	}
	return -math.Log(d), nil
}

func distanceFunc(a, b clip.Embedding) (float32, error) {
	// d, err = cosineSimilarity(a, b)
	// d = (d + 1) * 0.5
	// return
	d, err := negativeLogSimilarity(a, b)
	return float32(d), err
}

func softmax(x []float64) []float64 {
	sum := 0.0
	for _, xi := range x {
		sum += math.Exp(xi)
	}

	result := make([]float64, len(x))
	for i, xi := range x {
		result[i] = math.Exp(xi) / sum
	}

	return result
}

// func weightedContrastiveLoss(distances []float64, weights []float64) float64 {
// 	loss := 0.0
// 	for i := range distances {
// 		if weights[i] > 0.5 { // Positive pair
// 			loss += math.Pow(distances[i], 2) * weights[i] // Square the distance for emphasis
// 		} else { // Negative pair (optional)
// 			loss += math.Log(1+math.Exp(distances[i])) * (1 - weights[i]) // Encourage larger margins
// 		}
// 	}
// 	return loss
// }

// func softMarginLoss(distances []float64, weights []float64) float64 {
// 	loss := 0.0
// 	for i := range distances {
// 		loss += weights[i] * math.Log(1+math.Exp(distances[i]))
// 	}
// 	return loss
// }

// func softMarginLoss(a []float32, b []float32) (float32, error) {
// 	l := len(a)
// 	if l != len(b) {
// 		return 0, fmt.Errorf("slice lengths do not match, a %d b %d", l, len(b))
// 	}

// 	loss := float32(0)
// 	for i := 0; i < l; i++ {
// 		loss += float32(math.Log(1 + math.Exp(float64(a[i]*b[i]))))
// 	}
// 	return loss, nil
// }

func (source *Source) evaluate(train []taggedImage2, imageId ImageId) (float32, error) {
	emb, err := source.database.GetImageEmbedding(imageId)
	if err != nil {
		return 0, err
	}

	// m := float32(0.8)

	sum := float32(0)
	for _, p := range train {
		// dist, err := distanceFunc(p.Emb, emb)
		c, err := cosineSimilarity(p.Emb, emb)
		if err != nil {
			log.Printf("Unable to compute similarity: %s", err.Error())
			continue
		}
		// w := float32(-1.)
		// if p.Weight < 0 {
		// 	w = 1.
		// }
		// loss := float32(math.Log(1 + math.Exp(-float64(w*c))))
		// sum += loss
		sum += c * p.Weight
		// sum += loss * p.Weight
		// log.Printf("Cosine similarity between %d and %d (%d): %f", p.Id, imageId, p.TagId, c)
		// log.Printf("%d & %d (%d), cos %f, loss: %f", imageId, p.Id, p.TagId, c, loss)
		// diss := float32(math.Max(0, float64(m-dist)))
		// loss := (1-p.Weight)*0.5*dist*dist + p.Weight*0.5*diss*diss
		// log.Printf("Distance between %d and %d (%d): %f, loss: %f", p.Id, imageId, p.TagId, dist, loss)
		// sum += dist * p.Weight
		// sum += loss
	}

	// sum /= float32(len(train))
	log.Printf("Image %d: %f", imageId, sum)

	return sum, nil
	// sum := float64(0)
	// for _, p := range train {
	// 	dist, err := negativeLogSimilarity(p.Emb, emb)
	// 	log.Printf("Distance between %d and %d (%d): %f", p.Id, imageId, p.TagId, dist)
	// 	if err != nil {
	// 		log.Printf("Unable to compute distance: %s", err.Error())
	// 		continue
	// 	}
	// 	sum += dist * p.Weight
	// }
	// log.Printf("Image %d: %f", imageId, sum)
	// return sum, nil
}

func distances(embeddings []clip.Embedding, emb clip.Embedding) ([]float64, error) {
	distances := make([]float64, 0, len(embeddings))
	for _, e := range embeddings {
		dist, err := negativeLogSimilarity(e, emb)
		if err != nil {
			return nil, err
		}
		distances = append(distances, dist)
	}
	return distances, nil
}

func printv(v []float64) {
	for i, x := range v {
		log.Printf("%d: %f", i, x)
	}
}

func (source *Source) evaluate2(imageId ImageId, embp, embn []clip.Embedding) (float32, error) {
	emb, err := source.database.GetImageEmbedding(imageId)
	if err != nil {
		return 0, err
	}

	distp, err := distances(embp, emb)
	if err != nil {
		return 0, err
	}
	distn, err := distances(embn, emb)
	if err != nil {
		return 0, err
	}

	printv(distp)
	printv(distn)

	// smp := softmax([]float64(distp))
	// printv(smp)

	return 0, nil
}

type taggedImageDistance struct {
	taggedImage2
	distance float32
}

func (source *Source) evaluate3(train []taggedImage2, imageId ImageId) (float32, error) {

	k := 5

	emb, err := source.database.GetImageEmbedding(imageId)
	if err != nil {
		return 0, err
	}

	distances := make([]taggedImageDistance, 0, len(train))

	sum := float32(0)
	for _, p := range train {
		c, err := cosineSimilarity(p.Emb, emb)
		if err != nil {
			log.Printf("Unable to compute similarity: %s", err.Error())
			continue
		}
		dist := float32(math.Max(0, float64(1-c)))
		dist *= p.Weight
		distances = append(distances, taggedImageDistance{
			taggedImage2: p,
			distance:     dist,
		})
	}

	sort.Slice(distances, func(i, j int) bool {
		return distances[i].distance < distances[j].distance
	})

	// for _, p := range distances[:k] {
	// log.Printf("Distance between %d and %d (%d): %f", p.Id, imageId, p.TagId, p.distance)
	// }

	tagCounts := make(map[tag.Id]int)
	for _, p := range distances[:k] {
		tagCounts[p.TagId]++
		sum += p.distance
	}

	sum /= float32(k)

	var mostFrequentTagId tag.Id
	maxCount := 0
	for tagId, count := range tagCounts {
		if count > maxCount {
			mostFrequentTagId = tagId
			maxCount = count
		}
	}

	log.Printf("Image %d: %f, most frequent tag: %d", imageId, sum, mostFrequentTagId)

	return sum, nil
}

func (source *Source) textTaggedImage(text string, tagId tag.Id) taggedImage2 {
	var t taggedImage2
	emb, err := source.AI.EmbedText(text)
	if err != nil {
		log.Printf("Unable to get embedding for %s: %s", text, err.Error())
		return t
	}
	return taggedImage2{
		Id:    ImageId(0),
		TagId: tagId,
		Emb:   emb,
		// Weight: 0,
	}
}
