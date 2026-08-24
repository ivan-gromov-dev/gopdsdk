package runtime

import (
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

func TestOwnedFileIOAndLifetime(t *testing.T) {
	var writes []byte
	position := 0
	closed := 0
	file := NewOwnedFile(7, "save.bin", FileDriver{
		Read: func(_ uintptr, buffer []byte) (int, string) {
			copy(buffer, "ok")
			return 2, ""
		},
		Write: func(_ uintptr, buffer []byte) (int, string) {
			writes = append(writes, buffer...)
			return len(buffer), ""
		},
		Flush: func(uintptr) (int, string) { return 3, "" },
		Seek: func(_ uintptr, offset int32, _ int) (int, string) {
			position = int(offset)
			return 0, ""
		},
		Tell:  func(uintptr) (int, string) { return position, "" },
		Close: func(uintptr) (int, string) { closed++; return 0, "" },
	})

	buffer := make([]byte, 2)
	if count, err := file.Read(buffer); err != nil || count != 2 || string(buffer) != "ok" {
		t.Fatalf("Read() = %d, %q, %v", count, buffer, err)
	}
	if count, err := file.Write([]byte("save")); err != nil || count != 4 || string(writes) != "save" {
		t.Fatalf("Write() = %d, %q, %v", count, writes, err)
	}
	if err := file.Flush(); err != nil {
		t.Fatal(err)
	}
	if got, err := file.Seek(12, io.SeekStart); err != nil || got != 12 {
		t.Fatalf("Seek() = %d, %v", got, err)
	}
	if err := file.Close(); err != nil || closed != 1 {
		t.Fatalf("Close() = %v, calls %d", err, closed)
	}
	if err := file.Close(); !errors.Is(err, playdate.ErrFileClosed) || closed != 1 {
		t.Fatalf("second Close() = %v, calls %d", err, closed)
	}
}

func TestOwnedFileErrors(t *testing.T) {
	file := NewOwnedFile(9, "save.bin", FileDriver{
		Read:  func(uintptr, []byte) (int, string) { return 0, "" },
		Write: func(uintptr, []byte) (int, string) { return 1, "" },
		Flush: func(uintptr) (int, string) { return -1, "disk full" },
		Seek:  func(uintptr, int32, int) (int, string) { return -1, "bad seek" },
		Tell:  func(uintptr) (int, string) { return 0, "" },
		Close: func(uintptr) (int, string) { return -1, "close failed" },
	})

	if _, err := file.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("Read() error = %v", err)
	}
	if count, err := file.Write([]byte("ab")); count != 1 || !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("Write() = %d, %v", count, err)
	}
	if err := file.Flush(); !errors.Is(err, playdate.ErrFileIO) || err.Error() != `flush save.bin: Playdate filesystem operation failed: disk full` {
		t.Fatalf("Flush() error = %v", err)
	}
	if _, err := file.Seek(int64(1)<<32, io.SeekStart); !errors.Is(err, playdate.ErrFileOffset) {
		t.Fatalf("Seek(range) error = %v", err)
	}
	if err := file.Close(); !errors.Is(err, playdate.ErrFileIO) {
		t.Fatalf("Close() error = %v", err)
	}
	if err := file.Flush(); !errors.Is(err, playdate.ErrFileClosed) {
		t.Fatalf("Flush after failed close = %v", err)
	}
}

func TestOwnedFileSeekReturnsOfficialCurrentPosition(t *testing.T) {
	var gotOffset int32
	var gotWhence int
	file := NewOwnedFile(11, "save.bin", FileDriver{
		Seek: func(_ uintptr, offset int32, whence int) (int, string) {
			gotOffset = offset
			gotWhence = whence
			return 0, ""
		},
		Tell: func(uintptr) (int, string) { return 37, "" },
	})

	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil || position != 37 || gotOffset != 0 || gotWhence != io.SeekCurrent {
		t.Fatalf("Seek(0, SeekCurrent) = %d, %v; native seek = (%d, %d)", position, err, gotOffset, gotWhence)
	}
}

func TestOwnedFileSeekClassifiesTellFailureAtOperationBoundary(t *testing.T) {
	file := NewOwnedFile(13, "save.bin", FileDriver{
		Seek: func(uintptr, int32, int) (int, string) { return 0, "" },
		Tell: func(uintptr) (int, string) { return -1, "position unavailable" },
	})

	_, err := file.Seek(4, io.SeekStart)
	var failure playdate.FileOperationError
	if !errors.As(err, &failure) || !errors.Is(err, playdate.ErrFileIO) {
		t.Fatalf("Seek() error = %v, want FileOperationError classified as ErrFileIO", err)
	}
	if failure.Operation != "tell" || failure.Path != "save.bin" || failure.Message != "position unavailable" {
		t.Fatalf("Seek() failure = %#v", failure)
	}
}

func TestValidateFileInputs(t *testing.T) {
	for _, valid := range []string{"save.bin", "settings/config", "folder"} {
		if err := ValidateFilePath(valid, false); err != nil {
			t.Errorf("ValidateFilePath(%q) = %v", valid, err)
		}
	}
	for _, invalid := range []string{"", ".", "/save", "../save", "a/../save", `a\save`, "bad\x00path"} {
		if err := ValidateFilePath(invalid, false); !errors.Is(err, playdate.ErrFilePath) {
			t.Errorf("ValidateFilePath(%q) = %v", invalid, err)
		}
	}
	if err := ValidateFilePath("", true); err != nil {
		t.Fatalf("root path = %v", err)
	}
	for _, options := range []playdate.FileOptions{playdate.FileReadPackage, playdate.FileReadData, playdate.FileReadPackage | playdate.FileReadData, playdate.FileWrite, playdate.FileAppend} {
		if err := ValidateFileOptions(options); err != nil {
			t.Errorf("ValidateFileOptions(%d) = %v", options, err)
		}
	}
	if err := ValidateFileOptions(0); !errors.Is(err, playdate.ErrFileMode) {
		t.Fatalf("ValidateFileOptions(0) = %v", err)
	}
}

type filesystemContext struct {
	testContext
	calls []string
}

func (context *filesystemContext) OpenFile(path string, options playdate.FileOptions) (playdate.File, error) {
	context.calls = append(context.calls, "open:"+path)
	return nil, nil
}
func (context *filesystemContext) Stat(path string) (playdate.FileInfo, error) {
	context.calls = append(context.calls, "stat:"+path)
	return playdate.FileInfo{Size: 4}, nil
}
func (context *filesystemContext) List(path string, _ bool) ([]string, error) {
	context.calls = append(context.calls, "list:"+path)
	return []string{"save.bin"}, nil
}
func (context *filesystemContext) Mkdir(path string) error {
	context.calls = append(context.calls, "mkdir:"+path)
	return nil
}
func (context *filesystemContext) Remove(path string, _ bool) error {
	context.calls = append(context.calls, "remove:"+path)
	return nil
}
func (context *filesystemContext) Rename(from, to string) error {
	context.calls = append(context.calls, "rename:"+from+":"+to)
	return nil
}

type filesystemGame struct{}

func (filesystemGame) Init(context playdate.Context) error {
	files := any(context).(playdate.FileSystem)
	_, _ = files.OpenFile("save.bin", playdate.FileReadData)
	_, _ = files.Stat("save.bin")
	_, _ = files.List("", false)
	_ = files.Mkdir("data")
	_ = files.Remove("old", false)
	return files.Rename("temp", "save")
}
func (filesystemGame) Update(playdate.Context) (bool, error) { return false, nil }

func TestNewApplicationForwardsFileSystem(t *testing.T) {
	context := &filesystemContext{}
	application, err := NewApplication(filesystemGame{}, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.Handle(EventInit, 0); err != nil {
		t.Fatal(err)
	}
	want := []string{"open:save.bin", "stat:save.bin", "list:", "mkdir:data", "remove:old", "rename:temp:save"}
	if !reflect.DeepEqual(context.calls, want) {
		t.Fatalf("calls = %v, want %v", context.calls, want)
	}
}
