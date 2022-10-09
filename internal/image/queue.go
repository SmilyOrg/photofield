package image

func (source *Source) LoadInfo(path string) (Info, error) {
	var info Info
	var err error
	err = source.decoder.DecodeInfo(path, &info)
	if err != nil {
		return info, err
	}

	color, err := source.LoadImageColor(path)
	if err != nil {
		return info, err
	}
	info.SetColorRGBA(color)

	return info, nil
}

func (source *Source) LoadInfoMeta(path string) (Info, error) {
	var info Info
	err := source.decoder.DecodeInfo(path, &info)
	if err != nil {
		return info, err
	}
	return info, nil
}

func (source *Source) LoadInfoColor(path string) (Info, error) {
	var info Info
	color, err := source.LoadImageColor(path)
	if err != nil {
		return info, err
	}
	info.SetColorRGBA(color)
	return info, nil
}
