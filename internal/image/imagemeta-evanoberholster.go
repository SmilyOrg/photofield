package image

import (
	"fmt"
	"os"
	"time"

	"github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/meta"
)

type ImageMetaLoader struct{}

func NewImageMetaLoader() *ImageMetaLoader {
	return &ImageMetaLoader{}
}

func (decoder *ImageMetaLoader) DecodeInfo(path string, info *Info) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	println("start", path)
	err = decoder.DecodeInfoReader(f, info)
	println("stop", path)
	return err
}

func (decoder *ImageMetaLoader) DecodeInfoReader(r meta.Reader, info *Info) error {
	println("start")
	m, err := imagemeta.Parse(r)
	println("stop")
	if err != nil {
		return err
	}

	dimensions := m.Dimensions()
	info.Width = int(dimensions.Width)
	info.Height = int(dimensions.Height)

	e, _ := m.Exif()
	if e != nil {
		info.DateTime, _ = e.DateTime(time.UTC)
		info.Orientation = (Orientation)(e.Orientation())
	}

	if info.Orientation.SwapsDimensions() {
		info.Width, info.Height = info.Height, info.Width
	}

	return nil
}

func (decoder *ImageMetaLoader) DecodeBytes(path string, tagName string) ([]byte, error) {
	// f, err := os.Open(path)
	// if err != nil {
	// 	return nil, err
	// }
	// defer f.Close()

	// m, err := imagemeta.Parse(f)
	// if err != nil {
	// 	return nil, err
	// }

	// return io.ReadAll(m.PreviewImage())
	return nil, fmt.Errorf("not implemented")
}

func (decoder *ImageMetaLoader) Close() {}
