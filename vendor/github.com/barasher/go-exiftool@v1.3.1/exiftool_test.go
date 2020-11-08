package exiftool

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewExiftoolEmpty(t *testing.T) {
	e, err := NewExiftool()
	assert.Nil(t, err)

	defer e.Close()
}

func TestNewExifToolOptOk(t *testing.T) {
	var exec1, exec2 bool

	f1 := func(*Exiftool) error {
		exec1 = true
		return nil
	}

	f2 := func(*Exiftool) error {
		exec2 = true
		return nil
	}

	e, err := NewExiftool(f1, f2)
	assert.Nil(t, err)

	defer e.Close()

	assert.True(t, exec1)
	assert.True(t, exec2)
}

func TestNewExifToolOptKo(t *testing.T) {
	f := func(*Exiftool) error {
		return fmt.Errorf("err")
	}
	_, err := NewExiftool(f)
	assert.NotNil(t, err)
}
func TestSingleExtract(t *testing.T) {
	var tcs = []struct {
		tcID    string
		inFiles []string
		expOk   []bool
	}{
		{"single", []string{"./testdata/20190404_131804.jpg"}, []bool{true}},
		{"multiple", []string{"./testdata/20190404_131804.jpg", "./testdata/20190404_131804.jpg"}, []bool{true, true}},
		{"nonExisting", []string{"./testdata/nonExisting"}, []bool{false}},
		{"empty", []string{"./testdata/empty.jpg"}, []bool{true}},
	}

	for _, tc := range tcs {
		tc := tc // Pin variable
		t.Run(tc.tcID, func(t *testing.T) {
			e, err := NewExiftool()
			assert.Nilf(t, err, "error not nil: %v", err)
			defer e.Close()
			fms := e.ExtractMetadata(tc.inFiles...)
			assert.Equal(t, len(tc.expOk), len(fms))
			for i, fm := range fms {
				t.Log(fm)
				assert.Equalf(t, tc.expOk[i], fm.Err == nil, "#%v different", i)
			}
		})
	}
}

func TestMultiExtract(t *testing.T) {
	e, err := NewExiftool()

	assert.Nilf(t, err, "error not nil: %v", err)

	defer e.Close()

	f := e.ExtractMetadata("./testdata/20190404_131804.jpg", "./testdata/20190404_131804.jpg")

	assert.Equal(t, 2, len(f))
	assert.Nil(t, f[0].Err)
	assert.Nil(t, f[1].Err)

	f = e.ExtractMetadata("./testdata/nonExisting.bla")

	assert.Equal(t, 1, len(f))
	assert.NotNil(t, f[0].Err)

	f = e.ExtractMetadata("./testdata/20190404_131804.jpg")

	assert.Equal(t, 1, len(f))
	assert.Nil(t, f[0].Err)
}

func TestSplitReadyToken(t *testing.T) {
	rt := string(readyToken)

	var tcs = []struct {
		tcID    string
		in      string
		expOk   bool
		expVals []string
	}{
		{"mono", "a" + rt, true, []string{"a"}},
		{"multi", "a" + rt + "b" + rt, true, []string{"a", "b"}},
		{"empty", "", true, []string{}},
		{"monoNoFinalToken", "a", false, []string{}},
		{"multiNoFinalToken", "a" + rt + "b", false, []string{}},
		{"emptyWithToken", rt, true, []string{""}},
	}

	for _, tc := range tcs {
		tc := tc // Pin variable
		t.Run(tc.tcID, func(t *testing.T) {
			sc := bufio.NewScanner(strings.NewReader(tc.in))
			sc.Split(splitReadyToken)
			vals := []string{}
			for sc.Scan() {
				vals = append(vals, sc.Text())
			}
			assert.Equal(t, tc.expOk, sc.Err() == nil)
			if tc.expOk {
				assert.Equal(t, tc.expVals, vals)
			}
		})
	}
}

func TestCloseNominal(t *testing.T) {
	var rClosed, wClosed bool

	r := readWriteCloserMock{closed: &rClosed}
	w := readWriteCloserMock{closed: &wClosed}
	e := Exiftool{stdin: r, stdMergedOut: w}

	assert.Nil(t, e.Close())
	assert.True(t, rClosed)
	assert.True(t, wClosed)
}

func TestCloseErrorOnStdin(t *testing.T) {
	var rClosed, wClosed bool

	r := readWriteCloserMock{closed: &rClosed, closeErr: fmt.Errorf("error")}
	w := readWriteCloserMock{closed: &wClosed}
	e := Exiftool{stdin: r, stdMergedOut: w}

	assert.NotNil(t, e.Close())
	assert.True(t, rClosed)
	assert.True(t, wClosed)
}

func TestCloseErrorOnStdout(t *testing.T) {
	var rClosed, wClosed bool

	r := readWriteCloserMock{closed: &rClosed}
	w := readWriteCloserMock{closed: &wClosed, closeErr: fmt.Errorf("error")}
	e := Exiftool{stdin: r, stdMergedOut: w}

	assert.NotNil(t, e.Close())
	assert.True(t, rClosed)
	assert.True(t, wClosed)
}

type readWriteCloserMock struct {
	writeInt int
	writeErr error
	readInt  int
	readErr  error
	closeErr error
	closed   *bool
}

func (e readWriteCloserMock) Write(p []byte) (n int, err error) {
	return e.writeInt, e.writeErr
}

func (e readWriteCloserMock) Read(p []byte) (n int, err error) {
	return e.readInt, e.readErr
}

func (e readWriteCloserMock) Close() error {
	*(e.closed) = true
	return e.closeErr
}

func TestBuffer(t *testing.T) {
	e, err := NewExiftool()
	assert.Nil(t, err)
	defer e.Close()
	assert.Equal(t, false, e.bufferSet)

	buf := make([]byte, 128)
	assert.Nil(t, Buffer(buf, 64)(e))
	assert.Equal(t, true, e.bufferSet)
	assert.Equal(t, buf, e.buffer)
	assert.Equal(t, 64, e.bufferMaxSize)
}

func TestNewExifTool_WithBuffer(t *testing.T) {
	buf := make([]byte, 128*1000)
	e, err := NewExiftool(Buffer(buf, 64*1000))
	assert.Nil(t, err)
	defer e.Close()

	metas := e.ExtractMetadata("./testdata/20190404_131804.jpg")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)
}

func TestCharset(t *testing.T) {
	e, err := NewExiftool()
	assert.Nil(t, err)
	defer e.Close()
	lengthBefore := len(e.extraInitArgs)

	assert.Nil(t, Charset("charsetValue")(e))
	assert.Equal(t, lengthBefore+2, len(e.extraInitArgs))
	assert.Equal(t, "-charset", e.extraInitArgs[lengthBefore])
	assert.Equal(t, "charsetValue", e.extraInitArgs[lengthBefore+1])
}

func TestNewExifTool_WithCharset(t *testing.T) {
	e, err := NewExiftool(Charset("filename=utf8"))
	assert.Nil(t, err)
	defer e.Close()

	metas := e.ExtractMetadata("./testdata/20190404_131804.jpg")
	assert.Equal(t, 1, len(metas))
	assert.Nil(t, metas[0].Err)
}