package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"

	"nsgo-music/internal/i18n"
	"nsgo-music/internal/music"
)

const pagePad = 40

// page is the page shown, as the route says.
func (a *app) page(cx *core.Scope, p palette) core.Node {
	r := a.route.Get()
	switch r.page {
	case pageHome:
		return a.homePage(cx, p)
	case pageLiked:
		return a.likedPage(cx, p)
	case pagePlaylist:
		return a.playlistPage(cx, p, r.id)
	case pageAlbum:
		return a.albumPage(cx, p, r.id)
	case pageArtist:
		return a.artistPage(cx, p, r.id)
	case pageBrowse:
		return a.browsePage(cx, p, r.id)
	case pageSearch:
		return a.searchPage(cx, p, r)
	case pageSection:
		return a.sectionPage(cx, p, r)
	}
	return a.emptyNote(p, i18n.T("Nothing here"), "")
}

// pageScroll is a page that scrolls, its content padded from the window's
// edges.
func (a *app) pageScroll(p palette, children ...core.Node) core.Node {
	return core.Scroll{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Column, Padding: core.Edges{Top: 8, Right: pagePad - 8, Bottom: 48, Left: pagePad - 8}},
			Children: children},
	}}
}

// titleBlock is the greeting or page heading.
func (a *app) titleBlock(p palette, title, sub string) core.Node {
	return core.View{Style: core.Style{Padding: core.Symmetric(0, 12), Gap: 4, Margin: core.Edges{Bottom: 18}}, Children: []core.Node{
		core.Group{
			text(sub, 12, core.FontWeightSemibold, p.muted),
			text(title, 30, core.FontWeightBold, p.text),
		},
	}}
}

func greeting(now time.Time) string {
	switch h := now.Hour(); {
	case h < 5:
		return i18n.T("Good night")
	case h < 12:
		return i18n.T("Good morning")
	case h < 18:
		return i18n.T("Good afternoon")
	default:
		return i18n.T("Good evening")
	}
}

// homePage is what the library played lately, its Albums and its
// Playlists; with nothing yet, how to add music.
func (a *app) homePage(cx *core.Scope, p palette) core.Node {
	if a.home == nil && a.homeBusy {
		return a.emptyNote(p, i18n.T("Looking for music…"), i18n.T("Songs will show here as they're found."))
	}
	if a.home == nil || len(a.home.Sections) == 0 {
		return a.emptyHome(p)
	}
	w := cx.Env().Window().Width - sidebarWidth - 2*pagePad
	var body []core.Node
	body = append(body, a.titleBlock(p, greeting(time.Now()), i18n.Today(time.Now())))
	for i, sec := range a.home.Sections {
		body = append(body, a.browseSection(cx, p, sec, w, "", i)...)
	}
	return a.pageScroll(p, body...)
}

// emptyHome invites the listener to add a folder.
func (a *app) emptyHome(p palette) core.Node {
	title, sub := i18n.T("Add a music folder"), i18n.T("Choose where your music lives, and it will all be here: MP3, FLAC, Ogg, WAV and M4A files, with their tags, covers and lyrics.")
	if a.local != nil && len(a.local.Folders()) > 0 {
		title, sub = i18n.T("No music found"), i18n.T("There are no music files in the folders you chose. Try another.")
	}
	if a.local != nil && a.local.Scanning() {
		title, sub = i18n.T("Looking for music…"), i18n.T("Songs will show here as they're found.")
	}
	return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Padding: core.All(40), Gap: 10},
		Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(72), Height: core.Pt(72), CornerRadius: 20, Background: p.sidebar,
				Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Margin: core.Edges{Bottom: 8}},
				Children: []core.Node{icon(lucideFolder, 32, p.muted)}},
			text(title, 22, core.FontWeightBold, p.text),
			core.Text{Content: sub, Style: core.Style{MaxWidth: core.Pt(420), FontSize: 13.5, Color: p.muted, TextAlign: core.TextAlignCenter}},
			a.pillButton(p, lucideFolder, i18n.T("Add Folder…"), p.accent, p.onAccent, a.pickFolders),
		}}
}

// featureCard leads a source's Home with the songs of its first section: the
// first cover over the card's colors, the day and the section's name, and a
// play button. A press opens all of them.
func (a *app) featureCard(p palette, sec music.Section, index int, now time.Time) core.Node {
	tracks := sec.Tracks
	if len(tracks) == 0 {
		return core.Group{}
	}
	cover := tracks[0].Album.ImageURL
	open := func() { a.open(route{page: pageSection, query: strconv.Itoa(index)}) }
	play := func(core.PressEvent) { a.playTracks(tracks, 0) }
	title := sec.Title
	if title == "" {
		title = i18n.T("Recently Played")
	}
	artists := tracks[0].ArtistNames()
	card := base.Button{Label: title, OnPress: func(core.PressEvent) { open() }, StyleFunc: func() core.Style {
		return core.Style{Height: core.Pt(212), Direction: core.Row, AlignItems: core.AlignCenter, CornerRadius: 24,
			Background: core.RGB(0x242126), Overflow: core.Hidden,
			Gradients: []core.Gradient{core.LinearGradient(135, core.RGBA(0x3a3036, 0.96), core.RGBA(0x17151a, 0.98))},
			Hover:     &core.Style{Background: core.RGB(0x2c282e)}}
	}, Children: []core.Node{
		func() core.Node {
			if cover == "" {
				return core.Group{}
			}
			return core.Image{Source: core.Src(cover), ResizeMode: core.Cover, BlurRadius: 34,
				Style: core.Style{Position: core.Absolute, Top: core.Pt(-30), Right: core.Pt(-30), Bottom: core.Pt(-30), Left: core.Pt(-30),
					Opacity: core.Alpha(0.42)}}
		}(),
		core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
			Gradients: []core.Gradient{core.LinearGradient(120, core.RGBA(0x000000, 0.08), core.RGBA(0x000000, 0.62))}}},
		core.View{Style: core.Style{Position: core.Relative, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Column,
			Gap: 6, Padding: core.Symmetric(0, 32)}, Children: []core.Node{
			text(i18n.Today(now), 12, core.FontWeightSemibold, core.RGBA(0xffffff, 0.66)),
			text(title, 32, core.FontWeightBold, core.RGB(0xffffff)),
			core.Text{Content: artists, MaxLines: 1, Style: core.Style{FontSize: 13, Color: core.RGBA(0xffffff, 0.8)}},
			core.View{Style: core.Style{Direction: core.Row, Gap: 10, Margin: core.Edges{Top: 10}}, Children: []core.Node{
				base.Button{Label: i18n.T("Play"), OnPress: play, Style: core.Style{Height: core.Pt(36), Padding: core.Symmetric(0, 18),
					Gap: 8, Direction: core.Row, AlignItems: core.AlignCenter, CornerRadius: 999, Background: core.RGBA(0xffffff, 0.16),
					Hover: &core.Style{Background: core.RGBA(0xffffff, 0.24)}}, Children: []core.Node{
					icon(lucidePlay, 14, core.RGB(0xffffff)),
					text(i18n.T("Play"), 14, core.FontWeightSemibold, core.RGB(0xffffff)),
				}},
			}},
		}},
		func() core.Node {
			if a.cx.Env().Window().Width < 1000 {
				return core.Group{}
			}
			var picks []core.Node
			for i := 0; i < len(tracks) && i < 3; i++ {
				i := i
				trc := tracks[i]
				picks = append(picks, base.Button{Label: trc.Name, OnPress: func(core.PressEvent) { a.playTracks(tracks, i) },
					Style: core.Style{Height: core.Pt(34), Padding: core.Symmetric(0, 10), Gap: 10, Direction: core.Row, AlignItems: core.AlignCenter,
						CornerRadius: 8, Hover: &core.Style{Background: core.RGBA(0xffffff, 0.08)}}, Children: []core.Node{
						text(fmt.Sprint(i+1), 11, core.FontWeightNormal, core.RGBA(0xffffff, 0.4)),
						core.Text{Content: trc.Name, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0),
							FontSize: 13, FontWeight: core.FontWeightMedium, Color: core.RGB(0xffffff)}},
						text(clock(trc.Length()), 11, core.FontWeightNormal, core.RGBA(0xffffff, 0.4)),
					}})
			}
			return core.View{Style: core.Style{Position: core.Relative, Width: core.Pt(300), FlexShrink: 1, Direction: core.Column, Gap: 2,
				Padding: core.Edges{Right: 16}}, Children: picks}
		}(),
		core.View{Style: core.Style{Position: core.Relative, Width: core.Pt(24), Height: core.Pt(24), FlexShrink: 0}, Children: []core.Node{
			icon(lucideChevronRight, 18, core.RGBA(0xffffff, 0.55)),
		}},
	}}
	return core.View{Style: core.Style{Padding: core.Symmetric(0, 12), Margin: core.Edges{Bottom: 6}}, Children: []core.Node{card}}
}

// sectionOf finds the section a section page shows.
func (a *app) sectionOf(r route) *music.Section {
	var page *music.Browse
	if r.id == "" {
		page = a.home
	} else if d := a.browses[r.id]; d != nil {
		page = d.browse
	}
	at, err := strconv.Atoi(r.query)
	if page == nil || err != nil || at < 0 || at >= len(page.Sections) {
		return nil
	}
	return &page.Sections[at]
}

// sectionPage lists all the songs of a feature card, headed as the card is.
func (a *app) sectionPage(cx *core.Scope, p palette, r route) core.Node {
	sec := a.sectionOf(r)
	if sec == nil {
		return a.emptyNote(p, i18n.T("This page is gone"), "")
	}
	tracks := sec.Tracks
	title := sec.PageTitle
	if title == "" {
		title = sec.Title
	}
	if title == "" {
		title = i18n.T("Songs")
	}
	now := time.Now()
	var pic core.Node
	if len(tracks) > 0 && tracks[0].Album.ImageURL != "" {
		pic = a.cover(p, tracks[0].Album.ImageURL, 212, 14)
	} else {
		pic = core.View{Style: core.Style{Width: core.Pt(212), Height: core.Pt(212), CornerRadius: 14, Background: p.accent,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
			text(fmt.Sprint(now.Day()), 96, core.FontWeightBold, p.onAccent)}}
	}
	about := i18n.Songs(len(tracks))
	if sec.Subtitle != "" {
		about += " · " + sec.Subtitle
	}
	sub := core.Text{Content: about, Style: core.Style{FontSize: 13, Color: p.muted}}
	return a.pageScroll(p,
		a.heroNode(p, pic, i18n.Today(now), title, sub, tracks),
		a.trackList(p, tracks, true, a.currentID()),
	)
}

// browseSection is one heading of a source's page, and what it lists.
func (a *app) browseSection(cx *core.Scope, p palette, sec music.Section, width core.Points, pageID string, index int) []core.Node {
	var out []core.Node
	// The songs of a source's Home lead it with a card of their own.
	feature := pageID == "" && index == 0 && len(sec.Tracks) > 0
	if feature {
		out = append(out, a.featureCard(p, sec, index, time.Now()))
	} else if sec.Title != "" || sec.MorePage != "" {
		out = append(out, core.View{Style: core.Style{Padding: core.Symmetric(0, 12)}, Children: []core.Node{a.sectionHeader(p, sec.Title, sec.MorePage)}})
	}
	if len(sec.Tracks) > 0 && !feature {
		tracks := sec.Tracks
		out = append(out, core.View{Style: core.Style{Padding: core.Symmetric(0, 2)}, Children: []core.Node{
			core.List{Count: func() int { return len(tracks) }, Key: func(i int) any { return tracks[i].ID },
				EstimatedSize: 54, Viewport: core.ListViewportParent,
				Row: func(cx *core.Scope, row core.ListRow) core.Node {
					i := row.Index()
					cur := a.pl.Current()
					return a.trackRow(p, tracks, i, true, false, true, cur != nil && cur.ID == tracks[i].ID)
				}},
		}})
	}
	if len(sec.Albums) > 0 {
		out = append(out, a.cardGrid(p, width, 168, len(sec.Albums), func(i int) core.Node { return a.albumCard(p, sec.Albums[i], 168) }))
	}
	if len(sec.Artists) > 0 {
		out = append(out, a.cardGrid(p, width, 168, len(sec.Artists), func(i int) core.Node { return a.artistCard(p, sec.Artists[i], 168) }))
	}
	if len(sec.Playlists) > 0 {
		out = append(out, a.cardGrid(p, width, 168, len(sec.Playlists), func(i int) core.Node { return a.playlistCard(p, sec.Playlists[i], 168) }))
	}
	if len(sec.Links) > 0 {
		out = append(out, a.cardGrid(p, width, 168, len(sec.Links), func(i int) core.Node {
			l := sec.Links[i]
			return a.card(p, 168, func() { a.open(route{page: pageBrowse, id: l.Page}) }, a.cover(p, l.ImageURL, 168, 10), l.Title, l.Subtitle)
		}))
	}
	return out
}

// cardGrid lays n cards in as many columns as fit.
func (a *app) cardGrid(p palette, width core.Points, min core.Points, n int, card func(i int) core.Node) core.Node {
	cols := columns(width, min)
	var cards []core.Node
	for i := 0; i < n; i++ {
		cards = append(cards, card(i))
	}
	return core.View{Style: core.Style{Padding: core.Symmetric(0, 12)}, Children: []core.Node{
		core.View{Style: core.Style{Display: core.DisplayGrid, GridTemplateColumns: core.GridTracks(fmt.Sprintf("repeat(%d, minmax(0, 1fr))", cols)),
			Gap: 20, RowGap: 28, Margin: core.Edges{Top: 12, Bottom: 4}}, Children: cards},
	}}
}

// card is a picture over a name and what it says under it.
func (a *app) card(p palette, size core.Points, onPress func(), pic core.Node, name, sub string) core.Node {
	var children []core.Node
	children = append(children, core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: 10,
		ShadowColor: p.cardShade, ShadowRadius: 8, ShadowOffset: core.Point{Y: 4}}, Children: []core.Node{pic}})
	children = append(children, core.Text{Content: name, MaxLines: 2, Style: core.Style{FontSize: 13.5, FontWeight: core.FontWeightMedium,
		LineHeight: 1.35, Color: p.text}})
	if sub != "" {
		children = append(children, core.Text{Content: sub, MaxLines: 1, Style: core.Style{FontSize: 12, Color: p.muted, Margin: core.Edges{Top: -2}}})
	}
	return base.Button{Label: name, OnPress: func(core.PressEvent) { onPress() },
		StyleFunc: func() core.Style {
			return core.Style{Direction: core.Column, Gap: 6, Padding: core.All(8), CornerRadius: 12, Hover: &core.Style{Background: p.sidebar}}
		}, Children: children}
}

func (a *app) albumCard(p palette, al music.Album, size core.Points) core.Node {
	id := al.ID
	name := al.Name
	var meta []string
	if by := artistsOf(al.Artists); by != "" {
		meta = append(meta, by)
	}
	if len(al.Released) >= 4 {
		meta = append(meta, al.Released[:4])
	}
	return a.card(p, size, func() { a.open(route{page: pageAlbum, id: id}) }, a.cover(p, al.ImageURL, size, 10), name, strings.Join(meta, " · "))
}

func (a *app) artistCard(p palette, ar music.Artist, size core.Points) core.Node {
	id := ar.ID
	return a.card(p, size, func() { a.open(route{page: pageArtist, id: id}) }, a.coverRound(p, ar.ImageURL, size), ar.Name, "")
}

func (a *app) playlistCard(p palette, pl music.Playlist, size core.Points) core.Node {
	id := pl.ID
	pic := a.cover(p, pl.ImageURL, size, 10)
	if pl.ImageURL == "" {
		pic = core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: 10, Background: p.accent,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
			icon(lucideList, size*0.3, p.onAccent)}}
	}
	return a.card(p, size, func() { a.open(route{page: pagePlaylist, id: id}) }, pic, pl.Name, "")
}

func (a *app) coverRound(p palette, url string, size core.Points) core.Node {
	if url == "" {
		return core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2, Background: p.sidebar,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideUser, size*0.34, p.faint)}}
	}
	return core.Image{Source: core.Src(url), ResizeMode: core.Cover, FadeDuration: 160 * time.Millisecond,
		Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2, Background: p.sidebar}}
}

// artistsOf names a list of artists.
func artistsOf(list []music.Artist) string {
	names := make([]string, len(list))
	for i, ar := range list {
		names[i] = ar.Name
	}
	return strings.Join(names, ", ")
}

// trackList is tracks as rows that play, in a page that scrolls with its
// heading.
func (a *app) trackList(p palette, tracks []music.Track, showAlbum bool, current string) core.Node {
	return core.List{Count: func() int { return len(tracks) }, Key: func(i int) any { return tracks[i].ID },
		EstimatedSize: 54, Viewport: core.ListViewportParent,
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			i := row.Index()
			tr := tracks[i]
			r := a.trackRow(p, tracks, i, true, showAlbum, false, current == tr.ID)
			return ui.ContextMenu{Trigger: r, Children: a.trackMenu(tr, "", -1)}
		}}
}

// trackMenu is what a track's menu offers.
func (a *app) trackMenu(tr music.Track, playlist string, index int) []core.Node {
	var items []core.Node
	name, names := tr.Name, tr.ArtistNames()
	items = append(items, ui.MenuItem{Text: i18n.T("Copy Title"), Icon: lucide.Copy, OnSelect: func() {
		a.cx.SetClipboard(name)
	}})
	if names != "" {
		items = append(items, ui.MenuItem{Text: i18n.T("Copy Title and Artists"), Icon: lucide.Copy, OnSelect: func() {
			a.cx.SetClipboard(name + " - " + names)
		}})
	}
	if tr.URL != "" {
		url := tr.URL
		items = append(items, ui.MenuItem{Text: i18n.T("Copy Link"), Icon: lucide.Link2, OnSelect: func() { a.cx.SetClipboard(url) }})
	}
	items = append(items, ui.MenuSeparator{}, ui.MenuItem{Text: i18n.T("Play"), Icon: lucide.Play, OnSelect: func() { a.playTracks([]music.Track{tr}, 0) }})
	if len(a.mine) > 0 {
		var adds []core.Node
		for _, pl := range a.mine {
			id := pl.ID
			adds = append(adds, ui.MenuItem{Text: pl.Name, Icon: lucide.ListPlus, OnSelect: func() {
				a.addToPlaylist(id, []string{tr.ID})
			}})
		}
		items = append(items, ui.Submenu{Text: i18n.T("Add to Playlist"), Icon: lucide.ListPlus,
			Children: adds})
	}
	items = append(items, ui.MenuItem{Text: i18n.T("New Playlist…"), Icon: lucide.Plus, OnSelect: func() {
		pl, err := a.local.CreatePlaylist(tr.Name)
		if err != nil {
			a.say(err.Error())
			return
		}
		a.addToPlaylist(pl.ID, []string{tr.ID})
		a.reload()
	}})
	liked := a.likedSet[tr.ID]
	label := i18n.T("Like")
	if liked {
		label = i18n.T("Unlike")
	}
	items = append(items, ui.MenuSeparator{}, ui.MenuItem{Text: label, Icon: lucide.Heart, OnSelect: func() { a.toggleLike(tr.ID) }})
	if tr.Album.ID != "" {
		id, album := tr.Album.ID, tr.Album.Name
		items = append(items, ui.MenuItem{Text: i18n.Tf("Go to Album: %s", album), Icon: lucide.Disc3, OnSelect: func() { a.open(route{page: pageAlbum, id: id}) }})
	}
	for _, ar := range tr.Artists {
		if ar.ID == "" {
			continue
		}
		id, artist := ar.ID, ar.Name
		items = append(items, ui.MenuItem{Text: i18n.Tf("Go to Artist: %s", artist), Icon: lucide.User, OnSelect: func() {
			a.open(route{page: pageArtist, id: id})
		}})
	}
	if playlist != "" {
		id, at := tr.ID, index
		if at > 0 {
			items = append(items, ui.MenuItem{Text: i18n.T("Move Up"), Icon: lucide.ArrowUp, OnSelect: func() {
				if err := a.local.MovePlaylistTrack(playlist, at, at-1); err != nil {
					a.say(err.Error())
					return
				}
				if d := a.playlists[playlist]; d != nil {
					d.playlist = nil
					a.loadPlaylist(playlist, true)
				}
			}})
		}
		items = append(items, ui.MenuItem{Text: i18n.T("Move Down"), Icon: lucide.ArrowDown, OnSelect: func() {
			if err := a.local.MovePlaylistTrack(playlist, at, at+1); err != nil {
				a.say(err.Error())
				return
			}
			if d := a.playlists[playlist]; d != nil {
				d.playlist = nil
				a.loadPlaylist(playlist, true)
			}
		}})
		items = append(items, ui.MenuSeparator{}, ui.MenuItem{Text: i18n.T("Remove from Playlist"), Icon: lucide.Trash, Variant: ui.MenuItemDanger,
			OnSelect: func() { a.removeFromPlaylist(playlist, []string{id}) }})
	}
	return items
}

// playButtons are the Play and Shuffle buttons of a page.
func (a *app) playButtons(p palette, tracks []music.Track) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Gap: 10, Margin: core.Edges{Top: 10}}, Children: []core.Node{
		a.pillButton(p, lucidePlay, i18n.T("Play"), p.accent, p.onAccent, func() { a.playTracks(tracks, 0) }),
		a.pillButton(p, lucideShuffle, i18n.T("Shuffle"), p.hover, p.text, func() { a.pl.PlayShuffled(tracks); a.heartbeat() }),
	}}
}

// hero is a page's header: a large cover beside the title, what it is,
// and the buttons to play it.
func (a *app) hero(p palette, pic core.Node, kind, title, sub string, tracks []music.Track) core.Node {
	return a.heroNode(p, pic, kind, title, text(sub, 13, core.FontWeightNormal, p.muted), tracks)
}

// heroNode is hero with a line of the app's own under the title.
func (a *app) heroNode(p palette, pic core.Node, kind, title string, sub core.Node, tracks []music.Track) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Gap: 28, AlignItems: core.AlignEnd, Padding: core.Symmetric(0, 12),
		Margin: core.Edges{Bottom: 24}}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(212), Height: core.Pt(212), CornerRadius: 14,
			ShadowColor: p.cardShade, ShadowRadius: 18, ShadowOffset: core.Point{Y: 8}}, Children: []core.Node{pic}},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 8}, Children: []core.Node{
			text(kind, 12, core.FontWeightSemibold, p.accent),
			core.Text{Content: title, MaxLines: 2, Style: core.Style{FontSize: 30, FontWeight: core.FontWeightBold, LetterSpacing: -0.6, Color: p.text}},
			sub,
			core.Show{When: func() bool { return len(tracks) > 0 }, Then: func(*core.Scope) core.Node { return a.playButtons(p, tracks) }},
		}},
	}}
}

func (a *app) likedPage(cx *core.Scope, p palette) core.Node {
	tracks := a.liked
	if len(tracks) == 0 {
		pic := core.View{Style: core.Style{Width: core.Pt(212), Height: core.Pt(212), CornerRadius: 14, Background: p.accent,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideHeart, 64, p.onAccent)}}
		return a.pageScroll(p,
			a.hero(p, pic, i18n.T("Playlist"), i18n.T("Liked Songs"), i18n.Songs(0), nil),
			core.View{Style: core.Style{Direction: core.Column, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(48, 0)}, Children: []core.Node{
				icon(lucideHeart, 32, p.faint),
				text(i18n.T("Songs you like will show here"), 20, core.FontWeightBold, p.text),
				text(i18n.T("Touch the heart of a song to keep it."), 13, core.FontWeightNormal, p.muted),
			}})
	}
	pic := core.View{Style: core.Style{Width: core.Pt(212), Height: core.Pt(212), CornerRadius: 14, Background: p.accent,
		Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideHeart, 64, p.onAccent)}}
	return a.pageScroll(p,
		a.hero(p, pic, i18n.T("Playlist"), i18n.T("Liked Songs"), i18n.Songs(len(tracks)), tracks),
		a.trackList(p, tracks, false, a.currentID()),
	)
}

func (a *app) playlistPage(cx *core.Scope, p palette, id string) core.Node {
	d := a.playlists[id]
	if d == nil || d.playlist == nil {
		if d != nil && d.err != "" {
			return a.emptyNote(p, i18n.T("Couldn't load this playlist"), d.err)
		}
		return a.loadingPage(p)
	}
	pl := d.playlist
	if len(pl.Tracks) == 0 {
		return a.pageScroll(p, core.Group{
			a.playlistHero(p, pl),
			a.emptyNote(p, i18n.T("This playlist is empty"), i18n.T("Add songs to it from their menus.")),
		})
	}
	return a.pageScroll(p, core.Group{
		a.playlistHero(p, pl),
		core.List{Count: func() int { return len(pl.Tracks) }, Key: func(i int) any { return pl.Tracks[i].ID },
			EstimatedSize: 54, Viewport: core.ListViewportParent,
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				i := row.Index()
				tr := pl.Tracks[i]
				r := a.trackRow(p, pl.Tracks, i, true, false, true, a.currentID() == tr.ID)
				return ui.ContextMenu{Trigger: r, Children: a.trackMenu(tr, pl.ID, i)}
			}},
	})
}

func (a *app) playlistHero(p palette, pl *music.Playlist) core.Node {
	pic := a.cover(p, pl.ImageURL, 212, 14)
	if pl.ImageURL == "" {
		pic = core.View{Style: core.Style{Width: core.Pt(212), Height: core.Pt(212), CornerRadius: 14, Background: p.accent,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideList, 64, p.onAccent)}}
	}
	head := a.hero(p, pic, i18n.T("Playlist"), pl.Name, i18n.Songs(len(pl.Tracks)), pl.Tracks)
	id, name := pl.ID, pl.Name
	return ui.ContextMenu{Trigger: head, Children: []core.Node{
		ui.MenuItem{Text: i18n.T("Rename Playlist…"), Icon: lucide.Pencil, OnSelect: func() { a.renamePlaylist(id, name) }},
		ui.MenuItem{Text: i18n.T("Delete Playlist"), Icon: lucide.Trash, Variant: ui.MenuItemDanger, OnSelect: func() { a.confirmDelete(id, name) }},
	}}
}

func (a *app) albumPage(cx *core.Scope, p palette, id string) core.Node {
	d := a.albums[id]
	if d == nil || d.album == nil {
		if d != nil && d.err != "" {
			return a.emptyNote(p, i18n.T("Couldn't load this album"), d.err)
		}
		return a.loadingPage(p)
	}
	al := &d.album.Album
	tracks := d.album.Tracks
	var spans []core.Span
	for i, ar := range al.Artists {
		if i > 0 {
			spans = append(spans, core.Span{Content: ", "})
		}
		id := ar.ID
		spans = append(spans, core.Span{Content: ar.Name, Style: core.Style{Color: p.text},
			OnPress: func(core.PressEvent) { a.open(route{page: pageArtist, id: id}) }})
	}
	var meta []string
	if al.Released != "" {
		meta = append(meta, al.Released)
	}
	meta = append(meta, i18n.Songs(len(tracks)))
	if len(spans) > 0 {
		spans = append(spans, core.Span{Content: " · "})
	}
	spans = append(spans, core.Span{Content: strings.Join(meta, " · ")})
	sub := core.Text{Spans: spans, Style: core.Style{FontSize: 13, Color: p.muted}}
	return a.pageScroll(p,
		a.heroNode(p, a.cover(p, al.ImageURL, 212, 14), i18n.T("Album"), al.Name, sub, tracks),
		a.trackList(p, tracks, false, a.currentID()),
	)
}

func (a *app) artistPage(cx *core.Scope, p palette, id string) core.Node {
	d := a.artists[id]
	if d == nil || d.page == nil {
		if d != nil && d.err != "" {
			return a.emptyNote(p, i18n.T("Couldn't load this artist"), d.err)
		}
		return a.loadingPage(p)
	}
	page := d.page
	w := cx.Env().Window().Width - sidebarWidth - 2*pagePad

	// What the page lists follows what it has: songs only, albums only, or
	// the tab chosen when it has both.
	tab := d.tab
	switch {
	case len(page.Tracks) == 0:
		tab = 1
	case len(page.Albums) == 0:
		tab = 0
	}

	setTab := func(next int) func() {
		return func() {
			d.tab = next
			a.bump()
		}
	}

	var meta []string
	if len(page.Tracks) > 0 {
		meta = append(meta, i18n.Songs(len(page.Tracks)))
	}
	switch n := len(page.Albums); n {
	case 1:
		meta = append(meta, i18n.T("1 album"))
	case 0:
	default:
		meta = append(meta, i18n.Tf("%d albums", n))
	}

	header := core.View{Style: core.Style{Direction: core.Row, Gap: 28, AlignItems: core.AlignEnd, Padding: core.Symmetric(0, 12),
		Margin: core.Edges{Bottom: 8}}, Children: []core.Node{
		a.coverRound(p, page.Artist.ImageURL, 212),
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 8}, Children: []core.Node{
			text(i18n.T("Artist"), 12, core.FontWeightSemibold, p.accent),
			core.Text{Content: page.Artist.Name, MaxLines: 2, Style: core.Style{FontSize: 30, FontWeight: core.FontWeightBold,
				LetterSpacing: -0.6, LineHeight: 1.2, Color: p.text}},
			text(strings.Join(meta, " · "), 13, core.FontWeightNormal, p.muted),
			core.Show{When: func() bool { return len(page.Tracks) > 0 }, Then: func(*core.Scope) core.Node {
				return a.playButtons(p, page.Tracks)
			}},
		}},
	}}

	tabs := core.View{Style: core.Style{Direction: core.Row, Gap: 2, Padding: core.All(2), Background: p.hover,
		CornerRadius: 8, AlignSelf: core.AlignEnd}, Children: func() []core.Node {
		var buttons []core.Node
		for i, label := range []string{i18n.T("Songs"), i18n.T("Albums")} {
			index := i
			selected := tab == index
			bg, fg := core.Color(0), p.muted
			if selected {
				bg, fg = p.bg, p.text
			}
			buttons = append(buttons, base.Button{Label: label, OnPress: func(core.PressEvent) { setTab(index)() },
				Style: core.Style{Padding: core.Symmetric(5, 12), CornerRadius: 7, Background: bg,
					Hover: &core.Style{Background: p.sidebar}},
				Children: []core.Node{text(label, 12.5, core.FontWeightMedium, fg)}})
		}
		return buttons
	}()}
	var body []core.Node
	body = append(body, header)
	if len(page.Tracks) > 0 && len(page.Albums) > 0 {
		label := i18n.T("Songs")
		if tab == 1 {
			label = i18n.T("Albums")
		}
		body = append(body, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Margin: core.Edges{Top: 26, Bottom: 14}},
			Children: []core.Node{
				core.Text{Content: label, Style: core.Style{FlexGrow: 1, FontSize: 19, FontWeight: core.FontWeightBold,
					LetterSpacing: -0.3, Color: p.text}},
				tabs,
			}})
	} else {
		section := i18n.T("Songs")
		if tab == 1 {
			section = i18n.T("Albums")
		}
		body = append(body, core.View{Style: core.Style{Padding: core.Symmetric(0, 12)}, Children: []core.Node{a.sectionHeader(p, section, "")}})
	}
	if tab == 1 {
		body = append(body, a.cardGrid(p, w, 168, len(page.Albums), func(i int) core.Node { return a.albumCard(p, page.Albums[i], 168) }))
	} else {
		body = append(body, a.trackList(p, page.Tracks, true, a.currentID()))
	}
	return a.pageScroll(p, body...)
}

func (a *app) browsePage(cx *core.Scope, p palette, id string) core.Node {
	d := a.browses[id]
	if d == nil || d.browse == nil {
		if d != nil && d.err != "" {
			return a.emptyNote(p, i18n.T("Couldn't load this page"), d.err)
		}
		return a.loadingPage(p)
	}
	b := d.browse
	w := cx.Env().Window().Width - sidebarWidth - 2*pagePad
	var body []core.Node
	if b.Title != "" {
		body = append(body, a.titleBlock(p, b.Title, b.Subtitle))
	}
	for i, sec := range b.Sections {
		body = append(body, a.browseSection(cx, p, sec, w, id, i)...)
	}
	return a.pageScroll(p, body...)
}

func (a *app) loadingPage(p palette) core.Node {
	return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Children: []core.Node{text(i18n.T("Loading…"), 13, core.FontWeightNormal, p.muted)}}
}

// searchPage is a search's results, by kind.
func (a *app) searchPage(cx *core.Scope, p palette, r route) core.Node {
	if r.query == "" {
		return a.searchHistoryPage(p)
	}
	d := a.searches[r]
	if d == nil || d.loading && !d.loaded {
		return a.pageScroll(p, a.searchHeader(p, r), a.loadingPage(p))
	}
	if d.err != "" {
		return a.pageScroll(p, a.searchHeader(p, r), a.emptyNote(p, i18n.T("The search failed"), d.err))
	}
	w := cx.Env().Window().Width - sidebarWidth - 2*pagePad
	var body []core.Node
	switch r.kind {
	case music.SearchTracks:
		if len(d.res.Tracks) == 0 {
			body = append(body, a.emptyNote(p, i18n.T("No songs found"), i18n.T("Try another search.")))
		} else {
			body = append(body, a.trackList(p, d.res.Tracks, true, a.currentID()))
		}
	case music.SearchAlbums:
		if len(d.res.Albums) == 0 {
			body = append(body, a.emptyNote(p, i18n.T("No albums found"), i18n.T("Try another search.")))
		} else {
			body = append(body, a.cardGrid(p, w, 168, len(d.res.Albums), func(i int) core.Node { return a.albumCard(p, d.res.Albums[i], 168) }))
		}
	case music.SearchArtists:
		if len(d.res.Artists) == 0 {
			body = append(body, a.emptyNote(p, i18n.T("No artists found"), i18n.T("Try another search.")))
		} else {
			body = append(body, a.cardGrid(p, w, 168, len(d.res.Artists), func(i int) core.Node { return a.artistCard(p, d.res.Artists[i], 168) }))
		}
	case music.SearchPlaylists:
		if len(d.res.Playlists) == 0 {
			body = append(body, a.emptyNote(p, i18n.T("No playlists found"), i18n.T("Try another search.")))
		} else {
			body = append(body, a.cardGrid(p, w, 168, len(d.res.Playlists), func(i int) core.Node { return a.playlistCard(p, d.res.Playlists[i], 168) }))
		}
	}
	return a.pageScroll(p, append([]core.Node{a.searchHeader(p, r)}, body...)...)
}

// searchKinds are the kinds of search and their tabs.
var searchKinds = []struct {
	kind  music.SearchKind
	label string
}{
	{music.SearchTracks, i18n.N("Songs")},
	{music.SearchArtists, i18n.N("Artists")},
	{music.SearchAlbums, i18n.N("Albums")},
	{music.SearchPlaylists, i18n.N("Playlists")},
}

func (a *app) searchHeader(p palette, r route) core.Node {
	var tabs []core.Node
	for _, k := range searchKinds {
		kind := k.kind
		selected := r.kind == kind
		fg := p.muted
		bg := core.Color(0)
		if selected {
			fg, bg = p.text, p.hover
		}
		tabs = append(tabs, base.Button{Label: i18n.T(k.label), OnPress: func(core.PressEvent) {
			a.open(route{page: pageSearch, query: r.query, kind: kind})
		}, Style: core.Style{Padding: core.Symmetric(6, 14), CornerRadius: 999, Background: bg, Hover: &core.Style{Background: p.sidebar}},
			Children: []core.Node{text(i18n.T(k.label), 13, core.FontWeightSemibold, fg)}})
	}
	// A long search locks the field, so its results can be chosen.
	return core.View{Style: core.Style{Padding: core.Symmetric(0, 12), Gap: 14, Margin: core.Edges{Bottom: 18}}, Children: []core.Node{
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			text(i18n.T("Search"), 12, core.FontWeightSemibold, p.muted),
			text("“"+r.query+"”", 30, core.FontWeightBold, p.text),
		}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 6, AlignItems: core.AlignCenter}, Children: tabs},
	}}
}

func (a *app) searchHistoryPage(p palette) core.Node {
	var body []core.Node
	body = append(body, a.titleBlock(p, i18n.T("Search"), ""))
	if len(a.set.SearchHistory) == 0 {
		return a.pageScroll(p, append(body, a.emptyNote(p, i18n.T("Search your library"), i18n.T("Find songs, albums and artists.")))...)
	}
	var rows []core.Node
	rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 12)}, Children: []core.Node{
		text(i18n.T("Recent Searches"), 14, core.FontWeightSemibold, p.text),
		core.View{Style: core.Style{FlexGrow: 1}},
		a.footerRow(p, "clear-history", lucideTrash, i18n.T("Clear"), func() {
			a.set.SearchHistory = nil
			a.set.save()
			a.bump()
		}),
	}})
	for _, q := range a.set.SearchHistory {
		query := q
		rows = append(rows, base.Button{Label: query, OnPress: func(core.PressEvent) {
			a.open(route{page: pageSearch, query: query, kind: music.SearchTracks})
		}, StyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(38), Padding: core.Symmetric(0, 12), Gap: 10, Direction: core.Row, AlignItems: core.AlignCenter,
				CornerRadius: 8, Hover: &core.Style{Background: p.sidebar}}
		}, Children: []core.Node{icon(lucideSearch, 14, p.faint), text(query, 13, core.FontWeightNormal, p.text)}})
	}
	return a.pageScroll(p, body[0], core.View{Style: core.Style{Gap: 2}, Children: rows})
}

// currentID is the ID of the track playing, "" for none.
func (a *app) currentID() string {
	if t := a.pl.Current(); t != nil {
		return t.ID
	}
	return ""
}
