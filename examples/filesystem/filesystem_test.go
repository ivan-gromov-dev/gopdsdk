package filesystem

import (
	"bytes"
	"io"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

type memoryFile struct {
	*bytes.Reader
	data *[]byte
}

func (file *memoryFile) Write(value []byte) (int, error) {
	*file.data = append((*file.data)[:0], value...)
	return len(value), nil
}
func (*memoryFile) Flush() error { return nil }
func (*memoryFile) Close() error { return nil }

type memoryFiles struct{ data []byte }

func (files *memoryFiles) OpenFile(path string, options playdate.FileOptions) (playdate.File, error) {
	if options == playdate.FileWrite {
		return &memoryFile{Reader: bytes.NewReader(nil), data: &files.data}, nil
	}
	return &memoryFile{Reader: bytes.NewReader(files.data), data: &files.data}, nil
}
func (*memoryFiles) Stat(string) (playdate.FileInfo, error) {
	return playdate.FileInfo{Size: uint32(len(payload))}, nil
}
func (*memoryFiles) List(string, bool) ([]string, error) { return []string{"state.bin"}, nil }
func (*memoryFiles) Mkdir(string) error                  { return nil }
func (*memoryFiles) Remove(string, bool) error           { return nil }
func (*memoryFiles) Rename(string, string) error         { return nil }

func TestExercise(t *testing.T) {
	files := &memoryFiles{}
	if err := exercise(files); err != nil {
		t.Fatal(err)
	}
	if got := string(files.data); got != payload {
		t.Fatalf("payload = %q", got)
	}
}

var _ io.ReadWriteSeeker = (*memoryFile)(nil)
