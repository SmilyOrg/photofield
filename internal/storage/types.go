package photofield

import (
	. "photofield/internal"
)

type ImageInfoSource interface {
	GetImageInfo(path string) (*ImageInfo, error)
}
