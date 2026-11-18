//go:build windows && !woodwind_quiet

package desktoplyrics

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user             = windows.NewLazySystemDLL("user32.dll")
	gdi              = windows.NewLazySystemDLL("gdi32.dll")
	registerClass    = user.NewProc("RegisterClassExW")
	createWindow     = user.NewProc("CreateWindowExW")
	defaultProc      = user.NewProc("DefWindowProcW")
	postMessage      = user.NewProc("PostMessageW")
	getMessage       = user.NewProc("GetMessageW")
	dispatchMessage  = user.NewProc("DispatchMessageW")
	translateMessage = user.NewProc("TranslateMessage")
	setWindowPos     = user.NewProc("SetWindowPos")
	getWindowRect    = user.NewProc("GetWindowRect")
	invalidate       = user.NewProc("InvalidateRect")
	setWindowLong    = user.NewProc("SetWindowLongPtrW")
	getDPI           = user.NewProc("GetDpiForWindow")
	callback         = syscall.NewCallback(lyricProc)
	registry         sync.Map
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type message struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}
type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}
type nativeWindow struct {
	mu           sync.Mutex
	hwnd         uintptr
	frame        Frame
	events       []Event
	hover        bool // native thread only
	done         chan struct{}
	positioned   bool
	lastX, lastY int
}

func wide(s string) *uint16 { p, _ := windows.UTF16PtrFromString(s); return p }

func Open() (Window, error) {
	w := &nativeWindow{done: make(chan struct{})}
	ready := make(chan error, 1)
	go w.run(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return w, nil
}

func (w *nativeWindow) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(w.done)
	// Pixel coordinates and fonts are scaled together on each monitor.
	user.NewProc("SetThreadDpiAwarenessContext").Call(^uintptr(3))
	instance, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	wc := windowClass{Proc: callback, Instance: instance, ClassName: wide("WoodwindDesktopLyrics")}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if atom, _, err := registerClass.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 && err != syscall.Errno(1410) {
		ready <- fmt.Errorf("register desktop lyrics: %w", err)
		return
	}
	var work rect
	user.NewProc("SystemParametersInfoW").Call(0x30, 0, uintptr(unsafe.Pointer(&work)), 0)
	dpi, _, _ := user.NewProc("GetDpiForSystem").Call()
	if dpi == 0 {
		dpi = 96
	}
	width, height := min(int(work.Right-work.Left), 800*int(dpi)/96), 154*int(dpi)/96
	x, y := int(work.Left)+(int(work.Right-work.Left)-width)/2, int(work.Bottom)-height-40*int(dpi)/96
	hwnd, _, err := createWindow.Call(0x08080088, uintptr(unsafe.Pointer(wc.ClassName)), uintptr(unsafe.Pointer(wide("Desktop Lyrics"))), 0x80000000, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, instance, 0)
	if hwnd == 0 {
		ready <- fmt.Errorf("create desktop lyrics: %w", err)
		return
	}
	w.hwnd = hwnd
	registry.Store(hwnd, w)
	defer registry.Delete(hwnd)
	user.NewProc("SetLayeredWindowAttributes").Call(hwnd, 0, 255, 1) // black color key
	user.NewProc("ShowWindow").Call(hwnd, 4)                         // SW_SHOWNOACTIVATE
	ready <- nil
	var msg message
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (w *nativeWindow) Update(f Frame) {
	f.Words = append([]Word(nil), f.Words...)
	w.mu.Lock()
	w.frame = f
	w.mu.Unlock()
	postMessage.Call(w.hwnd, 0x8001, 0, 0)
}

func (w *nativeWindow) Poll() []Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	events := w.events
	w.events = nil
	return events
}
func (w *nativeWindow) Close()          { postMessage.Call(w.hwnd, 0x10, 0, 0); <-w.done }
func (w *nativeWindow) snapshot() Frame { w.mu.Lock(); defer w.mu.Unlock(); return w.frame }
func (w *nativeWindow) event(e Event)   { w.mu.Lock(); w.events = append(w.events, e); w.mu.Unlock() }

func lyricProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	v, ok := registry.Load(hwnd)
	if !ok {
		r, _, _ := defaultProc.Call(hwnd, uintptr(msg), wp, lp)
		return r
	}
	w := v.(*nativeWindow)
	switch msg {
	case 0x8001:
		f := w.snapshot()
		style := uintptr(0x08080088) // NOACTIVATE | LAYERED | TOPMOST | TOOLWINDOW
		if f.Locked {
			style |= 0x20
			w.hover = false
		}
		setWindowLong.Call(hwnd, ^uintptr(19), style) // GWL_EXSTYLE
		dpi, _, _ := getDPI.Call(hwnd)
		if dpi == 0 {
			dpi = 96
		}
		flags := uintptr(0x10 | 0x2) // NOACTIVATE | NOMOVE
		width, height := 800*int(dpi)/96, 154*int(dpi)/96
		var monitorInfo struct {
			Size          uint32
			Monitor, Work rect
			Flags         uint32
		}
		monitorInfo.Size = uint32(unsafe.Sizeof(monitorInfo))
		monitor, _, _ := user.NewProc("MonitorFromWindow").Call(hwnd, 2)
		user.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&monitorInfo)))
		if monitorInfo.Work.Right > monitorInfo.Work.Left {
			width = min(width, int(monitorInfo.Work.Right-monitorInfo.Work.Left))
		}
		x, y := f.X, f.Y
		if f.Positioned && (!w.positioned || f.X != w.lastX || f.Y != w.lastY) {
			target := rect{int32(f.X), int32(f.Y), int32(f.X + width), int32(f.Y + height)}
			if monitor, _, _ := user.NewProc("MonitorFromRect").Call(uintptr(unsafe.Pointer(&target)), 0); monitor != 0 {
				user.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&monitorInfo)))
				work := monitorInfo.Work
				width = min(width, int(work.Right-work.Left))
				x = max(int(work.Left), min(x, int(work.Right)-width))
				y = max(int(work.Top), min(y, int(work.Bottom)-height))
				flags = 0x10
			}
			w.positioned = true
			w.lastX = f.X
			w.lastY = f.Y
		}
		setWindowPos.Call(hwnd, ^uintptr(0), uintptr(x), uintptr(y), uintptr(width), uintptr(height), flags)
		invalidate.Call(hwnd, 0, 0)
		return 0
	case 0x21:
		return 3 // MA_NOACTIVATE
	case 0x84: // HTCAPTION outside the toolbar provides native dragging.
		if w.snapshot().Locked {
			return ^uintptr(0)
		}
		var bounds rect
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds)))
		x, y := int(int16(lp&0xffff))-int(bounds.Left), int(int16((lp>>16)&0xffff))-int(bounds.Top)
		scale := float64(bounds.Right-bounds.Left) / 800
		if y < int(34*scale) && x >= int(275*scale) && x < int(525*scale) {
			return 1
		}
		return 2
	case 0x200, 0xa0: // client and caption mouse movement both show controls
		track := struct {
			Size, Flags uint32
			Window      uintptr
			HoverTime   uint32
		}{Flags: 2, Window: hwnd}
		if msg == 0xa0 {
			track.Flags |= 0x10
		} // TME_NONCLIENT
		track.Size = uint32(unsafe.Sizeof(track))
		user.NewProc("TrackMouseEvent").Call(uintptr(unsafe.Pointer(&track)))
		if !w.hover {
			w.hover = true
			invalidate.Call(hwnd, 0, 0)
		}
		return 0
	case 0x2a3, 0x2a2:
		w.hover = false
		invalidate.Call(hwnd, 0, 0)
		return 0
	case 0x202:
		var bounds rect
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds)))
		scale := float64(bounds.Right-bounds.Left) / 800
		x, y := float64(int16(lp&0xffff))/scale, float64(int16(lp>>16))/scale
		if y < 34 && x >= 275 && x < 525 {
			w.event(Event{Action: []Action{Previous, TogglePlay, Next, Lock, Hide}[int((x-275)/50)]})
		}
		return 0
	case 0x232:
		var bounds rect
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&bounds)))
		// Keep lastX/Y as the last app-requested coordinates. A queued update
		// from before this drag must not move the window back before Poll.
		w.event(Event{Moved: true, X: int(bounds.Left), Y: int(bounds.Top)})
		return 0
	case 0x2e0: // WM_DPICHANGED: preserve the OS-provided physical placement.
		// LPARAM holds an OS-owned RECT pointer, not a Go heap address.
		bounds := *(**rect)(unsafe.Pointer(&lp))
		setWindowPos.Call(hwnd, ^uintptr(0), uintptr(bounds.Left), uintptr(bounds.Top), uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), 0x10)
		w.event(Event{Moved: true, X: int(bounds.Left), Y: int(bounds.Top)})
		return 0
	case 0xf:
		w.paint()
		return 0
	case 0x14:
		return 1
	case 0x10:
		user.NewProc("DestroyWindow").Call(hwnd)
		return 0
	case 2:
		user.NewProc("PostQuitMessage").Call(0)
		return 0
	}
	r, _, _ := defaultProc.Call(hwnd, uintptr(msg), wp, lp)
	return r
}

func rgb(v uint32) uintptr { return uintptr((v&255)<<16 | v&0xff00 | v>>16&255) }

func (w *nativeWindow) paint() {
	// PAINTSTRUCT has pointer-sized alignment on 64-bit Windows.
	var ps struct {
		DC                 uintptr
		Erase              int32
		Paint              rect
		Restore, IncUpdate int32
		Reserved           [32]byte
	}
	paintDC, _, _ := user.NewProc("BeginPaint").Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer user.NewProc("EndPaint").Call(w.hwnd, uintptr(unsafe.Pointer(&ps)))
	var bounds rect
	user.NewProc("GetClientRect").Call(w.hwnd, uintptr(unsafe.Pointer(&bounds)))
	// Compose offscreen so the transparent color-key clear cannot flicker
	// between the word frames.
	dc, _, _ := gdi.NewProc("CreateCompatibleDC").Call(paintDC)
	bitmap, _, _ := gdi.NewProc("CreateCompatibleBitmap").Call(paintDC, uintptr(bounds.Right), uintptr(bounds.Bottom))
	priorBitmap, _, _ := gdi.NewProc("SelectObject").Call(dc, bitmap)
	defer func() {
		gdi.NewProc("BitBlt").Call(paintDC, 0, 0, uintptr(bounds.Right), uintptr(bounds.Bottom), dc, 0, 0, 0x00cc0020)
		gdi.NewProc("SelectObject").Call(dc, priorBitmap)
		gdi.NewProc("DeleteObject").Call(bitmap)
		gdi.NewProc("DeleteDC").Call(dc)
	}()
	f := w.snapshot()
	scale := float64(bounds.Right) / 800
	bg := uint32(0)
	if w.hover && !f.Locked {
		bg = 0x202024
	}
	brush, _, _ := gdi.NewProc("CreateSolidBrush").Call(rgb(bg))
	user.NewProc("FillRect").Call(dc, uintptr(unsafe.Pointer(&bounds)), brush)
	gdi.NewProc("DeleteObject").Call(brush)
	gdi.NewProc("SetBkMode").Call(dc, 1)
	font := func(size int) uintptr {
		h, _, _ := gdi.NewProc("CreateFontW").Call(uintptr(-int32(float64(size)*scale)), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 4, 0, uintptr(unsafe.Pointer(wide("Microsoft YaHei UI"))))
		return h
	}
	selectObj := gdi.NewProc("SelectObject")
	textExtent := gdi.NewProc("GetTextExtentPoint32W")
	measure := func(s string) int32 {
		chars := syscall.StringToUTF16(s)
		var size point
		textExtent.Call(dc, uintptr(unsafe.Pointer(&chars[0])), uintptr(len(chars)-1), uintptr(unsafe.Pointer(&size)))
		return size.X
	}
	draw := func(s string, x, y int32, color uint32) {
		chars := syscall.StringToUTF16(s)
		gdi.NewProc("SetTextColor").Call(dc, rgb(color))
		gdi.NewProc("TextOutW").Call(dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&chars[0])), uintptr(len(chars)-1))
	}
	mainFont := font(max(1, f.FontSize))
	old, _, _ := selectObj.Call(dc, mainFont)
	defer func() { selectObj.Call(dc, old); gdi.NewProc("DeleteObject").Call(mainFont) }()
	total := int32(0)
	for _, word := range f.Words {
		total += measure(word.Text)
	}
	if total > bounds.Right-int32(32*scale) {
		selectObj.Call(dc, old)
		gdi.NewProc("DeleteObject").Call(mainFont)
		mainFont = font(max(1, int(float64(f.FontSize)*float64(bounds.Right-int32(32*scale))/float64(total))))
		selectObj.Call(dc, mainFont)
		total = 0
		for _, word := range f.Words {
			total += measure(word.Text)
		}
	}
	x, y := (bounds.Right-total)/2, int32(43*scale)
	for _, word := range f.Words {
		advance := measure(word.Text)
		for _, offset := range []point{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			draw(word.Text, x+offset.X, y+offset.Y, 0x010101)
		}
		draw(word.Text, x, y, 0xffffff)
		saved, _, _ := gdi.NewProc("SaveDC").Call(dc)
		gdi.NewProc("IntersectClipRect").Call(dc, uintptr(x), 0, uintptr(x+int32(float64(advance)*word.Progress)), uintptr(bounds.Bottom))
		draw(word.Text, x, y, f.Color)
		gdi.NewProc("RestoreDC").Call(dc, saved)
		x += advance
	}
	small := font(18)
	selectObj.Call(dc, small)
	secondaryWidth := measure(f.Secondary)
	if secondaryWidth > bounds.Right-int32(32*scale) {
		selectObj.Call(dc, old)
		gdi.NewProc("DeleteObject").Call(small)
		small = font(max(1, int(18*float64(bounds.Right-int32(32*scale))/float64(secondaryWidth))))
		selectObj.Call(dc, small)
		secondaryWidth = measure(f.Secondary)
	}
	sx, sy := (bounds.Right-secondaryWidth)/2, int32(110*scale)
	for _, offset := range []point{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		draw(f.Secondary, sx+offset.X, sy+offset.Y, 0x010101)
	}
	draw(f.Secondary, sx, sy, 0xffffff)
	if w.hover && !f.Locked {
		play := "▶"
		if f.Playing {
			play = "Ⅱ"
		}
		for i, label := range []string{"⏮", play, "⏭", "🔒", "×"} {
			draw(label, int32(float64(275+i*50)*scale)+(int32(50*scale)-measure(label))/2, int32(6*scale), 0xffffff)
		}
	}
	selectObj.Call(dc, old)
	gdi.NewProc("DeleteObject").Call(small)
}
