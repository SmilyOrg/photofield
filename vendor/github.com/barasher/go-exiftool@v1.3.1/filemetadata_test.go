package exiftool

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getExpectedFileMetadata() FileMetadata {
	return FileMetadata{
		Fields: map[string]interface{}{
			"stringMono":  "stringMonoValue",
			"float":       float64(3.14),
			"integer":     int64(42),
			"unsupported": int32(22),
			"strFloat":    "6.28",
			"strInt":      "84",
			"int32":       int32(32),
			"float32":     float32(32.32),
			"array":       []interface{}{"str", float64(64.64), float32(32.32), int64(64), true},
		},
	}
}
func TestGetInt(t *testing.T) {
	fm := getExpectedFileMetadata()

	tcs := []struct {
		inKey      string
		expIsError bool
		expError   error
		expVal     int64
	}{
		{"stringMono", true, nil, int64(0)},
		{"float", false, nil, int64(3)},
		{"integer", false, nil, int64(42)},
		{"unexisting", true, ErrKeyNotFound, int64(0)},
		{"strInt", false, nil, int64(84)},
		{"int32", false, nil, int64(32)},
	}
	for _, tc := range tcs {
		tc := tc // Pin variable
		t.Run(tc.inKey, func(t *testing.T) {
			v, err := fm.GetInt(tc.inKey)
			if tc.expIsError {
				assert.NotNil(t, err)
				if tc.expError != nil {
					assert.True(t, errors.Is(err, tc.expError))
				}
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expVal, v)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	fm := getExpectedFileMetadata()

	tcs := []struct {
		inKey      string
		expIsError bool
		expError   error
		expVal     float64
	}{
		{"stringMono", true, nil, float64(0.0)},
		{"float", false, nil, float64(3.14)},
		{"integer", false, nil, float64(42)},
		{"unexisting", true, ErrKeyNotFound, float64(0)},
		{"strFloat", false, nil, float64(6.28)},
		{"float32", false, nil, float64(32.32)},
	}
	for _, tc := range tcs {
		tc := tc // Pin variable
		t.Run(tc.inKey, func(t *testing.T) {
			v, err := fm.GetFloat(tc.inKey)
			if tc.expIsError {
				assert.NotNil(t, err)
				if tc.expError != nil {
					assert.True(t, errors.Is(err, tc.expError))
				}
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expVal, v)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	fm := getExpectedFileMetadata()

	tcs := []struct {
		inKey      string
		expIsError bool
		expError   error
		expVal     string
	}{
		{"stringMono", false, nil, "stringMonoValue"},
		{"float", false, nil, "3.14"},
		{"integer", false, nil, "42"},
		{"unsupported", false, nil, "22"},
		{"unexisting", true, ErrKeyNotFound, ""},
	}
	for _, tc := range tcs {
		tc := tc // Pin variable
		t.Run(tc.inKey, func(t *testing.T) {
			v, err := fm.GetString(tc.inKey)
			if tc.expIsError {
				assert.NotNil(t, err)
				if tc.expError != nil {
					assert.True(t, errors.Is(err, tc.expError))
				}
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expVal, v)
			}
		})
	}
}

func TestGetStrings(t *testing.T) {
	fm := getExpectedFileMetadata()

	tcs := []struct {
		inKey      string
		expIsError bool
		expError   error
		expVal     []string
	}{
		{"stringMono", false, nil, []string{"stringMonoValue"}},
		{"float", false, nil, []string{"3.14"}},
		{"integer", false, nil, []string{"42"}},
		{"unsupported", false, nil, []string{"22"}},
		{"unexisting", true, ErrKeyNotFound, []string{}},
		{"array", false, nil, []string{"str", "64.64", "32.32", "64", "true"}},
	}
	for _, tc := range tcs {
		tc := tc // Pin variable
		t.Run(tc.inKey, func(t *testing.T) {
			v, err := fm.GetStrings(tc.inKey)
			if tc.expIsError {
				assert.NotNil(t, err)
				if tc.expError != nil {
					assert.True(t, errors.Is(err, tc.expError))
				}
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expVal, v)
			}
		})
	}
}
