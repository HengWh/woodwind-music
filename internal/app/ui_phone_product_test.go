package app

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

type phoneProductSource struct{ music.Source }

func (s *phoneProductSource) Info() music.Info {
	return music.Info{ID: "phone-product", Name: "酷狗概念版", Navigation: []music.NavItem{
		{Builtin: "home"}, {Title: "每日推荐", Page: "daily", Icon: "calendar"}, {Title: "排行榜", Page: "charts", Icon: "chart"}, {Title: "私人 FM", Builtin: "stations", Icon: "radio"}, {Builtin: "liked"}, {Title: "最近播放", Page: "recent", Icon: "star"}, {Title: "音乐云盘", Page: "cloud", Icon: "playlist"},
	}}
}
func (s *phoneProductSource) Can(cap string) bool { return cap != "remotePlay" && cap != "remoteState" }

func phoneProductFixture(t *testing.T) (*nsgotest.App, *app) {
	t.Helper()
	ui, a := startPhone(t)
	var art []string
	if dir := os.Getenv("WOOD_WIND_MOBILE_ART_DIR"); dir != "" {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".jpg" || ext == ".png" || ext == ".jpeg" {
				art = append(art, music.FileURL(filepath.Join(dir, e.Name())))
			}
		}
	}
	if len(art) == 0 {
		_, file, _, _ := runtime.Caller(0)
		art = []string{music.FileURL(filepath.Join(filepath.Dir(file), "../../resources/icon.png"))}
	}
	sort.Strings(art)
	cover := func(i int) string { return art[i%len(art)] }
	tracks := []music.Track{
		{ID: "light", Source: "phone-product", Name: "最后一卷胶片", Artists: []music.Artist{{ID: "artist", Name: "买辣椒也用券"}}, Album: music.Album{ID: "album", Name: "最后一卷胶片", ImageURL: cover(0)}, Duration: 216000},
		{ID: "wind", Source: "phone-product", Name: "在你的身边", Artists: []music.Artist{{Name: "盛哲"}}, Album: music.Album{ImageURL: cover(1)}, Duration: 253000},
		{ID: "summer", Source: "phone-product", Name: "夏夜晚风", Artists: []music.Artist{{Name: "伍佰 & China Blue"}}, Album: music.Album{ImageURL: cover(2)}, Duration: 228000},
	}
	lists := []music.Playlist{
		{ID: "evening", Name: "留给傍晚的歌", ImageURL: cover(3), TrackCount: 28, Mine: true, Tracks: tracks},
		{ID: "travel", Name: "开车的时候听", ImageURL: cover(4), TrackCount: 46, Mine: true, Tracks: tracks},
		{ID: "quiet", Name: "一个人的安静时刻", ImageURL: cover(5), TrackCount: 19, Mine: true, Tracks: tracks},
	}
	a.post(func() {
		a.reloadGen++
		a.homeBusy = false
		a.playlistsBusy = false
		src := &phoneProductSource{Source: a.local}
		a.sources.items[src.Info().ID] = src
		a.active = src
		a.set.Language = i18n.Chinese
		a.applyLanguage()
		a.setAccount(src, &music.Account{ID: "listener", Name: "听风", ImageURL: cover(6), Page: "me"})
		a.home = &music.Browse{Title: "酷狗概念版", Sections: []music.Section{
			{Title: "最近播放", MorePage: "recent", Tracks: tracks}, {Title: "每日推荐", MorePage: "daily", Tracks: tracks},
			{Title: "为你精选的歌单", Playlists: lists},
		}}
		a.mine = lists
		a.liked = tracks
		a.recent = tracks
		a.likedSet = map[string]bool{"light": true}
		a.browses["daily"] = &browseData{browse: &music.Browse{Title: "每日推荐", Sections: []music.Section{{Tracks: tracks}}}}
		a.browses["charts"] = &browseData{browse: &music.Browse{Title: "排行榜", Sections: []music.Section{{Links: []music.Link{{Title: "飙升榜", Subtitle: "每天更新 · 发现正在流行的音乐", ImageURL: cover(3), Page: "rank"}, {Title: "TOP 500", Subtitle: "这些好歌值得反复聆听", ImageURL: cover(4), Page: "rank"}}}}}}
		a.playlists["evening"] = &playlistData{playlist: &lists[0]}
		a.albums["album"] = &albumData{album: &music.AlbumPage{Album: tracks[0].Album, Tracks: tracks}}
		a.artists["artist"] = &artistData{page: &music.ArtistPage{Artist: music.Artist{ID: "artist", Name: "买辣椒也用券", ImageURL: cover(0)}, Tracks: tracks, Albums: []music.Album{tracks[0].Album}}}
		a.pl.Cue(tracks)
		a.pl.Hold(42 * time.Second)
		a.pl.SetStream(&music.Stream{Type: "flac", Quality: "lossless"})
		a.set.SearchHistory = []string{"陈奕迅", "夏夜晚风", "MyGO!!!!!"}
		r := route{page: pageSearch, src: src.Info().ID, query: "晚风", kind: music.SearchTracks}
		a.searches[r] = &searchData{loaded: true, res: music.SearchResult{Tracks: tracks}}
		a.route.Set(route{page: pageHome, src: src.Info().ID})
		a.phoneTabs = [3]phoneTabState{{route: route{page: pageHome, src: src.Info().ID}}, {route: route{page: pageLibrary, src: src.Info().ID}}, {route: route{page: pageSearch, src: src.Info().ID}}}
		a.playRev.Update(func(n int) int { return n + 1 })
		a.bump()
	})
	ui.Settle()
	return ui, a
}

func TestPhoneProductInformationHierarchy(t *testing.T) {
	ui, a := phoneProductFixture(t)
	ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.15, SafeArea: core.Edges{Top: 24, Bottom: 24}})
	if label := ui.Find("id:daily").Props.Label; label != "每日推荐" {
		t.Fatalf("history displaced recommendations: %q", label)
	}
	hero := reviewBounds(ui.Find("id:daily"))
	if hero.Size.Height > 195 {
		t.Fatalf("feature dominates the phone: %+v", hero)
	}
	phoneWithin(t, ui, "track-light", 390, 844)
	ui.Press("id:phone-tab-library")
	phoneWithin(t, ui, "playlist-evening", 390, 844)
	phoneWithin(t, ui, "phone-sources", 390, 844)
	phoneWithin(t, ui, "phone-account", 390, 844)
	if ui.Exists("id:source-nav-0") {
		t.Fatal("library repeats the Discover destination")
	}
	ui.Press("id:phone-account")
	ui.Find("id:open-settings")
	ui.Back()
	ui.Press("id:phone-tab-search")
	ui.Find("text:今天想听些什么？")
	a.post(func() {
		a.query.Set("晚风")
		a.route.Set(route{page: pageSearch, src: a.source().Info().ID, query: "晚风", kind: music.SearchTracks})
		a.bump()
	})
	ui.Settle()
	ui.Find("id:search-tab-tracks")
}

func TestPhoneProductShots(t *testing.T) {
	if os.Getenv("WOOD_WIND_PRODUCT_SHOTS") == "" {
		t.Skip("set WOOD_WIND_PRODUCT_SHOTS=1 for native mobile visual review")
	}
	ui, a := phoneProductFixture(t)
	ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.15, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})
	ui.Resize(369, 820)
	ui.Advance(time.Second)
	for _, dark := range []bool{false, true} {
		a.post(func() {
			if dark {
				a.cx.SetAppearance(core.AppearanceDark)
			} else {
				a.cx.SetAppearance(core.AppearanceLight)
			}
		})
		ui.Settle()
		suffix := "light"
		if dark {
			suffix = "dark"
		}
		ui.Screenshot("product-discover-" + suffix)
		ui.Press("id:phone-tab-library")
		ui.Advance(200 * time.Millisecond)
		ui.Screenshot("product-library-" + suffix)
		ui.Press("id:phone-tab-search")
		ui.Screenshot("product-search-" + suffix)
		a.post(func() {
			a.query.Set("晚风")
			a.route.Set(route{page: pageSearch, src: a.source().Info().ID, query: "晚风", kind: music.SearchTracks})
			a.bump()
		})
		ui.Settle()
		ui.Screenshot("product-search-results-" + suffix)
		a.post(func() { a.route.Set(route{page: pagePlaylist, id: "evening", src: a.source().Info().ID}); a.bump() })
		ui.Settle()
		ui.Screenshot("product-playlist-" + suffix)
		a.post(func() { a.route.Set(route{page: pageBrowse, id: "charts", src: a.source().Info().ID}); a.bump() })
		ui.Settle()
		ui.Screenshot("product-charts-" + suffix)
		a.post(func() { a.route.Set(route{page: pageHome, src: a.source().Info().ID}); a.phoneTab = 0; a.bump() })
		ui.Settle()
	}
	a.post(func() { a.cx.SetAppearance(core.AppearanceLight); a.openNowPlaying() })
	ui.Settle()
	ui.Screenshot("product-player")
	ui.Press("id:np-queue")
	ui.Screenshot("product-queue")
	ui.Back()
	ui.Back()
	a.post(func() { a.openPhoneSheet("sources") })
	ui.Settle()
	ui.Screenshot("product-sources")
	ui.Back()
	ui.Resize(320, 568)
	ui.SetEnvironment(core.Environment{OS: core.OSAndroid, Locale: "zh-CN", FontScale: 1.5, ReduceMotion: true, SafeArea: core.Edges{Top: 24, Bottom: 24}})
	ui.Screenshot("product-small-large-type")
}
