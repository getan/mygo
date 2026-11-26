package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
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

// A fast flick of a trackpad glides as MYGO_SCROLL_SMOOTH asks, so one
// frame does not move the content by a line or more.
func TestSmoothFlickGlides(t *testing.T) {
	old := smoothFlicks
	smoothFlicks = true
	defer func() { smoothFlicks = old }()
	var s ScrollState
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			coreBox(c).Height(1000).Shrink(0)
		})
	}, 200, 200)
	tt.Move(100, 100)
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerScroll, X: 100, Y: 100, DY: 120, Precise: true})
	if s.Y != 0 {
		t.Fatalf("the flick scrolled at once to %v", s.Y)
	}
	tt.Advance(time.Second)
	if abs32(s.Y-120) > 0.01 {
		t.Fatalf("the flick ended at %v, want 120", s.Y)
	}
}

// A small precise event during a flick's glide joins it, rather than
// cancelling the glide and dropping its remaining travel.
func TestSmallScrollJoinsActiveFlick(t *testing.T) {
	old := smoothFlicks
	smoothFlicks = true
	defer func() { smoothFlicks = old }()
	var s ScrollState
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			coreBox(c).Height(2000).Shrink(0)
		})
	}, 200, 200)
	tt.Move(100, 100)
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerScroll, X: 100, Y: 100, DY: 120, Precise: true})
	tt.Advance(16 * time.Millisecond)
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerScroll, X: 100, Y: 100, DY: 10, Precise: true})
	tt.Advance(time.Second)
	if abs32(s.Y-130) > 0.01 {
		t.Fatalf("the small event dropped or doubled travel: at %v, want 130", s.Y)
	}
}

// A flick advances at most flickMaxSpeed in one frame, however large the
// event that started it.
func TestFlickMovesAtMostMaxSpeedPerFrame(t *testing.T) {
	old := smoothFlicks
	smoothFlicks = true
	defer func() { smoothFlicks = old }()
	var s ScrollState
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			coreBox(c).Height(2000).Shrink(0)
		})
	}, 200, 200)
	tt.Move(100, 100)
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerScroll, X: 100, Y: 100, DY: 300, Precise: true})
	if s.Y != 0 {
		t.Fatalf("the flick moved at once to %v", s.Y)
	}
	tt.Advance(time.Second / 60)
	if s.Y <= 0 || s.Y > flickMaxSpeed*0.020+0.5 {
		t.Fatalf("one frame moved %v, want at most %v", s.Y, flickMaxSpeed*0.020)
	}
	tt.Advance(time.Second)
	if abs32(s.Y-300) > 0.01 {
		t.Fatalf("the flick ended at %v, want 300", s.Y)
	}
}

// An anchor measured while the view builds applies before the next build:
// the frame that measured it shows its content where it planned it.
func TestAnchorAppliesToTheNextBuild(t *testing.T) {
	var s ScrollState
	anchored := false
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			coreBox(c).Height(1000).Shrink(0)
			if s.Y == 10 && !anchored {
				anchored = true
				s.AnchorBy(5)
			}
		})
	}, 200, 200)
	tt.Frame()
	scrolled := func() float64 {
		for _, st := range tt.rt.states {
			if st.track == &s {
				return st.scrollY
			}
		}
		return -1
	}
	s.Y = 10
	// One frame only: the anchor measured while it builds applies to the
	// next one, so this frame keeps the offset it planned with.
	tt.frameAt = tt.Now()
	tt.rt.runFrame()
	if got := scrolled(); got != 10 {
		t.Fatalf("the measuring frame scrolled to %v, want 10", got)
	}
	tt.frameAt = tt.Now()
	tt.rt.runFrame()
	if got := scrolled(); got != 15 {
		t.Fatalf("the next frame scrolled to %v, want 15", got)
	}
}
