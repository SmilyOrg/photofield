package pipeline

import (
	"io"

	img "photofield/internal/image"
	pio "photofield/internal/io"
	"photofield/internal/tag"
)

// Stage 1: File reference from database
type fileRef struct {
	ID   img.ImageId
	Path string
}

// Stage 2: File with metadata (loaded from DB OR extracted from file)
type fileWithMeta struct {
	fileRef
	Info    img.Info
	Tags    []tag.Tag
	Missing img.Missing // What data is missing for this file
}

// Stage 3: File with thumbnail (loaded from cache OR generated)
type fileWithThumb struct {
	fileRef
	Thumb       io.ReadSeeker
	Orientation pio.Orientation
	Missing     img.Missing // What data is missing for this file
}

// Stage 4: File after contents extraction (color + embedding done)
type fileWithContents struct {
	fileRef
}
