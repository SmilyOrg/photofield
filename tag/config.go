package tag

type Config struct {
	Enable bool `json:"enable"`

	// Deprecated: Use Enable instead, this is for backwards compatibility
	Enabled bool `json:"enabled"`

	Exif struct {
		Enable bool `json:"enable"`
	} `json:"exif"`
}
