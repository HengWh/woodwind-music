package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/offline"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func seedStored(t *testing.T, dir string) []music.Track {
	t.Helper()
	return seedStoredTracks(t, dir, []music.Track{{ID: "same", Source: "source-one", Name: "Saved First", Album: music.Album{ID: "album", Name: "Same Album"}, Artists: []music.Artist{{ID: "artist", Name: "First Artist"}}}, {ID: "same", Source: "source-two", Name: "Saved Second", Album: music.Album{ID: "album", Name: "Same Album"}, Artists: []music.Artist{{ID: "artist", Name: "Second Artist"}}}})
}

func seedStoredTracks(t *testing.T, dir string, tracks []music.Track) []music.Track {
	t.Helper()
	b, err := os.ReadFile("../player/testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, "songs"), 0o700)
	for _, tr := range tracks {
		base := filepath.Join(dir, "songs", offline.Key(tr.Source, tr.ID)+"-high")
		if err = os.WriteFile(base+".mp3", b, 0o600); err != nil {
			t.Fatal(err)
		}
		meta, _ := json.Marshal(music.Stream{Type: "mp3", Quality: "high", Size: int64(len(b))})
		os.WriteFile(base+".json", meta, 0o600)
	}
	s, err := offline.Open(dir, 64<<20, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, tr := range tracks {
		if err = s.Remember(tr, tr.Source); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.Pin(tracks[0], "", false); err != nil {
		t.Fatal(err)
	}
	return tracks
}

func TestSavedMusicBrowseSearchAndRetentionAcrossPlatforms(t *testing.T) {
	for _, platform := range []core.OS{core.OSMacOS, core.OSWindows, core.OSAndroid} {
		t.Run(string(platform), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", dir)
			tracks := seedStored(t, dir)
			var a *app
			width, height := 1180, 780
			if platform == core.OSAndroid {
				width, height = 390, 844
			}
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(core.Points(width), core.Points(height)), nsgotest.Environment(core.Environment{OS: platform, Locale: "en"}))
			ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
			if platform == core.OSAndroid {
				ui.Press("id:phone-tab-library")
				ui.Press("id:phone-stored")
			} else {
				ui.Press("id:nav-stored")
			}
			ui.Find("id:stored-page")
			for _, id := range []string{"stored-source", "stored-quality", "stored-complete", "stored-select-all", "stored-select-none", "stored-pin", "stored-unpin", "stored-delete"} {
				if ui.Exists("id:" + id) {
					t.Fatalf("unneeded initial control remains: %s", id)
				}
			}
			ui.Type("id:stored-search", "Saved First")
			ui.WaitUntil(time.Second, func() bool { return len(a.storedFilter(a.disk.List())) == 1 })
			ui.Clear("id:stored-search")
			ui.Type("id:stored-search", "Saved Second")
			ui.WaitUntil(time.Second, func() bool {
				return len(a.storedFilter(a.disk.List())) == 1 && a.storedFilter(a.disk.List())[0].Track.Source == "source-two"
			})
			ui.Clear("id:stored-search")
			if got := storedGroups(a.disk.List(), "albums"); len(got) != 2 {
				t.Fatal("same album ID across plugins was merged")
			}
			ui.Press("id:stored-tab-pinned")
			ui.WaitUntil(time.Second, func() bool { return len(a.storedFilter(a.disk.List())) == 1 })
			if a.source() != a.local {
				t.Fatal("saved music changed online source")
			}
			// Capacity management must preserve the copy explicitly kept by the user.
			if err := a.disk.ClearAutomatic(); err != nil {
				t.Fatal(err)
			}
			ui.Settle()
			list := a.disk.List()
			if len(list) != 1 || list[0].Track.ID != tracks[0].ID || list[0].Track.Source != tracks[0].Source {
				t.Fatal("clear deleted the wrong source's song")
			}
			a.post(func() { a.diskUI.mode = ""; a.diskUI.view = ""; a.diskUI.group = ""; a.bump() })
			ui.Settle()
			ui.Press("id:stored-play-" + list[0].ID)
			ui.WaitUntil(2*time.Second, func() bool { return a.pl.Current() != nil && a.pl.Current().LocalOnly })
			if a.pl.Current().Source != "source-one" {
				t.Fatal("lost original identity")
			}
		})
	}
}

func TestSavedMusicGlobalRouteKeepsSelectedPlugin(t *testing.T) {
	ui, a := startPhone(t)
	src := &phoneProductSource{Source: a.local}
	a.post(func() { a.sources.items[src.Info().ID] = src; a.active = src; a.openStored() })
	ui.Settle()
	if a.source() != src || a.route.GetUntracked().page != pageStored {
		t.Fatal("global saved page activated another source")
	}
	if !ui.Exists("id:stored-page") {
		t.Fatal("saved page was unavailable")
	}
}

func TestSavedMusicTaskMenuWithoutPlugin(t *testing.T) {
	ui, a := startPhone(t)
	a.post(a.openStored)
	ui.Settle()
	ui.Press("id:stored-tasks")
	ui.Find("text:暂无下载任务")
	if len(a.trackMenu(music.Track{ID: "removed", Source: "removed", Name: "Still present"}, "", -1)) == 0 {
		t.Fatal("removed plugin hid local actions")
	}
}

func TestSavedMusicUpdatesStayInsideLibrary(t *testing.T) {
	ui, a := startPhone(t)
	before := a.rev.GetUntracked()
	diskBefore := a.diskRev.GetUntracked()
	if err := a.disk.Remember(music.Track{ID: "metadata", Source: "removed", Name: "Metadata only"}, "Removed"); err != nil {
		t.Fatal(err)
	}
	ui.WaitUntil(time.Second, func() bool { return a.diskRev.GetUntracked() > diskBefore })
	if a.rev.GetUntracked() != before {
		t.Fatal("storage changes rebuilt unrelated pages")
	}
	a.post(a.openStored)
	ui.Settle()
	ui.Press("id:stored-tasks")
	if err := a.disk.Pin(music.Track{ID: "pending", Source: "removed", Name: "Waiting Song"}, "high", false); err != nil {
		t.Fatal(err)
	}
	ui.WaitFor("text:等待插件恢复", time.Second)
}

func TestPhoneNavigationKeepsButtonsDuringLibraryRefresh(t *testing.T) {
	ui, a := startPhone(t)
	before := ui.Find("id:phone-tab-library").ID
	a.post(a.bump)
	ui.Settle()
	if after := ui.Find("id:phone-tab-library").ID; after != before {
		t.Fatal("a content refresh recreated navigation buttons")
	}
	ui.Press("id:phone-tab-library")
	if a.route.GetUntracked().page != pageLibrary {
		t.Fatal("navigation did not reach library")
	}
}

func TestSavedMusicLongListScrollsAcrossPlatforms(t *testing.T) {
	for _, platform := range []core.OS{core.OSMacOS, core.OSWindows, core.OSAndroid} {
		t.Run(string(platform), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", dir)
			tracks := make([]music.Track, 80)
			for i := range tracks {
				tracks[i] = music.Track{ID: fmt.Sprintf("saved-%03d", i), Source: "removed", Name: fmt.Sprintf("Saved %03d", i)}
			}
			seedStoredTracks(t, dir, tracks)
			var a *app
			width := 1180
			if platform == core.OSAndroid {
				width = 390
			}
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a = newApp(cx)
				a.diskUI.sort = "title"
				return a.view(cx)
			}, nsgotest.Size(core.Points(width), 780), nsgotest.Environment(core.Environment{OS: platform, Locale: "en"}))
			ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
			a.post(a.openStored)
			ui.Settle()
			scroll := ui.Find("id:stored-scroll")
			page := reviewBounds(ui.Find("id:stored-page"))
			bounds := reviewBounds(scroll)
			if bounds.Size.Height <= 0 || bounds.Origin.Y+bounds.Size.Height > page.Origin.Y+page.Size.Height+1 {
				t.Fatalf("scroll viewport escaped page: scroll=%+v page=%+v content=%+v", bounds, page, scroll.Content)
			}
			end := scroll.Content.Height - scroll.Frame.Size.Height
			if end < 1000 {
				t.Fatalf("long list has no scrollable content: %v, records=%d scroll=%+v content=%+v", end, len(a.disk.List()), bounds, scroll.Content)
			}
			for y := core.Points(400); y < end; y += 400 {
				ui.ScrollTo("id:stored-scroll", core.Point{Y: y})
			}
			scroll = ui.Find("id:stored-scroll")
			ui.ScrollTo("id:stored-scroll", core.Point{Y: scroll.Content.Height - scroll.Frame.Size.Height})
			ui.Find(`label:"Saved 079"`)
			lastID := offline.Key(tracks[79].Source, tracks[79].ID)
			before := ui.Find("id:stored-scroll").Offset.Y
			ui.Press("id:stored-select-" + lastID)
			if after := ui.Find("id:stored-scroll").Offset.Y; after < before-1 {
				t.Fatalf("selecting a recycled row reset scroll: %v -> %v", before, after)
			}
			if selected := a.selectedStored(a.disk.List()); len(selected) != 1 || selected[0].Track.ID != tracks[79].ID {
				t.Fatalf("recycled row selected the wrong song: %+v", selected)
			}
			ui.ScrollTo("id:stored-scroll", core.Point{})
			ui.Find(`label:"Saved 000"`)
			ui.Press("id:stored-enqueue")
			if queue := a.pl.Queue(); len(queue) != 1 || queue[0].ID != tracks[79].ID || !queue[0].LocalOnly {
				t.Fatalf("batch action did not target only the selected saved song: %+v", queue)
			}
			ui.Type("id:stored-search", "Saved 000")
			if ui.Find("id:stored-scroll").Offset.Y != 0 || len(a.selectedStored(a.disk.List())) != 0 {
				t.Fatal("search did not reset scroll and selection")
			}
			if ui.Exists("id:stored-enqueue") {
				t.Fatal("batch actions remained visible without a selection")
			}
		})
	}
}
