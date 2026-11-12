package app

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"nsgo-music/internal/i18n"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestStarts(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	app.Find("text:Home")
	app.Find("text:Liked Songs")
	if a.local == nil {
		t.Fatal("no local source")
	}
	app.Press("text:Liked Songs")
	app.Find("text:Songs you like will show here")
	app.Press("text:Home")
	app.Find("text:Add a music folder")
}

func BenchmarkView(b *testing.B) {
	dir := b.TempDir()
	os.Setenv("NSGO_MUSIC_DATA_DIR", dir)
	app := nsgotest.Start(b, func(cx *core.Scope) core.Node {
		a := newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	for range b.N {
		app.Settle()
	}
}

// writeToneWAV writes a one-second sine tone as a WAV, for the library to
// find.
func writeToneWAV(t *testing.T, path string) {
	t.Helper()
	const rate, seconds = 44100, 1
	samples := rate * seconds
	data := make([]byte, samples*2)
	for i := range samples {
		v := int16(6000 * math.Sin(2*math.Pi*440*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*2))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlaysFile(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	writeToneWAV(t, filepath.Join(musicDir, "Haruhikage.wav"))
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.local.AddFolders(musicDir)
	app.WaitUntil(5*time.Second, func() bool {
		return a.local.TrackCount() == 1 && !a.local.Scanning() && !a.homeBusy && a.home != nil && len(a.home.Sections) > 0
	})
	app.Press("id:daily")
	app.WaitUntil(2*time.Second, func() bool { return app.Exists("text:Haruhikage") })
	app.Find("text:Haruhikage >> nth=0")
	app.Press("text:Haruhikage >> nth=0")
	app.WaitUntil(5*time.Second, func() bool { return a.pl.Current() != nil && a.pl.Current().Name == "Haruhikage" })
	app.Find("text:Haruhikage >> nth=0")
	app.Press("label:Now Playing")
	app.Find("text:Lyrics")
	app.Find("text:Up Next")
	app.Find("text:Haruhikage >> nth=0")
	app.Press("text:Up Next")
	app.Find("text:Haruhikage >> nth=3")
	app.Press("label:Close")
	app.WaitForGone("text:Up Next", 2*time.Second)
}

func TestBarStaysWhilePlaying(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	writeToneWAV(t, filepath.Join(musicDir, "Haruhikage.wav"))
	writeToneWAV(t, filepath.Join(musicDir, "Utakotoba.wav"))
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.local.AddFolders(musicDir)
	app.WaitUntil(5*time.Second, func() bool {
		return a.local.TrackCount() == 2 && !a.local.Scanning() && !a.homeBusy && a.home != nil && len(a.home.Sections) > 0
	})
	app.Press("id:daily")
	app.WaitUntil(2*time.Second, func() bool { return app.Exists("text:Haruhikage") })
	app.Press("text:Haruhikage >> nth=0")
	app.WaitUntil(2*time.Second, func() bool { return a.pl.Current() != nil && a.pl.Current().Name == "Haruhikage" })
	app.Find("id:play-pause")
	app.Press("id:play-pause")
	app.Settle()
	app.Find("id:play-pause")
	app.Find("id:bar-open")
}

// writeTaggedMP3 copies the test tone with the tags a library reads.
func writeTaggedMP3(t *testing.T, path, title, artist, album string) {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/tone.mp3")
	if err != nil {
		t.Fatal(err)
	}
	frame := func(id string, data []byte) []byte {
		var b bytes.Buffer
		b.WriteString(id)
		binary.Write(&b, binary.BigEndian, uint32(len(data)))
		b.Write([]byte{0, 0})
		b.Write(data)
		return b.Bytes()
	}
	text := func(id, s string) []byte { return frame(id, append([]byte{3}, []byte(s)...)) }
	var tags bytes.Buffer
	tags.Write(text("TIT2", title))
	tags.Write(text("TPE1", artist))
	tags.Write(text("TALB", album))
	var out bytes.Buffer
	out.WriteString("ID3\x03\x00\x00")
	n := tags.Len()
	out.Write([]byte{byte(n>>21) & 0x7f, byte(n>>14) & 0x7f, byte(n>>7) & 0x7f, byte(n) & 0x7f})
	out.Write(tags.Bytes())
	for range 20 {
		out.Write(raw)
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryPages(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	writeTaggedMP3(t, filepath.Join(musicDir, "01 Haruhikage.mp3"), "Haruhikage", "MyGO!!!!!", "Album One")
	writeTaggedMP3(t, filepath.Join(musicDir, "02 Utakotoba.mp3"), "Utakotoba", "MyGO!!!!!", "Album One")
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.local.AddFolders(musicDir)
	app.WaitUntil(5*time.Second, func() bool {
		return a.local.TrackCount() == 2 && !a.local.Scanning() && !a.homeBusy && a.home != nil && len(a.home.Sections) > 0
	})

	// Search.
	a.search("Haru")
	app.WaitFor("text:/“Haru”/", 5*time.Second)
	app.Find("text:Haruhikage >> nth=0")

	// An album's page.
	var albumID, artistID string
	for _, sec := range a.home.Sections {
		for _, al := range sec.Albums {
			if al.Name == "Album One" {
				albumID = al.ID
			}
		}
		for _, ar := range sec.Artists {
			if ar.Name == "MyGO!!!!!" {
				artistID = ar.ID
			}
		}
	}
	if albumID == "" {
		t.Fatal("the album is not in Home")
	}
	a.open(route{page: pageAlbum, id: albumID})
	app.WaitFor("text:Album One", 5*time.Second)
	app.Find("text:Utakotoba")
	if artistID != "" {
		a.open(route{page: pageArtist, id: artistID})
		app.WaitFor("text:MyGO!!!!!", 5*time.Second)
		app.Find("text:Haruhikage")
	}
}

// TestArtistCards lays an artist's name centered under a small, round
// picture, and keeps the cards within a window made narrower.
func TestArtistCards(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	names := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot"}
	for i, name := range names {
		writeTaggedMP3(t, filepath.Join(musicDir, fmt.Sprintf("%02d %s.mp3", i, name)), "Song "+name, name, "Album "+name)
	}
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.local.AddFolders(musicDir)
	app.WaitUntil(20*time.Second, func() bool {
		return a.local.TrackCount() == len(names) && !a.local.Scanning() && !a.homeBusy && a.home != nil
	})
	app.Press("id:nav-artists")
	name := app.WaitFor("text:Alpha", 5*time.Second)
	if got := name.TextStyle.Align; got != core.TextAlignCenter {
		t.Fatalf("artist name aligned %v, want the center", got)
	}
	card := name.Parent
	if card == nil || len(card.Children) == 0 {
		t.Fatal("artist card has no picture")
	}
	if got := card.Children[0].Frame.Size.Width; got > artistAvatarSize+0.5 {
		t.Fatalf("artist picture %v wide, want at most %v", got, artistAvatarSize)
	}

	// A narrower window must shrink the grid rather than clip its cards.
	app.Resize(900, 780)
	name = app.Find("text:Alpha")
	grid := name.Parent.Parent.Parent
	if grid == nil {
		t.Fatal("the artist grid is gone")
	}
	if got, max := grid.Frame.Size.Width, core.Points(900-sidebarWidth-2*(pagePad-8)); got > max+0.5 {
		t.Fatalf("the artist grid is %v wide in a 900 point window, want at most %v", got, max)
	}
}

func TestCreatesPlaylistAndLikes(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	writeTaggedMP3(t, filepath.Join(musicDir, "01 Haruhikage.mp3"), "Haruhikage", "MyGO!!!!!", "Album One")
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.local.AddFolders(musicDir)
	app.WaitUntil(5*time.Second, func() bool {
		return a.local.TrackCount() == 1 && !a.local.Scanning() && !a.homeBusy && a.home != nil && len(a.home.Sections) > 0
	})

	// A playlist named in the dialog.
	app.Press("label:New Playlist")
	app.Type("id:playlist-name", "Roadtrip")
	app.Press("id:name-save")
	app.WaitUntil(5*time.Second, func() bool { return app.Exists("text:Roadtrip") })
	lists, _ := a.local.Playlists()
	if len(lists) != 1 || lists[0].Name != "Roadtrip" {
		t.Fatalf("playlists: %+v", lists)
	}
	pl := lists[0]

	// A song added from its menu.
	id := a.likedTrackID()
	a.addToPlaylist(pl.ID, []string{id})
	app.WaitUntil(5*time.Second, func() bool {
		p, err := a.local.Playlist(pl.ID)
		return err == nil && len(p.Tracks) == 1
	})

	// A liked song.
	a.toggleLike(id)
	app.WaitUntil(5*time.Second, func() bool { return a.likedSet[id] })
	app.Press("id:nav-liked")
	app.WaitFor("text:Haruhikage >> nth=0", 2*time.Second)
}

// likedTrackID is the ID of the only track the library has.
func (a *app) likedTrackID() string {
	if a.home == nil {
		return ""
	}
	for _, sec := range a.home.Sections {
		if len(sec.Tracks) > 0 {
			return sec.Tracks[0].ID
		}
	}
	return ""
}

func TestSettingsView(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.settingsView(cx)
	}, nsgotest.Size(780, 640))
	app.Find("text:Language")
	app.Find("text:Appearance")
	app.Find("text:Library")
	app.Find("text:Cache")
	app.Press("text:Playback")
	app.Find("text:Even Out Loudness")
	app.Press("text:General")
	app.Find("text:App Icon")

	// A search finds rows of every page; none, says so.
	app.Type("id:settings-search", "cache")
	app.WaitFor("text:Song Cache", 2*time.Second)
	app.Type("id:settings-search", "zzz")
	app.WaitFor("text:No settings found", 2*time.Second)
	app.Clear("id:settings-search")
	app.WaitFor("text:Language", 2*time.Second)

	// Chinese from then on.
	a.set.Language = i18n.Chinese
	a.applyLanguage()
	a.bump()
	app.WaitFor("text:语言", 2*time.Second)
	app.Find("text:外观")
	app.Find("text:资料库")
}

func TestLyricsFollowTheTrack(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	path := filepath.Join(musicDir, "Haruhikage.wav")
	writeToneWAV(t, path)
	if err := os.WriteFile(filepath.Join(musicDir, "Haruhikage.lrc"), []byte("[00:00.00]迷子でもいい、迷子でも進め\n[00:00.40]答えを急がないで\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.local.AddFolders(musicDir)
	app.WaitUntil(5*time.Second, func() bool {
		return a.local.TrackCount() == 1 && !a.local.Scanning() && !a.homeBusy && a.home != nil && len(a.home.Sections) > 0
	})
	app.Press("id:daily")
	app.WaitUntil(2*time.Second, func() bool { return app.Exists("text:Haruhikage") })
	app.Press("text:Haruhikage >> nth=0")
	app.WaitUntil(5*time.Second, func() bool {
		_, loaded := a.pl.Lyrics()
		return loaded
	})
	app.Press("id:bar-open")
	app.Find("text:迷子でもいい、迷子でも進め")
	app.Find("text:答えを急がないで")
	app.Press("id:np-tab-3")
	app.Find("text:迷子でもいい、迷子でも進め")
}

func TestStageStyleMenu(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	writeToneWAV(t, filepath.Join(musicDir, "Haruhikage.wav"))
	if err := os.WriteFile(filepath.Join(musicDir, "Haruhikage.lrc"), []byte("[00:00.00]迷子でもいい、迷子でも進め\n[00:00.40]答えを急がないで\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.local.AddFolders(musicDir)
	app.WaitUntil(5*time.Second, func() bool {
		return a.local.TrackCount() == 1 && !a.local.Scanning() && !a.homeBusy && a.home != nil && len(a.home.Sections) > 0
	})
	app.Press("id:daily")
	app.WaitUntil(2*time.Second, func() bool { return app.Exists("text:Haruhikage") })
	app.Press("text:Haruhikage >> nth=0")
	app.WaitUntil(5*time.Second, func() bool {
		_, loaded := a.pl.Lyrics()
		return loaded
	})
	app.Press("id:bar-open")
	app.Find("text:Lyrics")
	app.Press("id:np-tab-3")
	app.Find("text:迷子でもいい、迷子でも進め")
	app.Press("id:stage-style")
	app.Press("text:Tilt")
	app.WaitUntil(2*time.Second, func() bool { return a.set.StageStyle == "tilt" })
	app.Press("id:stage-style")
	app.Press("text:Sparks")
	app.WaitUntil(2*time.Second, func() bool { return a.set.StageSparks })
}

func TestSearchSuggestsHistory(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.set.SearchHistory = []string{"Haru"}
	a.set.save()
	a.bump()
	app.Settle()
	a.searchRef.Focus()
	app.WaitFor("text:Haru", 2*time.Second)
	app.Press("text:Haru")
	app.WaitFor("text:/“Haru”/", 2*time.Second)
}

// TestSongsPageScrolls checks that a page longer than the window scrolls,
// its list instead of growing the page past the player bar.
func TestSongsPageScrolls(t *testing.T) {
	t.Setenv("NSGO_MUSIC_DATA_DIR", t.TempDir())
	musicDir := t.TempDir()
	const n = 60
	for i := 0; i < n; i++ {
		writeToneWAV(t, filepath.Join(musicDir, fmt.Sprintf("Song %02d.wav", i)))
	}
	var a *app
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	a.local.AddFolders(musicDir)
	app.WaitUntil(20*time.Second, func() bool {
		return a.local.TrackCount() == n && !a.local.Scanning() && !a.homeBusy && a.home != nil && len(a.home.Sections) > 0
	})
	app.Press("id:nav-songs")
	app.WaitFor("text:Song 00", 2*time.Second)
	bar := app.Find("id:play-pause")
	if bar.Frame.Origin.Y+bar.Frame.Size.Height > 780 {
		t.Fatalf("player bar pushed to %v: the page grew instead of scrolling", bar.Frame.Origin)
	}
	// A song page longer than the window scrolls.
	app.Scroll("text:Song 00", 400)
}
