package image

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	goio "io"

	"photofield/internal/ai"
	"photofield/internal/geo"
	"photofield/internal/io"
	"photofield/internal/io/djpeg"
	"photofield/internal/io/exiftool"
	"photofield/internal/io/ffmpeg"
	"photofield/internal/io/ristretto"
	"photofield/internal/io/sqlite"
	"photofield/internal/metrics"
	"photofield/internal/tag"

	"github.com/docker/go-units"
	"github.com/prometheus/client_golang/prometheus"
)

var ErrNotFound = errors.New("not found")
var ErrNotAnImage = errors.New("not a supported image extension, might be video")
var ErrUnavailable = errors.New("unavailable")

type ImageId uint32

func IdsToUint32(ids <-chan ImageId) <-chan uint32 {
	out := make(chan uint32)
	go func() {
		for id := range ids {
			out <- uint32(id)
		}
		close(out)
	}()
	return out
}

type SourcedInfo struct {
	Id         ImageId
	Similarity float32
	Info
}

type Missing struct {
	Metadata  bool
	Color     bool
	Embedding bool
	Faces     bool
}

type IdPath struct {
	Id   ImageId
	Path string
}

type MissingInfo struct {
	Id   ImageId
	Path string
	Missing
}

type InfoEmb struct {
	SourcedInfo
	Embedding ai.Embedding
}

type CacheConfig struct {
	MaxSize string `json:"max_size"`
}

func (config *CacheConfig) MaxSizeBytes() int64 {
	value, err := units.FromHumanSize(config.MaxSize)
	if err != nil {
		panic(err)
	}
	return value
}

type Caches struct {
	Image CacheConfig `json:"image"`
}

func (config *Config) MaxFaceFileSizeBytes() int64 {
	if config.MaxFaceFileSize == "" {
		return 0
	}
	value, err := units.FromHumanSize(config.MaxFaceFileSize)
	if err != nil {
		panic(err)
	}
	return value
}

type Config struct {
	DataDir   string
	AI        ai.AI
	TagConfig tag.Config `json:"-"`

	// Binary paths for external tools
	FFmpegPath   string `json:"ffmpeg_path"`
	DjpegPath    string `json:"djpeg_path"`
	ExifToolPath string `json:"exif_tool_path"`

	ExifToolCount        int    `json:"exif_tool_count"`
	SkipLoadInfo         bool   `json:"skip_load_info"`
	SkipCollectionCounts bool   `json:"skip_collection_counts"`
	ConcurrentMetaLoads  int    `json:"concurrent_meta_loads"`
	ConcurrentColorLoads int    `json:"concurrent_color_loads"`
	ConcurrentAILoads    int    `json:"concurrent_ai_loads"`
	MaxFaceFileSize      string `json:"max_face_file_size"`

	ListExtensions []string        `json:"extensions"`
	DateFormats    []string        `json:"date_formats"`
	Images         FileConfig      `json:"images"`
	Videos         FileConfig      `json:"videos"`
	SourceTypes    SourceTypeMap   `json:"source_types"`
	Sources        SourceConfigs   `json:"sources"`
	Thumbnail      ThumbnailConfig `json:"thumbnail"`

	Caches Caches `json:"caches"`
}

type FileConfig struct {
	Extensions []string `json:"extensions"`
}

type Source struct {
	Config

	Sources                                    io.Sources
	SourceLatencyHistogram                     *prometheus.HistogramVec
	SourceLatencyAbsDiffHistogram              *prometheus.HistogramVec
	SourcePerOriginalMegapixelLatencyHistogram *prometheus.HistogramVec
	SourcePerResizedMegapixelLatencyHistogram  *prometheus.HistogramVec

	decoder  *Decoder
	database *Database

	imageCache     *ristretto.Ristretto
	imageInfoCache InfoCache
	pathCache      PathCache

	thumbnailSources    []io.ReadDecoderSource
	thumbnailGenerators io.Sources
	thumbnailSink       *sqlite.Source

	Clip ai.AI
	Geo  *geo.Geo
}

func NewSource(config Config, migrations embed.FS, geo *geo.Geo) *Source {
	source := Source{}
	source.Config = config
	source.database = NewDatabase(filepath.Join(config.DataDir, "photofield.cache.db"), migrations)
	source.imageInfoCache = newInfoCache()
	source.pathCache = newPathCache()
	source.Geo = geo

	source.SourceLatencyHistogram = metrics.AddHistogram(
		"source_latency",
		[]float64{500, 1000, 2500, 5000, 10000, 25000, 50000, 100000, 150000, 200000, 250000, 500000, 1000000, 2000000, 5000000, 10000000},
		[]string{"source"},
	)

	source.SourceLatencyAbsDiffHistogram = metrics.AddHistogram(
		"source_latency_abs_diff",
		[]float64{50, 100, 250, 500, 1000, 2500, 5000, 10000, 25000, 50000, 100000, 200000, 500000, 1000000},
		[]string{"source"},
	)

	source.SourcePerOriginalMegapixelLatencyHistogram = metrics.AddHistogram(
		"source_per_original_megapixel_latency",
		[]float64{500, 1000, 2500, 5000, 10000, 25000, 50000, 100000, 150000, 200000, 250000, 500000, 1000000, 2000000, 5000000, 10000000},
		[]string{"source"},
	)

	source.SourcePerResizedMegapixelLatencyHistogram = metrics.AddHistogram(
		"source_per_resized_megapixel_latency",
		[]float64{500, 1000, 2500, 5000, 10000, 25000, 50000, 100000, 150000, 200000, 250000, 500000, 1000000, 2000000, 5000000, 10000000},
		[]string{"source"},
	)

	source.imageCache = ristretto.New(config.Caches.Image.MaxSizeBytes())

	// Use configured paths or fall back to automatic discovery
	ffmpegPath := config.FFmpegPath
	if ffmpegPath == "" {
		ffmpegPath = ffmpeg.FindPath()
	}

	djpegPath := config.DjpegPath
	if djpegPath == "" {
		djpegPath = djpeg.FindPath()
	}

	exifToolPath := config.ExifToolPath
	if exifToolPath == "" {
		exifToolPath = exiftool.FindPath()
	}

	// Create decoder with configured exiftool path
	source.decoder = NewDecoder(config.ExifToolCount, exifToolPath)

	env := SourceEnvironment{
		SourceTypes:  config.SourceTypes,
		FFmpegPath:   ffmpegPath,
		DjpegPath:    djpegPath,
		ExifToolPath: exifToolPath,
		ImageCache:   source.imageCache,
		DataDir:      config.DataDir,
	}

	// Sources used for rendering
	srcs, err := config.Sources.NewSources(&env)
	if err != nil {
		log.Fatalf("failed to create sources: %s", err)
	}
	source.Sources = srcs

	// Further sources should not be cached
	env.ImageCache = nil

	tsrcs, err := config.Thumbnail.Sources.NewSources(&env)
	if err != nil {
		log.Fatalf("failed to create thumbnail sources: %s", err)
	}
	for _, s := range tsrcs {
		rd, ok := s.(io.ReadDecoderSource)
		if !ok {
			log.Fatalf("source %s does not implement io.ReadDecoder", s.Name())
		}
		source.thumbnailSources = append(source.thumbnailSources, rd)
	}

	gens, err := config.Thumbnail.Generators.NewSources(&env)
	if err != nil {
		log.Fatalf("failed to create thumbnail generators: %s", err)
	}
	source.thumbnailGenerators = gens

	sink, err := config.Thumbnail.Sink.NewSource(&env)
	if err != nil {
		log.Fatalf("failed to create thumbnail sink: %s", err)
	}
	sqliteSink, ok := sink.(*sqlite.Source)
	if !ok {
		log.Fatalf("thumbnail sink %s is not a sqlite source", sink.Name())
	}
	source.thumbnailSink = sqliteSink

	if config.SkipLoadInfo {
		log.Printf("skipping load info")
	} else {

		source.Clip = config.AI
	}

	return &source
}

func (source *Source) DB() *Database {
	return source.database
}

func (source *Source) Decoder() *Decoder {
	return source.decoder
}

func (source *Source) ThumbSources() []io.ReadDecoderSource {
	return source.thumbnailSources
}

func (source *Source) ThumbGenerators() io.Sources {
	return source.thumbnailGenerators
}

func (source *Source) ThumbSink() *sqlite.Source {
	return source.thumbnailSink
}

func (source *Source) HandleDirUpdates(fn DirsFunc) {
	source.database.HandleDirUpdates(fn)
}

func (source *Source) Vacuum() error {
	return source.database.vacuum()
}

func (source *Source) Close() {
	source.decoder.Close()
	source.database.Close()
	source.imageCache.Close()
	source.imageInfoCache.Close()
	source.pathCache.Close()
	source.Sources.Close()
	source.thumbnailSink.Close()
}

func (source *Source) Shutdown() {
	source.database.Close()
	source.thumbnailSink.Close()
}

func (source *Source) IsSupportedImage(path string) bool {
	supportedImage := false
	pathExt := strings.ToLower(filepath.Ext(path))
	for _, ext := range source.Images.Extensions {
		if pathExt == ext {
			supportedImage = true
			break
		}
	}
	return supportedImage
}

func (source *Source) IsSupportedVideo(path string) bool {
	pathExt := strings.ToLower(filepath.Ext(path))
	for _, ext := range source.Videos.Extensions {
		if pathExt == ext {
			return true
		}
	}
	return false
}

func (source *Source) ListImages(dirs []string, maxPhotos int) <-chan string {
	return source.database.ListPaths(dirs, maxPhotos)
}

func (source *Source) ListImageIds(dirs []string, maxPhotos int) <-chan ImageId {
	return source.database.ListIds(dirs, maxPhotos, false)
}

func (source *Source) ListMissingEmbeddingIds(dirs []string, maxPhotos int) <-chan ImageId {
	return source.database.ListIds(dirs, maxPhotos, true)
}

func (source *Source) ListInfos(dirs []string, options ListOptions) (<-chan SourcedInfo, Dependencies) {
	defer metrics.Elapsed("list infos")()
	return source.database.List(dirs, options)
}

func (source *Source) ListInfosEmb(dirs []string, options ListOptions) <-chan InfoEmb {
	defer metrics.Elapsed("list infos embedded")()
	return source.database.ListWithEmbeddings(dirs, options)
}

// Prefer using ImageId over this unless you absolutely need the path
func (source *Source) GetImagePath(id ImageId) (string, error) {
	path, ok := source.pathCache.Get(id)
	if ok {
		return path, nil
	}

	path, ok = source.database.GetPathFromId(id)
	if !ok {
		return "", ErrNotFound
	}
	source.pathCache.Set(id, path)
	return path, nil
}

func (source *Source) GetImageEmbedding(id ImageId) (ai.Embedding, error) {
	return source.database.GetImageEmbedding(id)
}

func (source *Source) GetFaceEmbedding(faceId int) (ai.Embedding, error) {
	return source.database.GetFaceEmbedding(faceId)
}

func (source *Source) GetFacesByFileId(fileId ImageId) []FaceInfo {
	return source.database.GetFacesByFileId(fileId)
}

func (source *Source) ListFaces(dirs []string, options FaceListOptions) <-chan FaceInfo {
	return source.database.ListFaces(dirs, options)
}

func (source *Source) GetDir(dir string) Info {
	if source == nil {
		return Info{}
	}

	result, _ := source.database.GetDir(dir)
	return result.Info
}

func (source *Source) GetDirsCount(dirs []string) int {
	count, _ := source.database.GetDirsCount(dirs)
	return count
}

func (source *Source) GetImageReader(id ImageId, sourceName string, fn func(r goio.ReadSeeker, err error)) {
	ctx := context.TODO()
	path, err := source.GetImagePath(id)
	if err != nil {
		fn(nil, err)
		return
	}
	found := false
	for _, s := range source.Sources {
		if s.Name() != sourceName {
			continue
		}
		r, ok := s.(io.Reader)
		if !ok {
			continue
		}
		r.Reader(ctx, io.ImageId(id), path, func(r goio.ReadSeeker, err error) {
			// println(id, sourceName, s.Name(), r, ok, err)
			if err != nil {
				return
			}
			found = true
			fn(r, nil)
		})
		if found {
			break
		}
	}
	if !found {
		fn(nil, fmt.Errorf("unable to find image %d using %s", id, sourceName))
	}
}

func (source *Source) AddTag(name string) {
	done, _ := source.database.AddTag(name)
	<-done
}

func (source *Source) GetTag(id tag.Id) (tag.Tag, bool) {
	return source.database.GetTag(id)
}

func (source *Source) GetTagByName(name string) (tag.Tag, bool) {
	return source.database.GetTagByName(name)
}

func (source *Source) ListImageTags(id ImageId) <-chan tag.Tag {
	out := make(chan tag.Tag, 100)
	go func() {
		defer close(out)
		for tag := range source.database.ListImageTags(id) {
			out <- tag
		}
	}()
	return out
}

func (source *Source) ListTags(q string, limit int) <-chan tag.Tag {
	return source.database.ListTags(q, limit)
}

func (source *Source) ListTagsOfTag(id tag.Id, limit int) <-chan tag.Tag {
	return source.database.ListTagsOfTag(id, limit)
}

func (source *Source) AddTagIds(id tag.Id, ids Ids) time.Time {
	return source.database.AddTagIds(id, ids)
}

func (source *Source) RemoveTagIds(id tag.Id, ids Ids) time.Time {
	return source.database.RemoveTagIds(id, ids)
}

func (source *Source) InvertTagIds(id tag.Id, ids Ids) time.Time {
	return source.database.InvertTagIds(id, ids)
}

func (source *Source) IdChanToIds(ch <-chan ImageId) Ids {
	ids := NewIds()
	for id := range ch {
		ids.AddInt(int(id))
	}
	return ids
}

func (source *Source) GetTagId(name string) (tag.Id, bool) {
	return source.database.GetTagId(name)
}

func (source *Source) GetTagFilesCount(id tag.Id) (int, bool) {
	return source.database.GetTagFilesCount(id)
}

func (source *Source) GetOrCreateTagFromName(name string) (tag.Tag, error) {
	t, ok := source.database.GetTagByName(name)
	if !ok {
		source.AddTag(name)
		t, ok = source.database.GetTagByName(name)
		if !ok {
			return tag.Tag{}, ErrNotFound
		}
	}
	return t, nil
}

func (source *Source) GetTagImageIds(id tag.Id) Ids {
	return source.database.GetTagImageIds(id)
}

// WriteDummyFiles creates dummy database entries for testing purposes
func (source *Source) WriteDummyFiles(count int, seed int64) error {
	rng := rand.New(rand.NewSource(seed))

	for i := 0; i < count; i++ {
		// Generate dummy file path
		path := fmt.Sprintf("/dummy/images/image_%09d.jpg", i)

		// Generate random dummy info
		info := Info{
			Width:       rng.Intn(4000) + 800,                                         // 800-4800px
			Height:      rng.Intn(3000) + 600,                                         // 600-3600px
			DateTime:    time.Now().Add(-time.Duration(rng.Intn(365*24)) * time.Hour), // Random time in last year
			Color:       uint32(rng.Intn(0xFFFFFF)),                                   // Random RGB color as uint32
			Orientation: Orientation(-1),
			LatLng:      NaNLatLng(), // Use NaN LatLng for dummy data
		}

		if err := source.database.Write(path, Info{}, AppendPath); err != nil {
			return fmt.Errorf("failed to write dummy path %d: %v", i, err)
		}

		// Write to database
		if err := source.database.Write(path, info, UpdateMeta); err != nil {
			return fmt.Errorf("failed to write dummy meta %d: %v", i, err)
		}

		// Progress reporting every 10k entries
		if (i+1)%10000 == 0 || i+1 == count {
			log.Printf("created %d/%d dummy entries", i+1, count)
		}
	}

	// Wait for any pending database operations to drain
	for len(source.database.pending) > 0 {
		time.Sleep(10 * time.Millisecond)
	}

	source.database.WaitForCommit()
	source.database.Close()

	return nil
}

// WaitForDatabaseCommit waits for all pending database writes to complete
func (source *Source) WaitForDatabaseCommit() <-chan any {
	return source.database.CommitBarrier()
}
