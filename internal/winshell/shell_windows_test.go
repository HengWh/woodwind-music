//go:build windows

package winshell

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

var testWindowProc = syscall.NewCallback(func(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	r, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wp, lp)
	return r
})

// Exercise the native close-to-tray hook against a throwaway window with the
// same class as NSGo, including explicit exit and Explorer icon recreation.
func TestNativeTrayWindowLifetime(t *testing.T) {
	if os.Getenv("WOOD_WIND_WINDOWS_NATIVE") != "1" {
		t.Skip("set WOOD_WIND_WINDOWS_NATIVE=1 on a signed-in Windows desktop")
	}
	if err := Start("dev.oneo.nsgo.music.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(Stop)
	result := make(chan error, 1)
	var hwnd uintptr
	submit(func(s *shell) {
		instance, _, _ := getModuleHandle.Call(0)
		wc := windowClass{Instance: instance, Proc: testWindowProc, ClassName: wide("NsgoWindow")}
		wc.Size = uint32(unsafe.Sizeof(wc))
		registerClass.Call(uintptr(unsafe.Pointer(&wc)))
		var err error
		hwnd, _, err = createWindow.Call(0, uintptr(unsafe.Pointer(wc.ClassName)), uintptr(unsafe.Pointer(wide("Woodwind tray test"))), 0x00cf0000, 0, 0, 400, 300, 0, 0, instance, 0)
		if hwnd == 0 {
			result <- fmt.Errorf("test window: %w", err)
		} else {
			result <- nil
		}
	})
	wait := func() {
		t.Helper()
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("native shell did not respond")
		}
	}
	wait()
	t.Cleanup(func() { destroyWindow.Call(hwnd) })
	hidden := make(chan struct{}, 1)
	ConfigureTray("Woodwind test", func() {}, func() {}, func() { hidden <- struct{}{} })
	submit(func(s *shell) {
		if !trayPresent.Load() || mainWindow.Load() != hwnd {
			result <- fmt.Errorf("tray icon or main window hook missing")
			return
		}
		// Explorer's restart drops the icon without destroying our process.
		notifyIcon.Call(2, uintptr(unsafe.Pointer(&s.tray)))
		shellProc(s.hwnd, s.taskbarCreated, 0, 0)
		if !trayPresent.Load() {
			result <- fmt.Errorf("Explorer restart did not recreate the tray icon")
			return
		}
		postMessage.Call(hwnd, wmClose, 0, 0)
		result <- nil
	})
	wait()
	select {
	case <-hidden:
	case <-time.After(5 * time.Second):
		t.Fatal("close was not redirected to the tray")
	}
	submit(func(s *shell) {
		exists, _, _ := user32.NewProc("IsWindow").Call(hwnd)
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
		if exists == 0 || visible != 0 {
			result <- fmt.Errorf("close destroyed the window or left it visible")
			return
		}
		if !RestoreMain() {
			result <- fmt.Errorf("tray could not restore the window")
			return
		}
		AllowClose()
		user32.NewProc("SendMessageW").Call(hwnd, wmClose, 0, 0)
		exists, _, _ = user32.NewProc("IsWindow").Call(hwnd)
		if exists != 0 {
			result <- fmt.Errorf("explicit Quit was still intercepted")
			return
		}
		result <- nil
	})
	wait()
}
