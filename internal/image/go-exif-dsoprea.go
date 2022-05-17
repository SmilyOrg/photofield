package image

import (
	"fmt"
	"io"

	jpegstructure "github.com/dsoprea/go-jpeg-image-structure"
)

type GoexifDsoprea struct {
	parser *jpegstructure.JpegMediaParser
}

func NewGoexifDsoprea() *GoexifDsoprea {
	return &GoexifDsoprea{
		parser: jpegstructure.NewJpegMediaParser(),
	}
}

func (decoder *GoexifDsoprea) DecodeInfo(path string, info *Info) error {
	ec, err := decoder.parser.ParseFile(path)
	if err != nil {
		return err
	}

	sl := ec.(*jpegstructure.SegmentList)

	sl.FindExif()

}

func (decoder *GoexifDsoprea) DecodeInfoReader(r io.ReadSeeker, info *Info) error {
	return fmt.Errorf("not implemented")
}

func (decoder *GoexifDsoprea) DecodeBytes(path string, tagName string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (decoder *GoexifDsoprea) Close() {}
