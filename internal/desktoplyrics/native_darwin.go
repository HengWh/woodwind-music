//go:build darwin && !woodwind_quiet

package desktoplyrics

import (
	"fmt"
	"runtime"
	"structs"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// Use AppKit without cgo, as NSGo's standard packaging requires. Every native
// object and operation belongs to the main queue; Poll never waits on AppKit.
type cgPoint struct {
	_    structs.HostLayout
	X, Y float64
}
type cgSize struct {
	_             structs.HostLayout
	Width, Height float64
}
type cgRect struct {
	_      structs.HostLayout
	Origin cgPoint
	Size   cgSize
}

func rect(x, y, w, h float64) cgRect {
	return cgRect{Origin: cgPoint{X: x, Y: y}, Size: cgSize{Width: w, Height: h}}
}

var (
	macOnce       sync.Once
	macError      error
	macCaller     objc.ID
	macViewClass  objc.Class
	macJobsMu     sync.Mutex
	macJobs       []func()
	macWindows    = map[objc.ID]*nativeWindow{} // main thread only
	macSelectors  sync.Map
	macAttributes map[string]objc.ID
	fillRect      func(cgRect, uint64)
	clipRect      func(cgRect)
)

func sel(s string) objc.SEL {
	if v, ok := macSelectors.Load(s); ok {
		return v.(objc.SEL)
	}
	v := objc.RegisterName(s)
	macSelectors.Store(s, v)
	return v
}
func class(s string) objc.ID                         { return objc.ID(objc.GetClass(s)) }
func send(id objc.ID, s string, args ...any) objc.ID { return objc.Send[objc.ID](id, sel(s), args...) }
func nsString(s string) objc.ID {
	v := send(send(class("NSString"), "alloc"), "initWithBytes:length:encoding:", uintptr(unsafe.Pointer(unsafe.StringData(s))), uintptr(len(s)), uintptr(4))
	runtime.KeepAlive(s)
	return send(v, "autorelease")
}

func loadMac() {
	macOnce.Do(func() {
		lib, err := purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			macError = err
			return
		}
		purego.RegisterLibFunc(&fillRect, lib, "NSRectFillUsingOperation")
		purego.RegisterLibFunc(&clipRect, lib, "NSRectClip")
		macAttributes = map[string]objc.ID{}
		for _, name := range []string{"NSFontAttributeName", "NSForegroundColorAttributeName", "NSStrokeColorAttributeName", "NSStrokeWidthAttributeName"} {
			addr, err := purego.Dlsym(lib, name)
			if err != nil {
				macError = err
				return
			}
			macAttributes[name] = **(**objc.ID)(unsafe.Pointer(&addr))
		}
		caller, err := objc.RegisterClass("WoodwindLyricCaller", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{Cmd: sel("drain"), Fn: func(objc.ID, objc.SEL) {
			macJobsMu.Lock()
			jobs := macJobs
			macJobs = nil
			macJobsMu.Unlock()
			pool := send(class("NSAutoreleasePool"), "new")
			defer send(pool, "drain")
			for _, job := range jobs {
				job()
			}
		}}})
		if err != nil {
			macError = err
			return
		}
		macCaller = send(objc.ID(caller), "new")
		macViewClass, macError = objc.RegisterClass("WoodwindLyricView", objc.GetClass("NSView"), nil, nil, []objc.MethodDef{
			{Cmd: sel("isFlipped"), Fn: func(objc.ID, objc.SEL) bool { return true }},
			{Cmd: sel("acceptsFirstMouse:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool { return true }},
			{Cmd: sel("drawRect:"), Fn: func(id objc.ID, _ objc.SEL, _ cgRect) {
				if w := macWindows[id]; w != nil {
					w.draw()
				}
			}},
			{Cmd: sel("mouseEntered:"), Fn: func(id objc.ID, _ objc.SEL, _ objc.ID) {
				if w := macWindows[id]; w != nil {
					w.hover = true
					send(id, "setNeedsDisplay:", true)
				}
			}},
			{Cmd: sel("mouseExited:"), Fn: func(id objc.ID, _ objc.SEL, _ objc.ID) {
				if w := macWindows[id]; w != nil {
					w.hover = false
					send(id, "setNeedsDisplay:", true)
				}
			}},
			{Cmd: sel("mouseDown:"), Fn: func(id objc.ID, _ objc.SEL, event objc.ID) {
				if w := macWindows[id]; w != nil {
					w.mouseDown(event)
				}
			}},
			{Cmd: sel("mouseDragged:"), Fn: func(id objc.ID, _ objc.SEL, _ objc.ID) {
				if w := macWindows[id]; w != nil && w.dragging {
					p := objc.Send[cgPoint](class("NSEvent"), sel("mouseLocation"))
					send(w.panel, "setFrameOrigin:", cgPoint{X: w.windowOrigin.X + p.X - w.dragOrigin.X, Y: w.windowOrigin.Y + p.Y - w.dragOrigin.Y})
				}
			}},
			{Cmd: sel("mouseUp:"), Fn: func(id objc.ID, _ objc.SEL, _ objc.ID) {
				if w := macWindows[id]; w != nil && w.dragging {
					w.dragging = false
					f := objc.Send[cgRect](w.panel, sel("frame"))
					w.event(Event{Moved: true, X: int(f.Origin.X), Y: int(mainScreenTop() - f.Origin.Y - f.Size.Height)})
				}
			}},
		})
	})
}

func macMain(fn func()) {
	macJobsMu.Lock()
	macJobs = append(macJobs, fn)
	macJobsMu.Unlock()
	// Scheduling a selector avoids waiting on main while NSGo closes its
	// engine or waits briefly for a layout commit.
	send(macCaller, "performSelectorOnMainThread:withObject:waitUntilDone:", sel("drain"), objc.ID(0), false)
}

type nativeWindow struct {
	mu     sync.Mutex
	events []Event
	// The remaining state is owned exclusively by the main queue.
	panel, view, tracking               objc.ID
	frame                               Frame
	hover, dragging, closed, positioned bool
	dragOrigin, windowOrigin            cgPoint
	lastX, lastY                        int
}

func Open() (Window, error) {
	loadMac()
	if macError != nil {
		return nil, fmt.Errorf("desktop lyrics: %w", macError)
	}
	w := &nativeWindow{}
	macMain(func() {
		visible := objc.Send[cgRect](send(send(class("NSScreen"), "screens"), "firstObject"), sel("visibleFrame"))
		w.panel = send(send(class("NSPanel"), "alloc"), "initWithContentRect:styleMask:backing:defer:", rect(visible.Origin.X+(visible.Size.Width-800)/2, visible.Origin.Y+40, 800, 154), uintptr(1<<7), uintptr(2), false)
		send(w.panel, "setLevel:", int64(3))
		send(w.panel, "setOpaque:", false)
		send(w.panel, "setBackgroundColor:", send(class("NSColor"), "clearColor"))
		send(w.panel, "setHasShadow:", false)
		send(w.panel, "setHidesOnDeactivate:", false)
		send(w.panel, "setReleasedWhenClosed:", false)
		send(w.panel, "setCollectionBehavior:", uintptr(1|1<<8))
		w.view = send(send(objc.ID(macViewClass), "alloc"), "initWithFrame:", rect(0, 0, 800, 154))
		macWindows[w.view] = w
		w.tracking = send(send(class("NSTrackingArea"), "alloc"), "initWithRect:options:owner:userInfo:", rect(0, 0, 800, 154), uintptr(0x281), w.view, objc.ID(0))
		send(w.view, "addTrackingArea:", w.tracking)
		send(w.panel, "setContentView:", w.view)
		send(w.panel, "orderFrontRegardless")
	})
	return w, nil
}

func (w *nativeWindow) Update(f Frame) {
	f.Words = append([]Word(nil), f.Words...)
	macMain(func() {
		if w.closed {
			return
		}
		w.frame = f
		send(w.panel, "setIgnoresMouseEvents:", f.Locked)
		if f.Locked {
			w.hover = false
		}
		if f.Positioned && !w.dragging && (!w.positioned || w.lastX != f.X || w.lastY != f.Y) {
			target := rect(float64(f.X), mainScreenTop()-float64(f.Y)-154, 800, 154)
			screens := send(class("NSScreen"), "screens")
			for i, n := uintptr(0), objc.Send[uintptr](screens, sel("count")); i < n; i++ {
				screen := objc.Send[cgRect](send(screens, "objectAtIndex:", i), sel("visibleFrame"))
				if target.Origin.X < screen.Origin.X+screen.Size.Width && target.Origin.X+target.Size.Width > screen.Origin.X && target.Origin.Y < screen.Origin.Y+screen.Size.Height && target.Origin.Y+target.Size.Height > screen.Origin.Y {
					target.Origin.X = max(screen.Origin.X, min(target.Origin.X, screen.Origin.X+screen.Size.Width-target.Size.Width))
					target.Origin.Y = max(screen.Origin.Y, min(target.Origin.Y, screen.Origin.Y+screen.Size.Height-target.Size.Height))
					send(w.panel, "setFrameOrigin:", target.Origin)
					break
				}
			}
			w.positioned = true
			w.lastX = f.X
			w.lastY = f.Y
		}
		send(w.view, "setNeedsDisplay:", true)
	})
}

func mainScreenTop() float64 {
	screen := send(send(class("NSScreen"), "screens"), "firstObject")
	f := objc.Send[cgRect](screen, sel("frame"))
	return f.Origin.Y + f.Size.Height
}
func (w *nativeWindow) Poll() []Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	events := w.events
	w.events = nil
	return events
}
func (w *nativeWindow) event(e Event) { w.mu.Lock(); w.events = append(w.events, e); w.mu.Unlock() }
func (w *nativeWindow) Close() {
	macMain(func() {
		if w.closed {
			return
		}
		w.closed = true
		delete(macWindows, w.view)
		send(w.panel, "orderOut:", objc.ID(0))
		send(w.panel, "close")
		send(w.view, "removeTrackingArea:", w.tracking)
		send(w.tracking, "release")
		send(w.panel, "setContentView:", objc.ID(0))
		send(w.view, "release")
		send(w.panel, "release")
	})
}

func (w *nativeWindow) mouseDown(event objc.ID) {
	p := objc.Send[cgPoint](w.view, sel("convertPoint:fromView:"), objc.Send[cgPoint](event, sel("locationInWindow")), objc.ID(0))
	if w.hover && p.Y < 34 && p.X >= 275 && p.X < 525 {
		w.event(Event{Action: []Action{Previous, TogglePlay, Next, Lock, Hide}[int((p.X-275)/50)]})
		return
	}
	w.dragOrigin = objc.Send[cgPoint](class("NSEvent"), sel("mouseLocation"))
	w.windowOrigin = objc.Send[cgRect](w.panel, sel("frame")).Origin
	w.dragging = true
}

func macColor(rgb uint32) objc.ID {
	return send(class("NSColor"), "colorWithSRGBRed:green:blue:alpha:", float64(rgb>>16&255)/255, float64(rgb>>8&255)/255, float64(rgb&255)/255, float64(1))
}
func attributes(font objc.ID, color uint32, stroke bool) objc.ID {
	d := send(class("NSMutableDictionary"), "dictionary")
	for key, value := range map[string]objc.ID{"NSFontAttributeName": font, "NSForegroundColorAttributeName": macColor(color)} {
		send(d, "setObject:forKey:", value, macAttributes[key])
	}
	if stroke {
		send(d, "setObject:forKey:", macColor(0), macAttributes["NSStrokeColorAttributeName"])
		send(d, "setObject:forKey:", send(class("NSNumber"), "numberWithDouble:", float64(-5)), macAttributes["NSStrokeWidthAttributeName"])
	}
	return d
}

func (w *nativeWindow) draw() {
	f := w.frame
	send(send(class("NSColor"), "clearColor"), "setFill")
	fillRect(rect(0, 0, 800, 154), uint64(1)) // copy
	if w.hover && !f.Locked {
		send(send(class("NSColor"), "colorWithWhite:alpha:", float64(.06), float64(.8)), "setFill")
		fillRect(rect(0, 0, 800, 154), uint64(2)) // source over
	}
	font := send(class("NSFont"), "systemFontOfSize:weight:", float64(max(1, f.FontSize)), float64(.4))
	white := attributes(font, 0xffffff, true)
	highlight := attributes(font, f.Color, true)
	texts := make([]objc.ID, len(f.Words))
	widths := make([]float64, len(f.Words))
	total := float64(0)
	for i, word := range f.Words {
		texts[i] = nsString(word.Text)
		widths[i] = objc.Send[cgSize](texts[i], sel("sizeWithAttributes:"), white).Width
		total += widths[i]
	}
	context := class("NSGraphicsContext")
	scale := float64(1)
	if total > 768 {
		scale = 768 / total
	}
	send(context, "saveGraphicsState")
	transform := send(class("NSAffineTransform"), "transform")
	send(transform, "translateXBy:yBy:", 16+(768-total*scale)/2, float64(43))
	send(transform, "scaleBy:", scale)
	send(transform, "concat")
	x := float64(0)
	for i, word := range f.Words {
		send(texts[i], "drawAtPoint:withAttributes:", cgPoint{X: x, Y: 0}, white)
		send(context, "saveGraphicsState")
		clipRect(rect(x-1, -4, (widths[i]+2)*word.Progress, float64(f.FontSize)*1.7))
		send(texts[i], "drawAtPoint:withAttributes:", cgPoint{X: x, Y: 0}, highlight)
		send(context, "restoreGraphicsState")
		x += widths[i]
	}
	send(context, "restoreGraphicsState")
	small := send(class("NSFont"), "systemFontOfSize:weight:", float64(18), float64(.23))
	subAttrs := attributes(small, 0xffffff, true)
	sub := nsString(f.Secondary)
	subWidth := objc.Send[cgSize](sub, sel("sizeWithAttributes:"), subAttrs).Width
	subScale := float64(1)
	if subWidth > 768 {
		subScale = 768 / subWidth
	}
	send(context, "saveGraphicsState")
	transform = send(class("NSAffineTransform"), "transform")
	send(transform, "translateXBy:yBy:", (800-subWidth*subScale)/2, float64(110))
	send(transform, "scaleBy:", subScale)
	send(transform, "concat")
	send(sub, "drawAtPoint:withAttributes:", cgPoint{}, subAttrs)
	send(context, "restoreGraphicsState")
	if w.hover && !f.Locked {
		toolbarAttrs := attributes(small, 0xffffff, false)
		play := "▶"
		if f.Playing {
			play = "Ⅱ"
		}
		for i, label := range []string{"⏮", play, "⏭", "🔒", "×"} {
			s := nsString(label)
			width := objc.Send[cgSize](s, sel("sizeWithAttributes:"), toolbarAttrs).Width
			send(s, "drawAtPoint:withAttributes:", cgPoint{X: 275 + float64(i*50) + (50-width)/2, Y: 7}, toolbarAttrs)
		}
	}
}
