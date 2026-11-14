package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/player"
)

// Exercise the pathless Android picker, the real catalog and core.Video's
// protocol together. Media events stand in for the physical Android device.
func TestAndroidImportAndNativePlayback(t *testing.T) {
	t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	writeToneWAV(t, filepath.Join(musicDir, "Haruhikage.wav"))
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(390, 844), nsgotest.Environment(core.Environment{
		OS: core.OSAndroid, SafeArea: core.Edges{Top: 24, Bottom: 24},
	}))
	if a.pl.Native == nil {
		t.Fatal("Android did not select native playback")
	}
	app.Find("id:phone-header")
	app.PickDirectories = func(core.DirectoriesRequest) []core.Directory {
		return []core.Directory{{Name: "Music", FS: os.DirFS(musicDir)}}
	}
	app.Press("id:phone-add-folder")
	app.WaitUntil(3*time.Second, func() bool {
		return !a.importing && a.local.TrackCount() == 1 && !a.local.Scanning() && !a.homeBusy
	})
	res, err := a.local.Search("", music.SearchTracks, 0)
	if err != nil || len(res.Tracks) != 1 {
		t.Fatalf("imported tracks: %v, %v", res, err)
	}
	track := res.Tracks[0]
	app.WaitUntil(2*time.Second, func() bool { return app.Exists("id:track-" + track.ID) })
	app.Press("id:track-" + track.ID)
	app.WaitUntil(2*time.Second, func() bool { return a.video.request.GetUntracked() != nil })
	video := app.FindMounted("id:android-audio")
	if root := app.Find("id:app-root"); video.Parent != root.Parent {
		t.Fatal("Android audio must share the attached root with the UI")
	}
	id := video.ID
	if video.Video.Paused || !strings.Contains(video.Video.Source, "/imports/") {
		t.Fatalf("native source: %+v", video.Video)
	}
	app.Report(nsgotest.Event{Kind: nsgotest.EventLoad, ID: id, Duration: time.Minute})
	app.WaitUntil(time.Second, a.pl.Ready)
	app.Report(nsgotest.Event{Kind: nsgotest.EventProgress, ID: id, Time: 10 * time.Second, Duration: time.Minute})
	app.WaitUntil(time.Second, func() bool { return a.pl.Position() == 10*time.Second })
	app.Press("id:play-pause")
	if !video.Video.Paused || a.pl.Playing() {
		t.Fatal("pause did not reach core.Video")
	}
	a.pl.Seek(25 * time.Second)
	app.Settle()
	if len(video.Seeks) == 0 || video.Seeks[len(video.Seeks)-1] != 25*time.Second {
		t.Fatalf("seeks: %v", video.Seeks)
	}
	a.pl.SetVolume(0)
	app.Settle()
	if !video.Video.Muted {
		t.Fatal("zero volume became the Video default full volume")
	}
	a.pl.SetVolume(0.5)
	app.Settle()
	if video.Video.Muted || video.Video.Volume != 0.25 {
		t.Fatalf("volume: %+v", video.Video)
	}
	app.Press("id:play-pause")
	// Opening lyrics and changing the appearance must retain the same player.
	beforeTick := a.tick.GetUntracked()
	app.Advance(600 * time.Millisecond)
	if a.tick.GetUntracked() == beforeTick {
		t.Fatal("play-button redraw cancelled the playback clock")
	}
	app.Press("id:bar-open")
	a.cx.SetAppearance(core.AppearanceDark)
	app.Settle()
	if got := app.FindMounted("id:android-audio"); got != video {
		t.Fatal("navigation or theme recreated the player")
	}
	app.Find("id:np-close")
	app.Press("id:np-close")
	for _, selector := range []string{"id:phone-header", "id:player-bar", "id:play-pause", "id:mini-progress"} {
		v := app.Find(selector)
		if v.Frame.Size.Width > 390 {
			t.Fatalf("%s exceeds phone width: %v", selector, v.Frame)
		}
	}
	a.pl.SetPlaybackMode(player.ModeSingleLoop)
	if a.pl.Repeat() != player.RepeatOne {
		t.Fatal("not repeating one")
	}
	previous := a.video.request.GetUntracked()
	app.Report(nsgotest.Event{Kind: nsgotest.EventEnd, ID: id})
	app.WaitUntil(time.Second, func() bool {
		r := a.video.request.GetUntracked()
		// NSGo can reuse a deleted node's numeric ID. The request and
		// model view must both be new even when that number stays the same.
		return r != nil && r != previous && app.FindMounted("id:android-audio") != video
	})
	video = app.FindMounted("id:android-audio")
	id = video.ID
	if video.Video.Paused {
		t.Fatal("repeat did not start a fresh native player")
	}
	app.Report(nsgotest.Event{Kind: nsgotest.EventError, ID: id, Text: "unsupported media"})
	app.WaitUntil(time.Second, func() bool { return a.pl.Err() != "" })
	app.Find("text:unsupported media")
}
