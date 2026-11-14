package app

import (
	"errors"
	"net/url"
	"path/filepath"
	"sync"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/local"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/plugin"
	"github.com/nsgo-dev/nsgo/core"
)

// The player asks for sources off the UI goroutine as well as on it.
type sourceRegistry struct {
	sync.RWMutex
	items map[string]music.Source
}

func (a *app) sourceByID(id string) music.Source {
	if id == "" || id == local.ID {
		return a.local
	}
	a.sources.RLock()
	defer a.sources.RUnlock()
	return a.sources.items[id]
}
func (a *app) source() music.Source {
	if a.active != nil {
		return a.active
	}
	return a.local
}
func (a *app) sourceName() string {
	if a.source() == a.local {
		return i18n.T("Local Files")
	}
	return a.source().Info().Name
}
func (a *app) can(name string) bool  { return a.source() != nil && a.source().Can(name) }
func (a *app) pluginDir() string     { return filepath.Join(a.dir, "plugins") }
func (a *app) pluginDataDir() string { return filepath.Join(a.dir, "plugin-data") }

func (a *app) initSources() {
	a.sources.items = map[string]music.Source{}
	a.extra = map[route]*extraData{}
	music.ImageHeaders = a.imageHeaders
	plugin.SetAppVersion(core.App().Version)
	// Advertising an unavailable method prevents plugins from using QR sign-in.
	plugin.SetWebSignIn(nil)
	plugin.SetOpenURL(func(u string) error { a.post(func() { a.cx.OpenURL(u, nil) }); return nil })
	a.plugins = plugin.Discover(a.pluginDir(), a.pluginDataDir())
	for _, p := range a.plugins {
		a.registerPlugin(p)
	}
	a.active = a.local
	if src := a.sourceByID(a.set.Source); src != nil {
		a.active = src
	}
	if a.active != a.local {
		id := a.active.Info().ID
		a.route.Set(route{page: pageHome, src: id})
		for i := range a.phoneTabs {
			a.phoneTabs[i].route.src = id
		}
	}
}
func (a *app) registerPlugin(p *plugin.Plugin) {
	if p.Info().ID == local.ID {
		return
	}
	a.sources.Lock()
	a.sources.items[p.Info().ID] = p
	a.sources.Unlock()
	p.OnSignInPrompt(func(prompt music.SignInPrompt) {
		a.post(func() { a.signPrompt = prompt; a.bump() })
	})
}
func (a *app) closePlugins() {
	for _, p := range a.plugins {
		p.Close()
	}
	if a.installer.pending != nil {
		a.installer.pending.Discard()
	}
}

func (a *app) activateSource(id string) {
	src := a.sourceByID(id)
	if src == nil || src == a.source() {
		return
	}
	if a.trackLikes == nil {
		a.trackLikes = map[string]map[string]bool{}
	}
	a.trackLikes[a.source().Info().ID] = a.likedSet
	a.active = src
	a.set.Source = id
	a.set.save()
	a.reloadGen++
	a.home, a.homeErr, a.recent, a.mine, a.liked = nil, "", nil, nil, nil
	a.account = nil
	a.likedSet = map[string]bool{}
	a.playlists = map[string]*playlistData{}
	a.albums = map[string]*albumData{}
	a.artists = map[string]*artistData{}
	a.browses = map[string]*browseData{}
	a.searches = map[route]*searchData{}
	a.extra = map[route]*extraData{}
	a.lastSearch = route{}
	a.query.Set("")
	a.searchOpen.Set(false)
	a.scroll = map[string]core.Points{}
	a.reload()
	if src.Can("account") {
		go func() {
			account, _ := src.Account()
			a.post(func() {
				if a.source() == src {
					a.account = account
					a.bump()
				}
			})
		}()
	}
}
func (a *app) switchSource(id string) {
	if a.sourceByID(id) == nil || a.sourceByID(id) == a.source() {
		return
	}
	a.activateSource(id)
	a.history, a.forward = nil, nil
	a.phoneTabs = [3]phoneTabState{{route: route{page: pageHome, src: id}}, {route: route{page: pageLibrary, src: id}}, {route: route{page: pageSearch, src: id}}}
	a.phoneTab = 0
	a.show(route{page: pageHome, src: id})
}

// Run source mutations away from input dispatch: a network call can take time.
func (a *app) sourceAction(src music.Source, run func() error, done func()) {
	go func() {
		err := run()
		a.post(func() {
			if err != nil {
				a.say(err.Error())
				return
			}
			if a.source() == src {
				if done != nil {
					done()
				}
				a.reload()
			}
			a.bump()
		})
	}()
}

func (a *app) signInSource(src music.Source) {
	if a.signingIn {
		return
	}
	a.signingIn = true
	a.signSource = src
	a.signGeneration++
	generation := a.signGeneration
	a.signPrompt = music.SignInPrompt{Text: i18n.T("You'll finish signing in with the plugin, on a page it opens.")}
	a.bump()
	go func() {
		account, err := src.SignIn()
		a.post(func() {
			if generation != a.signGeneration {
				return
			}
			a.signingIn = false
			if err != nil {
				a.say(err.Error())
			} else if a.source() == src {
				a.account = account
				a.reload()
			}
			a.bump()
		})
	}()
}

// Plugins cannot silently fall back to the local library on an error.
func (a *app) sourceEmpty(p palette) core.Node {
	msg := a.homeErr
	if msg == "" {
		msg = i18n.T("Nothing here")
	}
	return a.pageScroll(p, a.titleBlock(p, a.sourceName(), msg),
		a.settingsAction(p, i18n.T("Refresh"), "source-refresh", false, a.reload),
		a.settingsAction(p, i18n.T("Settings"), "source-settings", false, func() { a.settingsPage = "plugins"; a.openSettings() }))
}

func (a *app) imageHeaders(u *url.URL) map[string]string {
	a.sources.RLock()
	defer a.sources.RUnlock()
	for _, src := range a.sources.items {
		if p, ok := src.(*plugin.Plugin); ok {
			if h := p.HeadersFor(u, plugin.KindImage); len(h) > 0 {
				return h
			}
		}
	}
	return nil
}

var errNoAnswer = errors.New("the source gave no answer")

func (a *app) openTrackPage(tr music.Track, page pageKind, id string) {
	src := tr.Source
	if src == "" {
		src = a.source().Info().ID
	}
	a.open(route{page: page, id: id, src: src})
}

func (a *app) cancelSignIn() {
	src := a.signSource
	a.signingIn = false
	a.signGeneration++
	a.signSource = nil
	if p, ok := src.(*plugin.Plugin); ok {
		go p.Close()
	}
	a.bump()
}
