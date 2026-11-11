//go:build darwin

package nowplaying

import (
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	once     sync.Once
	lib      uintptr
	center   objc.ID
	commands objc.ID
	props    map[string]objc.ID
	sels     = map[string]objc.SEL{}
	selsMu   sync.Mutex

	mu   sync.Mutex
	last *Info
)

func sel(name string) objc.SEL {
	selsMu.Lock()
	defer selsMu.Unlock()
	s, ok := sels[name]
	if !ok {
		s = objc.RegisterName(name)
		sels[name] = s
	}
	return s
}

func class(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

func send(id objc.ID, name string, args ...any) objc.ID {
	return objc.Send[objc.ID](id, sel(name), args...)
}

func sendBool(id objc.ID, name string, v bool) {
	var a uintptr
	if v {
		a = 1
	}
	objc.Send[struct{}](id, sel(name), a)
}

func nsString(s string) objc.ID {
	b := unsafe.StringData(s)
	str := send(send(class("NSString"), "alloc"), "initWithBytes:length:encoding:", uintptr(unsafe.Pointer(b)), uintptr(len(s)), uintptr(4))
	runtime.KeepAlive(s)
	if str == 0 {
		return 0
	}
	return send(str, "autorelease")
}

func nsData(b []byte) objc.ID {
	var p unsafe.Pointer
	if len(b) > 0 {
		p = unsafe.Pointer(&b[0])
	}
	d := send(class("NSData"), "dataWithBytes:length:", uintptr(p), uintptr(len(b)))
	runtime.KeepAlive(b)
	return d
}

func nsNumber(v float64) objc.ID { return send(class("NSNumber"), "numberWithDouble:", v) }

func withPool(fn func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := send(class("NSAutoreleasePool"), "new")
	fn()
	if pool != 0 {
		send(pool, "drain")
	}
}

// load opens MediaPlayer and the remote commands, once.
func load() bool {
	once.Do(func() {
		l, err := purego.Dlopen("/System/Library/Frameworks/MediaPlayer.framework/MediaPlayer", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if err != nil {
			return
		}
		lib = l
		props = map[string]objc.ID{}
		for _, name := range []string{
			"MPMediaItemPropertyTitle", "MPMediaItemPropertyArtist", "MPMediaItemPropertyAlbumTitle",
			"MPMediaItemPropertyPlaybackDuration", "MPMediaItemPropertyArtwork",
			"MPNowPlayingInfoPropertyElapsedPlaybackTime", "MPNowPlayingInfoPropertyPlaybackRate",
			"MPNowPlayingInfoPropertyDefaultPlaybackRate", "MPNowPlayingInfoPropertyMediaType",
		} {
			if p, err := purego.Dlsym(l, name); err == nil {
				props[name] = **(**objc.ID)(unsafe.Pointer(&p))
			}
		}
		withPool(func() {
			center = send(class("MPNowPlayingInfoCenter"), "defaultCenter")
			commands = send(class("MPRemoteCommandCenter"), "sharedCommandCenter")
		})
		if center == 0 || commands == 0 {
			return
		}
		register()
	})
	return center != 0
}

var remote = []struct {
	getter string
	cmd    Command
}{
	{"playCommand", Play},
	{"pauseCommand", Pause},
	{"togglePlayPauseCommand", Play},
	{"nextTrackCommand", Next},
	{"previousTrackCommand", Previous},
	{"changePlaybackPositionCommand", Seek},
}

// register adds a handler to each remote command, and enables the ones the
// app does.
func register() {
	withPool(func() {
		for _, r := range remote {
			cmd := r.cmd
			handler := objc.NewBlock(func(_ objc.Block, event objc.ID) int {
				var pos time.Duration
				if cmd == Seek {
					pos = time.Duration(objc.Send[float64](event, sel("positionTime")) * float64(time.Second))
				}
				fn := OnCommand
				if fn != nil {
					fn(cmd, pos)
				}
				return 0 // MPRemoteCommandHandlerStatusSuccess
			})
			objc.Send[objc.ID](send(commands, r.getter), sel("addTargetWithHandler:"), uintptr(handler))
			handler.Release()
			sendBool(send(commands, r.getter), "setEnabled:", true)
		}
	})
}

// Set shows info where the system shows media.
func Set(info Info) {
	if !load() {
		return
	}
	mu.Lock()
	c := info
	last = &c
	mu.Unlock()
	apply(info)
}

// Position moves what the system shows without changing anything else.
func Position(pos time.Duration, playing bool) {
	mu.Lock()
	if last == nil {
		mu.Unlock()
		return
	}
	last.Position, last.Playing = pos, playing
	info := *last
	mu.Unlock()
	apply(info)
}

// Clear removes what the system shows.
func Clear() {
	if !load() {
		return
	}
	mu.Lock()
	last = nil
	mu.Unlock()
	withPool(func() {
		send(center, "setNowPlayingInfo:", uintptr(0))
		send(center, "setPlaybackState:", uintptr(3)) // stopped
	})
}

// apply sends info to MPNowPlayingInfoCenter.
func apply(info Info) {
	const (
		playing = 1
		paused  = 2
	)
	withPool(func() {
		d := send(class("NSMutableDictionary"), "dictionary")
		put := func(key string, v objc.ID) {
			if k := props[key]; k != 0 && v != 0 {
				send(d, "setObject:forKey:", v, k)
			}
		}
		if info.Title != "" {
			put("MPMediaItemPropertyTitle", nsString(info.Title))
		}
		if info.Artist != "" {
			put("MPMediaItemPropertyArtist", nsString(info.Artist))
		}
		if info.Album != "" {
			put("MPMediaItemPropertyAlbumTitle", nsString(info.Album))
		}
		if info.Duration > 0 {
			put("MPMediaItemPropertyPlaybackDuration", nsNumber(info.Duration.Seconds()))
		}
		put("MPNowPlayingInfoPropertyMediaType", nsNumber(1))
		put("MPNowPlayingInfoPropertyElapsedPlaybackTime", nsNumber(info.Position.Seconds()))
		put("MPNowPlayingInfoPropertyDefaultPlaybackRate", nsNumber(1))
		rate, state := 0.0, paused
		if info.Playing {
			rate, state = 1, playing
		}
		put("MPNowPlayingInfoPropertyPlaybackRate", nsNumber(rate))
		if len(info.Artwork) > 0 {
			img := send(send(class("NSImage"), "alloc"), "initWithData:", nsData(info.Artwork))
			if img != 0 {
				art := send(send(class("MPMediaItemArtwork"), "alloc"), "initWithImage:", img)
				send(img, "release")
				if art != 0 {
					put("MPMediaItemPropertyArtwork", art)
					send(art, "release")
				}
			}
		}
		send(center, "setNowPlayingInfo:", d)
		send(center, "setPlaybackState:", uintptr(state))
	})
}
