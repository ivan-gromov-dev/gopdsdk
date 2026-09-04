package compliant

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

// analyzer-contract: application-entry-lifecycle positive
func New() playdate.Game { return &game{} }

// analyzer-contract: stencil-callback-nesting positive
func sequentialStencil(c playdate.BitmapCompositor, b playdate.Bitmap) error {
	return c.WithStencil(b, false, func() error { return nil })
}
