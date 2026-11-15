package app

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
)

type phonePolishSource struct {
	music.Source
	caps   map[string]bool
	search func(int) (*music.SearchResult, error)
}

func (s *phonePolishSource) Info() music.Info {
	return music.Info{ID: "phone-polish", Name: "测试音乐源"}
}
func (s *phonePolishSource) Can(cap string) bool { return s.caps[cap] }
func (s *phonePolishSource) Search(_ string, _ music.SearchKind, offset int) (*music.SearchResult, error) {
	return s.search(offset)
}
func polishTracks(n int) []music.Track {
	tracks := make([]music.Track, n)
	for i := range tracks {
		tracks[i] = music.Track{ID: fmt.Sprintf("polish-%d", i), Source: "phone-polish", Name: fmt.Sprintf("Polish song %03d", i), Artists: []music.Artist{{Name: "Artist"}}, Duration: 180000}
	}
	return tracks
}

func TestPhoneLyricsCenteredAndTransportKeepsList(t *testing.T) {
	for _, size := range []core.Size{{Width: 320, Height: 568}, {Width: 390, Height: 844}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			ui, a := startPhone(t)
			ui.Resize(size.Width, size.Height)
			ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.5, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})
			a.post(func() { reviewTrack(a); a.pl.Hold(0); a.playRev.Update(func(n int) int { return n + 1 }) })
			ui.Settle()
			ui.Press("id:phone-toggle-lyrics")
			for _, i := range []int{0, 5, 23} {
				a.post(func() { a.pl.Hold(time.Duration(i) * 6 * time.Second); a.tick.Update(func(n int) int { return n + 1 }) })
				ui.Settle()
				ui.Advance(time.Second)
				list := reviewBounds(ui.Find("id:lyrics-list"))
				line := reviewBounds(ui.Find(fmt.Sprintf("id:lyric-line-%d", i)))
				delta := math.Abs(float64(line.Origin.Y + line.Size.Height/2 - list.Origin.Y - list.Size.Height/2))
				if delta > 3 {
					t.Fatalf("line %d is %g points away from center: line=%+v viewport=%+v", i, delta, line, list)
				}
				if ui.Find(fmt.Sprintf("id:lyric-text-%d", i)).TextStyle.Align != core.TextAlignCenter {
					t.Fatal("phone lyrics are not horizontally centered")
				}
			}
			ui.Scroll("id:lyrics-list", -100)
			before := ui.Find("id:lyrics-list")
			id, offset := before.ID, before.Offset.Y
			a.post(func() { a.playRev.Update(func(n int) int { return n + 1 }) })
			ui.Settle()
			after := ui.Find("id:lyrics-list")
			if after.ID != id || after.Offset.Y != offset {
				t.Fatal("transport refresh reset lyric browsing")
			}
		})
	}
}

func TestPhoneLyricsResizeRetainsModeAndCenter(t *testing.T) {
	ui, a := startPhone(t)
	a.post(func() { reviewTrack(a); a.playRev.Update(func(n int) int { return n + 1 }) })
	ui.Settle()
	ui.Press("id:phone-toggle-lyrics")
	ui.Advance(time.Second)
	ui.Resize(320, 568)
	ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.5, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})
	ui.Advance(time.Second)
	list := reviewBounds(ui.Find("id:lyrics-list"))
	line := reviewBounds(ui.Find("id:lyric-line-3"))
	if delta := math.Abs(float64(line.Origin.Y + line.Size.Height/2 - list.Origin.Y - list.Size.Height/2)); delta > 3 {
		t.Fatalf("resized lyric is %g points from center: list=%+v line=%+v offset=%+v\n%s", delta, list, line, ui.Find("id:lyrics-list").Offset, ui.Dump())
	}
}

func TestPhoneDetailsOfferWorkingRetry(t *testing.T) {
	for _, kind := range []string{"playlist", "album", "artist", "page"} {
		t.Run(kind, func(t *testing.T) {
			ui, a := startPhone(t)
			src := &phonePolishSource{Source: a.local, caps: map[string]bool{}}
			r := route{src: src.Info().ID, id: "retry"}
			a.post(func() {
				a.sources.items[src.Info().ID] = src
				a.active = src
				switch kind {
				case "playlist":
					r.page = pagePlaylist
					a.playlists[r.id] = &playlistData{err: "offline"}
				case "album":
					r.page = pageAlbum
					a.albums[r.id] = &albumData{err: "offline"}
				case "artist":
					r.page = pageArtist
					a.artists[r.id] = &artistData{err: "offline"}
				case "page":
					r.page = pageBrowse
					a.browses[r.id] = &browseData{err: "offline"}
				}
				a.route.Set(r)
				a.bump()
			})
			ui.Settle()
			ui.Press("id:phone-" + kind + "-retry")
			ui.WaitUntil(time.Second, func() bool {
				switch kind {
				case "playlist":
					return a.playlists[r.id].playlist != nil
				case "album":
					return a.albums[r.id].album != nil
				case "artist":
					return a.artists[r.id].page != nil
				default:
					return a.browses[r.id].browse != nil
				}
			})
		})
	}
}
func (s *phonePolishSource) Playlist(id string) (*music.Playlist, error) {
	return &music.Playlist{ID: id, Name: "Recovered", Tracks: polishTracks(2)}, nil
}
func (s *phonePolishSource) Album(id string) (*music.AlbumPage, error) {
	return &music.AlbumPage{Album: music.Album{ID: id, Name: "Recovered"}, Tracks: polishTracks(2)}, nil
}
func (s *phonePolishSource) Artist(id string) (*music.ArtistPage, error) {
	return &music.ArtistPage{Artist: music.Artist{ID: id, Name: "Recovered"}, Tracks: polishTracks(2)}, nil
}
func (s *phonePolishSource) Browse(id string) (*music.Browse, error) {
	return &music.Browse{Title: "Recovered", Sections: []music.Section{{Tracks: polishTracks(2)}}}, nil
}

func TestPhoneQueueKeepsBrowsingAndReturnsToCurrent(t *testing.T) {
	ui, a := startPhone(t)
	a.post(func() { a.pl.Cue(polishTracks(100)); a.openPhoneSheet("queue") })
	ui.Settle()
	ui.Scroll("id:phone-queue-list", 600)
	before := ui.Find("id:phone-queue-list")
	id, offset := before.ID, before.Offset.Y
	a.post(func() { a.pl.Jump(1); a.playRev.Update(func(n int) int { return n + 1 }) })
	ui.Settle()
	after := ui.Find("id:phone-queue-list")
	if after.ID != id || after.Offset.Y != offset {
		t.Fatalf("switching songs reset queue browsing: %v -> %v", offset, after.Offset.Y)
	}
	ui.Press("id:phone-queue-current")
	ui.Advance(time.Second)
	if ui.Find("id:phone-queue-list").Offset.Y >= offset {
		t.Fatal("current song control did not return to the beginning")
	}
	// Broader data refreshes may remount the panel, but must retain its position.
	ui.Scroll("id:phone-queue-list", 700)
	offset = ui.Find("id:phone-queue-list").Offset.Y
	a.post(a.bump)
	ui.Settle()
	if math.Abs(float64(ui.Find("id:phone-queue-list").Offset.Y-offset)) > 1 {
		t.Fatal("data refresh reset queue offset")
	}
}

func TestPhoneSearchAppendFailureAndRetryKeepsScroll(t *testing.T) {
	ui, a := startPhone(t)
	tracks := polishTracks(40)
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var calls atomic.Int32
	src := &phonePolishSource{Source: a.local, caps: map[string]bool{"search": true}}
	src.search = func(offset int) (*music.SearchResult, error) {
		if offset != 20 {
			return nil, fmt.Errorf("unexpected offset %d", offset)
		}
		if calls.Add(1) == 1 {
			<-release
			return nil, errors.New("temporary network failure")
		}
		return &music.SearchResult{Tracks: tracks[20:]}, nil
	}
	r := route{page: pageSearch, src: src.Info().ID, query: "Polish", kind: music.SearchTracks}
	d := &searchData{loaded: true, more: true, res: music.SearchResult{Tracks: tracks[:20]}}
	a.post(func() {
		a.sources.items[src.Info().ID] = src
		a.active = src
		a.searches[r] = d
		a.route.Set(r)
		a.query.Set(r.query)
		a.bump()
	})
	ui.Settle()
	id := ui.Find("id:phone-page-scroll").ID
	ui.Scroll("id:phone-page-scroll", 650)
	ui.WaitUntil(time.Second, func() bool { return calls.Load() == 1 })
	if ui.Find("id:phone-page-scroll").ID != id {
		t.Fatal("starting pagination rebuilt the scroll view")
	}
	close(release)
	ui.WaitUntil(time.Second, func() bool { return d.err != "" })
	if len(d.res.Tracks) != 20 {
		t.Fatal("append failure discarded existing songs")
	}
	scroll := ui.Find("id:phone-page-scroll")
	ui.ScrollTo("id:phone-page-scroll", core.Point{Y: scroll.Content.Height - scroll.Frame.Size.Height})
	ui.Press("id:phone-search-more-retry")
	ui.WaitUntil(time.Second, func() bool { return len(d.res.Tracks) == 40 && !d.loading })
	if ui.Find("id:phone-page-scroll").ID != id || ui.Find("id:phone-page-scroll").Offset.Y < 600 {
		t.Fatal("appending results replaced the list or reset its position")
	}
	if calls.Load() != 2 {
		t.Fatalf("pagination retried without user action: %d", calls.Load())
	}
}

func TestPhoneSearchSuggestionsRespectComposition(t *testing.T) {
	ui, a := startPhone(t)
	a.post(func() { a.set.SearchHistory = []string{"春日影", "春风"} })
	ui.Settle()
	ui.Press("id:phone-tab-search")
	ui.Focus("id:search-input")
	ui.Compose("id:search-input", "chun")
	ui.Submit("id:search-input")
	if a.route.GetUntracked().query != "" {
		t.Fatal("composition submitted a search")
	}
	ui.Compose("id:search-input", "")
	ui.Clear("id:search-input")
	ui.Type("id:search-input", "春")
	ui.Find("id:phone-search-suggestions")
	ui.Press("id:phone-suggestion-1")
	ui.WaitUntil(time.Second, func() bool { return a.route.GetUntracked().query == "春日影" })
	if ui.Exists("id:phone-search-suggestions") {
		t.Fatal("suggestions did not close after choosing a search")
	}
}

func TestPhoneMenusFollowSourceCapabilities(t *testing.T) {
	ui, a := startPhone(t)
	src := &phonePolishSource{Source: a.local, caps: map[string]bool{}}
	track := polishTracks(1)[0]
	a.post(func() { a.sources.items[src.Info().ID] = src; a.active = src; a.openPhoneTrack(track, "", 0) })
	ui.Settle()
	for _, id := range []string{"phone-track-like", "phone-track-add", "phone-track-album", "phone-track-up", "phone-track-remove"} {
		if ui.Exists("id:" + id) {
			t.Fatalf("unsupported action %s is shown", id)
		}
	}
	a.post(func() {
		a.phoneSheet.Set("")
		a.active = a.local
		list := music.Playlist{ID: "fixed", Name: "Fixed", Mine: true, Fixed: true, Tracks: []music.Track{track}}
		a.playlists[list.ID] = &playlistData{playlist: &list}
		a.openPhoneTrack(track, list.ID, 0)
	})
	ui.Settle()
	if ui.Exists("id:phone-track-remove") {
		t.Fatal("fixed playlist offers removal")
	}
}

func TestPhoneAlbumGridVirtualizesLargeCollections(t *testing.T) {
	ui, a := startPhone(t)
	albums := make([]music.Album, 500)
	for i := range albums {
		albums[i] = music.Album{ID: fmt.Sprint(i), Name: fmt.Sprintf("Polish album %03d", i)}
	}
	a.post(func() {
		a.browses["big-grid"] = &browseData{browse: &music.Browse{Title: "Albums", Sections: []music.Section{{Albums: albums}}}}
		a.route.Set(route{page: pageBrowse, id: "big-grid"})
		a.bump()
	})
	ui.Settle()
	ui.Advance(time.Second)
	if mounted := len(ui.QueryMounted(`text:/Polish album/`)); mounted == 0 || mounted > 40 {
		t.Fatalf("mounted %d of 500 cards", mounted)
	}
	for _, y := range []core.Points{600, 1800, 5000, 12000, 25000, 45000} {
		ui.ScrollTo("id:phone-page-scroll", core.Point{Y: y})
	}
	scroll := ui.Find("id:phone-page-scroll")
	ui.ScrollTo("id:phone-page-scroll", core.Point{Y: scroll.Content.Height - scroll.Frame.Size.Height})
	ui.Advance(time.Second)
	ui.Find(`text:"Polish album 499"`)
	ui.ScrollTo("id:phone-page-scroll", core.Point{})
	ui.Advance(time.Second)
	ui.Find(`text:"Polish album 000"`)
}

func TestPhoneRadioSmallScreenControlsVisible(t *testing.T) {
	ui, a, src := startRadioTest(t, true)
	for _, size := range []core.Size{{Width: 320, Height: 568}, {Width: 390, Height: 844}} {
		ui.Resize(size.Width, size.Height)
		ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.5, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})
		phoneWithin(t, ui, "radio-start", size.Width, size.Height)
		a.post(func() {
			a.pl.Cue(radioTestTracks("a"))
			a.pl.AttachRadio(radioName(src.Info().ID, src.stations[0]), func(func([]music.Track)) {})
			a.playRev.Update(func(n int) int { return n + 1 })
		})
		ui.Settle()
		for _, id := range []string{"radio-like", "radio-play", "radio-next"} {
			phoneWithin(t, ui, id, size.Width, size.Height)
		}
		if ui.Exists("id:player-bar") {
			t.Fatal("FM duplicates its transport in the mini player")
		}
		a.post(a.exitRadio)
		ui.Settle()
	}
}

func TestPhonePolishShots(t *testing.T) {
	if os.Getenv("WOOD_WIND_POLISH_SHOTS") == "" {
		t.Skip("set WOOD_WIND_POLISH_SHOTS=1 for native visual review")
	}
	ui, a := phoneProductFixture(t)
	ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.15, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})
	ui.Press("id:phone-tab-library")
	ui.Press("id:open-settings")
	ui.Screenshot("polish-settings-index")
	ui.Press("id:phone-settings-general")
	ui.Screenshot("polish-settings-general")
	ui.Back()
	ui.Press("id:phone-settings-playback")
	ui.Screenshot("polish-settings-playback")
	a.post(func() { a.closeSettings(); reviewTrack(a); a.playRev.Update(func(n int) int { return n + 1 }) })
	ui.Settle()
	ui.Press("id:phone-toggle-lyrics")
	ui.Advance(time.Second)
	ui.Screenshot("polish-lyrics")
	ui.Resize(320, 568)
	ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.5, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})
	ui.Advance(time.Second)
	ui.Screenshot("polish-lyrics-small")
	a.post(func() { a.closeNowPlaying(); a.active = a.local; a.route.Set(route{page: pageHome}); a.bump() })
	ui.Settle()
	ui.Press("id:phone-tab-search")
	ui.Focus("id:search-input")
	ui.Type("id:search-input", strings.Repeat("风", 2))
	ui.Screenshot("polish-search-suggestions")
}
