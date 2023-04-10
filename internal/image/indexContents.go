package image

import (
	"bytes"
	"context"
	"fmt"
	"image"
	goio "io"
	"log"
	"photofield/internal/clip"
	"photofield/io"
	"time"
)

func (source *Source) indexContents(in <-chan interface{}) {
	ctx := context.TODO()
	for elem := range in {

		for source.metadataQueue.Length() > 0 {
			time.Sleep(1 * time.Second)
		}

		m := elem.(MissingInfo)
		id := io.ImageId(m.Id)
		path := m.Path

		done := false
		for _, src := range source.thumbnailSources {
			src.Reader(ctx, id, path, func(rs goio.ReadSeeker, err error) {
				if err != nil {
					return
				}

				// log.Printf("index contents source %s path %s\n", src.(io.Source).Name(), path)
				source.indexContentsReader(ctx, m, src, nil, rs)
				done = true
			})
			if done {
				break
			}
		}

		// Generate thumbnail if none loaded
		if !done {
			// log.Printf("index contents generate %s\n", path)
			img, rs, err := source.indexContentsGenerate(ctx, id, path)
			if err != nil {
				log.Println("Unable to generate image thumbnail", err)
				continue
			}
			source.indexContentsReader(ctx, m, nil, img, rs)
		}
	}
}

func (source *Source) indexContentsReader(ctx context.Context, m MissingInfo, src io.ReadDecoder, img image.Image, rs goio.ReadSeeker) {
	var err error
	if m.Color {
		// Decode image if needed
		if img == nil && rs != nil {
			img, err = source.indexContentsDecode(ctx, src, rs)
			if err != nil {
				log.Println("Unable to decode image thumbnail", err)
			}
		}

		// Extract colors
		if img != nil {
			color, err := extractProminentColor(img)
			if err != nil {
				log.Println("Unable to extract image color", err, m.Path)
			} else {
				info := Info{}
				info.SetColorRGBA(color)
				source.database.Write(m.Path, info, UpdateColor)
				source.imageInfoCache.Delete(m.Id)
			}
		}
	}

	// Extract AI embedding
	if m.Embedding && rs != nil {
		embedding, err := source.Clip.EmbedImageReader(rs)
		if err != clip.ErrNotAvailable {
			if err != nil {
				fmt.Println("Unable to get image embedding", err, m.Path)
			} else {
				source.database.WriteAI(m.Id, embedding)
			}
		}
	}
}

func (source *Source) indexContentsGenerate(ctx context.Context, id io.ImageId, path string) (image.Image, *bytes.Reader, error) {
	errs := make([]error, 0)
	for _, gen := range source.thumbnailGenerators {
		// Generate thumbnail
		r := gen.Get(ctx, id, path)
		if r.Image == nil || r.Error != nil {
			errs = append(errs, r.Error)
			continue
		}

		// Save thumbnail
		var b bytes.Buffer
		ok := source.thumbnailSink.SetWithBuffer(ctx, id, path, &b, r)
		if !ok {
			return r.Image, nil, fmt.Errorf("unable to save %s", path)
		}

		// Return encoded bytes
		rd := bytes.NewReader(b.Bytes())
		return r.Image, rd, nil
	}

	e := ""
	for _, err := range errs {
		e += err.Error() + " "
	}
	return nil, nil, fmt.Errorf("all generators failed: %s: %s", e, path)
}

func (source *Source) indexContentsDecode(ctx context.Context, d io.Decoder, rs goio.ReadSeeker) (image.Image, error) {
	if d == nil {
		return nil, fmt.Errorf("unable to decode, missing decoder")
	}
	r := d.Decode(ctx, rs)
	if r.Error != nil {
		return nil, fmt.Errorf("unable to decode %w", r.Error)
	}
	_, err := rs.Seek(0, goio.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("unable to seek to start %w", err)
	}
	return r.Image, nil
}
