package app

import (
	"fmt"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/music"
)

func TestDesktopSongListsRecycleRows(t *testing.T) {
	tracks := make([]music.Track, 120)
	for i := range tracks {
		tracks[i] = music.Track{ID: fmt.Sprintf("song-%03d", i), Name: fmt.Sprintf("Song %03d", i),
			Artists: []music.Artist{{Name: fmt.Sprintf("Artist %03d", i)}}, Duration: 180000}
	}
	for _, kind := range []string{"tracks", "browse", "playlist", "queue", "wall"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a := newApp(cx)
				p := a.palette(cx)
				var content core.Node
				switch kind {
				case "tracks":
					content = a.pageScroll(p, a.trackList(p, tracks, true, true, ""))
				case "browse":
					content = a.pageScroll(p, a.browseSection(p, music.Section{Tracks: tracks}, "songs", 0)...)
				case "playlist":
					a.playlists["test"] = &playlistData{playlist: &music.Playlist{ID: "test", Name: "Test playlist", Tracks: tracks}}
					content = a.playlistPage(cx, p, "test")
				case "queue":
					a.pl.Cue(tracks)
					content = a.queuePane(cx, p.text)
				case "wall":
					a.pl.Cue(tracks)
					content = a.wallPane(cx, p.text)
				}
				switch node := content.(type) {
				case core.Scroll:
					node.TestID = "song-scroll"
					content = node
				case core.List:
					node.TestID = "song-scroll"
					content = node
				}
				return core.View{Style: core.Style{Height: core.Pt(780), MinHeight: core.Pt(0)}, Children: []core.Node{content}}
			}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{Locale: "en", OS: core.OSMacOS, Idiom: core.IdiomDesktop}))

			// Gradual scrolling forces cells from the beginning to be reused.
			scroll := ui.Find("id:song-scroll")
			end := scroll.Content.Height - scroll.Frame.Size.Height
			for y := core.Points(400); y < end; y += 400 {
				ui.ScrollTo("id:song-scroll", core.Point{Y: y})
			}
			scroll = ui.Find("id:song-scroll")
			ui.ScrollTo("id:song-scroll", core.Point{Y: scroll.Content.Height - scroll.Frame.Size.Height})
			ui.Find(`text:"Song 119"`)
			ui.Find(`text:"Artist 119"`)
			if kind != "browse" {
				ui.RightClick(`label:Song 119`)
				ui.Press(`text:"Copy Title"`)
				if ui.Clipboard != "Song 119" {
					t.Fatalf("recycled row menu copied %q, want Song 119", ui.Clipboard)
				}
			}
			ui.ScrollTo("id:song-scroll", core.Point{})
			ui.Find(`text:"Song 000"`)
		})
	}
}
