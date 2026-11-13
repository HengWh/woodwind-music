//go:build windows

package winshell

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

type buttonArgs struct {
	vtable *[7]uintptr
	value  int32
}

var buttonVtable = [7]uintptr{6: syscall.NewCallback(func(a *buttonArgs, out *int32) uintptr { *out = a.value; return 0 })}

func TestEventDelegateCOMLifetime(t *testing.T) {
	got := int32(-1)
	h := newEvent(iidButtonsHandler, func(args *comObject) { args.call(6, uintptr(unsafe.Pointer(&got))) })
	obj := (*comObject)(unsafe.Pointer(h))
	queried := obj.query(&iidButtonsHandler)
	if queried == nil {
		t.Fatal("the typed event interface was refused")
	}
	queried.release()
	agile := obj.query(&iidAgile)
	if agile == nil {
		t.Fatal("worker-thread events require an agile delegate")
	}
	agile.release()
	args := &buttonArgs{vtable: &buttonVtable, value: 6}
	if hr := obj.call(3, 0, uintptr(unsafe.Pointer(args))); failed(hr) || got != 6 {
		t.Fatalf("native event invocation: HRESULT %#x, button %d", hr, got)
	}
	h.release()
	eventsMu.Lock()
	_, alive := events[h]
	eventsMu.Unlock()
	if alive {
		t.Fatal("released event delegate remained rooted")
	}
}

// This test intentionally needs a signed-in Windows desktop. It checks real
// COM activation, native metadata/cover/timeline updates, commands and Clear;
// it does not need audio hardware and does not play sound.
func TestNativeDesktopMedia(t *testing.T) {
	if os.Getenv("WOODWIND_WINDOWS_NATIVE") != "1" {
		t.Skip("set WOODWIND_WINDOWS_NATIVE=1 on a signed-in Windows desktop")
	}
	if err := Start("dev.oneo.nsgo.music.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Stop)
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	SetMedia(MediaInfo{Title: "音乐 #100%", Artist: "Artist", Album: "Album", Artwork: picture.Bytes(), Duration: 3 * time.Minute, Position: 42 * time.Second, Playing: true})
	result := make(chan error, 1)
	submit(func(s *shell) {
		if s.media == nil {
			result <- os.ErrNotExist
			return
		}
		m := s.media
		var status int32
		if err := check("read playback status", m.controls.call(6, uintptr(unsafe.Pointer(&status)))); err != nil {
			result <- err
			return
		}
		if status != 3 {
			result <- os.ErrInvalid
			return
		}
		var art *comObject
		if err := check("read thumbnail", m.updater.call(10, uintptr(unsafe.Pointer(&art)))); err != nil {
			result <- err
			return
		}
		if art == nil {
			result <- os.ErrInvalid
			return
		}
		art.release()
		if m.timeline == nil || m.seek == nil {
			result <- os.ErrNotExist
			return
		}
		var pos int64
		if err := check("read position", m.timeline.call(14, uintptr(unsafe.Pointer(&pos)))); err != nil {
			result <- err
			return
		}
		if pos != int64(42*time.Second/100) {
			result <- os.ErrInvalid
			return
		}
		result <- nil
	})
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("native media metadata: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native media did not respond")
	}
	pressed := make(chan int, 1)
	SetCommandHandler(func(cmd int, _ time.Duration) { pressed <- cmd })
	t.Cleanup(func() { SetCommandHandler(nil) })
	submit(func(s *shell) {
		args := &buttonArgs{vtable: &buttonVtable, value: 6}
		(*comObject)(unsafe.Pointer(s.media.buttons)).call(3, 0, uintptr(unsafe.Pointer(args)))
	})
	select {
	case cmd := <-pressed:
		if cmd != 2 {
			t.Fatalf("Next mapped to command %d", cmd)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("media button did not reach the player")
	}
	ClearMedia()
	submit(func(s *shell) {
		var enabled byte
		err := check("read IsEnabled", s.media.controls.call(10, uintptr(unsafe.Pointer(&enabled))))
		if enabled != 0 {
			err = os.ErrInvalid
		}
		result <- err
	})
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("Clear did not disable controls: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Clear did not respond")
	}
}
