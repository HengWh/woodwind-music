package app

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// Exercise every destination with long content at the device's font scale and
// a larger accessibility size, rather than comparing source constants alone.
func TestPhoneTypographyAcrossPages(t *testing.T) {
	for _, scale := range []float32{1, 1.15, 1.5} {
		for _, size := range []core.Size{{Width: 320, Height: 568}, {Width: 390, Height: 844}} {
			t.Run(fmt.Sprintf("%gx%g/font%g", size.Width, size.Height, scale), func(t *testing.T) {
				ui, a := startPhone(t)
				ui.Resize(size.Width, size.Height)
				ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: scale, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})
				track := music.Track{ID: "type-song", Name: "漫长的旅途 — A Long Song Title", Artists: []music.Artist{{ID: "type-artist", Name: "漫长旅途的歌手"}}, Album: music.Album{ID: "type-album", Name: "漫长旅途的专辑"}, Duration: 180000}
				album := track.Album
				album.Artists = track.Artists
				list := music.Playlist{ID: "type-playlist", Name: "漫长旅途的歌单", Tracks: []music.Track{track}, TrackCount: 1, Mine: true}
				section := music.Section{Title: "最近播放", Tracks: []music.Track{track}, Albums: []music.Album{album}, Artists: track.Artists, Playlists: []music.Playlist{list}}
				a.post(func() {
					a.home = &music.Browse{Sections: []music.Section{section}}
					a.mine = []music.Playlist{list}
					a.liked = []music.Track{track}
					a.albums[album.ID] = &albumData{album: &music.AlbumPage{Album: album, Tracks: []music.Track{track}}}
					a.artists["type-artist"] = &artistData{page: &music.ArtistPage{Artist: track.Artists[0], Tracks: []music.Track{track}, Albums: []music.Album{album}}}
					a.playlists[list.ID] = &playlistData{playlist: &list}
					for _, id := range []string{"songs", "albums", "artists"} {
						a.browses[id] = &browseData{browse: &music.Browse{Title: "音乐资料库", Sections: []music.Section{section}}}
					}
					reviewTrack(a)
					a.closeNowPlaying()
					a.playRev.Update(func(n int) int { return n + 1 })
					a.bump()
				})
				ui.Settle()
				check := func(name string) {
					t.Helper()
					ui.Settle()
					root := ui.FindMounted("id:app-root")
					var walk func(*nsgotest.View)
					walk = func(v *nsgotest.View) {
						if v.Text != "" && float64(v.TextStyle.Font.Size) > 22*float64(scale)+.05 {
							t.Errorf("%s oversized text %q: %g", name, v.Text, v.TextStyle.Font.Size)
						}
						for _, child := range v.Children {
							walk(child)
						}
					}
					walk(root)
					for _, v := range ui.QueryMounted("text:/.+/") {
						if v.Text != "" && float64(v.TextStyle.Font.Size) > 22*float64(scale)+.05 {
							t.Errorf("%s oversized overlay %q: %g", name, v.Text, v.TextStyle.Font.Size)
						}
					}
					for _, id := range []string{"play-pause", "bar-queue", "phone-tab-home"} {
						if ui.Exists("id:" + id) {
							b := ui.Find("id:" + id).Frame.Size
							if b.Height < 48 || b.Width < 48 {
								t.Errorf("%s undersized target %s: %+v", name, id, b)
							}
						}
					}
				}
				routes := []route{{page: pageHome}, {page: pageLibrary}, {page: pageLiked}, {page: pageBrowse, id: "songs"}, {page: pageBrowse, id: "albums"}, {page: pageBrowse, id: "artists"}, {page: pageAlbum, id: album.ID}, {page: pageArtist, id: "type-artist"}, {page: pagePlaylist, id: list.ID}, {page: pageSection, query: "0"}, {page: pageSearch}}
				for _, kind := range []music.SearchKind{music.SearchTracks, music.SearchAlbums, music.SearchArtists, music.SearchPlaylists} {
					r := route{page: pageSearch, query: "旅途", kind: kind}
					a.searches[r] = &searchData{loaded: true, res: music.SearchResult{Tracks: []music.Track{track}, Albums: []music.Album{album}, Artists: track.Artists, Playlists: []music.Playlist{list}}}
					routes = append(routes, r)
				}
				for _, r := range routes {
					a.post(func() { a.route.Set(r); a.bump() })
					ui.Settle()
					check(fmt.Sprint(r))
				}
				a.post(func() {
					a.artists["type-artist"].tab = 1
					a.route.Set(route{page: pageArtist, id: "type-artist"})
					a.bump()
				})
				ui.Settle()
				check("artist albums")
				a.post(func() { a.liked = nil; a.route.Set(route{page: pageLiked}); a.bump() })
				ui.Settle()
				check("empty liked")
				a.post(func() { a.openSettings() })
				ui.Settle()
				for _, category := range []string{"general", "playback", "library", "cache", "experiments"} {
					a.post(func() { a.settingsPage = category; a.bump() })
					ui.Settle()
					check("settings " + category)
				}
				a.post(func() { a.closeSettings(); a.openNowPlaying() })
				ui.Settle()
				for _, id := range []string{"np-cover", "np-title", "np-play-pause", "np-queue", "progress"} {
					phoneWithin(t, ui, id, size.Width, size.Height)
				}
				if got := ui.Find("id:np-title").TextStyle.Font.Size; math.Abs(float64(got)-20*float64(scale)) > .05 {
					t.Fatalf("system font scale lost: %g", got)
				}
				check("player")
				ui.Press("id:phone-toggle-lyrics")
				ui.Advance(time.Second)
				check("lyrics")
				phoneWithin(t, ui, "lyrics-list", size.Width, size.Height)
				for _, kind := range []string{"queue", "track", "add", "info", "playlist"} {
					a.post(func() { a.phoneTrack = track; a.phonePlaylist = list.ID; a.openPhoneSheet(kind) })
					ui.Settle()
					check("sheet " + kind)
					a.post(func() { a.phoneSheet.Set("") })
					ui.Settle()
				}
				a.post(func() { a.closeNowPlaying(); a.newPlaylist() })
				ui.Settle()
				check("new playlist")
				ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: scale, ReduceMotion: true, KeyboardHeight: 280, SafeArea: core.Edges{Top: 24, Bottom: 24}})
				dialogBounds := reviewBounds(ui.Find("id:phone-dialog-scroll"))
				if dialogBounds.Origin.Y < 24 || dialogBounds.Origin.Y+dialogBounds.Size.Height > size.Height-280+1 {
					t.Fatalf("dialog covered by keyboard: %+v", dialogBounds)
				}
				check("dialog with keyboard")
				ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: scale, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})

				for _, id := range []string{"playlist-name", "name-save", "name-cancel"} {
					b := ui.Find("id:" + id).Frame.Size
					if b.Height < 48 {
						t.Errorf("small dialog target %s: %+v", id, b)
					}
				}
				a.post(func() { a.naming = false; a.confirmDelete(list.ID, list.Name); a.bump() })
				ui.Settle()
				check("delete playlist")
				a.post(func() { a.deleting = false; a.renamePlaylist(list.ID, list.Name); a.bump() })
				ui.Settle()
				check("rename playlist")
				a.post(func() {
					a.naming = false
					list.Tracks = nil
					list.TrackCount = 0
					a.route.Set(route{page: pagePlaylist, id: list.ID})
					a.bump()
				})
				ui.Settle()
				check("empty playlist")
				a.post(func() { a.pl.SetLyrics(nil); a.openNowPlaying() })
				ui.Settle()
				ui.Press("id:phone-toggle-lyrics")
				check("no lyrics")
			})
		}
	}
}
