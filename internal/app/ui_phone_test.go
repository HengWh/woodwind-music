package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/music"
)

func startPhone(t *testing.T) (*nsgotest.App, *app) {
	t.Helper()
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(390, 844), nsgotest.Environment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", SafeArea: core.Edges{Top: 24, Bottom: 24}}))
	ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy })
	return ui, a
}

func phoneWithin(t *testing.T, ui *nsgotest.App, id string, width, height core.Points) {
	t.Helper()
	b := reviewBounds(ui.Find("id:" + id))
	if b.Size.Width <= 0 || b.Size.Height <= 0 || b.Origin.X < -1 || b.Origin.Y < 23 || b.Origin.X+b.Size.Width > width+1 || b.Origin.Y+b.Size.Height > height-23 {
		t.Fatalf("%s escapes the phone's safe area (%v x %v): %+v", id, width, height, b)
	}
}

func TestPhoneNavigationAndSearch(t *testing.T) {
	ui, a := startPhone(t)
	ui.Press("id:phone-tab-library")
	ui.Press("id:phone-songs")
	ui.WaitUntil(time.Second, func() bool { d := a.browses["songs"]; return d != nil && d.browse != nil })
	libraryRoute := a.route.GetUntracked()
	ui.Press("id:phone-tab-search")
	ui.Focus("id:search-input")
	input := ui.Find("id:search-input").ID
	ui.Type("id:search-input", "春日影")
	a.bump()
	ui.Settle()
	if v := ui.Find("id:search-input"); v.ID != input || v.Text != "春日影" || !v.Focused {
		t.Fatal("library refresh replaced the active phone search")
	}
	ui.Submit("id:search-input")
	ui.WaitUntil(time.Second, func() bool { d := a.searches[a.route.GetUntracked()]; return d != nil && d.loaded })
	searchRoute := a.route.GetUntracked()
	ui.Press("id:phone-tab-library")
	if a.route.GetUntracked() != libraryRoute || len(a.history) != 1 {
		t.Fatal("library tab did not retain its detail stack")
	}
	ui.Back()
	if a.route.GetUntracked().page != pageLibrary {
		t.Fatal("Back did not return to the library root")
	}
	ui.Press("id:open-settings")
	ui.Find("id:phone-settings")
	ui.Back()
	if a.settingsInMain {
		t.Fatal("Back did not close phone settings")
	}
	ui.Press("id:phone-tab-search")
	if a.route.GetUntracked() != searchRoute || ui.Find("id:search-input").Text != "春日影" {
		t.Fatal("search tab did not restore its query and result")
	}
	ui.Press("id:clear-search")
	if a.route.GetUntracked().page != pageSearch || a.query.GetUntracked() != "" {
		t.Fatal("clear left the Search destination")
	}
	ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", SafeArea: core.Edges{Top: 24, Bottom: 24}, KeyboardHeight: 300})
	if ui.Exists("id:phone-navigation") || ui.Exists("id:player-bar") {
		t.Fatal("bottom chrome covers the keyboard")
	}
	ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", SafeArea: core.Edges{Top: 24, Bottom: 24}})
	ui.Find("id:phone-navigation")
}

func TestPhonePlayerAndSheets(t *testing.T) {
	ui, a := startPhone(t)
	a.post(func() { reviewTrack(a); a.closeNowPlaying() })
	ui.Settle()
	for _, size := range []core.Size{{Width: 320, Height: 568}, {Width: 390, Height: 844}, {Width: 430, Height: 932}} {
		ui.Resize(size.Width, size.Height)
		for _, id := range []string{"phone-header", "player-bar", "phone-navigation", "play-pause", "bar-queue"} {
			phoneWithin(t, ui, id, size.Width, size.Height)
		}
		if ui.Find("id:player-bar").Frame.Size.Height > 68 {
			t.Fatal("mini player grew into desktop controls")
		}
		ui.Press("id:bar-open")
		for _, id := range []string{"np-close", "np-cover", "np-title", "np-play-pause", "np-queue", "progress"} {
			phoneWithin(t, ui, id, size.Width, size.Height)
		}
		ui.Press("id:np-cover")
		phoneWithin(t, ui, "lyrics-list", size.Width, size.Height)
		ui.Press("id:np-queue")
		ui.Find("id:phone-queue")
		ui.Key("escape")
		if a.phoneSheet.GetUntracked() != "" || !a.nowPlaying {
			t.Fatal("Escape did not dismiss only the queue")
		}
		ui.Press("id:np-queue")
		ui.Back()
		if a.phoneSheet.GetUntracked() != "" || !a.nowPlaying {
			t.Fatal("Back closed the player along with the queue")
		}
		ui.Press("id:phone-toggle-lyrics")
		ui.Find("id:np-cover")
		ui.Pan("id:phone-player-header", core.Point{Y: 15}, core.Point{Y: 100})
		if a.nowPlaying {
			t.Fatal("downward header swipe did not dismiss Now Playing")
		}
	}
	// Wide windows use the desktop shell with the same player and route.
	ui.Resize(1000, 780)
	ui.Find("id:nav-home")
	if ui.Exists("id:phone-navigation") {
		t.Fatal("phone navigation remained on a wide window")
	}
	ui.Resize(390, 844)
	ui.Find("id:phone-navigation")
}

func TestPhoneTrackActionsAndPlaylist(t *testing.T) {
	ui, a := startPhone(t)
	dir := t.TempDir()
	writeToneWAV(t, filepath.Join(dir, "song.wav"))
	a.local.AddFolders(dir)
	ui.WaitUntil(2*time.Second, func() bool { return a.local.TrackCount() == 1 && !a.local.Scanning() && !a.homeBusy })
	tracks, err := a.local.Search("", music.SearchTracks, 0)
	if err != nil {
		t.Fatal(err)
	}
	track := tracks.Tracks[0]
	pl, err := a.local.CreatePlaylist("随身听")
	if err != nil {
		t.Fatal(err)
	}
	ui.WaitUntil(time.Second, func() bool { return len(a.mine) == 1 && !a.homeBusy })
	ui.Press("id:track-more-" + track.ID)
	ui.Press("id:phone-track-like")
	ui.WaitUntil(time.Second, func() bool { return a.likedSet[track.ID] })
	ui.Press("id:track-more-" + track.ID)
	ui.Press("id:phone-track-add")
	ui.Press("id:phone-add-to-" + pl.ID)
	ui.WaitUntil(time.Second, func() bool { p, _ := a.local.Playlist(pl.ID); return p != nil && len(p.Tracks) == 1 })
	ui.Press("id:phone-tab-library")
	ui.Press("id:playlist-" + pl.ID)
	ui.WaitUntil(time.Second, func() bool { d := a.playlists[pl.ID]; return d != nil && d.playlist != nil })
	ui.Press("id:track-more-" + track.ID)
	ui.Press("id:phone-track-remove")
	ui.WaitUntil(time.Second, func() bool { p, _ := a.local.Playlist(pl.ID); return p != nil && len(p.Tracks) == 0 })
	ui.Press("id:phone-back")
	ui.Press("id:playlist-more-" + pl.ID)
	ui.Press("id:phone-playlist-rename")
	ui.Clear("id:playlist-name")
	ui.Type("id:playlist-name", "夜晚的风")
	ui.Press("id:name-save")
	ui.WaitUntil(time.Second, func() bool { return len(a.mine) == 1 && a.mine[0].Name == "夜晚的风" })
}

// Run on a native surface for visual review; the same test remains optional headless.
func TestPhoneShots(t *testing.T) {
	if os.Getenv("WOOD_WIND_PHONE_SHOTS") == "" {
		t.Skip("set WOOD_WIND_PHONE_SHOTS=1 for phone screenshots")
	}
	ui, a := startPhone(t)
	_, source, _, _ := runtime.Caller(0)
	art := music.FileURL(filepath.Join(filepath.Dir(source), "..", "..", "assets", "woodwind-icon-anime", "woodwind-icon-anime-512.png"))
	a.post(func() {
		reviewTrack(a)
		a.closeNowPlaying()
		a.set.Language = "zh-CN"
		a.applyLanguage()
		tracks := make([]music.Track, 8)
		for i := range tracks {
			tracks[i] = music.Track{ID: fmt.Sprintf("phone-demo-%d", i), Name: []string{"风吹过的地方", "春日影", "漫步森林", "夜空中的星"}[i%4], Artists: []music.Artist{{Name: "木听风"}}, Album: music.Album{ID: "demo", Name: "风与木的诗", ImageURL: art}, Duration: 180000}
		}
		a.pl.Cue(tracks)
		a.pl.Hold(42 * time.Second)
		a.playRev.Update(func(n int) int { return n + 1 })
		var albums []music.Album
		for i := range 8 {
			albums = append(albums, music.Album{ID: fmt.Sprint(i), Name: []string{"风与木的诗", "午后随想", "雨后的森林", "静静聆听"}[i%4], ImageURL: art, Artists: []music.Artist{{Name: "木听风"}}})
		}
		a.home = &music.Browse{Sections: []music.Section{{Title: "最近播放", Tracks: tracks}, {Title: "专辑", Albums: albums, MorePage: "albums"}}}
		a.bump()
	})
	ui.Settle()
	ui.Advance(time.Second)
	ui.Screenshot("phone-home")
	ui.Press("id:phone-tab-library")
	ui.Screenshot("phone-library")
	ui.Press("id:bar-open")
	ui.Screenshot("phone-player")
	ui.Press("id:phone-toggle-lyrics")
	ui.Screenshot("phone-lyrics")
	ui.Press("id:np-queue")
	ui.Screenshot("phone-queue")
	ui.Back()
	ui.Advance(300 * time.Millisecond)
	ui.Back()
	ui.Advance(300 * time.Millisecond)
	ui.Press("id:open-settings")
	ui.WaitUntil(time.Second, func() bool { return a.settingsInMain })
	ui.Screenshot("phone-settings")
}

func TestPhoneTabsRestoreScroll(t *testing.T) {
	ui, a := startPhone(t)
	a.post(func() {
		for i := range 30 {
			a.mine = append(a.mine, music.Playlist{ID: fmt.Sprint(i), Name: fmt.Sprintf("Playlist %d", i)})
		}
		a.bump()
	})
	ui.Settle()
	ui.Press("id:phone-tab-library")
	ui.Scroll("id:phone-page-scroll", 400)
	offset := ui.Find("id:phone-page-scroll").Offset.Y
	if offset < 300 {
		t.Fatal("library did not scroll")
	}
	ui.Press("id:phone-tab-home")
	ui.Press("id:phone-tab-library")
	if restored := ui.Find("id:phone-page-scroll").Offset.Y; restored != offset {
		t.Fatalf("library scroll reset: %v, want %v", restored, offset)
	}
}

func TestPhoneLargeLibrary(t *testing.T) {
	ui, a := startPhone(t)
	tracks := make([]music.Track, 100)
	for i := range tracks {
		tracks[i] = music.Track{ID: fmt.Sprintf("large-%d", i), Name: fmt.Sprintf("Song %d", i)}
	}
	a.post(func() {
		a.phoneTab = 1
		a.route.Set(route{page: pageBrowse, id: "songs"})
		a.browses["songs"] = &browseData{browse: &music.Browse{Title: "Songs", Sections: []music.Section{{Tracks: tracks}}}}
		a.bump()
	})
	ui.Settle()
	scroll := ui.Find("id:phone-page-scroll")
	ui.ScrollTo("id:phone-page-scroll", core.Point{Y: scroll.Content.Height - scroll.Frame.Size.Height})
	ui.Find("id:track-large-99")
	phoneWithin(t, ui, "track-more-large-99", 390, 844)
	ui.Press("id:track-more-large-99")
	if a.phoneTrack.ID != "large-99" {
		t.Fatal("virtualized row menu retained another song")
	}
}

func TestPhoneSystemBarsAndSheetGrip(t *testing.T) {
	ui, a := startPhone(t)
	if ui.StatusBar.Style != core.StatusBarDark {
		t.Fatal("light page needs dark system icons")
	}
	a.post(func() { reviewTrack(a) })
	ui.Settle()
	if ui.StatusBar.Style != core.StatusBarLight {
		t.Fatal("Now Playing needs light system icons")
	}
	ui.Press("id:np-queue")
	ui.Pan("id:phone-sheet-grip", core.Point{Y: 15}, core.Point{Y: 100})
	if a.phoneSheet.GetUntracked() != "" || !a.nowPlaying {
		t.Fatal("sheet grip did not close only the sheet")
	}
	ui.Press("id:np-close")
	if ui.StatusBar.Style != core.StatusBarDark {
		t.Fatal("closing Now Playing did not restore the system icons")
	}
}
