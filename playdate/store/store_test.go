package store

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestFirstSaveAndReplacement(t *testing.T) {
	files := newMemoryFileSystem()
	value := newTestStore(t, files, Config{Path: "save.bin", Version: 1, MaximumSize: 32})
	for _, payload := range [][]byte{[]byte("first"), []byte("replacement")} {
		if err := value.Save(payload); err != nil {
			t.Fatal(err)
		}
		actual, err := value.Load()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actual, payload) {
			t.Fatalf("Load() = %q, want %q", actual, payload)
		}
	}
	if _, exists := files.data["save.bin.tmp"]; exists {
		t.Fatal("temporary file remains after replacement")
	}
}

func TestLoadMigratesAndPersists(t *testing.T) {
	files := newMemoryFileSystem()
	old := newTestStore(t, files, Config{Path: "save.bin", Version: 1, MaximumSize: 32})
	if err := old.Save([]byte("v1")); err != nil {
		t.Fatal(err)
	}
	value := newTestStore(t, files, Config{
		Path: "save.bin", Version: 3, MaximumSize: 32,
		Migrations: []VersionMigration{
			{From: 1, Migrate: func(payload []byte) ([]byte, error) { return append(payload, '-', '2'), nil }},
			{From: 2, Migrate: func(payload []byte) ([]byte, error) { return append(payload, '-', '3'), nil }},
		},
	})
	actual, err := value.Load()
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != "v1-2-3" {
		t.Fatalf("Load() = %q", actual)
	}
	version, persisted, err := decode(files.data["save.bin"], 32)
	if err != nil || version != 3 || string(persisted) != "v1-2-3" {
		t.Fatalf("persisted version=%d payload=%q err=%v", version, persisted, err)
	}
}

func TestUnsupportedFutureVersion(t *testing.T) {
	files := newMemoryFileSystem()
	files.data["save.bin"] = encode(3, []byte("future"))
	value := newTestStore(t, files, Config{Path: "save.bin", Version: 2, MaximumSize: 32})
	_, err := value.Load()
	if !errors.Is(err, ErrFutureVersion) {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestCorruptValues(t *testing.T) {
	valid := encode(1, []byte("payload"))
	cases := map[string][]byte{
		"truncated": valid[:len(valid)-1],
		"bad magic": append([]byte("NOPE"), valid[4:]...),
		"checksum":  append(append([]byte(nil), valid[:len(valid)-1]...), 'x'),
		"oversize":  encode(1, bytes.Repeat([]byte{'x'}, 33)),
	}
	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			files := newMemoryFileSystem()
			files.data["save.bin"] = encoded
			value := newTestStore(t, files, Config{Path: "save.bin", Version: 1, MaximumSize: 32})
			_, err := value.Load()
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestInterruptedReplacementPreservesLastValue(t *testing.T) {
	files := newMemoryFileSystem()
	value := newTestStore(t, files, Config{Path: "save.bin", Version: 1, MaximumSize: 32})
	if err := value.Save([]byte("last valid")); err != nil {
		t.Fatal(err)
	}
	files.refuseOverwrite = true
	files.failPublishOnce = true
	if err := value.Save([]byte("not published")); err == nil {
		t.Fatal("Save() unexpectedly succeeded")
	}
	actual, err := value.Load()
	if err != nil || string(actual) != "last valid" {
		t.Fatalf("Load() = %q, %v", actual, err)
	}
}

func TestReplacementFallsBackWhenRenameDoesNotOverwrite(t *testing.T) {
	files := newMemoryFileSystem()
	value := newTestStore(t, files, Config{Path: "save.bin", Version: 1, MaximumSize: 32})
	if err := value.Save([]byte("first")); err != nil {
		t.Fatal(err)
	}
	files.refuseOverwrite = true
	if err := value.Save([]byte("replacement")); err != nil {
		t.Fatal(err)
	}
	actual, err := value.Load()
	if err != nil || string(actual) != "replacement" {
		t.Fatalf("Load() = %q, %v", actual, err)
	}
}

func TestLoadRecoversInterruptedBackupSwap(t *testing.T) {
	files := newMemoryFileSystem()
	files.data["save.bin.bak"] = encode(1, []byte("last valid"))
	files.data["save.bin.tmp"] = encode(1, []byte("unpublished"))
	value := newTestStore(t, files, Config{Path: "save.bin", Version: 1, MaximumSize: 32})
	actual, err := value.Load()
	if err != nil || string(actual) != "last valid" {
		t.Fatalf("Load() = %q, %v", actual, err)
	}
	if _, exists := files.data["save.bin.bak"]; exists {
		t.Fatal("backup remains after recovery")
	}
}

func TestShortWriteDoesNotReplaceLastValue(t *testing.T) {
	files := newMemoryFileSystem()
	value := newTestStore(t, files, Config{Path: "save.bin", Version: 1, MaximumSize: 32})
	if err := value.Save([]byte("last valid")); err != nil {
		t.Fatal(err)
	}
	files.shortWrite = true
	if err := value.Save([]byte("replacement")); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("Save() error = %v", err)
	}
	files.shortWrite = false
	actual, err := value.Load()
	if err != nil || string(actual) != "last valid" {
		t.Fatalf("Load() = %q, %v", actual, err)
	}
}

func TestFailedMigrationPreservesOriginal(t *testing.T) {
	files := newMemoryFileSystem()
	want := errors.New("invalid legacy field")
	files.data["save.bin"] = encode(1, []byte("original"))
	before := append([]byte(nil), files.data["save.bin"]...)
	value := newTestStore(t, files, Config{
		Path: "save.bin", Version: 2, MaximumSize: 32,
		Migrations: []VersionMigration{{From: 1, Migrate: func([]byte) ([]byte, error) {
			return nil, want
		}}},
	})
	_, err := value.Load()
	if !errors.Is(err, ErrMigration) || !errors.Is(err, want) {
		t.Fatalf("Load() error = %v", err)
	}
	if !bytes.Equal(files.data["save.bin"], before) {
		t.Fatal("failed migration changed the last valid value")
	}
}

func TestSizeBounds(t *testing.T) {
	files := newMemoryFileSystem()
	value := newTestStore(t, files, Config{Path: "save.bin", Version: 1, MaximumSize: 4})
	if err := value.Save([]byte("12345")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Save() error = %v", err)
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	tests := []Config{
		{Path: "", Version: 1, MaximumSize: 1},
		{Path: "../save", Version: 1, MaximumSize: 1},
		{Path: `directory\save`, Version: 1, MaximumSize: 1},
		{Path: "save", Version: 0, MaximumSize: 1},
		{Path: "save", Version: 1, MaximumSize: 0},
		{Path: "save", Version: 1, MaximumSize: 1, Migrations: []VersionMigration{{From: 1, Migrate: func(payload []byte) ([]byte, error) { return payload, nil }}}},
	}
	for _, config := range tests {
		if _, err := New(newMemoryFileSystem(), config); !errors.Is(err, ErrConfig) {
			t.Fatalf("New(%+v) error = %v", config, err)
		}
	}
}

func newTestStore(t *testing.T, files playdate.FileSystem, config Config) *Store {
	t.Helper()
	value, err := New(files, config)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

type memoryFileSystem struct {
	data            map[string][]byte
	shortWrite      bool
	refuseOverwrite bool
	failPublishOnce bool
}

func newMemoryFileSystem() *memoryFileSystem {
	return &memoryFileSystem{data: make(map[string][]byte)}
}

func (files *memoryFileSystem) OpenFile(path string, options playdate.FileOptions) (playdate.File, error) {
	switch options {
	case playdate.FileWrite:
		return &memoryFile{files: files, path: path, writable: true, shortWrite: files.shortWrite}, nil
	case playdate.FileReadData:
		data, ok := files.data[path]
		if !ok {
			return nil, playdate.ErrFileIO
		}
		return &memoryFile{reader: bytes.NewReader(append([]byte(nil), data...))}, nil
	default:
		return nil, playdate.ErrFileMode
	}
}

func (files *memoryFileSystem) Stat(path string) (playdate.FileInfo, error) {
	value, ok := files.data[path]
	if !ok {
		return playdate.FileInfo{}, playdate.ErrFileIO
	}
	return playdate.FileInfo{Size: uint32(len(value))}, nil
}
func (*memoryFileSystem) List(string, bool) ([]string, error) { return nil, nil }
func (*memoryFileSystem) Mkdir(string) error                  { return nil }
func (files *memoryFileSystem) Remove(path string, _ bool) error {
	delete(files.data, path)
	return nil
}

func (files *memoryFileSystem) Rename(from, to string) error {
	if _, exists := files.data[to]; exists && files.refuseOverwrite {
		return playdate.ErrFileIO
	}
	if files.failPublishOnce && from == "save.bin.tmp" && to == "save.bin" {
		if _, backupExists := files.data["save.bin.bak"]; backupExists {
			files.failPublishOnce = false
			return errors.New("interrupted")
		}
	}
	files.data[to] = append([]byte(nil), files.data[from]...)
	delete(files.data, from)
	return nil
}

type memoryFile struct {
	files      *memoryFileSystem
	path       string
	reader     *bytes.Reader
	buffer     bytes.Buffer
	writable   bool
	shortWrite bool
	closed     bool
}

func (file *memoryFile) Read(buffer []byte) (int, error) {
	if file.reader == nil {
		return 0, playdate.ErrFileMode
	}
	return file.reader.Read(buffer)
}

func (file *memoryFile) Write(buffer []byte) (int, error) {
	if !file.writable {
		return 0, playdate.ErrFileMode
	}
	if file.shortWrite {
		file.shortWrite = false
		return 0, nil
	}
	return file.buffer.Write(buffer)
}

func (file *memoryFile) Seek(offset int64, whence int) (int64, error) {
	if file.reader == nil {
		return 0, playdate.ErrFileMode
	}
	return file.reader.Seek(offset, whence)
}

func (*memoryFile) Flush() error { return nil }

func (file *memoryFile) Close() error {
	if file.closed {
		return playdate.ErrFileClosed
	}
	file.closed = true
	if file.writable {
		file.files.data[file.path] = append([]byte(nil), file.buffer.Bytes()...)
	}
	return nil
}
