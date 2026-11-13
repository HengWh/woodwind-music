//go:build windows

// Package winshell supplies the Windows desktop services not yet provided by
// NSGo: a notification-area icon and a media session that outlive a hidden UI.
package winshell

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	shell32          = windows.NewLazySystemDLL("shell32.dll")
	registerClass    = user32.NewProc("RegisterClassExW")
	createWindow     = user32.NewProc("CreateWindowExW")
	defWindowProc    = user32.NewProc("DefWindowProcW")
	destroyWindow    = user32.NewProc("DestroyWindow")
	getMessage       = user32.NewProc("GetMessageW")
	translateMessage = user32.NewProc("TranslateMessage")
	dispatchMessage  = user32.NewProc("DispatchMessageW")
	postMessage      = user32.NewProc("PostMessageW")
	postQuitMessage  = user32.NewProc("PostQuitMessage")
	showWindow       = user32.NewProc("ShowWindow")
	showWindowAsync  = user32.NewProc("ShowWindowAsync")
	isIconic         = user32.NewProc("IsIconic")
	setForeground    = user32.NewProc("SetForegroundWindow")
	getModuleHandle  = kernel32.NewProc("GetModuleHandleW")
	notifyIcon       = shell32.NewProc("Shell_NotifyIconW")
	extractIcon      = shell32.NewProc("ExtractIconExW")
	destroyIcon      = user32.NewProc("DestroyIcon")
	loadIcon         = user32.NewProc("LoadIconW")
	createPopupMenu  = user32.NewProc("CreatePopupMenu")
	appendMenu       = user32.NewProc("AppendMenuW")
	trackPopupMenu   = user32.NewProc("TrackPopupMenu")
	destroyMenu      = user32.NewProc("DestroyMenu")
	getCursorPos     = user32.NewProc("GetCursorPos")
	enumWindows      = user32.NewProc("EnumWindows")
	getWindowPID     = user32.NewProc("GetWindowThreadProcessId")
	getClassName     = user32.NewProc("GetClassNameW")
	setWindowLong    = user32.NewProc("SetWindowLongPtrW")
	getWindowLong    = user32.NewProc("GetWindowLongPtrW")
	callWindowProc   = user32.NewProc("CallWindowProcW")
	registerMessage  = user32.NewProc("RegisterWindowMessageW")
	setAppID         = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
)

const (
	wmClose   = 0x0010
	wmDestroy = 0x0002
	wmWork    = 0x8001
	wmTray    = 0x8002
)

type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}

type point struct{ X, Y int32 }
type message struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}

type iconData struct {
	Size                uint32
	Window              uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Timeout             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                windows.GUID
	BalloonIcon         uintptr
}

type shell struct {
	appID                   string
	hwnd                    uintptr
	done                    chan struct{}
	mu                      sync.Mutex
	jobs                    []func()
	stopping                bool
	tray                    iconData
	icon                    uintptr
	onClick, onMenu, onHide func()
	taskbarCreated          uint32
	media                   *mediaSession
}

var (
	current        atomic.Pointer[shell]
	lifecycle      sync.Mutex
	mainWindow     atomic.Uintptr
	mainProc       atomic.Uintptr
	allowClose     atomic.Bool
	trayPresent    atomic.Bool
	windowCallback = syscall.NewCallback(shellProc)
	mainCallback   = syscall.NewCallback(mainWindowProc)
)

func wide(s string) *uint16 { p, _ := windows.UTF16PtrFromString(s); return p }

// Start creates a separate native message loop; no app or test window is needed.
func Start(appID string) error {
	lifecycle.Lock()
	defer lifecycle.Unlock()
	if current.Load() != nil {
		return nil
	}
	s := &shell{appID: appID, done: make(chan struct{})}
	ready := make(chan error, 1)
	go s.run(ready)
	return <-ready
}

func (s *shell) run(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(s.done)
	if s.appID != "" {
		setAppID.Call(uintptr(unsafe.Pointer(wide(s.appID))))
	}
	instance, _, _ := getModuleHandle.Call(0)
	wc := windowClass{Proc: windowCallback, Instance: instance, ClassName: wide("WoodwindMusicShell")}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if atom, _, err := registerClass.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 && err != syscall.Errno(1410) {
		ready <- fmt.Errorf("register shell window: %w", err)
		return
	}
	// A hidden top-level window, rather than HWND_MESSAGE, receives Explorer's
	// TaskbarCreated broadcast and is a valid SMTC interop owner.
	hwnd, _, err := createWindow.Call(0x80, uintptr(unsafe.Pointer(wc.ClassName)), uintptr(unsafe.Pointer(wide("Woodwind Music"))), 0, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		ready <- fmt.Errorf("create shell window: %w", err)
		return
	}
	s.hwnd = hwnd
	taskbar, _, _ := registerMessage.Call(uintptr(unsafe.Pointer(wide("TaskbarCreated"))))
	s.taskbarCreated = uint32(taskbar)
	current.Store(s)
	allowClose.Store(false)
	if hr, _, _ := roInitialize.Call(1); !failed(hr) {
		defer roUninitialize.Call()
		s.media, err = openMedia(hwnd)
		if err != nil {
			log.Printf("Windows media controls: %v", err)
		}
	} else {
		log.Printf("Windows media controls: %v", check("RoInitialize", hr))
	}
	ready <- nil
	defer func() {
		if s.media != nil {
			s.media.close()
		}
		s.removeTray()
		if hwnd := mainWindow.Swap(0); hwnd != 0 {
			setWindowLong.Call(hwnd, ^uintptr(3), mainProc.Load())
		}
		current.Store(nil)
	}()
	var msg message
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func submit(fn func(*shell)) {
	s := current.Load()
	if s == nil {
		return
	}
	s.mu.Lock()
	if !s.stopping {
		s.jobs = append(s.jobs, func() { fn(s) })
	}
	s.mu.Unlock()
	postMessage.Call(s.hwnd, wmWork, 0, 0)
}

// Stop releases the media session and icon before process exit.
func Stop() {
	lifecycle.Lock()
	defer lifecycle.Unlock()
	s := current.Load()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	postMessage.Call(s.hwnd, wmClose, 0, 0)
	<-s.done
}

func shellProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	s := current.Load()
	if s != nil && s.hwnd == hwnd {
		switch msg {
		case wmWork:
			s.mu.Lock()
			jobs := s.jobs
			s.jobs = nil
			s.mu.Unlock()
			for _, job := range jobs {
				job()
			}
			return 0
		case wmTray:
			switch uint32(lp) & 0xffff {
			case 0x0202, 0x0203, 0x0400, 0x0401: // primary click / keyboard selection
				if s.onClick != nil {
					s.onClick()
				}
			case 0x0205, 0x007b: // secondary click / keyboard context menu
				if s.onMenu != nil {
					s.onMenu()
				}
			}
			return 0
		case wmDestroy:
			postQuitMessage.Call(0)
			return 0
		}
		if msg == s.taskbarCreated {
			s.addTray()
			if trayPresent.Load() {
				s.attachMain()
			}
			return 0
		}
	}
	r, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wp, lp)
	return r
}

// ConfigureTray runs callbacks off the UI thread; callers must post UI work.
func ConfigureTray(name string, onClick, onMenu, onHide func()) {
	submit(func(s *shell) {
		s.onClick, s.onMenu, s.onHide = onClick, onMenu, onHide
		if s.icon == 0 {
			exe, _ := os.Executable()
			extractIcon.Call(uintptr(unsafe.Pointer(wide(exe))), 0, 0, uintptr(unsafe.Pointer(&s.icon)), 1)
		}
		icon := s.icon
		if icon == 0 {
			icon, _, _ = loadIcon.Call(0, 32512)
		}
		s.tray = iconData{Window: s.hwnd, ID: 1, Flags: 1 | 2 | 4 | 0x80, Callback: wmTray, Icon: icon}
		s.tray.Size = uint32(unsafe.Sizeof(s.tray))
		copy(s.tray.Tip[:127], syscall.StringToUTF16(name))
		s.addTray()
		if trayPresent.Load() {
			s.attachMain()
		}
	})
}

func (s *shell) addTray() {
	if s.tray.Icon == 0 {
		return
	}
	r, _, _ := notifyIcon.Call(0, uintptr(unsafe.Pointer(&s.tray)))
	if r == 0 {
		r, _, _ = notifyIcon.Call(1, uintptr(unsafe.Pointer(&s.tray)))
	}
	trayPresent.Store(r != 0)
	if r == 0 {
		log.Print("Windows notification icon unavailable; closing the window will exit")
	} else {
		s.tray.Timeout = 4 // NOTIFYICON_VERSION_4: keyboard and context menu events.
		notifyIcon.Call(4, uintptr(unsafe.Pointer(&s.tray)))
		s.tray.Timeout = 0
	}
}

func (s *shell) removeTray() {
	trayPresent.Store(false)
	if s.tray.Icon != 0 {
		notifyIcon.Call(2, uintptr(unsafe.Pointer(&s.tray)))
		s.tray.Icon = 0
	}
	if s.icon != 0 {
		destroyIcon.Call(s.icon)
		s.icon = 0
	}
}

func RemoveTray() { submit(func(s *shell) { s.removeTray() }) }

func (s *shell) attachMain() {
	if mainWindow.Load() != 0 {
		return
	}
	enumWindows.Call(syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var pid uint32
		getWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != uint32(os.Getpid()) {
			return 1
		}
		var name [128]uint16
		getClassName.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		if syscall.UTF16ToString(name[:]) != "NsgoWindow" {
			return 1
		}
		// GWL_WNDPROC. Keep the prior procedure for every message we don't own.
		old, _, _ := getWindowLong.Call(hwnd, ^uintptr(3))
		mainProc.Store(old)
		old, _, _ = setWindowLong.Call(hwnd, ^uintptr(3), mainCallback)
		if old != 0 {
			mainProc.Store(old)
			mainWindow.Store(hwnd)
		}
		return 0
	}), 0)
}

func mainWindowProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	if msg == wmClose && trayPresent.Load() && !allowClose.Load() {
		showWindow.Call(hwnd, 0)
		if s := current.Load(); s != nil && s.onHide != nil {
			s.onHide()
		}
		return 0
	}
	r, _, _ := callWindowProc.Call(mainProc.Load(), hwnd, uintptr(msg), wp, lp)
	return r
}

// AllowClose distinguishes an explicit Quit from the close-to-tray button.
func AllowClose() { allowClose.Store(true) }

func RestoreMain() bool {
	hwnd := mainWindow.Load()
	if hwnd == 0 {
		return false
	}
	cmd := uintptr(5) // SW_SHOW preserves a maximized window.
	if r, _, _ := isIconic.Call(hwnd); r != 0 {
		cmd = 9
	}
	showWindowAsync.Call(hwnd, cmd)
	setForeground.Call(hwnd)
	return true
}

type MenuItem struct {
	Label     string
	Separator bool
	OnSelect  func()
}

func ShowMenu(items []MenuItem) {
	submit(func(s *shell) {
		menu, _, _ := createPopupMenu.Call()
		if menu == 0 {
			return
		}
		defer destroyMenu.Call(menu)
		for i, item := range items {
			flags := uintptr(0)
			if item.Separator {
				flags = 0x800
			}
			appendMenu.Call(menu, flags, uintptr(i+1), uintptr(unsafe.Pointer(wide(item.Label))))
		}
		var at point
		getCursorPos.Call(uintptr(unsafe.Pointer(&at)))
		setForeground.Call(s.hwnd)
		selected, _, _ := trackPopupMenu.Call(menu, 0x100|0x2, uintptr(at.X), uintptr(at.Y), 0, s.hwnd, 0)
		postMessage.Call(s.hwnd, 0, 0, 0)
		if selected > 0 && selected <= uintptr(len(items)) {
			if fn := items[selected-1].OnSelect; fn != nil {
				fn()
			}
		}
	})
}
