package app

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"nsgo-music/internal/i18n"
	"nsgo-music/internal/music"
)

const sidebarWidth = 232

// sidebar is the app's navigation: the source, a search field, the app's
// pages, the listener's playlists, and the settings at the foot.
func (a *app) sidebar(cx *core.Scope, top core.Points) core.Node {
	p := a.palette(cx)
	r := a.route.Get()
	return core.View{Style: core.Style{Width: core.Pt(sidebarWidth), Height: core.Pct(100), Background: p.sidebar,
		Direction: core.Column, Padding: core.Edges{Top: top + 4, Right: 14, Bottom: 14, Left: 14}, Gap: 4}, Children: []core.Node{
		// The top strip moves the window where the source row is not.
		core.View{Style: core.Style{Height: core.Pt(4)}, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}},
		core.Dynamic(func(cx *core.Scope) core.Node { a.rev.Get(); return a.sourceRow(p) }),
		a.searchField(p),
		core.Scroll{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node {
				a.rev.Get()
				return core.View{Style: core.Style{Direction: core.Column, Gap: 2, Padding: core.Symmetric(2, 0)}, Children: []core.Node{
					core.Group{
						a.navItem(p, "nav-home", lucideHome, i18n.T("Home"), r.page == pageHome, func() { a.open(route{page: pageHome}) }),
						a.navItem(p, "nav-songs", lucideNote, i18n.T("Songs"), r.page == pageBrowse && r.id == "songs", func() { a.open(route{page: pageBrowse, id: "songs"}) }),
						a.navItem(p, "nav-albums", lucideDisc, i18n.T("Albums"), r.page == pageBrowse && r.id == "albums", func() { a.open(route{page: pageBrowse, id: "albums"}) }),
						a.navItem(p, "nav-artists", lucideUser, i18n.T("Artists"), r.page == pageBrowse && r.id == "artists", func() { a.open(route{page: pageBrowse, id: "artists"}) }),
						a.navItem(p, "nav-liked", lucideHeart, i18n.T("Liked Songs"), r.page == pageLiked, func() { a.open(route{page: pageLiked}) }),
						core.Show{When: func() bool { return a.lastSearch.query != "" },
							Then: func(*core.Scope) core.Node {
								last := a.lastSearch
								return a.navItem(p, "nav-results", lucideSearch, i18n.T("Search Results"), r.page == pageSearch, func() { a.open(last) })
							}},
					},
					a.playlistSection(p, r),
				}}
			}),
		}},
		a.sidebarFooter(p),
	}}
}

// sourceRow names the source the library is of, and what is being scanned.
func (a *app) sourceRow(p palette) core.Node {
	count := ""
	if a.local != nil {
		if n := a.local.TrackCount(); n > 0 {
			count = i18n.Songs(n)
		}
		if a.local.Scanning() {
			count = i18n.T("Looking for music…")
		}
	}
	var sub []core.Node
	if count != "" {
		sub = append(sub, text(count, 11, core.FontWeightNormal, p.muted))
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(6, 8),
		CornerRadius: 8, Hover: &core.Style{Background: p.hover}}, OnPress: func(core.PressEvent) {
		a.open(route{page: pageHome})
	}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(28), Height: core.Pt(28), CornerRadius: 7, Background: p.accent,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
			icon(lucideNote, 15, p.onAccent),
		}},
		core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 1}, Children: []core.Node{
			text(i18n.T("Local Files"), 13, core.FontWeightSemibold, p.text),
			core.Group(sub),
		}},
		icon(lucideChevronDown, 12, p.faint),
	}}
}

// searchField is the sidebar's search box. Return searches; the cross
// empties it.
func (a *app) searchField(p palette) core.Node {
	return core.View{Style: core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6,
		Padding: core.Symmetric(0, 8), CornerRadius: 8, Background: p.hover, Margin: core.Edges{Top: 4, Bottom: 8}},
		Children: []core.Node{
			icon(lucideSearch, 14, p.muted),
			core.Input{Ref: a.searchRef, TestID: "search-input", Value: a.query.Get, OnChange: a.query.Set, OnSubmit: a.search, Placeholder: i18n.T("Search"),
				Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), Height: core.Pt(28), Background: core.Color(0), Color: p.text,
					PlaceholderColor: p.faint, FontSize: 12.5, BorderWidth: 0, OutlineWidth: 0}},
			base.Button{Label: i18n.T("Clear"), OnPress: func(core.PressEvent) { a.closeSearch() },
				Style: core.Style{Width: core.Pt(16), Height: core.Pt(16), Direction: core.Row, AlignItems: core.AlignCenter,
					JustifyContent: core.JustifyCenter, CornerRadius: 8},
				Children: []core.Node{icon(lucideX, 11, p.faint)}},
		}}
}

// navItem is a page in the sidebar.
func (a *app) navItem(p palette, id string, svg *core.SVG, label string, selected bool, onPress func()) core.Node {
	fg := p.text
	if selected {
		fg = p.accent
	}
	b := base.Button{Label: label, TestID: id, OnPress: func(core.PressEvent) { onPress() },
		Selected: func() bool { return selected },
		StyleFunc: func() core.Style {
			s := core.Style{Height: core.Pt(34), Padding: core.Symmetric(0, 10), Gap: 10, Direction: core.Row,
				AlignItems: core.AlignCenter, CornerRadius: 8, Color: p.text, Hover: &core.Style{Background: p.hover}}
			if selected {
				s.Background = p.press
			}
			return s
		}, Children: []core.Node{icon(svg, 17, fg), text(label, 13.5, core.FontWeightMedium, p.text)}}
	return b
}

// playlistSection is My Playlists, above the listener's lists.
func (a *app) playlistSection(p palette, r route) core.Node {
	var rows []core.Node
	for i := range a.mine {
		pl := a.mine[i]
		selected := r.page == pagePlaylist && r.id == pl.ID
		rows = append(rows, a.playlistRow(p, pl, selected))
	}
	head := core.View{Style: core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Margin: core.Edges{Top: 14}, Padding: core.Edges{Left: 10, Right: 2}},
		Children: []core.Node{
			text(i18n.T("My Playlists"), 11, core.FontWeightSemibold, p.muted),
			core.View{Style: core.Style{FlexGrow: 1}},
			base.Button{Label: i18n.T("New Playlist"), TestID: "new-playlist", OnPress: func(core.PressEvent) { a.newPlaylist() },
				Style: core.Style{Width: core.Pt(24), Height: core.Pt(24), Direction: core.Row, AlignItems: core.AlignCenter,
					JustifyContent: core.JustifyCenter, CornerRadius: 6, Hover: &core.Style{Background: p.hover}},
				Children: []core.Node{icon(lucidePlus, 14, p.muted)}},
		}}
	return core.View{Style: core.Style{Direction: core.Column, Gap: 2}, Children: append([]core.Node{head}, rows...)}
}

func (a *app) playlistRow(p palette, pl music.Playlist, selected bool) core.Node {
	fg := p.muted
	if selected {
		fg = p.accent
	}
	row := base.Button{Label: pl.Name, TestID: "playlist-" + pl.ID, OnPress: func(core.PressEvent) { a.open(route{page: pagePlaylist, id: pl.ID}) },
		Selected: func() bool { return selected },
		StyleFunc: func() core.Style {
			s := core.Style{Height: core.Pt(32), Padding: core.Symmetric(0, 10), Gap: 10, Direction: core.Row,
				AlignItems: core.AlignCenter, CornerRadius: 8, Hover: &core.Style{Background: p.hover}}
			if selected {
				s.Background = p.press
			}
			return s
		}, Children: []core.Node{icon(lucideList, 15, fg), core.Text{Content: pl.Name, MaxLines: 1,
			Style: core.Style{Color: p.text, FontSize: 13, FontWeight: core.FontWeightNormal}}}}
	id := pl.ID
	name := pl.Name
	return base.ContextMenu{Trigger: row, Children: []core.Node{
		base.MenuItem{Label: i18n.T("Rename Playlist…"), OnSelect: func() { a.renamePlaylist(id, name) }},
		base.MenuItem{Label: i18n.T("Delete Playlist"), OnSelect: func() { a.confirmDelete(id, name) }},
	}}
}

// sidebarFooter is the settings and the folders.
func (a *app) sidebarFooter(p palette) core.Node {
	return core.View{Style: core.Style{Direction: core.Column, Gap: 2, Padding: core.Edges{Top: 8}, BorderWidth: 1, BorderColor: p.hairline,
		BorderColors: core.EdgeColors{Top: p.hairline}}, Children: []core.Node{
		a.footerRow(p, "add-folder", lucideFolder, i18n.T("Add Folder…"), func() { a.pickFolders() }),
		a.footerRow(p, "open-settings", lucideSettings, i18n.T("Settings"), func() { a.openSettings() }),
	}}
}

func (a *app) footerRow(p palette, id string, svg *core.SVG, label string, onPress func()) core.Node {
	return base.Button{Label: label, TestID: id, OnPress: func(core.PressEvent) { onPress() },
		StyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(32), Padding: core.Symmetric(0, 10), Gap: 10, Direction: core.Row,
				AlignItems: core.AlignCenter, CornerRadius: 8, Hover: &core.Style{Background: p.hover}}
		}, Children: []core.Node{icon(svg, 15, p.muted), text(label, 13, core.FontWeightNormal, p.muted)}}
}
