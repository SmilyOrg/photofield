package tag

import "fmt"

func NewSelection(collectionId string) (Tag, error) {
	var t Tag

	rand, err := randomId()
	if err != nil {
		return t, err
	}

	t.Name = fmt.Sprintf("sys:select:col:%s:%s", collectionId, rand)
	return t, nil
}
