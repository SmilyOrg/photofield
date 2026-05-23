package pipeline

import (
	"context"
	"image"
	"image/color"
	"io"
	"log"

	"photofield/internal/ai"
	img "photofield/internal/image"
	pio "photofield/internal/io"

	"github.com/EdlinOrg/prominentcolor"
)

// AIService interface for AI operations
type AIService interface {
	Available() bool
	EmbedImageReader(r io.Reader) (ai.Embedding, error)
}

// ImageDecoder interface for decoding images
type ImageDecoder interface {
	Decode(ctx context.Context, r io.Reader) pio.Result
}

// ContentsProcessor extracts color and AI embeddings from thumbnails.
// Its Process method is designed to be called synchronously from within
// ThumbnailWorkers so that the thumbnail reader is still valid when read.
type contentsProcessor struct {
	db        *img.Database
	aiService AIService
	decoder   ImageDecoder
	force     bool
	progress  *multiProgress
}

// NewContentsProcessor creates a ContentsProcessor ready to process thumbnails.
func newContentsProcessor(db *img.Database, aiService AIService, decoder ImageDecoder, force bool) *contentsProcessor {
	return &contentsProcessor{
		db:        db,
		aiService: aiService,
		decoder:   decoder,
		force:     force,
		progress:  newMultiProgress("contents", 0, "color", "embedding"),
	}
}

// Process extracts color and AI embedding from one thumbnail.
// It must be called while thumb.Thumb is still readable (i.e. inside the
// ThumbnailWorkers process callback).
func (p *contentsProcessor) Process(ctx context.Context, thumb fileWithThumb) {
	needsColor := p.force || thumb.Missing.Color
	needsEmbedding := p.force || thumb.Missing.Embedding

	if needsColor {
		result := p.decoder.Decode(ctx, thumb.Thumb)
		if result.Error != nil {
			log.Printf("index error: decode for color %s: %v\n", thumb.Path, result.Error)
		} else if result.Image != nil {
			color, err := extractProminentColor(result.Image)
			if err != nil {
				log.Printf("index error: color extract %s: %v\n", thumb.Path, err)
			} else {
				info := img.Info{}
				info.SetColorRGBA(color)
				p.db.Write(thumb.Path, info, img.UpdateColor)
				p.progress.IncCounter("color", 1)
			}
		}
		thumb.Thumb.Seek(0, io.SeekStart)
	}

	if needsEmbedding && p.aiService.Available() {
		emb, err := p.aiService.EmbedImageReader(thumb.Thumb)
		if err != nil && err != ai.ErrNotAvailable {
			log.Printf("index error: embedding %s: %v\n", thumb.Path, err)
		} else if err == nil {
			p.db.WriteAI(thumb.ID, emb)
			p.progress.IncCounter("embedding", 1)
		}
	}

	p.progress.Inc(1)
}

// Done logs final contents extraction stats.
func (p *contentsProcessor) Done() {
	p.progress.Done()
}

func extractProminentColor(img image.Image) (color.RGBA, error) {
	centroids, err := prominentcolor.KmeansWithAll(1, img, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, prominentcolor.GetDefaultMasks())
	if err != nil {
		centroids, err = prominentcolor.KmeansWithAll(1, img, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, make([]prominentcolor.ColorBackgroundMask, 0))
		if err != nil {
			return color.RGBA{}, err
		}
	}
	promColor := centroids[0]
	return color.RGBA{
		A: 0xFF,
		R: uint8(promColor.Color.R),
		G: uint8(promColor.Color.G),
		B: uint8(promColor.Color.B),
	}, nil
}
