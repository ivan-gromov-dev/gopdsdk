package oomprobe

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

func (testContext) LoadSoundEffect(string) (playdate.SoundEffect, error) { return nil, nil }
func (testContext) LoadFilePlayer(string) (playdate.FilePlayer, error)   { return nil, nil }
