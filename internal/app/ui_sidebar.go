package app

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

const sidebarWidth = 232

// sidebar is the app's navigation: the source, a search field, the app's
// pages, the listener's playlists, and the settings at the foot.
func (a *app) sidebar(cx *core.Scope, top core.Points) core.Node {
	// Keep the search input mounted across navigation and library updates.
	// Rebuild the shell only when its appearance or language changes.
	cx.Env().Locale()
	p := a.palette(cx)
	left := desktop.TitleBar(cx).Left
	paddingTop := core.Points(14)
	if left > 0 {
		paddingTop = top + 4
	}
	return core.View{Style: core.Style{Width: core.Pt(sidebarWidth), Height: core.Pct(100), Background: p.sidebar,
		Direction: core.Column, Padding: core.Edges{Top: paddingTop, Right: 14, Bottom: 14, Left: 14}, Gap: 4},
		Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node { a.rev.Get(); return a.sourceRow(p, top, left) }),
			a.searchField(p),
			core.Scroll{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
				core.Dynamic(func(cx *core.Scope) core.Node {
					a.rev.Get()
					r := a.route.Get()
					return core.View{Style: core.Style{Direction: core.Column, Gap: 2, Padding: core.Symmetric(2, 0)}, Children: []core.Node{
						core.Group{
							core.Group(a.sourceNavigation(p, r, false)),
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

// sourceRow follows MyGo's compact source selector: beside the traffic lights
// on macOS, or a line above search elsewhere, only when plugins are installed.
func (a *app) sourceRow(p palette, top, left core.Points) core.Node {
	if len(a.plugins) == 0 {
		return core.Group{}
	}
	style := core.Style{Height: core.Pt(24), Padding: core.Symmetric(0, 7), Gap: 4,
		Direction: core.Row, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart,
		CornerRadius: 7, Color: p.muted, MaxWidth: core.Pt(sidebarWidth - 28),
		Margin:   core.Edges{Bottom: 4, Left: -7},
		Hover:    &core.Style{Background: p.hover, Color: p.text},
		Expanded: &core.Style{Background: p.hover, Color: p.text}}
	if left > 0 {
		style.Position = core.Absolute
		style.Top = core.Pt((top - 24) / 2)
		style.Left = core.Pt(left + 12)
		style.MaxWidth = core.Pt(sidebarWidth - left - 22)
		style.Margin = core.Edges{}
	}
	row := core.Group{
		core.Text{Content: a.sourceName(), MaxLines: 1, Style: core.Style{
			FontSize: 12.5, FontWeight: core.FontWeightSemibold, FlexShrink: 1, MinWidth: core.Pt(0)}},
		icon(lucide.ChevronsUpDown(), 11, p.faint),
	}
	var items []core.Node
	items = append(items, sourceMenuOption(p, "local", i18n.T("Local Files"),
		func() bool { return a.source() == a.local }, func() { a.switchSource("local") }))
	for _, plug := range a.plugins {
		id := plug.Info().ID
		items = append(items, sourceMenuOption(p, id, plug.Info().Name,
			func() bool { return a.source().Info().ID == id }, func() { a.switchSource(id) }))
	}
	fill, edge := core.RGB(0xf6f6f6), core.RGBA(0x000000, 0.12)
	if p.dark {
		fill, edge = core.RGB(0x2b2b2e), core.RGBA(0xffffff, 0.14)
	}
	return base.Menu{TestID: "source-switcher", Label: a.sourceName(), Trigger: row, Style: style,
		Placement: core.PlaceBottom, Align: core.AlignStart, Offset: 6, Margin: 8,
		PopupStyle: core.Style{MinWidth: core.Pt(180), MaxWidth: core.Pt(sidebarWidth + 60),
			Direction: core.Column, Background: fill, Color: p.text, Padding: core.All(5),
			BorderWidth: 0.5, BorderColor: edge, CornerRadius: 12,
			Shadows: []core.BoxShadow{{Offset: core.Point{Y: 8}, Blur: 24, Spread: -4, Color: core.RGBA(0x000000, 0.2)}}},
		Children: []core.Node{core.Scroll{TestID: "source-menu-options", Style: core.Style{MaxHeight: core.Pt(360)},
			Children: []core.Node{core.View{Style: core.Style{Direction: core.Column, Gap: 1}, Children: items}}}},
	}
}

func sourceMenuOption(p palette, id, label string, checked func() bool, selectSource func()) core.Node {
	return base.MenuItem{TestID: "source-option-" + id, Label: label, Role: core.RoleMenuItemRadio,
		Checked: checked, OnSelect: selectSource,
		Style: core.Style{Height: core.Pt(30), Direction: core.Row, AlignItems: core.AlignCenter,
			Padding: core.Symmetric(0, 8), Gap: 8, CornerRadius: 7,
			Hover: &core.Style{Background: p.press}, Focus: &core.Style{Background: p.press}},
		Indicator: icon(lucideCheck, 14, p.text),
		IndicatorStyle: core.Style{Width: core.Pt(14), Height: core.Pt(14), FlexShrink: 0, Opacity: core.Alpha(0),
			Checked: &core.Style{Opacity: core.Alpha(1)}},
		Children: []core.Node{core.Text{Content: label, MaxLines: 1,
			Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: 13, Color: p.text}}},
	}
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

// playlistSection keeps created and saved playlists in their own disclosures.
func (a *app) playlistSection(p palette, r route) core.Node {
	if !a.can("playlists") {
		return core.Group{}
	}
	var mine, saved []music.Playlist
	for _, pl := range a.mine {
		if pl.Mine {
			mine = append(mine, pl)
		} else {
			saved = append(saved, pl)
		}
	}
	return core.Group{
		a.playlistDisclosure(p, r, i18n.T("My Playlists"), "my-playlists", mine, true, &a.mineCollapsed),
		a.playlistDisclosure(p, r, i18n.T("Saved Playlists"), "saved-playlists", saved, false, &a.savedCollapsed),
	}
}

func (a *app) playlistDisclosure(p palette, r route, title, id string, lists []music.Playlist, mine bool, collapsed *bool) core.Node {
	canCreate := mine && a.can("createPlaylist")
	if len(lists) == 0 && !canCreate {
		return core.Group{}
	}
	arrow := lucideChevronDown
	if *collapsed {
		arrow = lucideChevronRight
	}
	heading := []core.Node{base.Button{TestID: id + "-toggle", Label: title, Expanded: func() bool { return !*collapsed },
		OnPress: func(core.PressEvent) { *collapsed = !*collapsed; a.bump() },
		Style: core.Style{FlexGrow: 1, Height: core.Pt(26), Padding: core.Symmetric(0, 4), Gap: 6, CornerRadius: 6,
			Direction: core.Row, AlignItems: core.AlignCenter, Hover: &core.Style{Background: p.hover}},
		Children: []core.Node{core.Text{Content: title, Style: core.Style{FlexGrow: 1, FontSize: 11.5, FontWeight: core.FontWeightSemibold, Color: p.faint}}, icon(arrow, 12, p.faint)}}}
	if canCreate {
		heading = append(heading, base.Button{Label: i18n.T("New Playlist"), TestID: "new-playlist", OnPress: func(core.PressEvent) { a.newPlaylist() },
			Style: core.Style{Width: core.Pt(26), Height: core.Pt(26), Direction: core.Row, AlignItems: core.AlignCenter,
				JustifyContent: core.JustifyCenter, CornerRadius: 6, Hover: &core.Style{Background: p.hover}},
			Children: []core.Node{icon(lucidePlus, 13, p.faint)}})
	}
	rows := []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter,
		Margin: core.Edges{Top: 18, Right: 4, Bottom: 6, Left: 6}}, Children: heading}}
	if !*collapsed {
		for _, pl := range lists {
			selected := r.page == pagePlaylist && r.id == pl.ID && !a.nowPlaying
			rows = append(rows, a.playlistRow(p, pl, selected))
		}
	}
	return core.View{Style: core.Style{Direction: core.Column}, Children: rows}
}

func (a *app) playlistRow(p palette, pl music.Playlist, selected bool) core.Node {
	row := base.Button{Label: pl.Name, TestID: "playlist-" + pl.ID, OnPress: func(core.PressEvent) { a.open(route{page: pagePlaylist, id: pl.ID}) },
		Selected: func() bool { return selected },
		StyleFunc: func() core.Style {
			s := core.Style{Height: core.Pt(34), Padding: core.Symmetric(0, 8), Gap: 10, Direction: core.Row,
				AlignItems: core.AlignCenter, CornerRadius: 8, Hover: &core.Style{Background: p.hover}}
			if selected {
				s.Background = p.press
			}
			return s
		}, Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(22), Height: core.Pt(22), FlexShrink: 0},
			Children: []core.Node{a.cover(p, pl.ImageURL, 22, 5)}}, core.Text{Content: pl.Name, MaxLines: 1,
			Style: core.Style{FlexShrink: 1, MinWidth: core.Pt(0), Color: p.text, FontSize: 13}}}}
	items := a.playlistMenu(pl)
	if !pl.Mine && a.can("savePlaylist") {
		items = []core.Node{ui.MenuItem{Text: i18n.T("Remove from Saved"), OnSelect: func() { a.setPlaylistSaved(pl.ID, false) }}}
	}
	if len(items) == 0 {
		return row
	}
	return ui.ContextMenu{Trigger: row, Children: items}
}

// sidebarFooter is the settings and the folders.
func (a *app) sidebarFooter(p palette) core.Node {
	return core.View{Style: core.Style{Direction: core.Column, Gap: 2, Padding: core.Edges{Top: 8}}, Children: []core.Node{
		core.Dynamic(func(cx *core.Scope) core.Node { a.rev.Get(); return a.accountRow(p) }),
	}}
}

// footerRow is the compact folder action used by an empty library page.
func (a *app) footerRow(p palette, id string, svg *core.SVG, label string, onPress func()) core.Node {
	return base.Button{Label: label, TestID: id, OnPress: func(core.PressEvent) { onPress() },
		Style: core.Style{Height: core.Pt(a.headingSize(32, 48)), Padding: core.Symmetric(0, 10), Gap: 10, Direction: core.Row,
			AlignItems: core.AlignCenter, CornerRadius: 8, Hover: &core.Style{Background: p.hover}},
		Children: []core.Node{icon(svg, 15, p.muted), text(label, 13, core.FontWeightNormal, p.muted)}}
}
