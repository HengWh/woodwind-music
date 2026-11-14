package app

import (
	"fmt"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

type extraData struct {
	busy, loaded, more bool
	err, draft         string
	shows              []music.Show
	show               *music.Show
	stations           []music.Station
	albums             []music.Album
	tracks             []music.Track
	comments           []music.Comment
}

func (a *app) loadExtra(r route, more bool) {
	d := a.extra[r]
	if d == nil {
		d = &extraData{}
		a.extra[r] = d
	}
	if d.busy || d.loaded && !more {
		return
	}
	d.busy = true
	d.err = ""
	src := a.source()
	offset := 0
	if more {
		switch r.page {
		case pageShow:
			offset = len(d.tracks)
		case pageComments:
			offset = len(d.comments)
		case pageSavedAlbums:
			offset = len(d.albums)
		}
	}
	go func() {
		var err error
		var result extraData
		switch r.page {
		case pageShows:
			result.shows, err = src.Shows()
		case pageShow:
			var page *music.ShowPage
			page, err = src.Show(r.id, offset)
			if page != nil {
				result.show = &page.Show
				result.tracks = page.Episodes
				result.more = page.More
			} else if err == nil {
				err = errNoAnswer
			}
		case pageStations:
			result.stations, err = src.Stations()
		case pageComments:
			var page *music.CommentPage
			page, err = src.Comments(r.id, offset)
			if page != nil {
				result.comments = page.Comments
				result.more = page.More
			} else if err == nil {
				err = errNoAnswer
			}
		case pageSavedAlbums:
			var page *music.AlbumList
			page, err = src.SavedAlbums(offset)
			if page != nil {
				result.albums = page.Albums
				result.more = page.More
			} else if err == nil {
				err = errNoAnswer
			}
		}
		a.post(func() {
			if src != a.source() || a.extra[r] != d {
				return
			}
			d.busy = false
			if err != nil {
				d.err = err.Error()
			} else {
				if !more {
					d.shows = nil
					d.stations = nil
					d.tracks = nil
					d.comments = nil
					d.albums = nil
				}
				d.shows = result.shows
				d.stations = result.stations
				d.show = result.show
				d.tracks = append(d.tracks, result.tracks...)
				d.comments = append(d.comments, result.comments...)
				d.albums = append(d.albums, result.albums...)
				d.more = result.more
				d.loaded = true
			}
			a.bump()
		})
	}()
}
func (a *app) showCard(p palette, show music.Show) core.Node {
	return a.cardFrame(p, show.Name, show.Publisher, 10, a.coverArt(p, show.ImageURL, 10, lucideNote), func() { a.open(route{page: pageShow, id: show.ID}) })
}
func (a *app) stationCard(p palette, station music.Station) core.Node {
	return a.cardFrame(p, station.Name, station.Description, 10, a.coverArt(p, station.ImageURL, 10, lucideNote), func() { a.playStation(station) })
}
func (a *app) playStation(station music.Station) {
	src := a.source()
	go func() {
		tracks, err := src.StationNext(station.ID)
		a.post(func() {
			if err != nil {
				a.say(err.Error())
				return
			}
			if len(tracks) == 0 {
				return
			}
			a.pl.PlayRadio(station.Name, tracks, func(got func([]music.Track)) {
				go func() {
					next, err := src.StationNext(station.ID)
					a.post(func() {
						if err != nil {
							a.say(err.Error())
						}
						got(next)
					})
				}()
			})
			a.heartbeat()
		})
	}()
}
func (a *app) extraPage(cx *core.Scope, p palette, r route) core.Node {
	d := a.extra[r]
	if d == nil || d.busy && !d.loaded {
		return a.loadingPage(p)
	}
	var body []core.Node
	title := i18n.T("Podcasts")
	if r.page == pageStations {
		title = i18n.T("Radio")
	}
	if r.page == pageComments {
		title = i18n.T("Comments")
	}
	if r.page == pageSavedAlbums {
		title = i18n.T("Saved Albums")
	}
	if d.show != nil {
		title = d.show.Name
	}
	body = append(body, a.titleBlock(p, title, ""))
	if d.err != "" {
		body = append(body, text(d.err, 14, core.FontWeightNormal, p.accent), a.settingsAction(p, i18n.T("Retry"), "source-retry", d.busy, func() { d.loaded = false; a.loadExtra(r, false); a.bump() }))
	}
	if d.show != nil {
		body = append(body, text(d.show.Description, 14, core.FontWeightNormal, p.muted))
		if a.can("subscribeShow") {
			show := *d.show
			label := i18n.T("Subscribe")
			if show.Subscribed {
				label = i18n.T("Unsubscribe")
			}
			src := a.source()
			body = append(body, a.settingsAction(p, label, "show-subscribe", d.busy, func() { a.sourceAction(src, func() error { return src.SubscribeShow(show.ID, !show.Subscribed) }, nil) }))
		}
		body = append(body, a.playButtons(p, d.tracks), a.trackList(p, d.tracks, true, false, a.currentID()))
	}
	if len(d.shows) > 0 {
		body = append(body, a.cardGrid(p, len(d.shows), func(i int) core.Node { return a.showCard(p, d.shows[i]) }))
	}
	if len(d.stations) > 0 {
		body = append(body, a.cardGrid(p, len(d.stations), func(i int) core.Node { return a.stationCard(p, d.stations[i]) }))
	}
	if len(d.albums) > 0 {
		body = append(body, a.cardGrid(p, len(d.albums), func(i int) core.Node { return a.albumCard(p, d.albums[i]) }))
	}
	for _, comment := range d.comments {
		c := comment
		children := []core.Node{text(c.Author, 14, core.FontWeightSemibold, p.text), text(c.Text, 14, core.FontWeightNormal, p.text)}
		if a.can("likeComment") {
			src := a.source()
			children = append(children, a.settingsAction(p, fmt.Sprintf("♥ %d", c.Likes), "comment-like-"+c.ID, false, func() { a.sourceAction(src, func() error { return src.LikeComment(r.id, c.ID, !c.Liked) }, nil) }))
		}
		body = append(body, core.View{Style: core.Style{Gap: 8, Padding: core.All(16), Background: p.sidebar, CornerRadius: 12, Margin: core.Edges{Bottom: 12}}, Children: children})
	}
	if r.page == pageComments && a.can("postComment") {
		src := a.source()
		body = append(body, base.Input{TestID: "comment-text", Label: i18n.T("Write a comment"), Placeholder: i18n.T("Write a comment"), Value: func() string { return d.draft }, OnChange: func(v string) { d.draft = v }, Style: core.Style{Height: core.Pt(48), Padding: core.All(12), Background: p.sidebar, Color: p.text, CornerRadius: 12}},
			a.settingsAction(p, i18n.T("Send"), "comment-send", false, func() {
				value := d.draft
				if value == "" {
					return
				}
				a.sourceAction(src, func() error { _, err := src.PostComment(r.id, value); return err }, nil)
			}))
	}
	if d.more {
		body = append(body, a.settingsAction(p, i18n.T("Load More"), "source-load-more", d.busy, func() { a.loadExtra(r, true); a.bump() }))
	}
	if d.loaded && len(body) == 1 {
		body = append(body, text(i18n.T("Nothing here"), 14, core.FontWeightNormal, p.muted))
	}
	return a.pageScroll(p, body...)
}

func (a *app) sourceNavigation(p palette, r route, phone bool) []core.Node {
	var rows []core.Node
	add := func(id, title string, svg *core.SVG, target route) {
		action := func() { a.open(target) }
		if phone {
			rows = append(rows, a.phoneLibraryRow(p, id, title, "", svg, action))
		} else {
			rows = append(rows, a.navItem(p, id, svg, title, r.page == target.page && r.id == target.id, action))
		}
	}
	if a.source() == a.local {
		add("nav-home", i18n.T("Home"), lucideHome, route{page: pageHome})
		add("nav-songs", i18n.T("Songs"), lucideNote, route{page: pageBrowse, id: "songs"})
		add("nav-albums", i18n.T("Albums"), lucideDisc, route{page: pageBrowse, id: "albums"})
		add("nav-artists", i18n.T("Artists"), lucideUser, route{page: pageBrowse, id: "artists"})
		add("nav-liked", i18n.T("Liked Songs"), lucideHeart, route{page: pageLiked})
		return rows
	}
	nav := a.source().Info().Navigation
	if len(nav) > 0 {
		for i, item := range nav {
			target := route{page: pageBrowse, id: item.Page}
			capability := "browse"
			switch item.Builtin {
			case "home":
				target = route{page: pageHome}
				capability = "home"
			case "liked":
				target = route{page: pageLiked}
				capability = "liked"
			case "shows":
				target = route{page: pageShows}
				capability = "shows"
			case "stations":
				target = route{page: pageStations}
				capability = "stations"
			case "savedAlbums":
				target = route{page: pageSavedAlbums}
				capability = "savedAlbums"
			}
			if a.can(capability) {
				title := item.Title
				if title == "" {
					switch target.page {
					case pageHome:
						title = i18n.T("Home")
					case pageLiked:
						title = i18n.T("Liked Songs")
					case pageSavedAlbums:
						title = i18n.T("Saved Albums")
					case pageShows:
						title = i18n.T("Podcasts")
					case pageStations:
						title = i18n.T("Radio")
					default:
						title = item.Page
					}
				}
				add(fmt.Sprintf("source-nav-%d", i), title, sourceNavIcon(item.Icon, target.page), target)
			}
		}
	} else {
		for _, item := range []struct {
			cap, label string
			page       pageKind
		}{{"home", i18n.T("Home"), pageHome}, {"liked", i18n.T("Liked Songs"), pageLiked}, {"savedAlbums", i18n.T("Saved Albums"), pageSavedAlbums}, {"shows", i18n.T("Podcasts"), pageShows}, {"stations", i18n.T("Radio"), pageStations}} {
			if a.can(item.cap) {
				add("source-"+item.cap, item.label, sourceNavIcon("", item.page), route{page: item.page})
			}
		}
	}
	return rows
}

func (a *app) phoneSources(p palette) core.Node {
	rows := []core.Node{text(i18n.T("Music Sources"), phoneFontTitle, core.FontWeightSemibold, p.text),
		a.phoneSheetAction(p, "source-local", i18n.T("Local Files"), lucideFolder, func() { a.phoneSheet.Set(""); a.switchSource("local") })}
	for _, p1 := range a.plugins {
		id := p1.Info().ID
		rows = append(rows, a.phoneSheetAction(p, "source-"+id, p1.Info().Name, lucideNote, func() { a.phoneSheet.Set(""); a.switchSource(id) }))
	}
	rows = append(rows, a.phoneSheetAction(p, "source-manage", i18n.T("Plugins"), lucideSettings, func() { a.phoneSheet.Set(""); a.settingsPage = "plugins"; a.openSettings() }))
	return core.Scroll{Style: core.Style{MaxHeight: core.Pt(a.cx.Env().Window().Height * .65)}, Children: rows}
}
func (a *app) pluginPhoneLibrary(p palette) core.Node {
	rows := []core.Node{a.titleBlock(p, a.sourceName(), ""), a.phoneLibraryRow(p, "phone-sources", i18n.T("Music Sources"), a.sourceName(), lucideNote, func() { a.openPhoneSheet("sources") })}
	if a.can("account") || a.can("signIn") || a.account != nil {
		rows = append(rows, a.accountRow(p))
	}
	rows = append(rows, a.sourceNavigation(p, a.route.GetUntracked(), true)...)
	for _, pl := range a.mine {
		list := pl
		rows = append(rows, a.phoneLibraryRow(p, "playlist-"+pl.ID, pl.Name, playlistLine(pl), lucideList, func() { a.open(route{page: pagePlaylist, id: list.ID}) }))
	}
	if a.can("createPlaylist") {
		rows = append(rows, a.phoneLibraryRow(p, "new-playlist", i18n.T("New Playlist"), "", lucidePlus, a.newPlaylist))
	}
	return a.pageScroll(p, rows...)
}

func sourceNavIcon(name string, page pageKind) *core.SVG {
	switch name {
	case "home":
		return lucideHome
	case "album":
		return lucideDisc
	case "artist":
		return lucideUser
	case "playlist":
		return lucideList
	case "heart":
		return lucideHeart
	case "search":
		return lucideSearch
	case "star":
		return lucide.Star()
	case "radio":
		return lucide.Radio()
	case "podcast":
		return lucide.Rss()
	}
	switch page {
	case pageHome:
		return lucideHome
	case pageLiked:
		return lucideHeart
	case pageSavedAlbums:
		return lucideDisc
	case pageShows, pageShow:
		return lucide.Rss()
	case pageStations:
		return lucide.Radio()
	}
	return lucideNote
}
