package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/desktoplyrics"
	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/lyrics"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/player"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestDesktopLyricsFollowsPlaybackAndTrackChanges(t *testing.T) {
	p := player.New(func(fn func()) { fn() })
	t.Cleanup(p.Close)
	a := &app{pl: p, set: defaultSettings()}
	if f := a.desktopLyricsFrame(); f.Words[0].Text != i18n.T("Not Playing") {
		t.Fatal("missing idle state")
	}
	p.Cue([]music.Track{{ID: "one", Name: "First"}})
	if f := a.desktopLyricsFrame(); f.Words[0].Text != "First" || f.Secondary != i18n.T("Loading…") {
		t.Fatal("loading lyrics displayed stale text")
	}
	lines := []lyrics.Line{
		{At: time.Second, End: 3 * time.Second, Text: "你好", Translation: "Hello", Words: []lyrics.Word{{Text: "你", Start: time.Second, End: 2 * time.Second}, {Text: "好", Start: 2 * time.Second, End: 3 * time.Second}}},
		{At: 5 * time.Second, Text: "再见", Words: []lyrics.Word{{Text: "再见", Start: 5 * time.Second, End: 5 * time.Second}}},
	}
	p.SetLyrics(lines)
	p.Hold(0)
	if f := a.desktopLyricsFrame(); f.Words[0].Text != "First" || f.Secondary != "你好" {
		t.Fatal("intro did not preview first line")
	}
	p.Hold(1500 * time.Millisecond)
	f := a.desktopLyricsFrame()
	if f.Words[0].Progress != .5 || f.Words[1].Progress != 0 || f.Secondary != "Hello" {
		t.Fatalf("word timing or translation: %+v", f)
	}
	a.set.DesktopLyrics.Translation = false
	if f := a.desktopLyricsFrame(); f.Secondary != "再见" {
		t.Fatal("translation toggle did not reveal next line")
	}
	p.Hold(5 * time.Second)
	if f := a.desktopLyricsFrame(); len(f.Words) != 1 || f.Words[0].Text != "再见" || f.Words[0].Progress != 1 || f.Secondary != "" {
		t.Fatalf("seek or zero-length word: %+v", f)
	}
	p.Hold(time.Second)
	if f := a.desktopLyricsFrame(); f.Words[0].Text != "你" || f.Words[0].Progress != 0 {
		t.Fatal("backward seek retained old word progress")
	}
	p.Cue([]music.Track{{ID: "two", Name: "Second"}})
	p.SetLyrics(nil)
	if f := a.desktopLyricsFrame(); f.Words[0].Text != "Second" || f.Secondary != i18n.T("No lyrics. Enjoy the music.") {
		t.Fatal("track change retained previous lyrics")
	}
}

type fakeLyricWindow struct {
	frames []desktoplyrics.Frame
	events []desktoplyrics.Event
	closed bool
}

func (w *fakeLyricWindow) Update(f desktoplyrics.Frame) { w.frames = append(w.frames, f) }
func (w *fakeLyricWindow) Poll() []desktoplyrics.Event  { e := w.events; w.events = nil; return e }
func (w *fakeLyricWindow) Close()                       { w.closed = true }

func TestDesktopLyricsWindowLifetimeAndControls(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", dir)
	var a *app
	var windows []*fakeLyricWindow
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		a.openLyricWindow = func() (desktoplyrics.Window, error) {
			w := &fakeLyricWindow{}
			windows = append(windows, w)
			return w, nil
		}
		a.pl.Cue([]music.Track{{ID: "one", Name: "First"}})
		a.pl.SetLyrics(lyrics.Build("[00:00]你好\n[00:05]再见", ""))
		a.pl.Hold(time.Second)
		return a.view(cx)
	}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Locale: "en"}))
	ui.Press("id:bar-desktop-lyrics")
	ui.WaitUntil(time.Second, func() bool { return a.lyricWindow != nil })
	w := windows[0]
	ui.Press("text:Liked Songs")
	a.post(a.bump)
	ui.Settle()
	ui.Advance(250 * time.Millisecond)
	if len(windows) != 1 || w.closed || len(w.frames) < 2 {
		t.Fatal("view rebuild ended lyrics window or playback clock")
	}
	w.events = []desktoplyrics.Event{{Action: desktoplyrics.Lock}, {Moved: true, X: 123, Y: 456}}
	ui.Advance(250 * time.Millisecond)
	if !a.set.DesktopLyrics.Locked || !a.set.DesktopLyrics.Positioned || a.set.DesktopLyrics.X != 123 || a.set.DesktopLyrics.PositionUnits != "points" || !w.frames[len(w.frames)-1].Locked {
		t.Fatal("lock or drag did not reach settings and native window")
	}
	saved := loadSettings(dir)
	if saved.DesktopLyrics != a.set.DesktopLyrics {
		t.Fatal("desktop lyric state did not persist")
	}
	a.post(a.toggleDesktopLyricsLock)
	ui.Settle()
	if w.frames[len(w.frames)-1].Locked {
		t.Fatal("external unlock did not release click-through")
	}
	w.events = []desktoplyrics.Event{{Action: desktoplyrics.TogglePlay}}
	ui.Advance(250 * time.Millisecond)
	if a.pl.Playing() {
		t.Fatal("floating play/pause control did not pause")
	}
	w.events = []desktoplyrics.Event{{Action: desktoplyrics.Hide}}
	ui.Advance(300 * time.Millisecond)
	if !w.closed || a.lyricWindow != nil || a.set.DesktopLyrics.Enabled {
		t.Fatal("close did not disable and release lyrics")
	}
	count := len(w.frames)
	ui.Advance(time.Second)
	if len(w.frames) != count {
		t.Fatal("closed lyrics still receive timer frames")
	}
	ui.Key("mod+shift+l")
	ui.WaitUntil(time.Second, func() bool { return len(windows) == 2 })
	if windows[1].frames[0].X != 123 || !windows[1].frames[0].Positioned {
		t.Fatal("reopening lost saved position")
	}
	a.post(a.closeDesktopLyrics)
	ui.Settle()
	if !windows[1].closed {
		t.Fatal("app cleanup did not close native lyrics")
	}
}

func TestDesktopLyricsFailureAndAndroidBoundary(t *testing.T) {
	for _, platform := range []core.OS{core.OSWindows, core.OSAndroid} {
		t.Run(string(platform), func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
			var a *app
			calls := 0
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a = newApp(cx)
				a.openLyricWindow = func() (desktoplyrics.Window, error) { calls++; return nil, errors.New("unavailable") }
				return a.view(cx)
			}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{OS: platform}))
			a.post(func() { a.setDesktopLyricsEnabled(true) })
			ui.Settle()
			if a.set.DesktopLyrics.Enabled || a.lyricWindow != nil {
				t.Fatal("failed/unsupported native window remained enabled")
			}
			if platform == core.OSAndroid {
				if calls != 0 || ui.Exists("id:bar-desktop-lyrics") {
					t.Fatal("Android invoked desktop native window")
				}
			} else if calls != 1 {
				t.Fatal("Windows did not attempt independent native window")
			}
		})
	}
}

func TestDesktopLyricsSettingsMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"volume":0.6}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := loadSettings(dir)
	if s.Volume != .6 || s.DesktopLyrics != defaultDesktopLyricsSettings() {
		t.Fatal("old settings lost defaults")
	}
	if err := os.WriteFile(path, []byte(`{"desktopLyrics":{"enabled":true,"locked":true,"fontSize":900,"color":"unknown","x":10,"y":20,"positioned":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s = loadSettings(dir)
	if s.DesktopLyrics.FontSize != 36 || s.DesktopLyrics.Color != "gold" || !s.DesktopLyrics.Enabled || !s.DesktopLyrics.Locked || s.DesktopLyrics.X != 10 {
		t.Fatal("invalid style was not normalized independently of saved window state")
	}
}

func TestDesktopLyricsRestoresAndSettingsApplyLive(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", dir)
	s := defaultSettings()
	s.path = filepath.Join(dir, "settings.json")
	s.DesktopLyrics.Enabled, s.DesktopLyrics.Locked = true, true
	s.DesktopLyrics.FontSize, s.DesktopLyrics.Color = 42, "pink"
	s.DesktopLyrics.X, s.DesktopLyrics.Y, s.DesktopLyrics.Positioned = 123, 456, true
	s.save()
	var a *app
	var windows []*fakeLyricWindow
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		a.openLyricWindow = func() (desktoplyrics.Window, error) {
			w := &fakeLyricWindow{}
			windows = append(windows, w)
			return w, nil
		}
		a.settingsPage = "playback"
		return a.settingsView(cx)
	}, nsgotest.Size(780, 1000), nsgotest.Environment(core.Environment{OS: core.OSWindows, Locale: "en"}))
	ui.WaitUntil(time.Second, func() bool { return len(windows) == 1 })
	f := windows[0].frames[0]
	if !f.Locked || f.FontSize != 42 || f.Color != 0xff92cd || f.X != 123 || !f.Positioned || !f.LegacyPixels {
		t.Fatal("startup did not restore saved desktop lyrics")
	}
	windows[0].events = []desktoplyrics.Event{{Moved: true, X: 61, Y: 228}}
	ui.Advance(250 * time.Millisecond)
	f = windows[0].frames[len(windows[0].frames)-1]
	if f.LegacyPixels || f.X != 61 || f.Y != 228 || loadSettings(dir).DesktopLyrics.PositionUnits != "points" {
		t.Fatal("converted position was not saved as points and would migrate twice")
	}
	ui.Press("id:desktop-lyrics-locked")
	ui.Press("id:desktop-lyrics-size")
	ui.Press("id:desktop-lyrics-size-4")
	ui.Press("id:desktop-lyrics-color")
	ui.Press("id:desktop-lyrics-color-1")
	ui.Press("id:desktop-lyrics-translation")
	f = windows[0].frames[len(windows[0].frames)-1]
	if f.Locked || f.FontSize != 48 || f.Color != 0x63e6ff || a.set.DesktopLyrics.Translation {
		t.Fatal("settings failed to apply to the existing native window")
	}
	ui.Press("id:desktop-lyrics-reset")
	ui.WaitUntil(time.Second, func() bool { return len(windows) == 2 })
	if !windows[0].closed || windows[1].frames[0].Positioned || a.set.DesktopLyrics.Positioned {
		t.Fatal("reset failed to return lyrics to the default position")
	}
	ui.Press("id:desktop-lyrics-enabled")
	if !windows[1].closed || a.set.DesktopLyrics.Enabled {
		t.Fatal("settings did not turn off the window")
	}
}

// Use the real window factory in the headless desktop host, so the quiet path
// verifies the same NSGo panel implementation that the packaged app uses.
func TestDesktopLyricsNSGoPanelSurvivesMainWindowChanges(t *testing.T) {
	for _, platform := range []core.OS{core.OSMacOS, core.OSWindows} {
		t.Run(string(platform), func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
			var a *app
			var main *desktop.Window
			ui := nsgotest.StartApp(t, func(cx *core.Scope) {
				desktop.SetShutdownMode(cx, desktop.ShutdownOnExplicit)
				a = newApp(cx)
				cx.SetMainView(a.view)
				main = desktop.MainWindow(cx)
			}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{OS: platform, Locale: "en"}))
			ui.Press("id:bar-desktop-lyrics")
			ui.WaitUntil(time.Second, func() bool { return a.lyricWindow != nil })
			first := a.lyricWindow
			panels := 0
			for _, cfg := range ui.Opened {
				if cfg.Kind == nsgotest.SurfacePanel {
					panels++
					if !cfg.Transparent || !cfg.NoActivate || cfg.HideOnBlur {
						t.Fatalf("incorrect lyrics panel options: %+v", cfg)
					}
				}
			}
			if panels != 1 {
				t.Fatalf("expected one NSGo lyrics panel, got %d", panels)
			}
			ui.Press("text:Liked Songs")
			a.post(a.bump)
			ui.Settle()
			a.post(main.Hide)
			ui.Settle()
			ui.Advance(250 * time.Millisecond)
			if main.Visible() || a.lyricWindow != first {
				t.Fatal("main window changes ended or replaced lyrics")
			}
			a.post(func() { a.setDesktopLyricsEnabled(false) })
			ui.Settle()
			if a.lyricWindow != nil || a.lyricTimer != nil {
				t.Fatal("closing lyrics leaked panel or update timer")
			}
			a.post(func() { a.setDesktopLyricsEnabled(true) })
			ui.WaitUntil(time.Second, func() bool { return a.lyricWindow != nil })
			if a.lyricWindow == first {
				t.Fatal("reopening reused a disposed panel")
			}
			a.post(a.closeDesktopLyrics)
			ui.Settle()
		})
	}
}
