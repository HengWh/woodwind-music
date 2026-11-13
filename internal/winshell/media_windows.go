//go:build windows

package winshell

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WinRT ABI definitions follow Microsoft's Windows.Media metadata:
// https://github.com/microsoft/windows-rs/blob/master/crates/libs/windows/src/Windows/Media/mod.rs
// IInspectable occupies vtable slots 0..5; typed event delegates use IUnknown.
var (
	combase                   = windows.NewLazySystemDLL("combase.dll")
	roInitialize              = combase.NewProc("RoInitialize")
	roUninitialize            = combase.NewProc("RoUninitialize")
	roGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
	roActivateInstance        = combase.NewProc("RoActivateInstance")
	windowsCreateString       = combase.NewProc("WindowsCreateString")
	windowsDeleteString       = combase.NewProc("WindowsDeleteString")
	shCreateMemStream         = windows.NewLazySystemDLL("shlwapi.dll").NewProc("SHCreateMemStream")
	createRandomAccessStream  = windows.NewLazySystemDLL("shcore.dll").NewProc("CreateRandomAccessStreamOverStream")
	iidInterop                = windows.GUID{Data1: 0xddb0472d, Data2: 0xc911, Data3: 0x4a1f, Data4: [8]byte{0x86, 0xd9, 0xdc, 0x3d, 0x71, 0xa9, 0x5f, 0x5a}}
	iidControls               = guid("99fa3ff4-1742-42a6-902e-087d41f965ec")
	iidControls2              = guid("ea98d2f6-7f3c-4af2-a586-72889808efb1")
	iidMusic2                 = guid("00368462-97d3-44b9-b00f-008afcefaf18")
	iidTimeline               = guid("5125316a-c3a2-475b-8507-93534dc88f15")
	iidStream                 = guid("905a0fe1-bc53-11df-8c49-001e4fc686da")
	iidStreamReferenceStatics = guid("857309dc-3fbf-4e7d-986f-ef3b1a07a964")
	iidUnknown                = guid("00000000-0000-0000-c000-000000000046")
	iidAgile                  = guid("94ea2b94-e9cc-49e0-c0ff-ee64ca8f5b90")
	// TypedEventHandler<SystemMediaTransportControls, event args>, generated
	// from the WinRT parameterized-interface signatures (UUID v5).
	iidButtonsHandler = guid("0557e996-7b23-5bae-aa81-ea0d671143a4")
	iidSeekHandler    = guid("44e34f15-bdc0-50a7-ace4-39e91fb753f1")
)

func guid(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return g
}
func failed(hr uintptr) bool { return int32(uint32(hr)) < 0 }
func check(operation string, hr uintptr) error {
	if failed(hr) {
		return fmt.Errorf("%s: HRESULT %#08x", operation, uint32(hr))
	}
	return nil
}

type comObject struct{ vtable *[40]uintptr }

//go:uintptrescapes
func (o *comObject) call(slot int, args ...uintptr) uintptr {
	a := make([]uintptr, 1, len(args)+1)
	a[0] = uintptr(unsafe.Pointer(o))
	a = append(a, args...)
	hr, _, _ := syscall.SyscallN(o.vtable[slot], a...)
	runtime.KeepAlive(o)
	return hr
}

func (o *comObject) release() {
	if o != nil {
		o.call(2)
	}
}
func (o *comObject) query(iid *windows.GUID) *comObject {
	var out *comObject
	if failed(o.call(0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))) {
		return nil
	}
	return out
}

func hstring(s string) uintptr {
	units := utf16.Encode([]rune(s))
	var out uintptr
	if len(units) > 0 {
		windowsCreateString.Call(uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)), uintptr(unsafe.Pointer(&out)))
	}
	return out
}

func factory(name string, iid *windows.GUID) (*comObject, error) {
	h := hstring(name)
	defer windowsDeleteString.Call(h)
	var out *comObject
	hr, _, _ := roGetActivationFactory.Call(h, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	return out, check("activate "+name, hr)
}

func activate(name string, iid *windows.GUID) *comObject {
	h := hstring(name)
	defer windowsDeleteString.Call(h)
	var out *comObject
	hr, _, _ := roActivateInstance.Call(h, uintptr(unsafe.Pointer(&out)))
	if failed(hr) {
		return nil
	}
	defer out.release()
	return out.query(iid)
}

func putString(o *comObject, slot int, value string) {
	h := hstring(value)
	defer windowsDeleteString.Call(h)
	o.call(slot, h)
}

type MediaInfo struct {
	Title, Artist, Album string
	Artwork              []byte
	Duration, Position   time.Duration
	Playing              bool
}

var commands struct {
	sync.RWMutex
	fn func(int, time.Duration)
}

func SetCommandHandler(fn func(int, time.Duration)) {
	commands.Lock()
	commands.fn = fn
	commands.Unlock()
}
func command(cmd int, pos time.Duration) {
	commands.RLock()
	fn := commands.fn
	commands.RUnlock()
	if fn != nil {
		fn(cmd, pos)
	}
}

// SetMedia and MoveMedia enqueue all COM work on the shell's MTA thread.
// Without Start they do nothing, so headless app tests have no system effects.
func SetMedia(info MediaInfo) {
	info.Artwork = bytes.Clone(info.Artwork)
	submit(func(s *shell) {
		if s.media != nil {
			s.media.set(info)
		}
	})
}
func MoveMedia(pos time.Duration, playing bool) {
	submit(func(s *shell) {
		if m := s.media; m != nil && m.last != nil {
			m.last.Position, m.last.Playing = pos, playing
			m.position(*m.last)
		}
	})
}
func ClearMedia() {
	submit(func(s *shell) {
		if s.media != nil {
			s.media.clear()
		}
	})
}

type mediaSession struct {
	controls, extended, updater, timeline *comObject
	buttons, seek                         *eventHandler
	buttonToken, seekToken                int64
	buttonSubscribed, seekSubscribed      bool
	last                                  *MediaInfo
}

func openMedia(hwnd uintptr) (_ *mediaSession, err error) {
	f, err := factory("Windows.Media.SystemMediaTransportControls", &iidInterop)
	if err != nil {
		return nil, err
	}
	defer f.release()
	m := &mediaSession{}
	defer func() {
		if err != nil {
			m.close()
		}
	}()
	if err = check("GetForWindow", f.call(6, hwnd, uintptr(unsafe.Pointer(&iidControls)), uintptr(unsafe.Pointer(&m.controls)))); err != nil {
		return nil, err
	}
	if err = check("DisplayUpdater", m.controls.call(8, uintptr(unsafe.Pointer(&m.updater)))); err != nil {
		return nil, err
	}
	for _, slot := range []int{13, 17, 25, 27} { // Play, Pause, Previous, Next
		if err = check("enable media button", m.controls.call(slot, 1)); err != nil {
			return nil, err
		}
	}
	m.buttons = newEvent(iidButtonsHandler, func(args *comObject) {
		var button int32
		if failed(args.call(6, uintptr(unsafe.Pointer(&button)))) {
			return
		}
		switch button {
		case 0:
			command(0, 0) // Play
		case 1, 2:
			command(1, 0) // Pause / Stop
		case 6:
			command(2, 0) // Next
		case 7:
			command(3, 0) // Previous
		}
	})
	if err = check("ButtonPressed", m.controls.call(32, uintptr(unsafe.Pointer(m.buttons)), uintptr(unsafe.Pointer(&m.buttonToken)))); err != nil {
		return nil, err
	}
	m.buttonSubscribed = true
	m.extended = m.controls.query(&iidControls2)
	if m.extended != nil {
		m.timeline = activate("Windows.Media.SystemMediaTransportControlsTimelineProperties", &iidTimeline)
		m.seek = newEvent(iidSeekHandler, func(args *comObject) {
			var ticks int64
			if !failed(args.call(6, uintptr(unsafe.Pointer(&ticks)))) {
				command(4, time.Duration(max(0, min(ticks, int64(1<<63-1)/100)))*100)
			}
		})
		if failed(m.extended.call(13, uintptr(unsafe.Pointer(m.seek)), uintptr(unsafe.Pointer(&m.seekToken)))) {
			m.seek.release()
			m.seek = nil
		} else {
			m.seekSubscribed = true
		}
	}
	return m, nil
}

func (m *mediaSession) set(info MediaInfo) {
	m.controls.call(11, 1) // IsEnabled
	if m.last == nil || info.Title != m.last.Title || info.Artist != m.last.Artist || info.Album != m.last.Album || !bytes.Equal(info.Artwork, m.last.Artwork) {
		m.updater.call(7, 1) // MediaPlaybackType.Music
		var music *comObject
		if !failed(m.updater.call(12, uintptr(unsafe.Pointer(&music)))) && music != nil {
			putString(music, 7, info.Title)
			putString(music, 11, info.Artist)
			if album := music.query(&iidMusic2); album != nil {
				putString(album, 7, info.Album)
				album.release()
			}
			music.release()
		}
		art := artworkStream(info.Artwork)
		m.updater.call(11, uintptr(unsafe.Pointer(art)))
		art.release()
		m.updater.call(17) // Update
	}
	m.last = &info
	m.position(info)
}

func (m *mediaSession) position(info MediaInfo) {
	status := uintptr(4) // Paused
	if info.Playing {
		status = 3
	}
	m.controls.call(7, status)
	if m.timeline != nil && m.extended != nil {
		end := max(info.Duration, 0) / 100
		pos := max(0, min(info.Position, info.Duration)) / 100
		m.timeline.call(7, 0) // StartTime
		m.timeline.call(9, uintptr(end))
		m.timeline.call(11, 0) // MinSeekTime
		m.timeline.call(13, uintptr(end))
		m.timeline.call(15, uintptr(pos))
		m.extended.call(12, uintptr(unsafe.Pointer(m.timeline)))
	}
}

func (m *mediaSession) clear() {
	if m.controls != nil {
		m.controls.call(7, 0)
		m.controls.call(11, 0)
	}
	if m.updater != nil {
		m.updater.call(16)
		m.updater.call(17)
	}
	m.last = nil
}

func (m *mediaSession) close() {
	m.clear()
	if m.buttons != nil {
		if m.buttonSubscribed {
			m.controls.call(33, uintptr(m.buttonToken))
		}
		m.buttons.release()
	}
	if m.seek != nil {
		if m.seekSubscribed {
			m.extended.call(14, uintptr(m.seekToken))
		}
		m.seek.release()
	}
	m.timeline.release()
	m.updater.release()
	m.extended.release()
	m.controls.release()
}

// artworkStream copies the cover into Windows-owned memory. No temporary
// files or asynchronous file permissions are involved.
func artworkStream(data []byte) *comObject {
	if len(data) == 0 {
		return nil
	}
	ptr, _, _ := shCreateMemStream.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	if ptr == 0 {
		return nil
	}
	stream := *(**comObject)(unsafe.Pointer(&ptr))
	defer stream.release()
	var random *comObject
	hr, _, _ := createRandomAccessStream.Call(ptr, 0, uintptr(unsafe.Pointer(&iidStream)), uintptr(unsafe.Pointer(&random)))
	if failed(hr) {
		return nil
	}
	defer random.release()
	f, err := factory("Windows.Storage.Streams.RandomAccessStreamReference", &iidStreamReferenceStatics)
	if err != nil {
		return nil
	}
	defer f.release()
	var ref *comObject
	if failed(f.call(8, uintptr(unsafe.Pointer(random)), uintptr(unsafe.Pointer(&ref)))) {
		return nil
	}
	return ref
}

type eventHandler struct {
	vtable *[4]uintptr
	refs   atomic.Int32
	iid    windows.GUID
	invoke func(*comObject)
}

var (
	eventVtable = [4]uintptr{syscall.NewCallback(eventQuery), syscall.NewCallback(eventAddRef), syscall.NewCallback(eventRelease), syscall.NewCallback(eventInvoke)}
	eventsMu    sync.Mutex
	events      = map[*eventHandler]struct{}{}
)

func newEvent(iid windows.GUID, fn func(*comObject)) *eventHandler {
	h := &eventHandler{vtable: &eventVtable, iid: iid, invoke: fn}
	h.refs.Store(1)
	eventsMu.Lock()
	events[h] = struct{}{}
	eventsMu.Unlock()
	return h
}
func eventQuery(h *eventHandler, iid *windows.GUID, out *unsafe.Pointer) uintptr {
	if *iid == iidUnknown || *iid == iidAgile || *iid == h.iid {
		*out = unsafe.Pointer(h)
		eventAddRef(h)
		return 0
	}
	*out = nil
	return 0x80004002 // E_NOINTERFACE
}
func eventAddRef(h *eventHandler) uintptr { return uintptr(h.refs.Add(1)) }
func eventRelease(h *eventHandler) uintptr {
	n := h.refs.Add(-1)
	if n == 0 {
		eventsMu.Lock()
		delete(events, h)
		eventsMu.Unlock()
	}
	return uintptr(n)
}
func (h *eventHandler) release() { eventRelease(h) }
func eventInvoke(h *eventHandler, _ uintptr, args *comObject) uintptr {
	if args != nil {
		h.invoke(args)
	}
	return 0
}
