package app

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"github.com/HengWh/woodwind-music/internal/music"
)

func TestSearchInputKeepsFocusAndText(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{Locale: "en", OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	before := ui.Find("id:search-input").ID
	ui.Focus("id:search-input")
	if field := ui.Find("id:search-input"); field.ID != before || !field.Focused {
		t.Fatal("focusing search replaced the input or lost focus")
	}
	for _, char := range []string{"春", "日", " ", "影"} {
		ui.Type("id:search-input", char)
	}
	if field := ui.Find("id:search-input"); field.Text != "春日 影" || !field.Focused {
		t.Fatalf("search input lost text or focus: %q, focused=%v", field.Text, field.Focused)
	}
	a.bump()
	ui.Settle()
	if field := ui.Find("id:search-input"); field.ID != before || field.Text != "春日 影" || !field.Focused {
		t.Fatal("a library/sidebar refresh replaced the active search input")
	}
}

func TestSearchSubmitsAndFindsEveryKind(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	// The native test runner starts its binary from its build directory.
	_, source, _, _ := runtime.Caller(0)
	t.Chdir(filepath.Dir(source))
	writeTaggedMP3(t, filepath.Join(dir, "spring.mp3"), "春日影", "MyGO!!!!!", "迷跡波")
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{Locale: "en", OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	a.local.AddFolders(dir)
	ui.WaitUntil(5*time.Second, func() bool { return a.local.TrackCount() == 1 && !a.local.Scanning() && !a.homeBusy })
	if _, err := a.local.CreatePlaylist("MyGO Live"); err != nil {
		t.Fatal(err)
	}
	// Use the actual input and Return, including whitespace and Unicode.
	ui.Type("id:search-input", "  春日影  ")
	ui.Submit("id:search-input")
	ui.WaitFor(`id:app-content >> text:"春日影"`, 2*time.Second)
	if field := ui.Find("id:search-input"); field.Text != "春日影" || !field.Focused {
		t.Fatalf("submitted field = %q, focused=%v", field.Text, field.Focused)
	}
	ui.Press(`id:app-content >> text:"春日影"`)
	ui.WaitUntil(3*time.Second, func() bool { return a.pl.Current() != nil && a.pl.Current().Name == "春日影" })
	if a.pl.Playing() {
		a.pl.TogglePlay()
	}
	for _, test := range []struct {
		query, result string
		kind          music.SearchKind
	}{
		{"mygo", "MyGO!!!!!", music.SearchArtists},
		{"迷跡", "迷跡波", music.SearchAlbums},
		{"mygo", "MyGO Live", music.SearchPlaylists},
	} {
		ui.Clear("id:search-input")
		ui.Type("id:search-input", test.query)
		ui.Submit("id:search-input")
		ui.Press("id:search-tab-" + string(test.kind))
		ui.WaitUntil(2*time.Second, func() bool {
			d := a.searches[a.route.GetUntracked()]
			return a.route.GetUntracked().kind == test.kind && d != nil && d.loaded && d.res.Len() == 1
		})
		ui.Find(`id:app-content >> text:"` + test.result + `" >> nth=0`)
	}
	ui.Clear("id:search-input")
	ui.Type("id:search-input", "no such song")
	ui.Submit("id:search-input")
	ui.WaitFor("text:No songs found", 2*time.Second)
	ui.Press("id:clear-search")
	if a.route.GetUntracked().page != pageHome || ui.Find("id:search-input").Text != "" || ui.Exists("id:search-suggestions") {
		t.Fatal("clearing search did not clear the input and dismiss results/suggestions")
	}
}

func TestSearchRespectsInputMethodComposition(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{Locale: "en", OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	ui.Focus("id:search-input")
	ui.Compose("id:search-input", "chun")
	ui.Submit("id:search-input")
	if a.route.GetUntracked().page != pageHome || len(a.set.SearchHistory) != 0 {
		t.Fatal("confirming an input-method candidate submitted a search")
	}
	// Return may commit the marked text itself on a native text client.
	// Start a fresh composition to verify candidate replacement separately.
	ui.Compose("id:search-input", "")
	ui.Clear("id:search-input")
	ui.Compose("id:search-input", "chun")
	ui.Commit("id:search-input", "春")
	ui.Type("id:search-input", "日影")
	ui.Submit("id:search-input")
	if a.route.GetUntracked().query != "春日影" {
		t.Fatalf("committed Chinese text was not searched: route=%+v, query=%q, field=%q\n%s", a.route.GetUntracked(), a.query.GetUntracked(), ui.Find("id:search-input").Text, ui.Dump())
	}
}

func TestSearchHistoryKeyboardAndDismiss(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{Locale: "en", OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	a.set.SearchHistory = []string{"Haru", "春日影"}
	a.bump()
	ui.Focus("id:search-input")
	ui.Find("id:search-suggestions")
	ui.Key("down")
	ui.Key("down")
	ui.Submit("id:search-input")
	if a.route.GetUntracked().query != "春日影" || a.set.SearchHistory[0] != "春日影" || ui.Find("id:search-input").Text != "春日影" {
		t.Fatal("keyboard history selection did not submit/synchronize the query")
	}
	ui.Clear("id:search-input")
	ui.Type("id:search-input", "ha")
	ui.Find("text:Haru")
	ui.Key("escape")
	if ui.Exists("id:search-suggestions") || ui.Find("id:search-input").Text != "ha" {
		t.Fatal("Escape should dismiss suggestions and keep the draft")
	}
	ui.Key("down")
	// Choose a navigation item below the popup, so this is an actual
	// outside click rather than a click on a suggestion covering Home.
	ui.Press("id:nav-liked")
	ui.Press("id:nav-home")
	if ui.Exists("id:search-suggestions") {
		t.Fatal("clicking outside search did not dismiss suggestions")
	}
	ui.Clear("id:search-input")
	ui.Key("down")
	ui.Press("text:Clear Recent Searches")
	if len(a.set.SearchHistory) != 0 || ui.Exists("id:search-suggestions") {
		t.Fatal("clearing recent searches did not update and close the popup")
	}
	ui.Type("id:search-input", "   ")
	ui.Submit("id:search-input")
	if len(a.set.SearchHistory) != 0 || a.route.GetUntracked().page != pageHome {
		t.Fatalf("an empty search should not search or enter history: route=%+v, history=%v, field=%q", a.route.GetUntracked(), a.set.SearchHistory, ui.Find("id:search-input").Text)
	}
}

func TestSearchNavigationAndShortcut(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{Locale: "en", OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	ui.Key("mod+f")
	if !ui.Find("id:search-input").Focused {
		t.Fatal("Find did not focus search")
	}
	ui.Type("id:search-input", "first")
	ui.Submit("id:search-input")
	ui.Press("id:search-tab-albums")
	ui.Clear("id:search-input")
	ui.Type("id:search-input", "second")
	ui.Submit("id:search-input")
	ui.Key("mod+[")
	if a.route.GetUntracked().query != "first" || ui.Find("id:search-input").Text != "first" || a.lastSearch.kind != music.SearchAlbums {
		t.Fatal("Back did not restore search text and category")
	}
	ui.Key("mod+]")
	if a.route.GetUntracked().query != "second" || ui.Find("id:search-input").Text != "second" {
		t.Fatal("Forward did not restore search text")
	}
	ui.Key("tab")
	ui.Key("mod+f")
	if !ui.Find("id:search-input").Focused || a.route.GetUntracked().query != "second" {
		t.Fatalf("Find on a results page did not refocus the existing query: route=%+v, field=%q, focused=%v", a.route.GetUntracked(), ui.Find("id:search-input").Text, ui.Find("id:search-input").Focused)
	}
	ui.Press("id:nav-home")
	ui.Press("id:nav-results")
	if ui.Find("id:search-input").Text != "second" {
		t.Fatal("Search Results did not restore the submitted query")
	}
}

func TestSearchLoadsAdditionalResults(t *testing.T) {
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		return a.view(cx)
	}, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{Locale: "en", OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy })
	for i := range 205 {
		if _, err := a.local.CreatePlaylist(fmt.Sprintf("Search Page %03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	ui.Type("id:search-input", "Search Page")
	ui.Submit("id:search-input")
	ui.Press("id:search-tab-playlists")
	r := a.route.GetUntracked()
	ui.WaitUntil(3*time.Second, func() bool { return a.searches[r] != nil && a.searches[r].loaded })
	d := a.searches[r]
	if d.res.Len() != 200 || !d.more {
		t.Fatalf("first page = %d results, more=%v", d.res.Len(), d.more)
	}
	ui.Press("id:search-load-more")
	ui.WaitUntil(3*time.Second, func() bool { return !d.loading && d.res.Len() == 205 })
	if d.more || ui.Exists("id:search-load-more") {
		t.Fatal("last search page still offers more results")
	}
	seen := map[string]bool{}
	for _, playlist := range d.res.Playlists {
		if seen[playlist.ID] {
			t.Fatal("pagination duplicated a result")
		}
		seen[playlist.ID] = true
	}
	ui.Press("id:nav-home")
	ui.Press("id:nav-results")
	if restored := a.searches[r]; restored != d || restored.res.Len() != 205 || restored.loading {
		t.Fatal("returning to search discarded the additional results")
	}
}
