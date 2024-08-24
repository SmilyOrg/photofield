package tag

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type Id uint32

type Tag struct {
	Id        Id        `json:"id"`
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	FileCount int       `json:"file_count"`
}

func (t Tag) ETag() string {
	b, _ := t.UpdatedAt.MarshalBinary()
	h := crc32.ChecksumIEEE(b)
	return fmt.Sprintf(`%x`, h)
}

type ExternalTag struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at,omitempty"`
	FileCount int    `json:"file_count"`
	ETag      string `json:"etag,omitempty"`
}

func randomId() (string, error) {
	return gonanoid.Generate("6789BCDFGHJKLMNPQRTWbcdfghjkmnpqrtwz", 10)
}

func (t Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(ExternalTag{
		Id:        t.Name,
		Name:      t.Name,
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
		FileCount: t.FileCount,
		ETag:      t.ETag(),
	})
}

func (t *Tag) UnmarshalJSON(data []byte) error {
	var externalTag ExternalTag
	err := json.Unmarshal(data, &externalTag)
	if err != nil {
		return err
	}
	tag := Tag{
		Name: externalTag.Name,
	}
	*t = tag
	return nil
}
