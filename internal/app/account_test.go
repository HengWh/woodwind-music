package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/plugin"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func installAccountPlugin(t *testing.T, dir string, signedIn bool) {
	t.Helper()
	path := filepath.Join(dir, "plugins", "account-test")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"plugin.json": `{"id":"account-test","name":"Account Test","main":"index.js"}`,
		"index.js": `
const me = {id:"listener",name:"Account Listener",page:"me"};
export function account(){return host.storage.get("account") || null;}
export function signIn(){host.storage.set("account",me);return me;}
export function signOut(){host.storage.remove("account");}
export function home(){return {title:"Home",sections:[]};}
export function browse({page}){const n=(host.storage.get("visits")||0)+1;host.storage.set("visits",n);return {title:page+" "+n,sections:[]};}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(path, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if signedIn {
		p, err := plugin.Load(path, filepath.Join(dir, "plugin-data"))
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()
		if _, err := p.SignIn(); err != nil {
			t.Fatal(err)
		}
	}
}

// A persisted login used to disappear from the UI on restart; its menu must
// restore, follow its source's profile link, and refresh that page on reopening.
func TestPluginAccountRestoredAndMyPage(t *testing.T) {
	for _, phone := range []bool{false, true} {
		name := "desktop"
		if phone {
			name = "phone"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("WOODWIND_MUSIC_DATA_DIR", dir)
			installAccountPlugin(t, dir, true)
			settings := loadSettings(dir)
			settings.Source = "account-test"
			settings.save()
			var a *app
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(1180, 780))
			defer a.closePlugins()
			ui.WaitUntil(2*time.Second, func() bool { return a.account != nil && !a.homeBusy })
			if phone {
				ui.Resize(390, 844)
				ui.Press("id:phone-tab-library")
			}
			if ui.Find("id:account-name").Text != a.account.Name {
				t.Fatal("stored account did not reach the account row")
			}
			ui.Press("id:account-menu")
			ui.Key("escape")
			ui.WaitForGone("id:account-my-page", time.Second)
			for visit := range 2 {
				ui.Press("id:account-menu")
				ui.Press("id:account-my-page")
				ui.WaitUntil(time.Second, func() bool {
					d := a.browses["me"]
					return d != nil && d.browse != nil && !d.loading && d.browse.Title == []string{"me 1", "me 2"}[visit]
				})
				if r := a.route.GetUntracked(); r.page != pageBrowse || r.id != "me" || r.src != "account-test" {
					t.Fatalf("profile opened the wrong source: %+v", r)
				}
				a.post(func() {
					if phone {
						a.open(route{page: pageLibrary})
					} else {
						a.open(route{page: pageHome})
					}
				})
				ui.Settle()
			}
			if os.Getenv("WOODWIND_ACCOUNT_SHOTS") != "" {
				ui.Press("id:account-menu")
				ui.Screenshot("account-" + name)
			}
		})
	}
}

// Signing in from another plugin's settings must select that source; signing
// out through its account menu must clear its stored login and restore Sign In.
func TestPluginAccountSignInFromSettingsAndSignOut(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", dir)
	installAccountPlugin(t, dir, false)
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(1180, 780), nsgotest.Environment(core.Environment{OS: core.OSWindows}))
	defer a.closePlugins()
	a.post(func() { a.settingsInMain = true; a.settingsPage = "plugins"; a.bump() })
	ui.WaitUntil(time.Second, func() bool { return ui.Exists("id:plugin-signin-account-test") })
	ui.Press("id:plugin-signin-account-test")
	ui.WaitUntil(time.Second, func() bool { return a.account != nil && !a.signingIn && !a.homeBusy })
	if a.source().Info().ID != "account-test" || !ui.Exists("id:plugin-signout-account-test") || ui.Exists("id:plugin-signin-account-test") {
		t.Fatal("signing in from settings did not select or update the source")
	}
	a.post(a.closeSettings)
	ui.Settle()
	ui.Press("id:account-menu")
	ui.Press("id:account-signout")
	ui.WaitUntil(time.Second, func() bool { return a.account == nil && !a.homeBusy && ui.Exists("id:account-signin") })
	account, err := a.source().Account()
	if err != nil || account != nil {
		t.Fatal("signing out did not clear the plugin login", account, err)
	}
	ui.Press("id:account-signin")
	ui.WaitUntil(time.Second, func() bool { return a.account != nil && !a.signingIn })
	a.post(func() { a.switchSource("local") })
	ui.WaitUntil(time.Second, func() bool { return !a.homeBusy && a.source() == a.local })
	if a.account != nil || ui.Exists("id:account-menu") {
		t.Fatal("plugin account leaked into the local source")
	}
	a.post(func() { a.switchSource("account-test") })
	ui.WaitUntil(time.Second, func() bool { return a.account != nil && !a.homeBusy })
	ui.Press("id:account-menu")
	ui.Press("id:account-settings")
	ui.WaitUntil(time.Second, func() bool { return a.settingsWin != nil || a.settingsInMain })
}

type delayedAccountSource struct {
	music.Source
	started, release chan struct{}
}

func (s *delayedAccountSource) Info() music.Info { return music.Info{ID: "delayed-account"} }
func (s *delayedAccountSource) Can(name string) bool {
	return name == "account" || name == "signOut"
}
func (s *delayedAccountSource) Account() (*music.Account, error) {
	close(s.started)
	<-s.release
	return &music.Account{ID: "old", Name: "Old Account"}, nil
}
func (s *delayedAccountSource) SignOut() error { return nil }

// An account response arriving after switching sources must stay attached to
// its original source instead of replacing the current source's account.
func TestPluginAccountOldRequestCannotReplaceNewSource(t *testing.T) {
	ui, a := startPhone(t)
	src := &delayedAccountSource{Source: a.local, started: make(chan struct{}), release: make(chan struct{})}
	a.post(func() {
		a.sources.items[src.Info().ID] = src
		a.switchSource(src.Info().ID)
	})
	ui.WaitUntil(time.Second, func() bool {
		select {
		case <-src.started:
			return true
		default:
			return false
		}
	})
	a.post(func() { a.switchSource("local") })
	ui.WaitUntil(time.Second, func() bool { return a.source() == a.local && !a.homeBusy })
	close(src.release)
	ui.WaitUntil(time.Second, func() bool { return a.accountState(src).asked })
	if a.account != nil || a.accountState(src).account == nil {
		t.Fatal("late account response replaced the current source's account")
	}
}

// A failed sign-in must leave a usable retry button instead of stranding the
// footer in an account-loading state with no request in flight.
func TestPluginAccountFailedSignInCanRetry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", dir)
	installAccountPlugin(t, dir, false)
	script := `export function account(){return null;} export function signIn(){throw new Error("Sign-in failed");}`
	if err := os.WriteFile(filepath.Join(dir, "plugins", "account-test", "index.js"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(1180, 780))
	defer a.closePlugins()
	a.post(func() { a.switchSource("account-test") })
	ui.WaitUntil(time.Second, func() bool { return ui.Exists("id:account-signin") })
	for range 2 {
		ui.Press("id:account-signin")
		ui.WaitUntil(time.Second, func() bool { return !a.signingIn && a.toast.GetUntracked() == "Sign-in failed" })
		if ui.Find("id:account-signin").Props.Disabled || ui.Exists("id:account-loading") {
			t.Fatal("failed sign-in stranded the account row")
		}
	}
}
