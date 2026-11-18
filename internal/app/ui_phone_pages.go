package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// All sources share the phone's editorial layout. The source supplies content,
// while the app keeps navigation, density and listening controls consistent.
func (a *app) phoneHome(cx *core.Scope, p palette) core.Node {
	if a.home == nil && a.homeErr == "" && a.homeBusy {
		return a.phoneSkeleton(p, true)
	}
	if a.home == nil || len(a.home.Sections) == 0 {
		if a.homeErr != "" {
			return a.pageScroll(p, a.phoneNotice(p, "phone-home-retry", i18n.T("Couldn't refresh your music"), a.homeErr, a.reload))
		}
		return a.emptyHome(p)
	}
	body := []core.Node{core.View{Style: core.Style{Margin: core.Edges{Top: 2, Bottom: 10}}, Children: []core.Node{
		text(i18n.Today(a.localTime()), phoneFontCaption, core.FontWeightNormal, p.muted),
	}}}
	if a.homeErr != "" {
		body = append(body, a.phoneNotice(p, "phone-home-retry", i18n.T("Couldn't refresh your music"), a.homeErr, a.reload))
	}
	featured := -1
	for i, sec := range a.home.Sections {
		if len(sec.Tracks) == 0 {
			continue
		}
		if featured < 0 {
			featured = i
		}
		if sec.MorePage == "daily" {
			featured = i
			break
		}
	}
	if featured >= 0 {
		body = append(body, a.phoneFeature(p, a.home.Sections[featured], featured))
	}
	if shortcuts := a.phoneDiscoveryShortcuts(p); len(shortcuts) > 0 {
		body = append(body, core.View{TestID: "phone-discovery-shortcuts", Style: core.Style{Direction: core.Row, Gap: 8, Margin: core.Edges{Top: 14, Bottom: 8}}, Children: shortcuts})
	}
	if featured >= 0 {
		body = append(body, a.phoneHomeSection(p, a.home.Sections[featured], featured, 3)...)
	}
	var recentSections []int
	for i, sec := range a.home.Sections {
		if i == featured {
			continue
		}
		if sec.MorePage == "recent" {
			recentSections = append(recentSections, i)
			continue
		}
		body = append(body, a.phoneHomeSection(p, sec, i, 3)...)
	}
	for _, i := range recentSections {
		sec := a.home.Sections[i]
		body = append(body, a.phoneSectionHeading(p, sec.Title, func() { a.openPhoneSection("", i, sec.MorePage) }), a.phoneRecentRail(p, sec.Tracks))
	}

	return a.pageScroll(p, body...)
}

func (a *app) openPhoneSection(pageID string, index int, more string) {
	if more != "" {
		a.open(route{page: pageBrowse, id: more})
		return
	}
	a.open(route{page: pageSection, id: pageID, query: strconv.Itoa(index)})
}

func (a *app) phoneFeature(p palette, sec music.Section, index int) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.playRev.Get()
		a.artRev.Get()
		tracks := sec.Tracks
		deep, rich, _ := dailyColors(a.coverTint(tracks[0].Album.ImageURL))
		white := core.RGB(0xffffff)
		cover := core.Points(104)
		if cx.Env().Window().Width < 350 {
			cover = 88
		}
		ours, playing := a.dailyPlaying(tracks)
		caption, title, by := i18n.T("A little music for your day"), sec.Title, dailyArtists(tracks)
		if a.source() == a.local {
			caption, title, by = i18n.T("Continue listening"), tracks[0].Name, tracks[0].ArtistNames()
		}
		glyph, label := lucidePlay, i18n.T("Play")
		if ours && playing {
			glyph, label = lucidePause, i18n.T("Pause")
		}
		play := base.Button{TestID: "daily-play", Label: label, OnPress: func(core.PressEvent) {
			if current, _ := a.dailyPlaying(tracks); current {
				a.toggle()
			} else {
				a.playTracks(tracks, 0)
			}
		}, Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), CornerRadius: 24, Background: white, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Active: &core.Style{Opacity: core.Alpha(.8)}}, Children: []core.Node{icon(glyph, 22, deep)}}
		return core.View{TestID: "daily", Role: core.RoleButton, Label: sec.Title, OnPress: func(core.PressEvent) { a.openPhoneSection("", index, sec.MorePage) },
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16, MinHeight: core.Pt(144), Padding: core.All(16), CornerRadius: 20, Overflow: core.Hidden, Background: deep,
				Gradients: []core.Gradient{core.LinearGradient(125, deep, rich)}, Active: &core.Style{Opacity: core.Alpha(.94)}}, Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 7}, Children: []core.Node{
					text(caption, 10.5, core.FontWeightMedium, white.Faded(.65)),
					core.Text{Content: title, MaxLines: 2, Style: core.Style{FontSize: phoneFontTitle, FontWeight: core.FontWeightSemibold, LineHeightMultiple: 1.18, Color: white}},
					core.Text{Content: by, MaxLines: 1, Style: core.Style{FontSize: 10.5, Color: white.Faded(.7)}}, play,
				}},
				core.View{Style: core.Style{Width: core.Pt(cover), Height: core.Pt(cover), FlexShrink: 0, CornerRadius: 14, ShadowColor: core.RGBA(0x000000, .25), ShadowRadius: 16, ShadowOffset: core.Point{Y: 6}}, Children: []core.Node{a.cover(p, tracks[0].Album.ImageURL, cover, 14)}},
			}}
	})
}

func (a *app) phoneHomeSection(p palette, sec music.Section, index, limit int) []core.Node {
	var body []core.Node
	if len(sec.Tracks) > 0 {
		title := sec.Title
		if sec.MorePage == "daily" {
			title = i18n.T("Featured songs")
		}
		body = append(body, a.phoneSectionHeading(p, title, func() { a.openPhoneSection("", index, sec.MorePage) }))
		for i := range min(limit, len(sec.Tracks)) {
			body = append(body, a.phoneTrackRow(p, sec.Tracks, i, ""))
		}
	}
	cards := a.phoneSectionCards(p, sec)
	if len(cards) > 0 {
		// A source can put several content types in one section. Keep the heading
		// once and let its actual artwork set the visual rhythm.
		if len(sec.Tracks) == 0 {
			body = append(body, a.phoneSectionHeading(p, sec.Title, func() { a.openPhoneSection("", index, sec.MorePage) }))
		}
		body = append(body, a.phoneRail(cards))
	}
	if sec.Empty != "" && len(sec.Tracks) == 0 && len(cards) == 0 {
		body = append(body, text(sec.Empty, phoneFontBody, core.FontWeightNormal, p.muted))
	}
	return body
}

func (a *app) phoneSectionCards(p palette, sec music.Section) []core.Node {
	var cards []core.Node
	wrap := func(card core.Node) {
		cards = append(cards, core.View{Style: core.Style{Width: core.Pt(142), FlexShrink: 0}, Children: []core.Node{card}})
	}
	for _, v := range sec.Albums {
		wrap(a.albumCard(p, v))
	}
	for _, v := range sec.Playlists {
		wrap(a.playlistCard(p, v))
	}
	for _, v := range sec.Artists {
		wrap(a.artistCard(p, v))
	}
	for _, v := range sec.Shows {
		wrap(a.showCard(p, v))
	}
	for _, v := range sec.Stations {
		wrap(a.stationCard(p, v))
	}
	for _, v := range sec.Links {
		wrap(a.linkCard(p, v))
	}
	return cards
}

func (a *app) phoneRecentRail(p palette, tracks []music.Track) core.Node {
	var cards []core.Node
	for i := range min(8, len(tracks)) {
		tr, index := tracks[i], i
		cards = append(cards, base.Button{TestID: "phone-recent-" + tr.ID, Label: tr.Name, OnPress: func(core.PressEvent) { a.playTracks(tracks, index) },
			Style: core.Style{Width: core.Pt(104), FlexShrink: 0, Gap: 7, Active: &core.Style{Opacity: core.Alpha(.7)}}, Children: []core.Node{
				a.cover(p, tr.Album.ImageURL, 104, 12), core.Text{Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: phoneFontCaption, FontWeight: core.FontWeightMedium, Color: p.text}},
				core.Text{Content: tr.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: 10.5, Color: p.muted}},
			}})
	}
	return a.phoneRail(cards)
}

func (a *app) phoneRail(cards []core.Node) core.Node {
	return core.Scroll{Horizontal: true, HideIndicator: true, Style: core.Style{Margin: core.Edges{Bottom: 10}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12, Padding: core.Edges{Bottom: 6}}, Children: cards},
	}}
}

func (a *app) phoneSectionHeading(p palette, title string, more func()) core.Node {
	var action core.Node = core.Group{}
	if more != nil {
		action = a.phoneIcon(p, "", i18n.T("See All"), lucideChevronRight, p.muted, more)
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(48), Margin: core.Edges{Top: 12, Bottom: 2}}, Children: []core.Node{
		core.Text{Content: title, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: phoneFontSection, FontWeight: core.FontWeightSemibold, Color: p.text}}, action,
	}}
}

// Source navigation is promoted into listening destinations, with stable IDs
// kept for source-provided pages. Unknown destinations remain available below
// the listener's own collection rather than taking over its first screen.
func phoneNavRoute(item music.NavItem) (route, string) {
	switch item.Builtin {
	case "home":
		return route{page: pageHome}, "home"
	case "liked":
		return route{page: pageLiked}, "liked"
	case "savedAlbums":
		return route{page: pageSavedAlbums}, "savedAlbums"
	case "stations":
		return route{page: pageStations}, "stations"
	case "shows":
		return route{page: pageShows}, "shows"
	default:
		return route{page: pageBrowse, id: item.Page}, "browse"
	}
}
func (a *app) phoneDiscoveryShortcuts(p palette) []core.Node {
	var result []core.Node
	for i, item := range a.phoneNavigationItems() {
		target, cap := phoneNavRoute(item)
		if !a.can(cap) || cap == "home" || cap == "liked" || cap == "savedAlbums" || cap == "shows" || item.Page == "recent" || item.Page == "cloud" {
			continue
		}
		label := item.Title
		if label == "" {
			label = i18n.T("Radio")
		}
		result = append(result, a.phoneShortcut(p, fmt.Sprintf("source-nav-%d", i), label, sourceNavIcon(item.Icon, target.page), func() { a.open(target) }))
		if len(result) == 3 {
			break
		}
	}
	return result
}
func (a *app) phoneShortcut(p palette, id, label string, svg *core.SVG, action func()) core.Node {
	return base.Button{TestID: id, Label: label, OnPress: func(core.PressEvent) { action() }, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), MinHeight: core.Pt(68), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 7, Padding: core.Symmetric(8, 2), CornerRadius: 12, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(34), Height: core.Pt(30), CornerRadius: 10, Background: p.accent.Faded(.07), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(svg, 19, p.accent)}},
		core.Text{Content: label, MaxLines: 1, Style: core.Style{FontSize: phoneFontCaption, FontWeight: core.FontWeightMedium, Color: p.text}},
	}}
}

func (a *app) phoneLibrary(cx *core.Scope, p palette) core.Node {
	var body []core.Node
	body = append(body, a.phoneLibraryRow(p, "phone-stored", i18n.T("Saved Music"), i18n.T("Offline copies and automatic cache"), lucideFolder, a.openStored))
	if a.source() != a.local && (a.can("account") || a.can("signIn")) {
		body = append(body, core.View{Style: core.Style{Margin: core.Edges{Bottom: 12}}, Children: []core.Node{a.accountRow(p)}})
	}
	likedID := "phone-liked"
	for i, item := range a.phoneNavigationItems() {
		if item.Builtin == "liked" {
			likedID = fmt.Sprintf("source-nav-%d", i)
		}
	}
	if a.can("liked") {
		likedCount := i18n.Songs(len(a.liked))
		if a.likedBusy && len(a.liked) == 0 {
			likedCount = i18n.T("Loading…")
		}
		body = append(body, base.Button{TestID: likedID, Label: i18n.T("Liked Songs"), OnPress: func(core.PressEvent) { a.open(route{page: pageLiked}) }, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14, Padding: core.All(14), MinHeight: core.Pt(84), CornerRadius: 16, Background: p.accent.Faded(.065), Active: &core.Style{Background: p.accent.Faded(.1)}}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(52), Height: core.Pt(52), CornerRadius: 14, Background: p.accent, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideHeart, 25, p.onAccent)}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{text(i18n.T("Liked Songs"), phoneFontSong, core.FontWeightSemibold, p.text), text(likedCount, phoneFontCaption, core.FontWeightNormal, p.muted)}},
			icon(lucideChevronRight, 18, p.muted),
		}})
	}
	var shortcuts []core.Node
	if a.source() == a.local {
		for _, item := range []struct {
			id, label, page string
			svg             *core.SVG
		}{{"phone-songs", i18n.T("Songs"), "songs", lucideNote}, {"phone-albums", i18n.T("Albums"), "albums", lucideDisc}, {"phone-artists", i18n.T("Artists"), "artists", lucideUser}} {
			it := item
			shortcuts = append(shortcuts, a.phoneShortcut(p, it.id, it.label, it.svg, func() { a.open(route{page: pageBrowse, id: it.page}) }))
		}
	} else {
		for i, item := range a.phoneNavigationItems() {
			target, cap := phoneNavRoute(item)
			if !a.can(cap) || !(cap == "savedAlbums" || cap == "shows" || item.Page == "recent" || item.Page == "cloud") {
				continue
			}
			title := item.Title
			if title == "" {
				if cap == "shows" {
					title = i18n.T("Podcasts")
				} else {
					title = i18n.T("Saved Albums")
				}
			}
			shortcuts = append(shortcuts, a.phoneShortcut(p, fmt.Sprintf("source-nav-%d", i), title, sourceNavIcon(item.Icon, target.page), func() { a.open(target) }))
		}
	}
	if len(shortcuts) > 0 {
		body = append(body, core.View{Style: core.Style{Direction: core.Row, Gap: 6, Margin: core.Edges{Top: 8}}, Children: shortcuts})
	}
	if a.can("playlists") || a.can("createPlaylist") || len(a.mine) > 0 {
		body = append(body, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Margin: core.Edges{Top: 18, Bottom: 6}}, Children: []core.Node{
			core.Text{Content: i18n.T("My Playlists"), Style: core.Style{FlexGrow: 1, FontSize: phoneFontSection, FontWeight: core.FontWeightSemibold, Color: p.text}},
			core.Show{When: func() bool { return a.can("createPlaylist") }, Then: func(*core.Scope) core.Node {
				return a.phoneIcon(p, "new-playlist", i18n.T("New Playlist"), lucidePlus, p.text, a.newPlaylist)
			}},
		}})
		if a.playlistsBusy && len(a.mine) == 0 {
			body = append(body, a.phoneSkeletonRows(p, 2))
		} else if a.playlistsErr != "" {
			body = append(body, a.phoneNotice(p, "phone-library-retry", i18n.T("Couldn't refresh your playlists"), a.playlistsErr, a.reload))
		}
		if len(a.mine) == 0 && !a.playlistsBusy && a.playlistsErr == "" {
			body = append(body, core.View{Style: core.Style{Padding: core.Symmetric(16, 4), Gap: 6}, Children: []core.Node{
				text(i18n.T("Make room for your favorites"), phoneFontBody, core.FontWeightMedium, p.text), core.Text{Content: i18n.T("Create a playlist for your favorite moments"), Style: core.Style{FontSize: phoneFontCaption, Color: p.muted}},
			}})
		}
		for _, list := range a.mine {
			body = append(body, a.phonePlaylistRow(p, list))
		}
	}

	if a.source() != a.local {
		var more []core.Node
		for i, item := range a.phoneNavigationItems() {
			target, cap := phoneNavRoute(item)
			if !a.can(cap) || cap == "home" || cap == "liked" || cap == "savedAlbums" || cap == "shows" || item.Page == "recent" || item.Page == "cloud" {
				continue
			}
			title := item.Title
			if title == "" {
				title = i18n.T("Radio")
			}
			more = append(more, a.phoneLibraryRow(p, fmt.Sprintf("source-nav-%d", i), title, "", sourceNavIcon(item.Icon, target.page), func() { a.open(target) }))
		}
		if len(more) > 0 {
			body = append(body, a.phoneSectionHeading(p, i18n.T("Explore more"), nil), core.Group(more))
		}
	} else {
		body = append(body, core.View{Style: core.Style{Margin: core.Edges{Top: 20}}, Children: []core.Node{a.phoneLibraryRow(p, "phone-library-import", i18n.T("Add Folder…"), i18n.T("Import local music"), lucideFolder, a.pickFolders)}})
	}
	return a.pageScroll(p, body...)
}

func (a *app) phonePlaylistRow(p palette, pl music.Playlist) core.Node {
	var more core.Node = core.Group{}
	if a.playlistEditable(pl) {
		more = a.phoneIcon(p, "playlist-more-"+pl.ID, i18n.T("More"), lucideMore, p.muted, func() { a.phonePlaylist = pl.ID; a.openPhoneSheet("playlist") })
	}
	sub := playlistLine(pl)
	if sub == "" {
		sub = i18n.Songs(pl.TrackCount)
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2, MinHeight: core.Pt(68)}, Children: []core.Node{
		base.Button{TestID: "playlist-" + pl.ID, Label: pl.Name, OnPress: func(core.PressEvent) { a.open(route{page: pagePlaylist, id: pl.ID}) }, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 13, Padding: core.Symmetric(8, 0), CornerRadius: 12, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
			a.cover(p, pl.ImageURL, 48, 10), core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{
				core.Text{Content: pl.Name, MaxLines: 2, Style: core.Style{FontSize: phoneFontSong, FontWeight: core.FontWeightNormal, Color: p.text}}, text(sub, phoneFontCaption, core.FontWeightNormal, p.muted),
			}},
		}}, more,
	}}
}

// Secondary destinations look like lightweight list rows; collections and
// artwork, rather than configuration cards, own the library's visual weight.
func (a *app) phoneLibraryRow(p palette, id, title, sub string, svg *core.SVG, action func()) core.Node {
	var labels []core.Node
	labels = append(labels, core.Text{Content: title, MaxLines: 1, Style: core.Style{FontSize: phoneFontBody, FontWeight: core.FontWeightMedium, Color: p.text}})
	if strings.TrimSpace(sub) != "" {
		labels = append(labels, core.Text{Content: sub, MaxLines: 1, Style: core.Style{FontSize: phoneFontCaption, Color: p.muted}})
	}
	return base.Button{TestID: id, Label: title, OnPress: func(core.PressEvent) { action() }, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(52), Gap: 12, Padding: core.Symmetric(8, 4), CornerRadius: 10, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
		icon(svg, 20, p.muted), core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 4}, Children: labels}, icon(lucideChevronRight, 17, p.faint),
	}}
}

func (a *app) phoneNavigationItems() []music.NavItem {
	if nav := a.source().Info().Navigation; len(nav) > 0 {
		return nav
	}
	return []music.NavItem{{Builtin: "home"}, {Builtin: "liked"}, {Builtin: "savedAlbums"}, {Builtin: "shows"}, {Builtin: "stations"}}
}
