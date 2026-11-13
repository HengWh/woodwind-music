package app

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/music"
)

// Simulate Windows even on the Mac development host: settings must remain
// usable when the backend has a main surface but no secondary windows.
func TestWindowsSettingsStayInMainWindow(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		reviewTrack(a)
		a.closeNowPlaying()
		a.open(route{page: pageLiked})
		return a.view(cx)
	}, nsgotest.Size(880, 580), nsgotest.Environment(core.Environment{
		OS: core.OSWindows, TitleBar: core.Edges{Top: 40, Right: 100},
	}))
	for _, close := range []string{"back", "escape", "mod+w"} {
		ui.Press("id:open-settings")
		ui.WaitUntil(time.Second, func() bool { return a.settingsInMain })
		if a.settingsWin != nil {
			t.Fatal("Windows settings opened an unsupported secondary window")
		}
		ui.Key("mod+f")
		if !ui.Find("id:settings-search").Focused {
			t.Fatal("Find did not focus settings search")
		}
		ui.Press("id:settings-nav-playback")
		ui.Press("id:settings-normalize")
		if close == "back" {
			ui.Press("id:settings-back")
		} else {
			ui.Key(close)
		}
		ui.WaitUntil(time.Second, func() bool { return !a.settingsInMain })
		if a.route.GetUntracked().page != pageLiked || a.pl.Current().ID != "review" {
			t.Fatal("closing settings lost the library route or playback queue")
		}
	}
}

func TestLocalCoverUsesLoadableSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "音乐 #100%.png")
	u := music.FileURL(path)
	want := u
	if runtime.GOOS == "windows" {
		want = path
	}
	if got := imageSource(u).URI; got != want {
		t.Fatalf("image source = %q, want %q", got, want)
	}
	remote := "https://example.com/cover.png"
	if got := imageSource(remote).URI; got != remote {
		t.Fatalf("remote source changed: %q", got)
	}
}
