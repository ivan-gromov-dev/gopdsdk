// Package jsoncodec exercises bounded reflection-free JSON on Playdate.
package jsoncodec

import (
	"io"
	"strconv"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
	pdjson "github.com/ivan-gromov-dev/gopdsdk/playdate/json"
)

type game struct{ summary string }

// New creates the P11.3 JSON acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	files, ok := context.(playdate.FileSystem)
	if !ok {
		return playdate.ErrFileUnavailable
	}
	file, err := files.OpenFile("config.json", playdate.FileReadPackage)
	if err != nil {
		return err
	}
	value, decodeErr := pdjson.Decode(file, pdjson.Limits{MaxBytes: 1024, MaxDepth: 8, MaxNodes: 64, MaxStringBytes: 128})
	closeErr := file.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return g.accept(value)
}

func (g *game) accept(value pdjson.Value) error {
	title, titleOK := value.Lookup("title")
	level, levelOK := value.Lookup("level")
	flags, flagsOK := value.Lookup("flags")
	if !titleOK || title.Type != pdjson.String || !levelOK || level.Type != pdjson.Number || !flagsOK || flags.Type != pdjson.Array {
		return pdjson.SyntaxError{Message: "unexpected acceptance schema"}
	}
	value.Members = append(value.Members, pdjson.Member{Name: "verified", Value: pdjson.Value{Type: pdjson.Bool, Boolean: true}})
	var storage [512]byte
	writer := fixedWriter{buffer: storage[:]}
	if err := pdjson.Encode(&writer, value, pdjson.EncodeOptions{}); err != nil {
		return err
	}
	g.summary = "JSON: " + title.Text + " L" + level.Text + " flags " + strconv.Itoa(len(flags.Elements)) + " bytes " + strconv.Itoa(writer.used)
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText(g.summary, 10, 110)
	return true, nil
}

type fixedWriter struct {
	buffer []byte
	used   int
}

func (w *fixedWriter) Write(value []byte) (int, error) {
	if len(value) > len(w.buffer)-w.used {
		return 0, io.ErrShortBuffer
	}
	copy(w.buffer[w.used:], value)
	w.used += len(value)
	return len(value), nil
}
