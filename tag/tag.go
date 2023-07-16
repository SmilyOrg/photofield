package tag

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

type Id uint32

type Tag struct {
	Id       Id
	Name     string
	Revision int
}

type ExternalTag struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Revision int    `json:"revision"`
}

func randomId() (string, error) {
	return gonanoid.Generate("6789BCDFGHJKLMNPQRTWbcdfghjkmnpqrtwz", 10)
}

func (t Tag) MarshalJSON() ([]byte, error) {
	return json.Marshal(ExternalTag{
		Id:       t.NameRev(),
		Name:     t.Name,
		Revision: t.Revision,
	})
}

func (t *Tag) UnmarshalJSON(data []byte) error {
	var externalTag ExternalTag
	err := json.Unmarshal(data, &externalTag)
	if err != nil {
		return err
	}
	tag, err := FromNameRev(externalTag.Id)
	if err != nil {
		return err
	}
	*t = tag
	return nil
}

func FromNameRev(id string) (Tag, error) {
	var t Tag
	revIndex := strings.LastIndexByte(id, ':')
	if revIndex < 0 {
		return t, errors.New("invalid tag id")
	}
	t.Name = id[:revIndex]
	revStr := id[revIndex+1:]
	if revStr[:1] != "r" {
		return t, errors.New("expected r in tag revision")
	}
	revStr = revStr[1:]
	rev, err := strconv.Atoi(revStr)
	if err != nil {
		return t, err
	}
	t.Revision = rev
	return t, nil
}

func (t Tag) NameRev() string {
	return fmt.Sprintf("%s:r%d", t.Name, t.Revision)
}
