package app

import (
	"fmt"
	"github.com/HengWh/woodwind-music/internal/music"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/plugin"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func demoPluginPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "examples", "demo-plugin")
}

func installDemo(t *testing.T, dir string) {
	t.Helper()
	p, err := plugin.Open(demoPluginPath(), filepath.Join(dir, "plugins"))
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Install(filepath.Join(dir, "plugins")); err != nil {
		t.Fatal(err)
	}
}
func TestPluginSourcesOnDesktopAndPhone(t *testing.T) {
	for _, phone := range []bool{false, true} {
		name := "desktop"
		if phone {
			name = "phone"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("WOODWIND_MUSIC_DATA_DIR", dir)
			installDemo(t, dir)
			var a *app
			width, height := core.Points(1180), core.Points(780)
			environment := core.Environment{OS: core.OSMacOS}
			if phone {
				width, height = 390, 844
				environment = core.Environment{OS: core.OSAndroid, SafeArea: core.Edges{Top: 24, Bottom: 24}}
			}
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(width, height), nsgotest.Environment(environment))
			t.Cleanup(a.closePlugins)
			ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
			if phone {
				ui.Press("id:phone-tab-library")
				ui.Press("id:phone-sources")
				ui.Press("id:source-demo")
			} else {
				ui.Press("id:source-switcher")
				ui.Press("text:Demo")
			}
			ui.WaitUntil(2*time.Second, func() bool { return a.source().Info().ID == "demo" && !a.homeBusy && a.home != nil })
			if len(a.home.Sections) == 0 || a.home.Sections[0].Tracks[0].Source != "demo" {
				t.Fatal("plugin home not stamped with source")
			}
			if phone {
				ui.Press("id:phone-tab-library")
			}
			ui.Find("id:source-nav-1")
			a.post(func() { a.search("song") })
			ui.WaitUntil(2*time.Second, func() bool { d := a.searches[a.route.GetUntracked()]; return d != nil && d.loaded })
			d := a.searches[a.route.GetUntracked()]
			if len(d.res.Tracks) != 3 {
				t.Fatal("search did not use plugin")
			}
			tr := d.res.Tracks[0]
			a.post(func() { a.toggleTrackLike(tr) })
			ui.WaitUntil(2*time.Second, func() bool { return a.likedSet[tr.ID] && !a.homeBusy })
			for _, r := range []route{{page: pageShows}, {page: pageStations}, {page: pageComments, id: tr.ID}} {
				a.post(func() { a.open(r) })
				ui.WaitUntil(2*time.Second, func() bool { d := a.extra[a.route.GetUntracked()]; return d != nil && d.loaded })
				if e := a.extra[a.route.GetUntracked()].err; e != "" {
					t.Fatal(e)
				}
			}
			a.post(func() { a.switchSource("local") })
			ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy && a.source() == a.local })
			if len(a.liked) > 0 {
				t.Fatal("plugin favorites leaked into local source")
			}
			if a.sourceByID("missing") != nil {
				t.Fatal("missing plugin resolved to local source")
			}
			if a.sourceByID("demo") == nil {
				t.Fatal("switching source unloaded player source")
			}
		})
	}
}

func TestPhonePluginInstallReviewAndRemove(t *testing.T) {
	ui, a := startPhone(t)
	a.post(func() {
		a.preparePlugin(func() (*plugin.Pending, error) {
			return plugin.Open(demoPluginPath(), a.pluginDir())
		})
	})
	ui.WaitUntil(2*time.Second, func() bool { return a.installer.pending != nil && !a.installer.busy })
	if len(a.plugins) != 0 {
		t.Fatal("installed before review")
	}
	ui.Press("id:plugin-confirm")
	ui.WaitUntil(2*time.Second, func() bool { return len(a.plugins) == 1 && !a.installer.open })
	ui.Press("id:phone-tab-library")
	ui.Press("id:open-settings")
	ui.Press("id:phone-settings-plugins")
	ui.Find("id:plugin-setting-demo/greeting")
	ui.Type("id:plugin-setting-demo/greeting", "Hello Woodwind")
	ui.Press("id:plugin-save-demo/greeting")
	ui.WaitUntil(time.Second, func() bool { return a.plugins[0].Setting("greeting") == "Hello Woodwind" })
	a.post(func() { a.installer.open = true; a.installer.remove = "demo"; a.bump() })
	ui.Settle()
	ui.Press("id:plugin-remove-confirm")
	ui.WaitUntil(2*time.Second, func() bool { return len(a.plugins) == 0 && !a.installer.busy })
	if _, err := os.Stat(filepath.Join(a.pluginDataDir(), "demo", "settings.json")); err != nil {
		t.Fatal("uninstall discarded settings", err)
	}
}

func TestPluginOldSearchCannotReplaceNewSource(t *testing.T) {
	ui, a := startPhone(t)
	// A real async JS plugin deliberately finishes after switching sources.
	dir := filepath.Join(a.pluginDir(), "slow")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id":"slow"}`), 0600)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(`let finished=false;export function account(){return {id:finished?"finished":"waiting"}};export async function search() { await new Promise(r=>setTimeout(r,80)); finished=true; return {tracks:[{id:"same",name:"Old Source"}]}; }`), 0600)
	p, err := plugin.Load(dir, a.pluginDataDir())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	a.post(func() {
		a.plugins = append(a.plugins, p)
		a.registerPlugin(p)
		a.switchSource("slow")
		a.search("same")
		a.switchSource("local")
		a.search("same")
	})
	ui.WaitUntil(2*time.Second, func() bool { d := a.searches[a.route.GetUntracked()]; return d != nil && d.loaded })
	ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
	ui.WaitUntil(time.Second, func() bool { account, err := p.Account(); return err == nil && account.ID == "finished" })
	if a.source() != a.local || len(a.searches[a.route.GetUntracked()].res.Tracks) != 0 {
		t.Fatal("old plugin result replaced local search")
	}
}

func TestAndroidPluginAuthenticatedStream(t *testing.T) {
	ui, a := startPhone(t)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer song-token" {
			t.Error("missing stream token")
		}
		if r.Header.Get("Range") != "bytes=8-15" {
			t.Error("missing seek range")
		}
		w.Header().Set("Content-Range", "bytes 8-15/100")
		w.WriteHeader(206)
		w.Write([]byte("abcdefgh"))
	}))
	defer origin.Close()
	dir := filepath.Join(a.pluginDir(), "private")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id":"private"}`), 0600)
	script := fmt.Sprintf(`export function stream(){return {url:%q,type:"mp3",headers:{Authorization:"Bearer song-token"}}}`, origin.URL)
	os.WriteFile(filepath.Join(dir, "index.js"), []byte(script), 0600)
	p, err := plugin.Load(dir, a.pluginDataDir())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	a.post(func() {
		a.plugins = append(a.plugins, p)
		a.registerPlugin(p)
		a.playTracks([]music.Track{{ID: "song", Source: "private", Name: "Private Song"}}, 0)
	})
	ui.WaitUntil(time.Second, func() bool { return a.video.request.GetUntracked() != nil })
	video := ui.FindMounted("id:android-audio")
	if strings.Contains(video.Video.Source, "song-token") || !strings.HasPrefix(video.Video.Source, "http://127.0.0.1:") {
		t.Fatal("native player received unadapted stream", video.Video.Source)
	}
	request, _ := http.NewRequest("GET", video.Video.Source, nil)
	request.Header.Set("Range", "bytes=8-15")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 206 {
		t.Fatal(response.Status)
	}
	ui.Report(nsgotest.Event{Kind: nsgotest.EventLoad, ID: video.ID, Duration: time.Minute})
	ui.WaitUntil(time.Second, a.pl.Ready)
	a.post(func() { a.openSettings() })
	ui.Settle()
	if current := ui.FindMounted("id:android-audio"); current.ID != video.ID {
		t.Fatal("settings replaced plugin audio node")
	}
	a.post(a.pl.Clear)
	ui.Settle()
	if response, err := http.Get(video.Video.Source); err == nil {
		response.Body.Close()
		t.Fatal("clearing playback left the relay open")
	}
}

func TestPluginShots(t *testing.T) {
	if os.Getenv("WOODWIND_PLUGIN_SHOTS") == "" {
		t.Skip("set WOODWIND_PLUGIN_SHOTS=1 for visual review")
	}
	dir := t.TempDir()
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", dir)
	installDemo(t, dir)
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		a = newApp(cx)
		a.set.Language = "zh-CN"
		a.applyLanguage()
		return a.view(cx)
	}, nsgotest.Size(1180, 780))
	defer a.closePlugins()
	a.post(func() { a.switchSource("demo") })
	ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy && a.home != nil })
	ui.Screenshot("plugins-desktop")
	ui.Resize(390, 844)
	ui.Press("id:phone-tab-library")
	ui.Screenshot("plugins-phone-library")
	a.post(func() { a.settingsPage = "plugins"; a.settingsInMain = true; a.bump() })
	ui.Settle()
	ui.Screenshot("plugins-phone-settings")
	ui.Resize(320, 568)
	ui.Screenshot("plugins-phone-settings-small")
	category := reviewBounds(ui.Find("id:phone-settings-plugins"))
	content := reviewBounds(ui.Find("id:settings-content"))
	if category.Origin.Y+category.Size.Height > content.Origin.Y+1 {
		t.Fatalf("categories overlap content: %+v / %+v", category, content)
	}
}

func TestDesktopPluginPlaybackErrorIsVisible(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", dir)
	source := filepath.Join(dir, "plugins", "restricted")
	os.MkdirAll(source, 0700)
	os.WriteFile(filepath.Join(source, "plugin.json"), []byte(`{"id":"restricted"}`), 0600)
	os.WriteFile(filepath.Join(source, "index.js"), []byte(`export function stream(){const e=new Error("This account cannot play the full track");e.code="unavailable";throw e;}`), 0600)
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(1180, 780))
	defer a.closePlugins()
	a.post(func() { a.playTracks([]music.Track{{ID: "sample", Source: "restricted", Name: "Restricted track"}}, 0) })
	ui.WaitUntil(time.Second, func() bool { return a.pl.Err() != "" })
	if ui.Find("id:bar-error").Text != "This account cannot play the full track" {
		t.Fatal("plugin's access explanation was hidden")
	}
	ui.Press("id:bar-open")
	if ui.Find("id:np-status").Text != "This account cannot play the full track" {
		t.Fatal("Now Playing hid the playback error")
	}
}

// A custom menu trigger must still have a popup surface. Check the visible
// geometry as well as selection, since a transparent menu remains clickable.
func TestSourceSwitcherMenuSurface(t *testing.T) {
	for _, platform := range []struct {
		name string
		env  core.Environment
	}{
		{"macos", core.Environment{OS: core.OSMacOS, TitleBar: core.Edges{Top: 40, Left: 78}}},
		{"windows", core.Environment{OS: core.OSWindows, TitleBar: core.Edges{Top: 40, Right: 100}}},
	} {
		for _, appearance := range []string{"light", "dark"} {
			t.Run(platform.name+"/"+appearance, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("WOODWIND_MUSIC_DATA_DIR", dir)
				installDemo(t, dir)
				var a *app
				ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
					a = newApp(cx)
					a.set.Appearance = appearance
					if appearance == "dark" {
						cx.SetAppearance(core.AppearanceDark)
					} else {
						cx.SetAppearance(core.AppearanceLight)
					}
					return a.view(cx)
				}, nsgotest.Size(880, 580), nsgotest.Environment(platform.env))
				defer a.closePlugins()
				ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
				ui.Press("id:source-switcher")
				menu := ui.Find("role:menu")
				popup, trigger := reviewBounds(menu), reviewBounds(ui.Find("id:source-switcher"))
				// The card used to push search down; the Mac selector must instead
				// fit beside the window controls and leave the search row clear.
				search := reviewBounds(ui.Find("id:search-box"))
				if trigger.Origin.Y+trigger.Size.Height > search.Origin.Y || trigger.Origin.X+trigger.Size.Width > sidebarWidth-10 {
					t.Fatalf("source selector overlaps search or leaves sidebar: trigger=%+v search=%+v", trigger, search)
				}
				if nsgotest.Target() == "" && platform.env.TitleBar.Left > 0 && (trigger.Origin.X < platform.env.TitleBar.Left+12 || trigger.Origin.Y+trigger.Size.Height > platform.env.TitleBar.Top) {
					t.Fatalf("source selector does not fit beside window controls: %+v", trigger)
				}
				if menu.Style.Background&0xff != 0xff || menu.Style.BorderWidth <= 0 {
					t.Fatal("source menu has no opaque popup surface")
				}
				if popup.Size.Width < trigger.Size.Width || popup.Origin.Y < trigger.Origin.Y+trigger.Size.Height || popup.Origin.X+popup.Size.Width > 880 {
					t.Fatalf("source menu is misplaced: popup=%+v trigger=%+v", popup, trigger)
				}
				for _, id := range []string{"source-option-local", "source-option-demo"} {
					option := reviewBounds(ui.Find("id:" + id))
					if option.Origin.X < popup.Origin.X || option.Origin.Y < popup.Origin.Y || option.Origin.X+option.Size.Width > popup.Origin.X+popup.Size.Width || option.Origin.Y+option.Size.Height > popup.Origin.Y+popup.Size.Height {
						t.Fatalf("%s escapes its popup: option=%+v popup=%+v", id, option, popup)
					}
				}
				if !ui.Find("id:source-option-local").Props.Checked {
					t.Fatal("current source is not marked")
				}
				if os.Getenv("WOODWIND_PLUGIN_SHOTS") != "" {
					ui.Screenshot("source-menu-" + platform.name + "-" + appearance)
				}
				ui.Key("escape")
				ui.WaitForGone("role:menu", time.Second)
				ui.Press("id:source-switcher")
				ui.Press("id:source-option-demo")
				ui.WaitUntil(time.Second, func() bool { return a.source().Info().ID == "demo" && !a.homeBusy })
				ui.WaitForGone("role:menu", time.Second)
				ui.Press("id:source-switcher")
				if !ui.Find("id:source-option-demo").Props.Checked {
					t.Fatal("selected plugin is not marked after switching")
				}
				ui.Key("escape")
			})
		}
	}
}
