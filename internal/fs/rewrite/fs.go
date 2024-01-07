package rewrite

import (
	"io"
	"io/fs"
	"regexp"
	"strings"
	"testing/fstest"
	"time"
)

type FileSystem struct {
	FS        fs.FS
	rewritten fstest.MapFS
}

func FS(fs fs.FS, exts []string, regex string, replace string) (*FileSystem, error) {
	rfs := &FileSystem{
		FS:        fs,
		rewritten: make(fstest.MapFS),
	}
	var err error
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	err = rfs.replace(exts, re, replace)
	if err != nil {
		return nil, err
	}
	return rfs, nil
}

func (rfs *FileSystem) Open(name string) (fs.File, error) {
	if f, err := rfs.rewritten.Open(name); err == nil {
		return f, nil
	}
	return rfs.FS.Open(name)
}

func (rfs *FileSystem) replace(exts []string, re *regexp.Regexp, repl string) error {
	fs.WalkDir(rfs.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".html") &&
			!strings.HasSuffix(path, ".css") &&
			!strings.HasSuffix(path, ".js") {
			return nil
		}

		f, err := rfs.FS.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}

		str := string(data)

		r := re.ReplaceAllString(str, repl)
		contains := str != r
		if contains {
			rfs.rewritten[path] = &fstest.MapFile{
				Data:    []byte(r),
				Mode:    0644,
				ModTime: time.Now(),
			}
		}
		return nil
	})
	return nil
}
