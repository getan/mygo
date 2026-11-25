package ui

import (
	"bytes"
	"testing"

	"github.com/egoist/mygo/internal/raster"
)

// A scroll container of text must present exactly what a full redraw of
// the same scene would, frame by frame: no pixels of the frames before.
func TestScrollTextMatchesFullDrawing(t *testing.T) {
	var s ScrollState
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			for i := range 120 {
				coreTextf(c, "Row %03d some text to draw", i).Height(20).Shrink(0)
			}
		})
	}, 300, 200)
	tt.Frame()
	for step := range 80 {
		s.Y += 6.5
		tt.Frame()
		want := raster.NewImage(tt.rt.scene.Width, tt.rt.scene.Height)
		raster.Render(want, &tt.rt.scene)
		got := &tt.h.img.Image
		if !bytes.Equal(got.Pix, want.Pix) {
			for y := 0; y < got.H; y++ {
				for x := 0; x < got.W; x++ {
					i := y*got.Stride + 4*x
					if !bytes.Equal(got.Pix[i:i+4], want.Pix[i:i+4]) {
						t.Fatalf("step %d: stale pixel at (%d,%d): got %v want %v", step, x, y, got.Pix[i:i+4], want.Pix[i:i+4])
					}
				}
			}
		}
	}
}
