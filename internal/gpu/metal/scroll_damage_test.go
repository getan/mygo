//go:build darwin

package metal

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/internal/scene"
)

// TestScrolledGlyphRowsMatchWhole draws a text column scrolling up in
// steps, into three targets in turn as a layer's drawables are, each frame
// only where it changed since its target was drawn, and requires the
// pixels of a frame drawn whole: a scrolled row must leave none of its
// pixels behind.
func TestScrolledGlyphRowsMatchWhole(t *testing.T) {
	r, err := newRenderer()
	if err != nil {
		unavailable(t, err)
	}
	defer r.Release()
	atlas := scene.NewAtlas(1, 128, 128)
	var masks [][4]uint16
	for range 4 {
		w, h := 6, 10
		x, y, ok := atlas.Alloc(w, h)
		if !ok {
			break
		}
		px := make([]byte, w*h)
		for i := range px {
			px[i] = 0xff
		}
		atlas.Put(x, y, w, h, px, w)
		masks = append(masks, [4]uint16{uint16(x), uint16(y), uint16(w), uint16(h)})
	}
	if len(masks) == 0 {
		t.Fatal("no glyph masks")
	}
	s := &scene.Scene{Width: 320, Height: 240, Clear: scene.Color{R: 250, G: 250, B: 250, A: 255},
		MaskAtlas: atlas, Text: scene.TextParams{GammaRatios: scene.GammaRatios(1.8), Contrast: 1}}
	build := func(offset float32) {
		s.Ops, s.Glyphs = s.Ops[:0], s.Glyphs[:0]
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 30, Y: 15, W: 260, H: 210}})
		for row := range 24 {
			y := float32(row*20) - offset
			if y < 0 || y > 230 {
				continue
			}
			start := int32(len(s.Glyphs))
			for ch := range 20 {
				k := masks[(row+ch)%len(masks)]
				s.Glyphs = append(s.Glyphs, scene.Glyph{
					X: 40 + float32(ch*8), Y: y,
					W: float32(k[2]), H: float32(k[3]),
					U: k[0], V: k[1], UW: k[2], VH: k[3],
					Color: scene.Color{A: 255},
				})
			}
			s.Ops = append(s.Ops, scene.Op{Kind: scene.OpGlyphs, Start: start, End: int32(len(s.Glyphs))})
		}
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpPopClip})
	}
	offset := float32(0)
	build(offset)
	partialFrames(t, r, s, false, 30, func(step int) string {
		offset += 7.5
		build(offset)
		return fmt.Sprintf("scroll %.1f", offset)
	})
}
