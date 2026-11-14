package app

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/music"
)

func playlistReview(t *testing.T) (*app, *nsgotest.App) {
	t.Helper()
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) },
		nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{Locale: "en", OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
	a.post(func() {
		var tracks []music.Track
		for i, name := range []string{"Northern Lights", "Slow Tide", "A Long Way North", "Paper Boats", "The Harbor at Night", "Quiet Hours", "Last Ferry Home", "Snowfield"} {
			tracks = append(tracks, music.Track{ID: fmt.Sprint(i), Name: name,
				Artists: []music.Artist{{ID: "band", Name: "The Polar Lights"}},
				Album:   music.Album{ID: "polar-night", Name: "Polar Night"}, Duration: int64(180000 + i*21000)})
		}
		a.mine = []music.Playlist{{ID: "walks", Name: "Evening Walks", Mine: true}, {ID: "focus", Name: "Focus", Mine: true}, {ID: "picks", Name: "Staff Picks", Saved: true}}
		a.playlists["walks"] = &playlistData{playlist: &music.Playlist{ID: "walks", Name: "Evening Walks", Owner: "Alex", Description: "For the long way home.", Mine: true, Tracks: tracks}}
		a.route.Set(route{page: pagePlaylist, id: "walks"})
		a.bump()
	})
	ui.Settle()
	return a, ui
}

// Each disclosure keeps its state through navigation, and New Playlist reopens the listener's lists.
func TestPlaylistSidebarDisclosures(t *testing.T) {
	a, ui := playlistReview(t)
	ui.Press("id:my-playlists-toggle")
	if ui.Exists("id:playlist-walks") || !ui.Exists("id:playlist-picks") {
		t.Fatal("collapsing created playlists also hid the saved playlists")
	}
	ui.Press("id:saved-playlists-toggle")
	ui.Press("id:nav-home")
	if !a.mineCollapsed || !a.savedCollapsed || ui.Exists("id:playlist-picks") {
		t.Fatal("navigation discarded playlist disclosure state")
	}
	ui.Press("id:new-playlist")
	if a.mineCollapsed || !a.savedCollapsed || !a.namingNew {
		t.Fatal("creating a playlist did not reopen only the created playlists")
	}
	ui.Press("id:name-cancel")
	ui.Press("id:playlist-walks")
	if a.route.GetUntracked().id != "walks" {
		t.Fatal("expanded playlist did not navigate")
	}
}

// The visible More button edits the selected playlist; fixed lists offer no editing actions.
func TestPlaylistHeaderActions(t *testing.T) {
	a, ui := playlistReview(t)
	ui.Press("id:playlist-more")
	ui.Press("id:playlist-rename")
	if !a.naming || a.namingID != "walks" || a.namingText != "Evening Walks" {
		t.Fatal("More renamed a different playlist")
	}
	ui.Press("id:name-cancel")
	ui.Press("id:playlist-more")
	ui.Press("id:playlist-delete")
	if !a.deleting || a.deletingID != "walks" {
		t.Fatal("More deleted a different playlist")
	}
	ui.Key("escape")
	a.post(func() { a.playlists["walks"].playlist.Fixed = true; a.bump() })
	ui.Settle()
	if ui.Exists("id:playlist-more") {
		t.Fatal("a fixed playlist retained its edit menu")
	}
	ui.RightClick("id:track-0")
	if ui.Exists(`text:"Remove from Playlist"`) || ui.Exists(`text:"Move Down"`) {
		t.Fatal("a fixed playlist allowed editing songs")
	}
}

// Album links remain clickable after the playlist switches between wide and narrow layouts.
func TestPlaylistAlbumColumnResizes(t *testing.T) {
	a, ui := playlistReview(t)
	ui.Find("id:track-album-0")
	ui.Resize(880, 780)
	if ui.Exists("id:track-album-0") {
		t.Fatal("the album column crowded a narrow playlist")
	}
	ui.Resize(1180, 780)
	ui.Press("id:track-album-0")
	if r := a.route.GetUntracked(); r.page != pageAlbum || r.id != "polar-night" {
		t.Fatalf("album column navigated to %+v", r)
	}
}

// Optional real-window captures compare playlist layout with MyGo at desktop and phone sizes.
func TestPlaylistUIShots(t *testing.T) {
	if os.Getenv("WOODWIND_PLAYLIST_SHOTS") == "" {
		t.Skip("set WOODWIND_PLAYLIST_SHOTS=1 to review playlists")
	}
	a, ui := playlistReview(t)
	art := image.NewRGBA(image.Rect(0, 0, 320, 320))
	for y := range 320 {
		for x := range 320 {
			art.SetRGBA(x, y, color.RGBA{R: uint8(40 + y/2), G: uint8(90 + x/8), B: uint8(170 - y/12), A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "cover.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, art); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	a.post(func() {
		for i := range a.mine {
			a.mine[i].ImageURL = path
		}
		pl := a.playlists["walks"].playlist
		pl.ImageURL = path
		for i := range pl.Tracks {
			pl.Tracks[i].Album.ImageURL = path
		}
		a.bump()
	})
	ui.Settle()
	for _, dark := range []bool{false, true} {
		a.post(func() {
			appearance := core.AppearanceLight
			if dark {
				appearance = core.AppearanceDark
			}
			a.cx.SetAppearance(appearance)
		})
		ui.Settle()
		for _, width := range []core.Points{1180, 880, 390} {
			ui.Resize(width, 780)
			ui.Screenshot(fmt.Sprintf("playlist-%v-dark-%t", width, dark))
		}
	}
}
