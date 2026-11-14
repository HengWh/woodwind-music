package app

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func dailyReview(t *testing.T) (*app, *nsgotest.App, []music.Track) {
	t.Helper()
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "First Light.wav")
	writeToneWAV(t, path)
	wav, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wav = append(wav[:44:44], bytes.Repeat(wav[44:], 60)...)
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(wav)-44))
	for _, name := range []string{"First Light", "Ocean Drive", "Evening Breeze"} {
		if err := os.WriteFile(filepath.Join(dir, name+".wav"), wav, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var a *app
	view := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) },
		nsgotest.Size(1500, 820), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Locale: "en", Idiom: core.IdiomDesktop}))
	a.local.AddFolders(dir)
	view.WaitUntil(5*time.Second, func() bool { return a.local.TrackCount() == 3 && !a.local.Scanning() && !a.homeBusy })
	page, err := a.local.Browse("songs")
	if err != nil {
		t.Fatal(err)
	}
	tracks := page.Sections[0].Tracks
	a.post(func() {
		a.local.OnChange(nil)
		a.home = &music.Browse{Sections: []music.Section{{Title: "Recently Played", Tracks: tracks}}}
		a.bump()
	})
	view.Settle()
	t.Cleanup(func() { a.post(a.pl.Clear); view.Settle() })
	return a, view, tracks
}

// Playback changes must refresh the card's button; it previously kept the initial Play action after starting.
func TestDailyPlaybackControls(t *testing.T) {
	a, view, tracks := dailyReview(t)
	view.Press("id:daily-play")
	view.WaitUntil(3*time.Second, func() bool { return a.pl.Playing() })
	view.Hover("id:daily-play")
	if view.Find("id:daily-play").Props.Label != "Pause" {
		t.Fatalf("the card did not follow playback: %q, current %v, queue %d, home %d", view.Find("id:daily-play").Props.Label, a.pl.Current(), len(a.pl.Queue()), len(a.home.Sections[0].Tracks))
	}
	if a.route.GetUntracked().page != pageHome || len(a.pl.Queue()) != len(tracks) {
		t.Fatal("the play button opened the card or lost its queue")
	}
	view.Press("id:daily-play")
	if a.pl.Playing() {
		t.Fatal("the card restarted the queue instead of pausing it")
	}
	if view.Find("id:daily-play").Props.Label != "Resume" {
		t.Fatal("paused playback was not reflected in the card")
	}
	view.Press("id:daily-play")
	view.WaitUntil(3*time.Second, func() bool { return a.pl.Playing() })
	view.Press("id:daily-pick-" + tracks[1].ID)
	view.WaitUntil(3*time.Second, func() bool { return a.currentID() == tracks[1].ID })
	if a.route.GetUntracked().page != pageHome {
		t.Fatal("a song pick opened the card")
	}
}

// A card opens its full section, and a source-provided destination takes precedence.
func TestDailyNavigation(t *testing.T) {
	a, view, _ := dailyReview(t)
	view.Press("id:daily")
	if r := a.route.GetUntracked(); r.page != pageSection || r.query != "0" {
		t.Fatalf("opened %+v", r)
	}
	a.post(func() { a.home.Sections[0].MorePage = "songs"; a.open(route{page: pageHome}) })
	view.Settle()
	view.Press("id:daily")
	if r := a.route.GetUntracked(); r.page != pageBrowse || r.id != "songs" {
		t.Fatalf("opened %+v", r)
	}
}

// Only this card's queue drives its record; leaving Home and reduced motion stop its timer.
func TestDailyMotionFollowsItsQueue(t *testing.T) {
	a, view, tracks := dailyReview(t)
	a.post(func() { a.playTracks(tracks[:1], 0) })
	view.WaitUntil(3*time.Second, func() bool { return a.pl.Playing() })
	if playing, _ := a.dailyMotion(a.cx.Env()); playing {
		t.Fatal("a different queue animated this card")
	}
	view.Press("id:daily-play")
	view.WaitUntil(3*time.Second, func() bool { return a.pl.Playing() && len(a.pl.Queue()) == 3 })
	if playing, _ := a.dailyMotion(a.cx.Env()); !playing {
		t.Fatal("the card's queue did not drive its record")
	}
	view.Press("id:nav-liked")
	view.Advance(50 * time.Millisecond)
	if a.dailyTicking {
		t.Fatal("the hidden card kept its timer alive")
	}
	view.Press("id:nav-home")
	view.WaitUntil(time.Second, func() bool { return a.dailyTicking })
	view.SetEnvironment(core.Environment{OS: core.OSMacOS, Locale: "en", Idiom: core.IdiomDesktop, ReduceMotion: true})
	view.Advance(50 * time.Millisecond)
	view.Hover("id:daily")
	if a.dailyTicking {
		t.Fatal("reduced motion kept the record spinning")
	}
}

// Optional real-window captures cover the original card's responsive layout and both appearances.
func TestDailyUIShots(t *testing.T) {
	if os.Getenv("WOODWIND_DAILY_SHOTS") == "" {
		t.Skip("set WOODWIND_DAILY_SHOTS=1 to review the home card")
	}
	a, view, tracks := dailyReview(t)
	_, source, _, _ := runtime.Caller(0)
	art := filepath.Join(filepath.Dir(source), "..", "..", "assets", "woodwind-icon-anime", "woodwind-icon-anime-512.png")
	if _, err := os.Stat(art); err != nil {
		t.Fatal(err)
	}
	a.post(func() {
		for i := range tracks {
			tracks[i].Album.ImageURL = music.FileURL(art)
			tracks[i].Artists = []music.Artist{{Name: "Wood & Wind"}}
		}
		a.bump()
	})
	view.WaitUntil(3*time.Second, func() bool { return a.tintDone[music.FileURL(art)] && a.tints[music.FileURL(art)] != 0 })
	for _, dark := range []bool{false, true} {
		a.post(func() {
			appearance := core.AppearanceLight
			if dark {
				appearance = core.AppearanceDark
			}
			a.cx.SetAppearance(appearance)
		})
		view.Settle()
		for _, width := range []core.Points{1500, 1180, 880} {
			view.Resize(width, 820)
			view.HoverOut()
			view.Advance(time.Second)
			view.Screenshot(fmt.Sprintf("daily-%v-dark-%t", width, dark))
		}
	}
	view.Resize(1500, 820)
	view.Hover("id:daily")
	view.Advance(1200 * time.Millisecond)
	view.Screenshot("daily-hover")
	view.Press("id:daily-play")
	view.WaitUntil(3*time.Second, func() bool { return a.pl.Playing() })
	view.Advance(time.Second)
	view.Screenshot("daily-playing")
}

// Run creates the app outside its main window; using that app scope's zero width hid the cover stack.
func TestDailyUsesMainWindowSize(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	root := core.NewRoot(core.RootOptions{Post: func(func()) {}, Clock: core.NewManualClock(time.Unix(0, 0)), Windows: &core.WindowOptions{Width: 1180, Height: 780}})
	defer root.Dispose()
	application := core.NewTree(root, core.ApproxTextEngine{})
	application.SetEnvironment(core.Environment{OS: core.OSMacOS, Locale: "en", Idiom: core.IdiomDesktop})
	application.Mount(core.AppRoot(func(cx *core.Scope) {
		a := newApp(cx)
		sec := music.Section{Title: "Recently Played", Tracks: []music.Track{{ID: "one"}, {ID: "two"}, {ID: "three"}}}
		a.home = &music.Browse{Sections: []music.Section{sec}}
		cx.SetMainView(func(cx *core.Scope) core.Node { return a.featureCard(a.palette(cx), sec, "", 0, time.Unix(0, 0)) })
	}))
	application.Commit()
	if len(application.Surfaces()) != 1 {
		t.Fatal("the app did not open its main window")
	}
	window := application.Surfaces()[0]
	stackMounted := func() bool {
		batch := window.Commit()
		if batch == nil {
			return false
		}
		for _, item := range batch.Items {
			if item.ViewProps != nil && item.ViewProps.TestID == "daily-stack" {
				return true
			}
		}
		return false
	}
	if !stackMounted() {
		t.Fatal("the cover stack was hidden in a 1180-point main window while the app scope had zero width")
	}
	window.Resize(core.Size{Width: 880, Height: 780})
	window.Commit()
	window.Resize(core.Size{Width: 1180, Height: 780})
	if !stackMounted() {
		t.Fatal("widening the main window did not restore the cover stack")
	}
}
