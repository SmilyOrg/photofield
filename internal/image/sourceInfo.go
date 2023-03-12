package image

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (source *Source) loadInfosMeta(ids <-chan uint32) {
	for iduint := range ids {
		id := ImageId(iduint)
		path, err := source.GetImagePath(id)
		if err != nil {
			fmt.Println("Unable to find image path", err, path)
			continue
		}
		info, err := source.LoadInfoMeta(path)
		if err != nil {
			fmt.Println("Unable to load image info meta", err, path)
			continue
		}
		source.database.Write(path, info, UpdateMeta)
		source.imageInfoCache.Delete(id)
	}
}

func (source *Source) loadInfosColor(ids <-chan uint32) {
	for iduint := range ids {
		id := ImageId(iduint)
		path, err := source.GetImagePath(id)
		if err != nil {
			fmt.Println("Unable to find image path", err, path)
			continue
		}
		info, err := source.LoadInfoColor(path)
		if err != nil {
			fmt.Println("Unable to load image info color", err, path)
			continue
		}
		source.database.Write(path, info, UpdateColor)
		source.imageInfoCache.Delete(id)
	}
}

func (source *Source) loadInfosAI(ids <-chan uint32) {
	for iduint := range ids {
		id := ImageId(iduint)
		path, err := source.GetImagePath(id)
		if err != nil {
			fmt.Println("Unable to find image path", err, path)
			continue
		}

		minSize := 200
		f, err := source.OpenSmallestThumbnail(path, minSize)
		if err != nil {
			fmt.Println("Unable to load smallest image", err, path)
			continue
		}

		embedding, err := source.Clip.EmbedImageReader(f)
		f.Close()

		if err != nil {
			fmt.Println("Unable to get image embedding", err, path)
			continue
		}

		source.database.WriteAI(id, embedding)
	}
}

func (source *Source) heuristicFromPath(path string) (Info, error) {
	var info Info

	info.Width = 4000
	info.Height = 3000
	info.Color = 0xFFE8EAED

	baseName := filepath.Base(path)
	name := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	for _, format := range source.DateFormats {
		date, err := time.Parse(format, name)
		if err == nil {
			info.DateTime = date
			break
		}
	}

	if info.DateTime.IsZero() {
		fileInfo, err := os.Stat(path)
		if err == nil {
			info.DateTime = fileInfo.ModTime()
		}
	}

	return info, nil
}

func (source *Source) GetInfo(id ImageId) Info {
	var info Info
	var found bool

	logging := false

	totalStartTime := time.Now()

	startTime := time.Now()
	info, found = source.imageInfoCache.Get(id)
	cacheGetMs := time.Since(startTime).Milliseconds()
	if found {
		// if (logging) log.Printf("image info %5d ms get cache\n", cacheGetMs)
		return info
	}

	startTime = time.Now()
	result, found := source.database.Get(id)
	info = result.Info
	dbGetMs := time.Since(startTime).Milliseconds()
	needsMeta := result.NeedsMeta()
	if found && !needsMeta {
		startTime = time.Now()
		source.imageInfoCache.Set(id, info)
		cacheSetMs := time.Since(startTime).Milliseconds()
		if logging {
			log.Printf("image info %5d ms get cache, %5d ms get db, %5d ms set cache\n", cacheGetMs, dbGetMs, cacheSetMs)
		}
	}

	startTime = time.Now()
	// needsColor := result.NeedsColor()
	// if needsMeta || needsColor {
	// 	if needsMeta {
	// 		source.MetaQueue.Append(uint32(id))
	// 	}
	// 	if needsColor {
	// 		source.ColorQueue.Append(uint32(id))
	// 	}
	// }
	addPendingMs := time.Since(startTime).Milliseconds()

	if found && !needsMeta {
		return info
	}

	startTime = time.Now()
	{
		path, err := source.GetImagePath(id)
		if err == nil {
			info, err = source.heuristicFromPath(path)
			if err != nil {
				fmt.Println("Unable to load image info heuristic", err, path)
			}
		} else {
			fmt.Println("Unable to get path from image id", err, id)
		}
	}
	heuristicGetMs := time.Since(startTime).Milliseconds()

	startTime = time.Now()
	source.imageInfoCache.Set(id, info)
	cacheSetMs := time.Since(startTime).Milliseconds()

	totalMs := time.Since(totalStartTime).Milliseconds()

	logging = totalMs > 1000

	if logging {
		log.Printf("image info %5d ms get cache, %5d ms get db, %5d ms add pending, %5d ms get heuristic, %5d ms set cache\n", cacheGetMs, dbGetMs, addPendingMs, heuristicGetMs, cacheSetMs)
	}
	return info
}
