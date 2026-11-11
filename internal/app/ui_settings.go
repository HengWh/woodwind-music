package app

import (
	"path/filepath"
	"slices"
	"strings"

	"nsgo-music/internal/player"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"

	"nsgo-music/internal/i18n"
)

// settingsRowDef is a row of the settings: what it is, what it means, and
// the control that sets it.
type settingsRowDef struct {
	label, sub, keywords string
	control              func(p palette) core.Node
}

// settingsGroupDef is related rows under a title.
type settingsGroupDef struct {
	title string
	rows  []settingsRowDef
}

// settingsCatDef is a page of the settings, in the sidebar.
type settingsCatDef struct {
	id, label string
	icon      *core.SVG
	groups    []settingsGroupDef
}

// settingsView is the settings window: a sidebar of pages, a search through
// every row, and the rows of the page shown.
func (a *app) settingsView(cx *core.Scope) core.Node {
	desktop.SetTitleBar(cx, desktop.TitleBarHidden, 40)
	desktop.SetWindowControls(cx, core.WindowControls{TrafficLights: core.Point{X: 20, Y: 18}})
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		p := a.palette(cx)
		cats := a.settingsCategories(p)
		return a.settingsLayout(cx, p, cats)
	})
}

func (a *app) settingsLayout(cx *core.Scope, p palette, cats []settingsCatDef) core.Node {
	top := desktop.TitleBar(cx).Top
	if top <= 0 {
		top = 40
	}
	if a.settingsPage == "" {
		a.settingsPage = cats[0].id
	}
	current := cats[0]
	for _, c := range cats {
		if c.id == a.settingsPage {
			current = c
		}
	}
	query := strings.ToLower(strings.TrimSpace(a.settingsQuery))

	side := core.View{Style: core.Style{Width: core.Pt(200), FlexShrink: 0, Background: p.sidebar, Direction: core.Column,
		Padding: core.Edges{Top: top + 4, Right: 12, Bottom: 14, Left: 12}, Gap: 2},
		Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, Children: []core.Node{
			a.settingsSearchField(p),
			core.View{Style: core.Style{Direction: core.Column, Gap: 2, Margin: core.Edges{Top: 8}}, Children: func() []core.Node {
				var rows []core.Node
				for _, c := range cats {
					rows = append(rows, a.settingsNavItem(p, c, query == "" && c.id == current.id))
				}
				return rows
			}()},
		}}

	var page []core.Node
	switch {
	case query != "":
		page = append(page, a.settingsResults(p, cats, query)...)
	default:
		for _, g := range current.groups {
			if len(g.rows) > 0 {
				page = append(page, a.settingsGroupCard(p, g))
			}
		}
	}
	if len(page) == 0 {
		page = append(page, core.View{Style: core.Style{Padding: core.All(24)}, Children: []core.Node{
			text(i18n.T("No settings found"), 13, core.FontWeightNormal, p.muted)}})
	}
	title := current.label
	if query != "" {
		title = i18n.T("Search Results")
	}
	main := core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), Direction: core.Column, Background: p.bg}, Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(56), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter,
			Padding: core.Edges{Left: 28, Right: 28}, BorderWidth: 1, BorderColor: p.hairline,
			BorderColors: core.EdgeColors{Bottom: p.hairline}}, Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn},
			Children: []core.Node{
				text(title, 15, core.FontWeightBold, p.text),
			}},
		core.Scroll{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Column, Padding: core.All(28), Gap: 22, MaxWidth: core.Pt(720)}, Children: page},
		}},
	}}
	return core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100), Direction: core.Row,
		Background: p.bg, Color: p.text}, Children: []core.Node{side, main}}
}

// settingsSearchField finds rows across every page.
func (a *app) settingsSearchField(p palette) core.Node {
	return core.View{Style: core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6,
		Padding: core.Symmetric(0, 8), CornerRadius: 8, Background: p.hover, BorderWidth: 1, BorderColor: p.hairline},
		Children: []core.Node{
			icon(lucide.Search(), 14, p.muted),
			core.Input{TestID: "settings-search", Value: func() string { return a.settingsQuery }, OnChange: func(v string) {
				a.settingsQuery = v
				a.bump()
			}, Placeholder: i18n.T("Search"), Label: i18n.T("Search Settings"), Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0),
				Height: core.Pt(28), Background: core.Color(0), Color: p.text, PlaceholderColor: p.faint, FontSize: 12.5, BorderWidth: 0, OutlineWidth: 0}},
		}}
}

func (a *app) settingsNavItem(p palette, c settingsCatDef, selected bool) core.Node {
	bg := core.Color(0)
	fg := p.muted
	if selected {
		bg, fg = p.press, p.accent
	}
	return base.Button{Label: c.label, TestID: "settings-nav-" + c.id, OnPress: func(core.PressEvent) {
		a.settingsPage, a.settingsQuery = c.id, ""
		a.bump()
	}, Selected: func() bool { return selected }, Style: core.Style{Height: core.Pt(32), Padding: core.Symmetric(0, 9), Gap: 9,
		Direction: core.Row, AlignItems: core.AlignCenter, CornerRadius: 8, Background: bg, Hover: &core.Style{Background: p.hover}},
		Children: []core.Node{icon(c.icon, 15, fg), text(c.label, 13, core.FontWeightMedium, p.text)}}
}

// settingsResults shows the rows a search finds, under their page's name.
func (a *app) settingsResults(p palette, cats []settingsCatDef, query string) []core.Node {
	var out []core.Node
	for _, c := range cats {
		var found []settingsGroupDef
		for _, g := range c.groups {
			var rows []settingsRowDef
			for _, r := range g.rows {
				if settingsMatch(query, c.label, g.title, r.label, r.sub, r.keywords) {
					rows = append(rows, r)
				}
			}
			if len(rows) > 0 {
				found = append(found, settingsGroupDef{title: g.title, rows: rows})
			}
		}
		if len(found) == 0 {
			continue
		}
		out = append(out, text(c.label, 13, core.FontWeightBold, p.text))
		for _, g := range found {
			out = append(out, a.settingsGroupCard(p, g))
		}
	}
	return out
}

func settingsMatch(query string, texts ...string) bool {
	for _, t := range texts {
		if strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}
	return false
}

// settingsGroupCard draws rows under a title, in the app's card.
func (a *app) settingsGroupCard(p palette, g settingsGroupDef) core.Node {
	var children []core.Node
	if g.title != "" {
		children = append(children, core.View{Style: core.Style{Padding: core.Edges{Bottom: 8}}, Children: []core.Node{
			text(i18n.T(g.title), 12, core.FontWeightSemibold, p.muted)}})
	}
	var rows []core.Node
	for i, r := range g.rows {
		var control core.Node
		if r.control != nil {
			control = r.control(p)
		}
		row := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16, Padding: core.Symmetric(11, 12),
			Hover: &core.Style{Background: p.hover}}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 3}, Children: []core.Node{
				text(r.label, 13.5, core.FontWeightMedium, p.text),
				core.Text{Content: r.sub, Style: core.Style{FontSize: 12, Color: p.muted, LineHeightMultiple: 1.4}},
			}},
			control,
		}}
		if i < len(g.rows)-1 {
			row.Style.BorderWidth = 1
			row.Style.BorderColor = p.hairline
			row.Style.BorderColors = core.EdgeColors{Bottom: p.hairline}
		}
		rows = append(rows, row)
	}
	children = append(children, core.View{Style: core.Style{Direction: core.Column, Background: p.sidebar, CornerRadius: 12, Padding: core.All(4),
		BorderWidth: 1, BorderColor: p.hairline}, Children: rows})
	return core.View{Style: core.Style{Direction: core.Column, Gap: 0}, Children: children}
}

// settingsCategories are the pages of the settings.
func (a *app) settingsCategories(p palette) []settingsCatDef {
	general := settingsCatDef{id: "general", label: i18n.T("General"), icon: lucide.Settings(),
		groups: []settingsGroupDef{{rows: []settingsRowDef{
			{label: i18n.T("Language"), control: a.languageControl},
			{label: i18n.T("Appearance"), control: a.appearanceControl},
		}}}}

	playback := settingsCatDef{id: "playback", label: i18n.T("Playback"), icon: lucide.Volume2(),
		groups: []settingsGroupDef{{rows: []settingsRowDef{
			{label: i18n.T("Normalize volume"), sub: i18n.T("Adjust each song's volume so they sound as loud as each other"),
				control: a.normalizeControl},
		}}}}

	folders := a.folderRows()
	library := settingsCatDef{id: "library", label: i18n.T("Library"), icon: lucide.Folder(),
		groups: []settingsGroupDef{{rows: folders}}}

	cacheRows := []settingsRowDef{{label: i18n.T("Keep songs on disk"), sub: i18n.T("Songs played again open at once, and play offline"),
		control: a.cacheControl}}
	if a.pl != nil && a.pl.Cache != nil {
		cacheRows = append(cacheRows, settingsRowDef{label: i18n.T("Clear the Cache"), sub: i18n.T("Delete all the songs kept here"),
			control: a.clearCacheControl})
	}
	cache := settingsCatDef{id: "cache", label: i18n.T("Cache"), icon: lucide.Database(), groups: []settingsGroupDef{{rows: cacheRows}}}

	experiments := settingsCatDef{id: "experiments", label: i18n.T("Experiments"), icon: lucide.FlaskConical(),
		groups: []settingsGroupDef{{rows: []settingsRowDef{
			{label: i18n.T("Smoke on the Progress Bar"), sub: i18n.T("The progress bar turns into a light trailing smoke, moving with the music. Uses a little more power."),
				control: a.smokeControl},
			{label: i18n.T("Grow From the Cover"), sub: i18n.T("Now Playing opens out of the cover in the player bar and shrinks back into it, rather than sliding up from the bottom"),
				control: a.growFromCoverControl},
		}}}}

	return []settingsCatDef{general, playback, library, cache, experiments}
}

func (a *app) languageControl(p palette) core.Node {
	options := []ui.Option[string]{{Value: i18n.System, Label: i18n.T("Follow System")}}
	for _, l := range i18n.Languages {
		options = append(options, ui.Option[string]{Value: l.Code, Label: l.Name})
	}
	return ui.Select[string]{Options: options, Value: func() string {
		if a.set.Language == "" {
			return i18n.System
		}
		return a.set.Language
	}, OnChange: func(v string) {
		a.set.Language = v
		a.set.save()
		a.applyLanguage()
		if v == i18n.System {
			a.cx.SetLocale("")
		} else {
			a.cx.SetLocale(v)
		}
		a.menu(a.cx)
		a.bump()
	}, Label: i18n.T("Language"), Style: core.Style{Width: core.Pt(170)}}
}

func (a *app) appearanceControl(p palette) core.Node {
	var buttons []core.Node
	for _, o := range []struct{ value, label string }{
		{"system", i18n.T("System")}, {"light", i18n.T("Light")}, {"dark", i18n.T("Dark")},
	} {
		value := o.value
		selected := a.set.Appearance == value
		bg := core.Color(0)
		fg := p.muted
		if selected {
			bg, fg = p.hover, p.text
		}
		buttons = append(buttons, base.Button{Label: o.label, OnPress: func(core.PressEvent) {
			a.set.Appearance = value
			a.set.save()
			switch value {
			case "light":
				a.cx.SetAppearance(core.AppearanceLight)
			case "dark":
				a.cx.SetAppearance(core.AppearanceDark)
			default:
				a.cx.SetAppearance(core.AppearanceSystem)
			}
			a.bump()
		}, Style: core.Style{Padding: core.Symmetric(5, 12), CornerRadius: 7, Background: bg,
			Hover: &core.Style{Background: p.hover}}, Children: []core.Node{text(o.label, 12.5, core.FontWeightMedium, fg)}})
	}
	return core.View{Style: core.Style{Direction: core.Row, Gap: 2, Background: p.hover, Padding: core.All(2), CornerRadius: 8}, Children: buttons}
}

func (a *app) normalizeControl(p palette) core.Node {
	return ui.Switch{Checked: func() bool { return a.set.Normalize }, OnChange: func(on bool) {
		a.set.Normalize = on
		a.set.save()
		a.bump()
	}, Label: i18n.T("Normalize volume")}
}

func (a *app) smokeControl(p palette) core.Node {
	return ui.Switch{Checked: func() bool { return a.set.Smoke }, OnChange: func(on bool) {
		a.set.Smoke = on
		a.set.save()
		a.bump()
	}, Label: i18n.T("Smoke on the Progress Bar")}
}

func (a *app) growFromCoverControl(p palette) core.Node {
	return ui.Switch{Checked: func() bool { return a.set.GrowFromCover }, OnChange: func(on bool) {
		a.set.GrowFromCover = on
		a.set.save()
		a.bump()
	}, Label: i18n.T("Grow From the Cover")}
}

func (a *app) folderRows() []settingsRowDef {
	var rows []settingsRowDef
	for _, f := range a.local.Folders() {
		path := f
		rows = append(rows, settingsRowDef{label: path, control: func(p palette) core.Node {
			return base.Button{Label: i18n.T("Remove"), OnPress: func(core.PressEvent) { a.removeFolder(path); a.bump() },
				Style: core.Style{Padding: core.Symmetric(4, 10), CornerRadius: 7, Background: p.hover,
					Hover: &core.Style{Background: p.press}}, Children: []core.Node{text(i18n.T("Remove"), 12, core.FontWeightMedium, p.muted)}}
		}})
	}
	if len(rows) == 0 {
		rows = append(rows, settingsRowDef{label: i18n.T("No folders yet"), sub: i18n.T("Add the folders where your music lives")})
	}
	rows = append(rows, settingsRowDef{label: i18n.T("Add Folder…"), control: func(p palette) core.Node {
		return base.Button{Label: i18n.T("Add Folder…"), OnPress: func(core.PressEvent) { a.pickFolders(); a.bump() },
			Style: core.Style{Padding: core.Symmetric(4, 10), CornerRadius: 7, Background: p.hover,
				Hover: &core.Style{Background: p.press}}, Children: []core.Node{text(i18n.T("Add"), 12, core.FontWeightMedium, p.text)}}
	}})
	return rows
}

func (a *app) cacheControl(p palette) core.Node {
	options := []ui.Option[int]{{Value: 0, Label: i18n.T("Off")}}
	values := []int{0, 512, 1024, 2048, 4096, 10240}
	for _, v := range values[1:] {
		options = append(options, ui.Option[int]{Value: v, Label: cacheLabel(v)})
	}
	if !slices.Contains(values, a.set.CacheMB) {
		options = append(options, ui.Option[int]{Value: a.set.CacheMB, Label: cacheLabel(a.set.CacheMB)})
	}
	return ui.Select[int]{Options: options, Value: func() int { return a.set.CacheMB }, OnChange: func(v int) {
		a.set.CacheMB = v
		a.set.save()
		if v > 0 && a.pl.Cache == nil && a.dir != "" {
			a.pl.Cache = player.NewCache(filepath.Join(a.dir, "songs"), int64(v)<<20)
		} else if a.pl.Cache != nil {
			a.pl.Cache.SetLimit(int64(v) << 20)
		}
		a.bump()
	}, Label: i18n.T("Cache"), Style: core.Style{Width: core.Pt(140)}}
}

func (a *app) clearCacheControl(p palette) core.Node {
	return base.Button{Label: i18n.T("Clear"), OnPress: func(core.PressEvent) {
		if a.pl != nil && a.pl.Cache != nil {
			a.pl.Cache.Clear()
		}
		a.bump()
	}, Style: core.Style{Padding: core.Symmetric(5, 12), CornerRadius: 7, Background: p.hover,
		Hover: &core.Style{Background: p.press}}, Children: []core.Node{text(i18n.T("Clear"), 12, core.FontWeightMedium, p.text)}}
}

func cacheLabel(mb int) string {
	switch {
	case mb >= 1024:
		return i18n.Tf("%.1f GB", float64(mb)/1024)
	case mb > 0:
		return i18n.Tf("%d MB", mb)
	}
	return i18n.T("Off")
}
