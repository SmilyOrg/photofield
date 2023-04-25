package image

import (
	"fmt"
	"math"

	
)

func (source *Source) indexMetadata(in <-chan interface{}) {
	for elem := range in {
		m := elem.(MissingInfo)
		id := m.Id
		path := m.Path

		var info Info
		err := source.decoder.DecodeInfo(path, &info)
		if err != nil {
			fmt.Println("Unable to load image info meta", err, path)
			continue
		}

		if !math.IsNaN(info.Latitude) {
			loc, err := source.rg.ReverseGeocode([]float64{info.Longitude, info.Latitude})
			if err != nil {
				// Handle error
				fmt.Println("RGEO ERR", err)
			}
			fmt.Println("RGeo", loc.City, loc.Province, loc.Country)
			if loc.City == "" && loc.Country == "" {
				info.Location = ""
			} else if loc.City != "" {
				info.Location = fmt.Sprintf("%s, %s, %s", loc.City, loc.Province, loc.Country)
			} else {
				info.Location = fmt.Sprintf("%s, %s", loc.Province, loc.Country)
			}
		}
		
		source.database.Write(path, info, UpdateMeta)
		source.imageInfoCache.Delete(id)
	}
}
