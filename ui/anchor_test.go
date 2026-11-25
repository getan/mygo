package ui

import (
	"testing"
	"time"
)

// AnchorBy is a layout correction, not an app jump: a wheel notch already
// gliding must keep its remaining distance after the content above the view
// changes height.
func TestWheelAnchorByKeepsGlide(t *testing.T) {
	var s ScrollState
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			coreBox(c).Height(1000).Shrink(0)
		})
	}, 200, 200)
	tt.Move(100, 100)
	tt.Wheel(100, 100, 0, 100)
	tt.Frame()
	if s.Y != 0 {
		t.Fatalf("notch moved at once to %v", s.Y)
	}
	s.AnchorBy(10)
	tt.Advance(time.Second)
	if s.Y != 110 {
		t.Fatalf("anchor plus wheel ended at %v, want 110", s.Y)
	}
}

// A direct Y write remains an app takeover and stops the wheel, as before.
func TestDirectScrollWriteStillStopsWheel(t *testing.T) {
	var s ScrollState
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			coreBox(c).Height(1000).Shrink(0)
		})
	}, 200, 200)
	tt.Move(100, 100)
	tt.Wheel(100, 100, 0, 100)
	tt.Frame()
	s.Y = 10
	tt.Advance(time.Second)
	if s.Y != 10 {
		t.Fatalf("direct write did not take over: %v", s.Y)
	}
}
