package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestAndroidMediaServiceBridge(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	folder := t.TempDir()
	writeToneWAV(t, filepath.Join(folder, "Native.wav"))
	host := core.NewHost()
	if err := host.SetParams(map[string]any{"woodwind.android": true}); err != nil {
		t.Fatal(err)
	}
	var updates []map[string]any
	host.Handle("playback.update", func(call *core.HostCall) { updates = append(updates, call.Data.(map[string]any)); call.Resolve(nil) })
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(390, 844), nsgotest.Environment(core.Environment{OS: core.OSAndroid}), nsgotest.Host(host))
	a.local.AddFolders(folder)
	app.WaitUntil(2*time.Second, func() bool { return a.local.TrackCount() == 1 && !a.local.Scanning() })
	tracks, _ := a.local.Search("", music.SearchTracks, 0)
	a.playTracks(tracks.Tracks, 0)
	app.WaitUntil(time.Second, func() bool { return a.video.request.GetUntracked() != nil })
	v := app.FindMounted("id:android-audio")
	if v.Video.AudioSession != 0 {
		t.Fatal("core.Video competes with the service for audio focus")
	}
	app.Report(nsgotest.Event{Kind: nsgotest.EventLoad, ID: v.ID, Duration: time.Minute})
	app.WaitUntil(time.Second, a.pl.Ready)
	if len(updates) == 0 {
		t.Fatal("no system metadata published")
	}
	last := updates[len(updates)-1]
	if last["title"] != "Native" || last["playing"] != true || last["loading"] != false {
		t.Fatalf("metadata: %+v", last)
	}
	send := func(command string) {
		if err := host.Send("android.media.command", map[string]any{"command": command}); err != nil {
			t.Fatal(err)
		}
		app.Settle()
	}
	send("focus-pause")
	if a.pl.Playing() {
		t.Fatal("did not pause for a call")
	}
	send("focus-resume")
	if !a.pl.Playing() {
		t.Fatal("did not resume after a transient interruption")
	}
	send("focus-pause")
	a.toggle() // explicit user play and pause cancels automatic resumption
	a.toggle()
	send("focus-resume")
	if a.pl.Playing() {
		t.Fatal("overrode an explicit user pause")
	}
	send("play")
	if !a.pl.Playing() {
		t.Fatal("media play command failed")
	}
	send("pause")
	if a.pl.Playing() {
		t.Fatal("media pause command failed")
	}
	if err := host.Send("android.media.command", map[string]any{"command": "seek", "position": float64(15000)}); err != nil {
		t.Fatal(err)
	}
	app.Settle()
	if a.pl.Position() != 15*time.Second {
		t.Fatal("media seek failed")
	}
	if got := updates[len(updates)-1]["position"]; got != float64(15000) {
		t.Fatalf("system position %v", got)
	}
}
