// Package filesystem exercises the P4.1 owned filesystem contract.
package filesystem

import (
	"errors"

	"github.com/ivan-gromov-dev/gopdsdk/playdate"
)

var (
	errReadPayload = errors.New("filesystem payload does not match")
	errStatPayload = errors.New("filesystem metadata does not match")
	errListPayload = errors.New("filesystem listing does not match")
)

const (
	directory = "p4-filesystem"
	temporary = directory + "/state.tmp"
	final     = directory + "/state.bin"
	payload   = "P4.1 filesystem OK"
)

type game struct{ result string }

// New creates the P4.1 filesystem acceptance scene.
func New() playdate.Game { return &game{} }

func (g *game) Init(context playdate.Context) error {
	files, ok := any(context).(playdate.FileSystem)
	if !ok {
		g.result = "FAIL: capability unavailable"
		return nil
	}
	if err := exercise(files); err != nil {
		g.result = "FAIL: " + err.Error()
		return nil
	}
	g.result = payload
	return nil
}

func (g *game) Update(context playdate.Context) (bool, error) {
	context.Clear()
	context.DrawText("P4.1 FILESYSTEM", 120, 72)
	context.DrawText(g.result, 72, 112)
	context.DrawText("WRITE READ STAT LIST RENAME REMOVE", 40, 152)
	return true, nil
}

func exercise(files playdate.FileSystem) error {
	_ = files.Remove(directory, true)
	if err := files.Mkdir(directory); err != nil {
		return err
	}

	output, err := files.OpenFile(temporary, playdate.FileWrite)
	if err != nil {
		return err
	}
	if _, err = output.Write([]byte(payload)); err == nil {
		err = output.Flush()
	}
	if closeErr := output.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = files.Rename(temporary, final); err != nil {
		return err
	}

	input, err := files.OpenFile(final, playdate.FileReadData)
	if err != nil {
		return err
	}
	buffer := make([]byte, len(payload))
	read := 0
	for read < len(buffer) && err == nil {
		var count int
		count, err = input.Read(buffer[read:])
		read += count
	}
	if closeErr := input.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if string(buffer) != payload {
		return errReadPayload
	}
	info, err := files.Stat(final)
	if err != nil {
		return err
	}
	if info.IsDir || info.Size != uint32(len(payload)) {
		return errStatPayload
	}
	entries, err := files.List(directory, false)
	if err != nil {
		return err
	}
	if len(entries) != 1 || entries[0] != "state.bin" {
		return errListPayload
	}
	if err = files.Remove(directory, true); err != nil {
		return err
	}
	return nil
}
