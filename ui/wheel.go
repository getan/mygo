package ui

import (
	"math"
	"slices"
	"time"
)

// wheelAnimation is how the notches of a mouse wheel scroll: along a short
// ease out, as browsers scroll them, rather than the whole notch at once.
var wheelAnimation = Animation{Duration: 150 * time.Millisecond, Ease: EaseOut}

// wheelGlide is the notches of a wheel on their way through a scroll
// container: x and y go from 0 to as far as all of them scroll, and done
// is how far the content has scrolled with them so far. It moves the
// content by what it went since the last frame, so that what moves the
// content meanwhile, as a List measuring its rows, does not throw it off.
// reveal tells a ScrollIntoView gliding there instead (glideScroll).
type wheelGlide struct {
	x, y         channel
	doneX, doneY float64
	reveal       bool
}

// ahead returns how far the notches of the glide have yet to scroll s,
// as scrollBy takes them.
func (g *wheelGlide) ahead() (dx, dy float64) {
	if g == nil || g.reveal {
		return 0, 0
	}
	return g.x.target - g.doneX, g.y.target - g.doneY
}

// shiftY moves the whole wheel gesture by d, including the part already
// applied, so a layout anchor that moved the content by d leaves the same
// remaining wheel distance instead of applying that distance twice.
func (g *wheelGlide) shiftY(d float64) {
	if g == nil || d == 0 {
		return
	}
	g.y.shift(d)
	g.doneY += d
}

// wheelMoves reports whether a notch scrolling dx, dy moves container s
// further than the notches on their way there already do.
func wheelMoves(s *state, dx, dy float32) bool {
	ax, ay := s.wheel.ahead()
	if s.rtl {
		dx, ax = -dx, -ax
	}
	mx, my := max(0, s.contentW-float64(s.w)), max(0, s.contentH-float64(s.h))
	if dy != 0 && s.flags&flagScrollY != 0 {
		at := max(0, min(s.scrollY+ay, my))
		if max(0, min(at+float64(dy), my)) != at {
			return true
		}
	}
	if dx != 0 && s.flags&flagScrollX != 0 {
		at := max(0, min(s.scrollX+ax, mx))
		if max(0, min(at+float64(dx), mx)) != at {
			return true
		}
	}
	return false
}

// wheelScroll scrolls container s by a notch of dx, dy smoothly, on top
// of the notches on their way there.
func (rt *engine) wheelScroll(s *state, dx, dy float32) {
	rt.wheelScrollWith(s, dx, dy, wheelAnimation)
}

// flickScroll is precise travel waiting to be applied at up to
// flickMaxSpeed, with the time it last moved.
type flickScroll struct {
	x, y float64
	at   time.Time
}

// flickScroll adds a precise scroll to s' pending travel. glideFlicks
// applies it over the frames that follow at a constant speed: no animation
// is restarted for each event, so the content never jolts.
func (rt *engine) flickScroll(s *state, dx, dy float32) {
	f := s.flick
	if f == nil {
		f = &flickScroll{at: rt.now()}
		s.flick = f
	}
	f.x += float64(dx)
	f.y += float64(dy)
	if !slices.Contains(rt.flicking, s.id) {
		rt.flicking = append(rt.flicking, s.id)
	}
	rt.requestFrame()
}

// glideFlicks applies each container's pending precise travel at no more
// than flickMaxSpeed, the velocity held from frame to frame.
func (rt *engine) glideFlicks(now time.Time) {
	kept := rt.flicking[:0]
	for _, id := range rt.flicking {
		s := rt.states[id]
		if s == nil || s.flick == nil {
			continue
		}
		f := s.flick
		dt := now.Sub(f.at)
		if dt <= 0 {
			dt = time.Second / 120
		}
		// A stalled frame must not turn into one huge jump.
		if dt > 20*time.Millisecond {
			dt = 20 * time.Millisecond
		}
		maxStep := flickMaxSpeed * dt.Seconds()
		dx, dy := clampFlick(f.x, maxStep), clampFlick(f.y, maxStep)
		f.x, f.y, f.at = f.x-dx, f.y-dy, now
		if dx != 0 || dy != 0 {
			if !scrollBy(s, float32(dx), float32(dy)) {
				// At the end of the content, or there.
				s.flick = nil
				continue
			}
			rt.animating = true
		}
		if absFlick(f.x)+absFlick(f.y) > 0.5 {
			kept = append(kept, id)
		} else {
			s.flick = nil
		}
	}
	rt.flicking = kept
}

func clampFlick(v, limit float64) float64 {
	if v > limit {
		return limit
	}
	if v < -limit {
		return -limit
	}
	return v
}

func absFlick(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// wheelScrollWith scrolls s by dx, dy over a, as wheelScroll does with the
// wheel's animation.
func (rt *engine) wheelScrollWith(s *state, dx, dy float32, a Animation) {
	now := rt.now()
	g := s.wheel
	if g == nil || g.reveal {
		// The user takes over from ScrollIntoView too.
		g = &wheelGlide{x: channel{at: now}, y: channel{at: now}}
		s.wheel = g
		if !slices.Contains(rt.wheeling, s.id) {
			rt.wheeling = append(rt.wheeling, s.id)
		}
	}
	if s.track != nil {
		s.track.glide = nil // the user takes over from ScrollTo
	}
	if l := s.list; l != nil {
		l.scrolled = false // and from the list's
	}
	g.x.advance(now)
	g.y.advance(now)
	if dx != 0 {
		g.x.retarget(wheelTurn(&g.x, dx, float64(s.scrollX), &g.doneX), a)
	}
	if dy != 0 {
		g.y.retarget(wheelTurn(&g.y, dy, float64(s.scrollY), &g.doneY), a)
	}
	rt.requestFrame()
}

// wheelTurn returns the target for a notch of d on ch, scrolled to actual:
// when the notch goes the other way than the remaining travel, the
// remaining travel stops, and the notch starts over from where the content
// is, so that reversing the wheel at the end of the content turns at once
// instead of being absorbed by the notches still on their way.
func wheelTurn(ch *channel, d float32, actual float64, done *float64) float64 {
	if ch.moving && (d > 0) != (ch.target > ch.value) {
		ch.value = actual
		ch.velocity = 0
		*done = actual
		return actual + float64(d)
	}
	return ch.target + float64(d)
}

// glideWheels moves the containers that notches of a wheel scroll, or
// ScrollIntoView glides, to the frame's time.
func (rt *engine) glideWheels(now time.Time) {
	kept := rt.wheeling[:0]
	for _, id := range rt.wheeling {
		s := rt.states[id]
		if s == nil || s.wheel == nil {
			continue
		}
		takeover := s.list != nil && s.list.scrolled
		if t := s.track; t != nil && t.anchorY == 0 &&
			(t.X != float32(s.scrollX) || t.Y != float32(s.scrollY) || t.glide != nil) {
			takeover = true
		}
		if takeover {
			// The app set the offset, or a ScrollTo, of its ScrollState
			// or its ListState: it takes over.
			s.wheel = nil
			continue
		}
		g := s.wheel
		g.x.advance(now)
		g.y.advance(now)
		dx, dy := g.x.value-g.doneX, g.y.value-g.doneY
		g.doneX, g.doneY = g.x.value, g.y.value
		moved := scrollBy(s, float32(dx), float32(dy))
		if !moved && (dx != 0 || dy != 0) || !g.x.moving && !g.x.pending && !g.y.moving && !g.y.pending {
			// At the end of the content, or there.
			s.wheel = nil
			if g.reveal && s.snap != nil {
				// It rests at the place nearest.
				s.snap.scrolled(rt, s, 0)
			}
			continue
		}
		rt.animating = true
		kept = append(kept, id)
	}
	rt.wheeling = kept
}

// glideScroll scrolls container s smoothly to offset x, y as a says, as
// a wheel's notches glide (ScrollIntoView with an Animation): the user
// scrolling, or the app, takes over.
func (rt *engine) glideScroll(s *state, x, y float64, a Animation, now time.Time) {
	dx, dy := x-s.scrollX, y-s.scrollY
	if dx == 0 && dy == 0 {
		if s.wheel != nil && s.wheel.reveal {
			s.wheel = nil // there already
		}
		return
	}
	if s.rtl {
		dx = -dx // as scrollBy takes it
	}
	if g := s.wheel; g != nil && g.reveal && math.Abs(g.x.goal()-g.doneX-dx) < snapEps && math.Abs(g.y.goal()-g.doneY-dy) < snapEps {
		// On its way there already, as when asked again.
		rt.animating = true
		return
	}
	g := &wheelGlide{x: channel{at: now}, y: channel{at: now}, reveal: true}
	headFor(&g.x, dx, a, now)
	headFor(&g.y, dy, a, now)
	s.wheel = g
	if !slices.Contains(rt.wheeling, s.id) {
		rt.wheeling = append(rt.wheeling, s.id)
	}
	if s.track != nil {
		s.track.glide = nil // it takes over from ScrollTo
	}
	if l := s.list; l != nil {
		l.scrolled = false // and from the list's
	}
	s.haltSnap()
	rt.animating = true
}
