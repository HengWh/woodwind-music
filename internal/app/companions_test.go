package app

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

type localCompanionProvider struct {
	music.Source
	track   music.Track
	release <-chan struct{}
}

func (s *localCompanionProvider) Info() music.Info     { return music.Info{ID: "companions-test"} }
func (s *localCompanionProvider) Can(what string) bool { return what == "search" || what == "lyrics" }
func (s *localCompanionProvider) Search(string, music.SearchKind, int) (*music.SearchResult, error) {
	<-s.release
	return &music.SearchResult{Tracks: []music.Track{s.track}}, nil
}
func (s *localCompanionProvider) Lyrics(string) (*music.Lyrics, error) {
	return &music.Lyrics{LRC: "[00:01.00]自动补全歌词"}, nil
}

func TestLocalCompanionsOnDesktopAndAndroid(t *testing.T) {
	var art bytes.Buffer
	if err := png.Encode(&art, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(art.Bytes()) }))
	defer server.Close()
	for _, platform := range []core.OS{core.OSMacOS, core.OSWindows, core.OSAndroid} {
		t.Run(string(platform), func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
			folder := t.TempDir()
			writeTaggedMP3(t, filepath.Join(folder, "阴天.mp3"), "阴天", "莫文蔚", "你可以")
			var a *app
			width, height := core.Points(1180), core.Points(780)
			if platform == core.OSAndroid {
				width, height = 390, 844
			}
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(width, height), nsgotest.Environment(core.Environment{OS: platform}))
			a.local.AddFolders(folder)
			ui.WaitUntil(3*time.Second, func() bool { return a.local.TrackCount() == 1 && !a.local.Scanning() && !a.homeBusy })
			res, err := a.local.Search("阴天", music.SearchTracks, 0)
			if err != nil || len(res.Tracks) != 1 {
				t.Fatal("missing local song", err)
			}
			tr := res.Tracks[0]
			remote := tr
			remote.ID = "remote"
			remote.Album.ImageURL = server.URL + "/cover.png"
			release := make(chan struct{})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			provider := &localCompanionProvider{Source: a.local, track: remote, release: release}
			a.sources.Lock()
			a.sources.items[provider.Info().ID] = provider
			a.sources.Unlock()
			a.post(func() { a.playTracks([]music.Track{tr}, 0) })
			if platform == core.OSAndroid {
				ui.WaitUntil(time.Second, func() bool { return a.video.request.GetUntracked() != nil })
				video := ui.FindMounted("id:android-audio")
				ui.Report(nsgotest.Event{Kind: nsgotest.EventLoad, ID: video.ID, Duration: time.Minute})
			}
			ui.WaitUntil(time.Second, func() bool { _, loaded := a.pl.Lyrics(); return a.pl.Ready() && loaded })
			a.post(func() { a.openNowPlaying(); a.phoneLyrics.Set(true) })
			ui.Settle()
			ui.Find("id:lyrics-empty")
			close(release)
			ui.WaitUntil(3*time.Second, func() bool {
				l, _ := a.pl.Lyrics()
				return a.pl.Ready() && a.pl.Current().Album.ImageURL != "" && len(l) > 0
			})
			ui.WaitUntil(time.Second, func() bool { return ui.Exists("text:自动补全歌词") })
			if a.pl.Current().Source != "local" || a.source() != a.local {
				t.Fatal("companion lookup switched playback source")
			}
			ui.WaitUntil(time.Second, func() bool {
				res, _ := a.local.Search("阴天", music.SearchTracks, 0)
				return len(res.Tracks) == 1 && res.Tracks[0].Album.ImageURL != ""
			})
		})
	}
}
