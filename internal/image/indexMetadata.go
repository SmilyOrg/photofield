package image

import (
	"fmt"
)

func (source *Source) indexMetadata(in <-chan interface{}) {
	for elem := range in {
		m := elem.(MissingInfo)
		id := m.Id
		path := m.Path
		info, err := source.LoadInfoMeta(path)
		if err != nil {
			fmt.Println("Unable to load image info meta", err, path)
			continue
		}
		source.database.Write(path, info, UpdateMeta)
		source.imageInfoCache.Delete(id)
	}
}
