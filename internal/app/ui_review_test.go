package app

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/lyrics"
	"github.com/HengWh/woodwind-music/internal/music"
)

// reviewTrack holds a song in place so screenshots and layout checks do not
// depend on the audio device, decoding speed or the length of a test tone.
func reviewTrack(a *app) {
	a.pl.Cue([]music.Track{{ID: "review", Name: "迷子でもいい、迷子でも進め — A Long Song Title",
		Artists: []music.Artist{{Name: "MyGO!!!!!"}}, Duration: 180000}})
	var lines []lyrics.Line
	texts := []string{"迷子でもいい、迷子でも進め", "答えを急がないで", "そのままでいい", "小さな声を重ねて", "明日へと続いていく", "Long lyrics wrap naturally across the available space"}
	for i := range 24 {
		at := time.Duration(i) * 6 * time.Second
		line := lyrics.Line{At: at, End: at + 6*time.Second, Text: texts[i%len(texts)]}
		line.Words = lyrics.SpreadWords(line.Text, line.At, line.End)
		if i%3 == 0 {
			line.Translation = "即使迷失方向，也继续向前"
		}
		lines = append(lines, line)
	}
	a.pl.SetLyrics(lines)
	a.pl.Hold(20 * time.Second)
	a.nowPlaying = true
	a.np.Set(true)
}

func TestNowPlayingViewport(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a := newApp(cx)
		reviewTrack(a)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	for _, size := range []core.Size{{Width: 1180, Height: 780}, {Width: 880, Height: 580}, {Width: 1600, Height: 1000}} {
		ui.Resize(size.Width, size.Height)
		ui.Advance(time.Second)
		for _, id := range []string{"np-cover", "np-title", "np-volume", "lyrics-list"} {
			b := reviewBounds(ui.Find("id:" + id))
			if b.Origin.X < 0 || b.Origin.Y < 0 || b.Origin.X+b.Size.Width > size.Width+1 || b.Origin.Y+b.Size.Height > size.Height+1 {
				t.Fatalf("%s extends beyond the %vx%v window: %+v", id, size.Width, size.Height, b)
			}
		}
	}
}

func TestPlayerBarFitsNarrowWindows(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a := newApp(cx)
		reviewTrack(a)
		a.closeNowPlaying()
		return a.view(cx)
	}, nsgotest.Size(880, 580))
	for _, width := range []core.Points{880, 1180} {
		ui.Resize(width, 580)
		for _, id := range []string{"bar-open", "bar-open-title", "play-pause", "bar-volume"} {
			b := reviewBounds(ui.Find("id:" + id))
			if b.Origin.X < sidebarWidth || b.Origin.X+b.Size.Width > width+1 || b.Origin.Y+b.Size.Height > 581 {
				t.Fatalf("%s extends beyond the %v point playback bar: %+v", id, width, b)
			}
		}
	}
}

func TestAppearanceUpdatesTheWholeWindow(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		cx.SetAppearance(core.AppearanceLight)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	ui.WaitUntil(2*time.Second, func() bool { return !a.cx.Dark() })
	light := ui.Find("id:app-content").Style.Background
	a.post(func() { a.cx.SetAppearance(core.AppearanceDark); a.bump() })
	ui.WaitUntil(2*time.Second, func() bool { return a.cx.Dark() && ui.Find("id:app-content").Style.Background != light })
	dark := ui.Find("id:app-content").Style.Background
	if light == dark {
		t.Fatal("the page retained its light background after switching to dark appearance")
	}
	a.post(func() { a.cx.SetAppearance(core.AppearanceLight); a.bump() })
	ui.WaitUntil(2*time.Second, func() bool { return !a.cx.Dark() && ui.Find("id:app-content").Style.Background == light })
	if got := ui.Find("id:app-content").Style.Background; got != light {
		t.Fatal("the page did not return to its light appearance")
	}
}

func TestLyricsWordsAndBrowsing(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		reviewTrack(a)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	ui.Advance(time.Second)
	words := ui.Find("id:lyric-text-3").Spans
	if len(words) < 2 || words[0].Style.Color&0xff <= words[len(words)-1].Style.Color&0xff {
		t.Fatal("the sung words are not brighter than the upcoming words")
	}
	before := words[len(words)-1].Style.Color & 0xff
	a.post(func() { a.pl.Hold(23900 * time.Millisecond); a.tick.Update(func(n int) int { return n + 1 }) })
	ui.Settle()
	ui.Advance(100 * time.Millisecond)
	words = ui.Find("id:lyric-text-3").Spans
	if words[len(words)-1].Style.Color&0xff <= before {
		t.Fatal("word highlighting did not progress with playback")
	}
	ui.Scroll("id:lyrics-list", 200)
	offset := ui.Find("id:lyrics-list").Offset.Y
	a.post(func() { a.pl.Hold(90 * time.Second); a.tick.Update(func(n int) int { return n + 1 }) })
	ui.Settle()
	ui.Advance(200 * time.Millisecond)
	if got := ui.Find("id:lyrics-list").Offset.Y; got != offset {
		t.Fatalf("following interrupted manual browsing: %v -> %v", offset, got)
	}
	ui.Advance(5 * time.Second)
	if got := ui.Find("id:lyrics-list").Offset.Y; got <= offset {
		t.Fatalf("following did not resume at the current line: %v -> %v", offset, got)
	}
	// A recycled cell must keep its own line and seek time.
	ui.Press("id:lyric-line-16")
	if got := a.pl.Position(); got != 96*time.Second {
		t.Fatalf("clicked lyric seeks to %v, want 1m36s", got)
	}
	a.post(func() { a.pl.Hold(138 * time.Second); a.tick.Update(func(n int) int { return n + 1 }) })
	ui.Settle()
	ui.Advance(time.Second)
	line := reviewBounds(ui.Find("id:lyric-line-23"))
	list := reviewBounds(ui.Find("id:lyrics-list"))
	if y := line.Origin.Y - list.Origin.Y; y < 0 || y > list.Size.Height*0.40 {
		t.Fatalf("the last lyric cannot reach the reading position: y=%v, viewport=%v", y, list.Size.Height)
	}
}

func TestSettingsControls(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		a.set.Language = i18n.English
		a.applyLanguage()
		return a.settingsView(cx)
	}, nsgotest.Size(640, 420))
	ui.Press("id:settings-language")
	ui.Press("id:settings-language-2")
	if a.set.Language != i18n.Chinese {
		t.Fatal("language selection did not apply")
	}
	ui.Find("text:应用图标")
	ui.Press("id:settings-appearance-dark")
	if a.set.Appearance != "dark" {
		t.Fatal("appearance selection did not apply")
	}
	ui.Press("id:settings-language")
	ui.Key("up")
	ui.Key("enter")
	if a.set.Language != i18n.English {
		t.Fatalf("keyboard chose language %q, want English", a.set.Language)
	}
	ui.Press("id:settings-nav-playback")
	ui.Press("id:settings-normalize")
	if a.set.Normalize {
		t.Fatal("loudness switch did not turn off")
	}
	ui.Press("id:settings-quality")
	ui.Press("id:settings-quality-2")
	if a.set.Quality != "lossless" {
		t.Fatal("quality selection did not apply")
	}
	ui.Press("id:settings-nav-general")
	for _, id := range []string{"settings-language", "settings-app-icon", "settings-appearance-dark"} {
		b := reviewBounds(ui.Find("id:" + id))
		if b.Origin.X < 201 || b.Origin.X+b.Size.Width > 640 {
			t.Fatalf("%s overflows the settings page: %+v", id, b)
		}
	}
}

// reviewBounds resolves a view's local layout into window coordinates.
func reviewBounds(v *nsgotest.View) core.Bounds {
	b := v.Frame
	for p := v.Parent; p != nil; p = p.Parent {
		b.Origin.X += p.Frame.Origin.X - p.Offset.X
		b.Origin.Y += p.Frame.Origin.Y - p.Offset.Y
	}
	return b
}

func TestHSVColorKeepsBrightness(t *testing.T) {
	if got := hsvColor(0, 0, 0.5); got != core.RGB(0x808080) {
		t.Fatalf("half-bright gray is %x, want 808080ff", got)
	}
	if got := hsvColor(0, 0.5, 1); got != core.RGB(0xff8080) {
		t.Fatalf("half-saturated red is %x, want ff8080ff", got)
	}
}

// TestUIShots runs only on request, from internal/app:
// WOODWIND_UI_SHOTS=1 go tool nsgo test -target macos -run TestUIShots.
// Every view uses isolated application data.
func TestUIShots(t *testing.T) {
	if os.Getenv("WOODWIND_UI_SHOTS") == "" {
		t.Skip("set WOODWIND_UI_SHOTS=1 to review the settings and lyrics")
	}
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprintf("settings-dark-%t", dark), func(t *testing.T) {
			t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
			var a *app
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a = newApp(cx)
				if dark {
					cx.SetAppearance(core.AppearanceDark)
				} else {
					cx.SetAppearance(core.AppearanceLight)
				}
				return a.settingsView(cx)
			}, nsgotest.Size(780, 640))
			ui.Screenshot(fmt.Sprintf("settings-dark-%t", dark))
			ui.Press("id:settings-language")
			ui.Screenshot(fmt.Sprintf("settings-menu-dark-%t", dark))
			ui.Key("Escape")
			ui.Press("id:settings-nav-playback")
			ui.Screenshot(fmt.Sprintf("settings-playback-dark-%t", dark))
			ui.Resize(640, 420)
			ui.Press("id:settings-nav-general")
			ui.Screenshot(fmt.Sprintf("settings-narrow-dark-%t", dark))
		})
	}
	t.Run("lyrics", func(t *testing.T) {
		t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
		var a *app
		ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			a = newApp(cx)
			reviewTrack(a)
			return a.view(cx)
		}, nsgotest.Size(1180, 780))
		ui.Advance(time.Second)
		ui.Screenshot("lyrics")
		ui.Resize(880, 580)
		ui.Screenshot("lyrics-narrow")
		a.post(func() {
			a.pl.SetLyrics(nil)
			a.playRev.Update(func(n int) int { return n + 1 })
		})
		ui.WaitUntil(2*time.Second, func() bool { return ui.Exists("id:lyrics-empty") })
		ui.Screenshot("lyrics-empty")
		ui.Press("id:np-close")
		ui.WaitForGone("id:np-close", 2*time.Second)
		ui.Screenshot("sidebar-settings")
	})
}
