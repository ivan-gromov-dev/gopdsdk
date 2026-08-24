package persistence

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type memoryFiles struct{ values map[string][]byte }

func (files *memoryFiles) OpenFile(path string, options playdate.FileOptions) (playdate.File, error) {
	switch options {
	case playdate.FileWrite:
		return &memoryFile{files: files, path: path}, nil
	case playdate.FileReadData:
		value, ok := files.values[path]
		if !ok {
			return nil, playdate.ErrFileIO
		}
		return &memoryFile{Reader: bytes.NewReader(value)}, nil
	default:
		return nil, playdate.ErrFileMode
	}
}

func (files *memoryFiles) Stat(path string) (playdate.FileInfo, error) {
	value, ok := files.values[path]
	if !ok {
		return playdate.FileInfo{}, playdate.ErrFileIO
	}
	return playdate.FileInfo{Size: uint32(len(value))}, nil
}
func (*memoryFiles) List(string, bool) ([]string, error) { return nil, nil }
func (*memoryFiles) Mkdir(string) error                  { return nil }
func (*memoryFiles) Remove(string, bool) error           { return nil }

func (files *memoryFiles) Rename(from, to string) error {
	value, ok := files.values[from]
	if !ok {
		return errors.New("temporary file missing")
	}
	files.values[to] = append([]byte(nil), value...)
	delete(files.values, from)
	return nil
}

type memoryFile struct {
	*bytes.Reader
	files  *memoryFiles
	path   string
	buffer bytes.Buffer
}

func (file *memoryFile) Read(value []byte) (int, error) {
	if file.Reader == nil {
		return 0, playdate.ErrFileMode
	}
	return file.Reader.Read(value)
}

func (file *memoryFile) Write(value []byte) (int, error) { return file.buffer.Write(value) }

func (file *memoryFile) Seek(offset int64, whence int) (int64, error) {
	if file.Reader == nil {
		return 0, playdate.ErrFileMode
	}
	return file.Reader.Seek(offset, whence)
}

func (*memoryFile) Flush() error { return nil }

func (file *memoryFile) Close() error {
	if file.files != nil {
		file.files.values[file.path] = append([]byte(nil), file.buffer.Bytes()...)
	}
	return nil
}

func TestExercise(t *testing.T) {
	files := &memoryFiles{values: make(map[string][]byte)}
	if err := exercise(files); err != nil {
		t.Fatal(err)
	}
	if _, exists := files.values[path+".tmp"]; exists {
		t.Fatal("temporary value remains")
	}
}
