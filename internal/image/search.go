package image

import (
	"log"
	"path/filepath"
	"photofield/internal/clip"
	"photofield/internal/metrics"
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

		close(out)
	}()
	return out
}
