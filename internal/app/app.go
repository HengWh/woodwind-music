// Package app is Woodwind Music, built on NSGo: the music files on this
// computer, with their tags, covers and lyrics, in one window.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/local"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/nowplaying"
	"github.com/HengWh/woodwind-music/internal/player"
)

const appName = "Woodwind Music"

// Run runs the app until it quits, its music playing on in the menu bar
// when its window closes.
func Run() error {
	if err := nowplaying.Initialize(); err != nil {
		log.Printf("desktop integration: %v", err)
	}
	defer nowplaying.Shutdown()
	nsgo.Run(func(cx *core.Scope) {
		desktop.SetShutdownMode(cx, desktop.ShutdownOnExplicit)
		a := newApp(cx)
		a.openMain = func() { nsgo.SetMainView(cx, a.view) }
		a.openMain()
		if w := desktop.MainWindow(cx); w != nil {
			w.OnClosing(cx, func(*desktop.WindowClosing) {
				a.saveSession()
				a.post(a.applyAppIcon)
			})
		}
		a.applyAppIcon()
		desktop.OnReopen(cx, a.openMain)
	}, nsgo.WindowSize(1180, 780),
		nsgo.WindowConstraints(core.WindowConstraints{MinWidth: 880, MinHeight: 580}))
	return nil
}

// Root is the app's main view, for a test or a host that shows it.
func Root(cx *core.Scope) core.Node {
	a := newApp(cx)
	return a.view(cx)
}

// pageKind is which page the window shows.
type pageKind int

const (
	pageHome pageKind = iota
	pageLiked
	pagePlaylist
	pageAlbum
	pageArtist
	pageSearch
	pageBrowse
	pageSection
)

// route is the page shown. A page is of the one source, the local files.
type route struct {
	page  pageKind
	id    string // the playlist, album, artist or browse page shown
	query string
	kind  music.SearchKind
}

// settings are the app's own, kept in settings.json in its data directory.
type settings struct {
	Language   string  `json:"language"`
	Appearance string  `json:"appearance"`
	Volume     float64 `json:"volume"`
	Normalize  bool    `json:"normalize"`
	Quality    string  `json:"quality"`
	CacheMB    int     `json:"cacheMB"`
	// SearchHistory keeps submitted keywords, most recent first.
	SearchHistory []string `json:"searchHistory,omitempty"`
	// LyricsStage shows lyrics on the stage rather than scrolling.
	LyricsStage bool `json:"lyricsStage"`
	// GrowFromCover opens Now Playing out of the player bar's cover.
	GrowFromCover bool `json:"growFromCover"`
	// Smoke makes the progress bar a comet as music plays.
	Smoke bool `json:"smoke"`
	// Fog fills Now Playing's backdrop with fog of the cover's colors.
	Fog bool `json:"fog"`
	// AppIcon is where the app's icon shows on macOS (appIcons).
	AppIcon string `json:"appIcon"`
	// StageStyle is how the stage sets its lyrics: Flow, Steps or Tilt.
	StageStyle string `json:"stageStyle,omitempty"`
	// StageSparks throws sparks off each word as it is sung.
	StageSparks bool `json:"stageSparks,omitempty"`

	path string
}

func defaultSettings() settings {
	return settings{Language: i18n.System, Appearance: "system", Volume: 0.8, Normalize: true, Quality: "high",
		CacheMB: 2048, AppIcon: "dock-and-menubar"}
}

func loadSettings(dir string) settings {
	s := defaultSettings()
	if dir == "" {
		return s
	}
	s.path = filepath.Join(dir, "settings.json")
	if b, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func (s settings) save() {
	if s.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return
	}
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		_ = os.WriteFile(s.path, b, 0o600)
	}
}

// albumData is what the app has of an album.
type albumData struct {
	album   *music.AlbumPage
	loading bool
	err     string
}

// artistData is what the app has of an artist.
type artistData struct {
	page    *music.ArtistPage
	loading bool
	err     string
	// tab is what the page lists: 0 for songs, 1 for albums.
	tab int
}

// playlistData is what the app has of a playlist.
type playlistData struct {
	playlist *music.Playlist
	loading  bool
	err      string
}

// browseData is a page the source makes up itself.
type browseData struct {
	browse  *music.Browse
	loading bool
	err     string
}

// searchData is what a search found, as far as its pages have loaded.
type searchData struct {
	res     music.SearchResult
	loading bool
	loaded  bool
	more    bool
	err     string
}

// app is the state the window shows. Everything in it is read and
// changed on the engine goroutine; goroutines reach it through post.
type app struct {
	cx      *core.Scope
	dir     string
	dataDir string
	set     settings

	local *local.Source
	pl    *player.Player

	// Signals: rev changes when any data the pages show does; route is
	// the page shown; tick moves as what plays does, for the clock.
	rev        *core.Signal[int]
	playRev    *core.Signal[int]
	route      *core.Signal[route]
	tick       *core.Signal[int]
	query      *core.Signal[string]
	searchOpen *core.Signal[bool]
	np         *core.Signal[bool]
	npPane     *core.Signal[int]
	stageTick  *core.Signal[int]
	toast      *core.Signal[string]

	// artRev moves as a cover's colors are found, for the daily card.
	artRev       *core.Signal[int]
	dailyHover   *core.Signal[bool]
	dailyTick    *core.Signal[int]
	dailyPlay    *core.Signal[bool]
	dailySpin    spinState
	dailyBars    [4]float32
	dailyTicking bool
	tints        map[string]core.Color
	tintDone     map[string]bool
	tintBusy     map[string]bool

	history  []route
	forward  []route
	scrolled route

	home     *music.Browse
	homeBusy bool
	homeErr  string
	recent   []music.Track
	mine     []music.Playlist
	liked    []music.Track
	likedSet map[string]bool

	searchRef *core.Ref

	playlists map[string]*playlistData
	albums    map[string]*albumData
	artists   map[string]*artistData
	browses   map[string]*browseData
	searches  map[route]*searchData

	// Layout state.
	volumeOpen bool
	ticking    bool
	lastSaved  time.Time
	lastSearch route
	reloadGen  int
	mutedFrom  float64
	nowPlaying bool
	queueAt    int
	scroll     map[string]core.Points

	// Dialogs.
	naming           bool
	namingNew        bool
	namingText       string
	namingID         string
	deleting         bool
	deletingID       string
	deletingName     string
	settingsWin      *desktop.Window
	settingsInMain   bool
	settingsRef      *core.Ref
	settingsPage     string
	settingsQuery    string
	settingsQueryRev *core.Signal[int]
	// cacheUsed is what the song cache takes, in bytes: -1 until measured.
	cacheUsed    int64
	stageTicking bool

	tray     trayIcon
	openMain func()
}

// newApp makes the app and starts reading the library.
func newApp(cx *core.Scope) *app {
	dir := appDataDir(cx.DataDir())
	a := &app{
		cx:               cx,
		dir:              dir,
		dataDir:          dir,
		set:              loadSettings(dir),
		rev:              core.NewSignal(cx, 0),
		playRev:          core.NewSignal(cx, 0),
		route:            core.NewSignal(cx, route{page: pageHome}),
		tick:             core.NewSignal(cx, 0),
		query:            core.NewSignal(cx, ""),
		searchOpen:       core.NewSignal(cx, false),
		np:               core.NewSignal(cx, false),
		npPane:           core.NewSignal(cx, 0),
		stageTick:        core.NewSignal(cx, 0),
		toast:            core.NewSignal(cx, ""),
		artRev:           core.NewSignal(cx, 0),
		dailyHover:       core.NewSignal(cx, false),
		dailyTick:        core.NewSignal(cx, 0),
		dailyPlay:        core.NewSignal(cx, false),
		tints:            map[string]core.Color{},
		tintDone:         map[string]bool{},
		tintBusy:         map[string]bool{},
		searchRef:        new(core.Ref),
		settingsRef:      new(core.Ref),
		settingsQueryRev: core.NewSignal(cx, 0),
		cacheUsed:        -1,
		likedSet:         map[string]bool{},
		playlists:        map[string]*playlistData{},
		albums:           map[string]*albumData{},
		artists:          map[string]*artistData{},
		browses:          map[string]*browseData{},
		searches:         map[route]*searchData{},
		queueAt:          -1,
		scroll:           map[string]core.Points{},
	}
	a.applyLanguage()
	a.local = local.New(dir)
	a.local.OnChange(func() { a.post(a.reload) })
	a.pl = player.New(func(fn func()) { a.post(fn) })
	a.pl.Source = func(string) music.Source { return a.local }
	a.pl.Quality = func() string { return a.set.Quality }
	a.pl.Normalize = func() bool { return a.set.Normalize }
	if dir != "" && a.set.CacheMB > 0 {
		a.pl.Cache = player.NewCache(filepath.Join(dir, "songs"), int64(a.set.CacheMB)<<20)
	}
	a.pl.Changed = func() { a.heartbeat(); a.playRev.Update(func(n int) int { return n + 1 }) }
	a.pl.SetVolume(a.set.Volume)
	player.System.Set = func(info player.NowPlaying) {
		nowplaying.Set(nowplaying.Info{Title: info.Title, Artist: info.Artist, Album: info.Album,
			Artwork: info.Artwork, Duration: info.Duration, Position: info.Position, Playing: info.Playing})
	}
	player.System.Position = func(pos time.Duration, playing bool) { nowplaying.Position(pos, playing) }
	player.System.Clear = nowplaying.Clear
	nowplaying.OnCommand = func(cmd nowplaying.Command, pos time.Duration) {
		a.post(func() {
			switch cmd {
			case nowplaying.Play:
				if !a.pl.Playing() {
					a.pl.TogglePlay()
				}
			case nowplaying.Pause:
				if a.pl.Playing() {
					a.pl.TogglePlay()
				}
			case nowplaying.Next:
				a.pl.Next()
			case nowplaying.Previous:
				a.pl.Prev()
			case nowplaying.Seek:
				a.pl.Seek(pos)
			}
			a.heartbeat()
		})
	}
	a.restoreSession()
	a.local.Rescan()
	a.reload()
	a.heartbeat()
	a.setup(cx)
	cx.OnQuit(func(context.Context) { a.saveSession() })
	return a
}

func (a *app) quit() {
	a.saveSession()
	platformAllowQuit()
	a.cx.Quit()
}

// post runs fn on the engine goroutine.
func (a *app) post(fn func()) {
	a.cx.Post(fn)
}

// bump says that what the pages show changed.
func (a *app) bump() { a.rev.Update(func(n int) int { return n + 1 }) }

// heartbeat keeps the clock moving while something plays.
func (a *app) heartbeat() {
	a.tick.Update(func(n int) int { return n + 1 })
	if time.Since(a.lastSaved) > 30*time.Second {
		a.lastSaved = time.Now()
		a.saveSession()
	}
	if a.ticking {
		return
	}
	a.ticking = true
	var beat func()
	beat = func() {
		if a.cx.Disposed() {
			return
		}
		if a.pl != nil && (a.pl.Playing() || a.pl.Loading()) {
			a.tick.Update(func(n int) int { return n + 1 })
			a.cx.AfterFunc(200*time.Millisecond, beat)
			return
		}
		a.ticking = false
		a.tick.Update(func(n int) int { return n + 1 })
	}
	a.cx.AfterFunc(200*time.Millisecond, beat)
}

// applyLanguage speaks the chosen language, or the system's.
func (a *app) applyLanguage() {
	code := a.set.Language
	if code == i18n.System || code == "" {
		code = i18n.Resolve(i18n.System, []string{a.cx.Env().Locale()})
	}
	i18n.SetLanguage(code)
}

// reload reads what the sidebar and Home show: the playlists, the liked
// songs, what played lately and Home.
func (a *app) reload() {
	a.homeBusy = true
	a.reloadGen++
	gen := a.reloadGen
	src := a.local
	go func() {
		home, homeErr := src.Home()
		playlists, _ := src.Playlists()
		liked, _ := src.Liked(0)
		recent, _ := src.Recent()
		a.post(func() {
			if gen != a.reloadGen {
				return // a later read won
			}
			a.homeBusy = false
			a.home, a.homeErr = home, ""
			if homeErr != nil {
				a.homeErr = homeErr.Error()
			}
			a.mine = playlists
			a.liked = nil
			if liked != nil {
				a.liked = liked.Tracks
			}
			a.likedSet = map[string]bool{}
			for _, t := range a.liked {
				a.likedSet[t.ID] = true
			}
			a.recent = recent
			// What pages loaded is of the library before: read it again as
			// they open, keeping only what the sidebar shows.
			a.playlists = map[string]*playlistData{}
			a.albums = map[string]*albumData{}
			a.artists = map[string]*artistData{}
			a.browses = map[string]*browseData{}
			a.searches = map[route]*searchData{}
			a.reloadPage(a.route.GetUntracked())
			a.bump()
		})
	}()
}

// reloadPage loads what the page of r shows, unless it has.
func (a *app) reloadPage(r route) {
	switch r.page {
	case pageHome:
		// Read by reload.
	case pageLiked:
		// Read by reload.
	case pagePlaylist:
		a.loadPlaylist(r.id, false)
	case pageAlbum:
		a.loadAlbum(r.id)
	case pageArtist:
		a.loadArtist(r.id)
	case pageBrowse:
		a.loadBrowse(r.id, false)
	case pageSearch:
		a.loadSearch(r, false)
	case pageSection:
		// Read from the page the section is on.
	}
}

func (a *app) loadPlaylist(id string, again bool) {
	d := a.playlists[id]
	if d == nil {
		d = &playlistData{}
		a.playlists[id] = d
	}
	if d.loading || d.playlist != nil && !again {
		return
	}
	d.loading, d.err = true, ""
	go func() {
		pl, err := a.local.Playlist(id)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err = err.Error()
				return
			}
			d.playlist = pl
			a.bump()
		})
	}()
}

func (a *app) loadAlbum(id string) {
	d := a.albums[id]
	if d == nil {
		d = &albumData{}
		a.albums[id] = d
	}
	if d.loading || d.album != nil {
		return
	}
	d.loading, d.err = true, ""
	go func() {
		al, err := a.local.Album(id)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err = err.Error()
				return
			}
			d.album = al
			a.bump()
		})
	}()
}

func (a *app) loadArtist(id string) {
	d := a.artists[id]
	if d == nil {
		d = &artistData{}
		a.artists[id] = d
	}
	if d.loading || d.page != nil {
		return
	}
	d.loading, d.err = true, ""
	go func() {
		p, err := a.local.Artist(id)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err = err.Error()
				return
			}
			d.page = p
			a.bump()
		})
	}()
}

func (a *app) loadBrowse(id string, again bool) {
	d := a.browses[id]
	if d == nil {
		d = &browseData{}
		a.browses[id] = d
	}
	if d.loading || d.browse != nil && !again {
		return
	}
	d.loading, d.err = true, ""
	go func() {
		b, err := a.local.Browse(id)
		a.post(func() {
			d.loading = false
			if err != nil {
				d.err = err.Error()
				return
			}
			d.browse = b
			a.bump()
		})
	}()
}

func (a *app) loadSearch(r route, again bool) {
	if strings.TrimSpace(r.query) == "" {
		return
	}
	d := a.searches[r]
	if d == nil {
		d = &searchData{}
		a.searches[r] = d
	}
	if d.loading || d.loaded && !again {
		return
	}
	d.loading, d.loaded, d.err = true, false, ""
	go func() {
		res, err := a.local.Search(r.query, r.kind, 0)
		a.post(func() {
			if a.searches[r] != d {
				return
			}
			d.loading = false
			if err != nil {
				d.err = err.Error()
				a.bump()
				return
			}
			d.res = *res
			d.loaded = true
			d.more = res.More
			a.bump()
		})
	}()
}

func (a *app) loadMoreSearch(r route) {
	d := a.searches[r]
	if d == nil || !d.loaded || d.loading || !d.more {
		return
	}
	d.loading, d.err = true, ""
	offset := d.res.Len()
	a.bump()
	go func() {
		res, err := a.local.Search(r.query, r.kind, offset)
		a.post(func() {
			if a.searches[r] != d {
				return
			}
			d.loading = false
			if err != nil {
				d.err = err.Error()
			} else {
				d.res.Tracks = append(d.res.Tracks, res.Tracks...)
				d.res.Albums = append(d.res.Albums, res.Albums...)
				d.res.Artists = append(d.res.Artists, res.Artists...)
				d.res.Playlists = append(d.res.Playlists, res.Playlists...)
				d.res.More, d.more = res.More, res.More
			}
			a.bump()
		})
	}()
}

// open shows r, remembering the page before.
func (a *app) open(r route) {
	if r == a.route.GetUntracked() {
		a.closeNowPlaying()
		return
	}
	a.history = append(a.history, a.route.GetUntracked())
	a.forward = nil
	a.show(r)
}

func (a *app) show(r route) {
	a.closeNowPlaying()
	a.searchOpen.Set(false)
	if r.page == pageSearch && r.query != "" {
		a.query.Set(r.query)
		a.lastSearch = r
	}
	a.route.Set(r)
	a.reloadPage(r)
	a.bump()
}

func (a *app) back() {
	if a.nowPlaying {
		a.closeNowPlaying()
		return
	}
	if len(a.history) == 0 {
		return
	}
	a.forward = append(a.forward, a.route.GetUntracked())
	r := a.history[len(a.history)-1]
	a.history = a.history[:len(a.history)-1]
	a.show(r)
}

func (a *app) goForward() {
	if len(a.forward) == 0 {
		return
	}
	a.history = append(a.history, a.route.GetUntracked())
	r := a.forward[len(a.forward)-1]
	a.forward = a.forward[:len(a.forward)-1]
	a.show(r)
}

func (a *app) closeNowPlaying() {
	if a.nowPlaying {
		a.nowPlaying = false
		a.np.Set(false)
	}
}

func (a *app) openNowPlaying() {
	a.nowPlaying = true
	// Keep the queue where it was as it opens.
	a.queueAt = -1
	a.np.Set(true)
}

// playTracks replaces the queue and plays the one at i.
func (a *app) playTracks(tracks []music.Track, i int) {
	if len(tracks) == 0 {
		return
	}
	a.pl.PlayTrack(tracks, i)
	a.heartbeat()
}

func (a *app) toggle() {
	a.pl.TogglePlay()
	a.heartbeat()
}

// toggleShuffle turns shuffling on or off, and shows it.
func (a *app) toggleShuffle() {
	a.pl.ToggleShuffle()
	a.playRev.Update(func(n int) int { return n + 1 })
	a.menu(a.cx)
}

// cycleRepeat moves on to the next kind of repeat, and shows it.
func (a *app) cycleRepeat() {
	a.pl.CycleRepeat()
	a.playRev.Update(func(n int) int { return n + 1 })
	a.menu(a.cx)
}

// mute turns the sound off, and back to where it was.
func (a *app) mute() {
	if a.set.Volume > 0 {
		a.mutedFrom = a.set.Volume
		a.set.Volume = 0
	} else {
		a.set.Volume = a.mutedFrom
		if a.set.Volume == 0 {
			a.set.Volume = 0.8
		}
	}
	a.pl.SetVolume(a.set.Volume)
	a.set.save()
	a.tick.Update(func(n int) int { return n + 1 })
}

// toggleLike likes or unlikes the track, and says so.
func (a *app) toggleLike(id string) {
	on := !a.likedSet[id]
	if err := a.local.Like(id, on); err != nil {
		a.say(err.Error())
		return
	}
	if on {
		a.likedSet[id] = true
	} else {
		delete(a.likedSet, id)
	}
	a.playRev.Update(func(n int) int { return n + 1 })
	a.say(i18n.T("Added to Liked Songs"))
	if !on {
		a.say(i18n.T("Removed from Liked Songs"))
	}
}

func (a *app) say(msg string) {
	a.toast.Set(msg)
	a.cx.AfterFunc(2600*time.Millisecond, func() {
		if a.toast.GetUntracked() == msg {
			a.toast.Set("")
		}
	})
}

// newPlaylist opens the dialog that names one.
func (a *app) newPlaylist() {
	a.naming, a.namingNew, a.namingText, a.namingID = true, true, "", ""
	a.bump()
}

func (a *app) renamePlaylist(id, name string) {
	a.naming, a.namingNew, a.namingText, a.namingID = true, false, name, id
	a.bump()
}

func (a *app) saveName() {
	name := strings.TrimSpace(a.namingText)
	if name == "" {
		return
	}
	if a.namingNew {
		pl, err := a.local.CreatePlaylist(name)
		if err != nil {
			a.say(err.Error())
		} else {
			a.playlists[pl.ID] = &playlistData{playlist: pl}
			a.open(route{page: pagePlaylist, id: pl.ID})
			a.reload()
		}
	} else if err := a.local.RenamePlaylist(a.namingID, name); err != nil {
		a.say(err.Error())
	} else if d := a.playlists[a.namingID]; d != nil && d.playlist != nil {
		d.playlist.Name = name
	}
	a.naming = false
	a.bump()
}

func (a *app) confirmDelete(id, name string) {
	a.deleting, a.deletingID, a.deletingName = true, id, name
	a.bump()
}

func (a *app) deletePlaylist() {
	id := a.deletingID
	if err := a.local.DeletePlaylist(id); err != nil {
		a.say(err.Error())
	}
	a.deleting = false
	if a.route.GetUntracked().page == pagePlaylist && a.route.GetUntracked().id == id {
		a.back()
	}
	a.reload()
	a.bump()
}

// addToPlaylist puts tracks into the playlist.
func (a *app) addToPlaylist(id string, ids []string) {
	if err := a.local.AddToPlaylist(id, ids); err != nil {
		a.say(err.Error())
		return
	}
	a.say(i18n.T("Added to the playlist"))
	if d := a.playlists[id]; d != nil {
		d.playlist = nil
		a.loadPlaylist(id, true)
	}
	a.reload()
}

func (a *app) removeFromPlaylist(id string, ids []string) {
	if err := a.local.RemoveFromPlaylist(id, ids); err != nil {
		a.say(err.Error())
		return
	}
	if d := a.playlists[id]; d != nil {
		d.playlist = nil
		a.loadPlaylist(id, true)
	}
	a.reload()
}

// pickFolders asks for folders with the system's picker and adds them.
func (a *app) pickFolders() {
	a.cx.PickDirectories(core.PickDirectoriesOptions{Multiple: true}, func(dirs []core.Directory, err error) {
		if err != nil || len(dirs) == 0 {
			return
		}
		var paths []string
		for _, d := range dirs {
			if d.Path != "" {
				paths = append(paths, d.Path)
			}
		}
		if len(paths) > 0 {
			a.local.AddFolders(paths...)
			a.reload()
			a.bump()
		}
	})
}

// removeFolder forgets a folder and what was found in it.
func (a *app) removeFolder(path string) {
	a.local.RemoveFolder(path)
	a.reload()
}

func (a *app) search(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		a.query.Set("")
		a.searchOpen.Set(false)
		return
	}
	a.query.Set(q)
	a.searchOpen.Set(false)
	kind := music.SearchTracks
	if r := a.route.GetUntracked(); r.page == pageSearch && r.query == q {
		kind = r.kind
	}
	r := route{page: pageSearch, query: q, kind: kind}
	a.lastSearch = r
	a.rememberSearch(q)
	if r == a.route.GetUntracked() {
		a.loadSearch(r, true)
		a.bump()
	} else {
		a.open(r)
	}
}

const searchHistoryLimit = 20

func (a *app) rememberSearch(q string) {
	history := []string{q}
	for _, s := range a.set.SearchHistory {
		if s != q && len(history) < searchHistoryLimit {
			history = append(history, s)
		}
	}
	a.set.SearchHistory = history
	a.set.save()
}

func (a *app) closeSearch() {
	a.query.Set("")
	a.searchOpen.Set(false)
	if a.route.GetUntracked().page == pageSearch {
		a.open(route{page: pageHome})
	}
}

// sessionPath is where playback was left.
func (a *app) sessionPath() string {
	if a.dir == "" {
		return ""
	}
	return filepath.Join(a.dir, "session.json")
}

func (a *app) saveSession() {
	s, ok := a.pl.Session()
	if !ok {
		return
	}
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	path := a.sessionPath()
	if path == "" {
		return
	}
	if os.WriteFile(path+".part", b, 0o600) == nil {
		_ = os.Rename(path+".part", path)
	}
}

func (a *app) restoreSession() {
	path := a.sessionPath()
	if path == "" {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var s player.Session
	if json.Unmarshal(b, &s) == nil {
		a.pl.Restore(s)
	}
}

// openSettings shows the settings window, opening it the first time.
func (a *app) openSettings() {
	// OpenWindow belongs to the current reactive owner. A button's owner
	// goes away as the sidebar redraws, so open under the app's stable scope.
	a.post(a.showSettings)
}

func (a *app) showSettings() {
	// NSGo's current Windows backend has one native surface. Keep settings
	// within it until additional native windows are supported there.
	if a.cx.Env().OS() == core.OSWindows {
		a.settingsInMain = true
		a.measureCache()
		a.showMain()
		a.bump()
		return
	}
	if a.settingsWin != nil && !a.settingsWin.Closed() {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		a.measureCache()
		a.applyAppIcon()
		return
	}
	a.measureCache()
	w := desktop.OpenWindow(a.cx, desktop.WindowOptions{Title: i18n.T("Settings"), Width: 780, Height: 640, Key: "settings",
		Constraints: core.WindowConstraints{MinWidth: 640, MinHeight: 420}}, a.settingsView)
	if w == nil {
		return
	}
	a.settingsWin = w
	w.OnClosing(a.cx, func(*desktop.WindowClosing) {
		a.settingsWin = nil
		a.post(a.applyAppIcon)
	})
	a.applyAppIcon()
	a.bump()
}

// view is the window.
func (a *app) view(cx *core.Scope) core.Node {
	a.setupWindow(cx)
	return core.Dynamic(func(cx *core.Scope) core.Node {
		p := a.palette(cx)
		titlebar := desktop.TitleBar(cx)
		top := titlebar.Top
		if top <= 0 {
			top = 40
		}
		root := core.View{TestID: "app-root", Style: core.Style{Width: core.Pct(100), Height: core.Pct(100), Direction: core.Row,
			Background: p.bg, Color: p.text, FontSize: 13}}
		root.Children = []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node { return a.sidebar(cx, top) }),
			core.View{TestID: "app-content", Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0),
				Direction: core.Column, Background: p.bg}, Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0), Direction: core.Column}, Children: []core.Node{
					core.Dynamic(func(cx *core.Scope) core.Node { a.rev.Get(); a.route.Get(); return a.pageTop(cx, p, top) }),
					core.Dynamic(func(cx *core.Scope) core.Node { a.rev.Get(); return a.page(cx, p) }),
				}},
				a.playerBar(cx, p),
			}},
			core.Show{When: a.np.Get, Then: func(cx *core.Scope) core.Node { return a.nowPlayingView(cx, p) }},
			a.nameDialog(cx, p),
			a.deleteDialog(cx, p),
			core.Show{When: func() bool { return a.toast.Get() != "" }, Then: a.toastView},
			core.Show{When: func() bool { a.rev.Get(); return a.settingsInMain }, Then: func(cx *core.Scope) core.Node {
				return core.View{TestID: "settings-in-main", Style: core.Style{Position: core.Absolute,
					Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0), ZIndex: 20},
					Children: []core.Node{a.settingsView(cx)}}
			}},
		}
		return root
	})
}

// setup sets the theme and the appearance once.
func (a *app) setup(cx *core.Scope) {
	t := core.DefaultTheme()
	p := a.palette(cx)
	t.Light.Accent, t.Dark.Accent = p.accent, p.accent
	t.Light.AccentText, t.Dark.AccentText = core.RGB(0xffffff), core.RGB(0xffffff)
	cx.SetTheme(t)
	switch a.set.Appearance {
	case "light":
		cx.SetAppearance(core.AppearanceLight)
	case "dark":
		cx.SetAppearance(core.AppearanceDark)
	}
	a.menu(cx)
}

// setupWindow makes the window the app draws its own top strip in, and
// binds its keys.
func (a *app) setupWindow(cx *core.Scope) {
	desktop.SetTitleBar(cx, desktop.TitleBarHidden, 40)
	desktop.SetWindowControls(cx, core.WindowControls{TrafficLights: core.Point{X: 20, Y: 18}})
	a.shortcuts(cx)
}

// shortcuts binds the keys of Music.
func (a *app) shortcuts(cx *core.Scope) {
	cx.OnKeyWith("space", core.KeyOptions{When: core.KeyNotEditing}, a.toggle)
	cx.OnKey("mod+f", func() {
		if a.settingsInMain {
			a.settingsRef.Focus()
			return
		}
		a.searchFieldFocus(cx)
	})
	cx.OnKey("mod+q", a.quit)
	cx.OnKey("mod+[", a.back)
	cx.OnKey("mod+]", a.goForward)
	cx.OnKey("mod+n", a.newPlaylist)
	cx.OnKey("mod+,", a.openSettings)
	cx.OnKey("mod+left", a.pl.Prev)
	cx.OnKey("mod+right", a.pl.Next)
	cx.OnKey("escape", func() {
		if a.settingsInMain {
			a.closeSettings()
			return
		}
		if a.nowPlaying {
			a.closeNowPlaying()
		}
	})
	for i, pane := range []int{0, 3, 1, 2} {
		pane := pane
		cx.OnKey(fmt.Sprintf("mod+%d", i+1), func() {
			if a.nowPlaying {
				a.npPane.Set(pane)
			}
		})
	}
}

func (a *app) closeSettings() { a.settingsInMain = false; a.bump() }

// searchFieldFocus focuses the sidebar's search field.
func (a *app) searchFieldFocus(cx *core.Scope) {
	if a.route.GetUntracked().page != pageSearch {
		a.open(route{page: pageSearch})
	}
	a.searchRef.Focus()
	a.searchOpen.Set(true)
}

// pageTop is the strip along the top of a page: it moves the window, and
// holds the back button once there is a page behind.
func (a *app) pageTop(cx *core.Scope, p palette, height core.Points) core.Node {
	var crumbs []core.Node
	if len(a.history) > 0 || len(a.forward) > 0 {
		back := a.roundButton(cx, p, 16, p.muted, i18n.T("Back"), lucideChevronLeft, func() {
			if len(a.history) > 0 {
				a.back()
			}
		})
		crumbs = append(crumbs, back)
		if len(a.forward) > 0 {
			crumbs = append(crumbs, a.roundButton(cx, p, 16, p.muted, i18n.T("Forward"), lucideChevronRight, a.goForward))
		}
	}
	return core.View{Style: core.Style{Height: core.Pt(height), Direction: core.Row, AlignItems: core.AlignCenter,
		Padding: core.Symmetric(0, 20), Gap: 4, FlexShrink: 0}, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn},
		Children: crumbs}
}

// toastView is the message shown for a moment at the foot.
func (a *app) toastView(cx *core.Scope) core.Node {
	return core.View{Style: core.Style{Position: core.Absolute, Left: core.Pct(50), Bottom: core.Pt(playerBarHeight + 20),
		Transform: core.Transform{TranslateXPercent: -50}}, Children: []core.Node{
		core.Text{ContentFunc: a.toast.Get, Style: core.Style{Background: core.RGBA(0x1c1c1e, 0.92), Color: core.RGB(0xffffff),
			Padding: core.Symmetric(8, 14), CornerRadius: 10, FontSize: 12.5, FontWeight: core.FontWeightMedium}},
	}}
}
