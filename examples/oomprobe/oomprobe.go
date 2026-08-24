// Package oomprobe intentionally exhausts the bounded device heap for acceptance.
package oomprobe

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

const (
	blockSize = 16 * 1024
	maxBlocks = 32
)

type game struct {
	blocks [maxBlocks][]byte
	frame  uint32
}

// New creates the intentional out-of-memory acceptance workload.
func New() playdate.Game { return &game{} }

func (*game) Init(playdate.Context) error { return nil }

func (g *game) Update(context playdate.Context) (bool, error) {
	block := make([]byte, blockSize)
	for index := 0; index < len(block); index += 256 {
		block[index] = byte(g.frame) ^ byte(index>>8)
	}
	g.blocks[g.frame] = block
	g.frame++

	context.Clear()
	context.DrawText("OOM probe: filling heap", 16, 16)
	return true, nil
}
