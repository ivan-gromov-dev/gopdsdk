package noncompliant

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

type lifecycleValue struct{}

func (lifecycleValue) Init(playdate.Context) error                                      { return nil }
func (lifecycleValue) Update(playdate.Context) (bool, error)                            { return true, nil }
func (*lifecycleValue) HandleLifecycle(playdate.Context, playdate.LifecycleEvent) error { return nil }

// analyzer-contract: application-entry-lifecycle negative
func New() playdate.Game { return lifecycleValue{} }

// analyzer-contract: stencil-callback-nesting negative
func nestedStencil(c playdate.BitmapCompositor, b playdate.Bitmap) error {
	return c.WithStencil(b, false, func() error { return c.WithStencil(b, false, func() error { return nil }) })
}
