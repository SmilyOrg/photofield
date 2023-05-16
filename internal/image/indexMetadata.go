package image

import (
	"fmt"
)

func (source *Source) indexMetadata(in <-chan interface{}) {
	for elem := range in {
		m := elem.(MissingInfo)
		id := m.Id
		path := m.Path

		var info Info
		tags, err := source.decoder.DecodeInfo(path, &info)
		if err != nil {
			fmt.Println("Unable to load image info meta", err, path)
			continue
		}
		source.database.Write(path, info, UpdateMeta)
		if source.Config.TagConfig.Exif.Enable {
			source.database.WriteTags(id, tags)
		}
		source.imageInfoCache.Delete(id)
	}
}
