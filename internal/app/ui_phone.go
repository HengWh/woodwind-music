package app

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HengWh/woodwind-music/internal/i18n"
)

func (a *app) phoneLayout(cx *core.Scope) bool {
	return a.video != nil && cx.Env().Window().Width < 720
}

func (a *app) phoneNavigation(cx *core.Scope, p palette) core.Node {
	menu := core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		a.route.Get()
		items := []core.Node{
			ui.MenuItem{Text: i18n.T("Home"), OnSelect: func() { a.open(route{page: pageHome}) }},
			ui.MenuItem{Text: i18n.T("Songs"), OnSelect: func() { a.open(route{page: pageBrowse, id: "songs"}) }},
			ui.MenuItem{Text: i18n.T("Albums"), OnSelect: func() { a.open(route{page: pageBrowse, id: "albums"}) }},
			ui.MenuItem{Text: i18n.T("Artists"), OnSelect: func() { a.open(route{page: pageBrowse, id: "artists"}) }},
			ui.MenuItem{Text: i18n.T("Liked Songs"), OnSelect: func() { a.open(route{page: pageLiked}) }},
		}
		for _, list := range a.mine {
			id := list.ID
			items = append(items, ui.MenuItem{Text: list.Name, OnSelect: func() { a.open(route{page: pagePlaylist, id: id}) }})
		}
		items = append(items, ui.MenuSeparator{},
			ui.MenuItem{Text: i18n.T("Up Next"), OnSelect: func() { a.openNowPlaying(); a.npPane.Set(1) }},
			ui.MenuItem{Text: i18n.T("Settings…"), OnSelect: a.openSettings})
		return ui.Menu{Trigger: ui.Button{Label: i18n.T("Local Files"), Variant: ui.ButtonGhost, Side: 36, TestID: "phone-navigation"},
			Face: icon(lucideList, 22, p.text), Children: items}
	})
	return core.View{TestID: "phone-header", Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter,
		FlexShrink: 0, Padding: core.Symmetric(8, 12), Gap: 8}, Children: []core.Node{
		menu,
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: []core.Node{a.searchField(p)}},
		base.Button{Label: i18n.T("Add Folder…"), TestID: "phone-add-folder", OnPress: func(core.PressEvent) { a.pickFolders() },
			Style:    core.Style{Width: core.Pt(36), Height: core.Pt(36), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{icon(lucideFolder, 22, p.accent)}},
	}}
}

func (a *app) phonePlayerBar(cx *core.Scope, p palette) core.Node {
	return core.View{TestID: "player-bar", Style: core.Style{FlexShrink: 0, Direction: core.Column,
		Padding: core.Symmetric(8, 16), Gap: 6, Background: p.bg, Borders: core.Edges{Top: 1}, BorderColor: p.hairline}, Children: []core.Node{
		core.Dynamic(func(cx *core.Scope) core.Node { a.playRev.Get(); return a.barTrack(p) }),
		core.Text{TestID: "playback-status", ContentFunc: func() string {
			a.rev.Get()
			a.tick.Get()
			a.playRev.Get()
			if err := a.pl.Err(); err != "" {
				return err
			}
			if a.importing {
				return i18n.T("Importing music…")
			}
			if a.pl.Loading() {
				return i18n.T("Buffering…")
			}
			return ""
		}, MaxLines: 2, Style: core.Style{FontSize: 11, Color: p.muted}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			core.Text{ContentFunc: func() string { a.tick.Get(); return clock(a.pl.Position()) }, Style: core.Style{FontSize: 11, Color: p.muted}},
			core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0)}, Children: []core.Node{a.progress(p, false)}},
			core.Text{ContentFunc: func() string { a.tick.Get(); return clock(a.pl.Duration()) }, Style: core.Style{FontSize: 11, Color: p.muted}},
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, Gap: 8}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node {
				a.playRev.Get()
				return core.Group(a.transport(p, p.muted, p.accent, 18))
			}),
			a.volumeSlider(p),
		}},
	}}
}
