package app

import (
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"

	"nsgo-music/internal/i18n"
	"nsgo-music/internal/player"
)

// settingsRowDef is a row of the settings: what it is, what it means, and
// the control that sets it.
type settingsRowDef struct {
	label, sub string
	// icon leads the row, for a folder.
	icon *core.SVG
	// small sets the row's name in the smaller type, as a path's.
	small   bool
	control func(p palette) core.Node
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

// qualityLevels are what can be asked of sources, by what they sound like.
var qualityLevels = []struct{ level, label string }{
	{"standard", i18n.N("Standard")},
	{"high", i18n.N("High")},
	{"lossless", i18n.N("Lossless")},
}

// cacheSizes are the sizes the song cache may take, in megabytes.
var cacheSizes = []int{0, 1024, 2048, 5120, 10240}

// settingsView is the settings window: a sidebar of pages, a search through
// every row, and the rows of the page shown.
func (a *app) settingsView(cx *core.Scope) core.Node {
	desktop.SetTitleBar(cx, desktop.TitleBarHidden, 40)
	desktop.SetWindowControls(cx, core.WindowControls{TrafficLights: core.Point{X: 20, Y: 18}})
	if a.cacheUsed < 0 {
		a.measureCache()
	}
	win := desktop.CurrentWindow(cx)
	cx.OnKey("mod+w", func() {
		if win != nil && !win.Closed() {
			win.Close()
		}
	})
	cx.OnKey("mod+f", func() { a.settingsRef.Focus() })
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		p := a.palette(cx)
		return a.settingsLayout(cx, p, a.settingsCategories(p))
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
	side := core.View{Style: core.Style{Width: core.Pt(200), FlexShrink: 0, Background: p.sidebar, Direction: core.Column,
		Padding: core.Edges{Top: top + 4, Right: 12, Bottom: 14, Left: 12}, Gap: 2},
		Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, Children: []core.Node{
			a.settingsSearchField(p),
			core.Dynamic(func(cx *core.Scope) core.Node {
				a.settingsQueryRev.Get()
				p := a.palette(cx)
				query := strings.ToLower(strings.TrimSpace(a.settingsQuery))
				var rows []core.Node
				for _, c := range cats {
					rows = append(rows, a.settingsNavItem(p, c, query == "" && c.id == current.id))
				}
				return core.View{Style: core.Style{Direction: core.Column, Gap: 2}, Children: rows}
			}),
		}}

	main := core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), Direction: core.Column, Background: p.bg},
		Children: []core.Node{core.Dynamic(func(cx *core.Scope) core.Node {
			a.settingsQueryRev.Get()
			p := a.palette(cx)
			query := strings.ToLower(strings.TrimSpace(a.settingsQuery))
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
				page = append(page, core.View{Style: core.Style{Padding: core.Edges{Top: 4, Right: 4, Bottom: 4, Left: 4}}, Children: []core.Node{
					text(i18n.T("No settings found"), 13.5, core.FontWeightNormal, p.muted)}})
			}
			title := current.label
			if query != "" {
				title = i18n.T("Search Results")
			}
			return core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Direction: core.Column}, Children: []core.Node{
				core.View{Style: core.Style{Height: core.Pt(top), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter,
					Padding: core.Edges{Left: 28, Right: 28}, BorderWidth: 1, BorderColors: core.EdgeColors{Bottom: p.hairline}},
					Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, Children: []core.Node{
						text(title, 15, core.FontWeightBold, p.text),
					}},
				core.Scroll{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
					core.View{Style: core.Style{Direction: core.Column, Padding: core.Edges{Top: 28, Right: 28, Bottom: 32, Left: 28}, Gap: 8},
						Children: page},
				}},
			}}
		})}}
	divider := core.View{Style: core.Style{Width: core.Pt(1), FlexShrink: 0, Background: p.hairline}}
	return core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100), Direction: core.Row,
		Background: p.bg, Color: p.text}, Children: []core.Node{side, divider, main}}
}

// settingsSearchField finds rows across every page.
func (a *app) settingsSearchField(p palette) core.Node {
	return core.View{Style: core.Style{Height: core.Pt(30), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6,
		Padding: core.Symmetric(0, 8), CornerRadius: 8, Background: p.hover, BorderWidth: 1, BorderColor: p.hairline,
		Margin: core.Edges{Bottom: 10}}, Children: []core.Node{
		icon(lucideSearch, 13, p.muted),
		core.Input{Ref: a.settingsRef, TestID: "settings-search", Value: func() string { return a.settingsQuery },
			OnChange: func(v string) {
				a.settingsQuery = v
				// Only the pages follow the query: the field keeps the
				// focus while it types.
				a.settingsQueryRev.Update(func(n int) int { return n + 1 })
			},
			Placeholder: i18n.T("Search"), Label: i18n.T("Search Settings"), Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0),
				Height: core.Pt(26), Background: core.Color(0), Color: p.text, PlaceholderColor: p.faint, FontSize: 13,
				BorderWidth: 0, OutlineWidth: 0}},
	}}
}

func (a *app) settingsNavItem(p palette, c settingsCatDef, selected bool) core.Node {
	bg := core.Color(0)
	fg := p.muted
	style := core.Style{Height: core.Pt(32), Padding: core.Symmetric(0, 9), Gap: 9, Direction: core.Row,
		AlignItems: core.AlignCenter, CornerRadius: 8}
	if selected {
		bg, fg = p.press, p.accent
		style.Background = bg
	} else {
		style.Hover = &core.Style{Background: p.hover}
	}
	return base.Button{Label: c.label, TestID: "settings-nav-" + c.id, OnPress: func(core.PressEvent) {
		a.settingsPage, a.settingsQuery = c.id, ""
		a.bump()
	}, Selected: func() bool { return selected }, Style: style,
		Children: []core.Node{icon(c.icon, 15, fg), text(c.label, 13, core.FontWeightMedium, p.text)}}
}

// settingsResults shows the rows a search finds, under their page's name.
func (a *app) settingsResults(p palette, cats []settingsCatDef, query string) []core.Node {
	var out []core.Node
	for _, c := range cats {
		all := settingsMatch(query, c.label)
		for _, g := range c.groups {
			var rows []settingsRowDef
			for _, r := range g.rows {
				if all || settingsMatch(query, r.label, r.sub) {
					rows = append(rows, r)
				}
			}
			if len(rows) == 0 {
				continue
			}
			out = append(out, a.settingsGroupCard(p, settingsGroupDef{title: c.label, rows: rows}))
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
		children = append(children, core.View{Style: core.Style{Margin: core.Edges{Top: 14, Right: 4, Bottom: 4, Left: 4}},
			Children: []core.Node{text(g.title, 12, core.FontWeightSemibold, p.muted)}})
	}
	var rows []core.Node
	for i, r := range g.rows {
		var children []core.Node
		if r.control != nil {
			children = append(children, r.control(p))
		}
		info := r.info(p)
		if r.icon != nil {
			info = core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Row,
				AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{icon(r.icon, 15, p.muted), info}}
		}
		// The control follows what the row says, at the card's end.
		children = append([]core.Node{info}, children...)
		row := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16,
			MinHeight: core.Pt(48), Padding: core.Symmetric(12, 16)}, Children: children}
		if i < len(g.rows)-1 {
			row.Style.BorderWidth = 1
			row.Style.BorderColor = p.hairline
			row.Style.BorderColors = core.EdgeColors{Bottom: p.hairline}
		}
		rows = append(rows, row)
	}
	children = append(children, core.View{Style: core.Style{Direction: core.Column, Background: p.bg, CornerRadius: 12,
		BorderWidth: 1, BorderColor: p.hairline}, Children: rows})
	return core.View{Style: core.Style{Direction: core.Column}, Children: children}
}

// info is what a row says: its name, and what it means.
func (r settingsRowDef) info(p palette) core.Node {
	label := text(r.label, 13.5, core.FontWeightMedium, p.text)
	label.MaxLines = 1
	if r.small {
		label = text(r.label, 12.5, core.FontWeightNormal, p.text)
		label.MaxLines = 1
	}
	children := []core.Node{label}
	if r.sub != "" {
		children = append(children, core.Text{Content: r.sub, Style: core.Style{FontSize: 12, Color: p.muted, LineHeightMultiple: 1.4}})
	}
	return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 3},
		Children: children}
}

// settingsCategories are the pages of the settings.
func (a *app) settingsCategories(p palette) []settingsCatDef {
	general := settingsCatDef{id: "general", label: i18n.T("General"), icon: lucide.Settings(),
		groups: []settingsGroupDef{{rows: a.generalRows()}}}

	playback := settingsCatDef{id: "playback", label: i18n.T("Playback"), icon: lucide.Volume2(),
		groups: []settingsGroupDef{{rows: []settingsRowDef{
			{label: i18n.T("Quality"), sub: i18n.T("What's asked of plugins that have a choice, from the next song on"),
				control: a.qualityControl},
			{label: i18n.T("Even Out Loudness"), sub: i18n.T("Adjust each song's volume by what its plugin measured of it, for them to sound as loud as each other"),
				control: a.normalizeControl},
		}}}}

	library := settingsCatDef{id: "library", label: i18n.T("Library"), icon: lucide.Folder(),
		groups: []settingsGroupDef{{rows: a.libraryRows()}}}

	cache := settingsCatDef{id: "cache", label: i18n.T("Cache"), icon: lucide.Database(),
		groups: []settingsGroupDef{{rows: a.cacheRows()}}}

	experiments := settingsCatDef{id: "experiments", label: i18n.T("Experiments"), icon: lucide.FlaskConical(),
		groups: []settingsGroupDef{{rows: []settingsRowDef{
			{label: i18n.T("Smoke on the Progress Bar"), sub: i18n.T("The progress bar turns into a light trailing smoke, moving with the music. Uses a little more power."),
				control: a.smokeControl},
			{label: i18n.T("Fog Backdrop"), sub: i18n.T("Now Playing's backdrop turns into fog in the cover's colors, rising with the bass. Still being polished; uses a little more power."),
				control: a.fogControl},
			{label: i18n.T("Grow From the Cover"), sub: i18n.T("Now Playing opens out of the cover in the player bar and shrinks back into it, rather than sliding up from the bottom"),
				control: a.growFromCoverControl},
		}}}}

	return []settingsCatDef{general, playback, library, cache, experiments}
}

// generalRows are the language, the appearance, and on macOS where the
// app's icon shows.
func (a *app) generalRows() []settingsRowDef {
	rows := []settingsRowDef{
		{label: i18n.T("Language"), control: a.languageControl},
		{label: i18n.T("Appearance"), control: a.appearanceControl},
	}
	if runtime.GOOS == "darwin" {
		rows = append(rows, settingsRowDef{label: i18n.T("App Icon"),
			sub:     i18n.T("Shown in the menu bar, it opens the window again after you close it"),
			control: a.appIconControl})
	}
	return rows
}

// libraryRows are the built-in source and the folders it reads.
func (a *app) libraryRows() []settingsRowDef {
	desc := i18n.T("Add the folders where your music lives")
	folders := a.local.Folders()
	if a.local.Scanning() {
		desc = i18n.T("Looking for music…")
	} else if len(folders) > 0 {
		desc = i18n.Tf("%d songs found", a.local.TrackCount())
	}
	rows := []settingsRowDef{{label: i18n.T("Local Files"), sub: i18n.T("Built-in") + " · " + desc,
		control: a.localFilesControl}}
	for _, f := range folders {
		path := f
		rows = append(rows, settingsRowDef{label: path, icon: lucide.Folder(), small: true,
			control: func(p palette) core.Node {
				return a.settingsAction(p, i18n.T("Remove"), "settings-remove-folder", false, func() {
					a.removeFolder(path)
					a.bump()
				})
			}})
	}
	return rows
}

// cacheRows are the songs kept on disk, and clearing them.
func (a *app) cacheRows() []settingsRowDef {
	rows := []settingsRowDef{{label: i18n.T("Song Cache"),
		sub:     i18n.Tf("Songs from elsewhere are kept here to play again at once, without the network. %s", a.cacheUsage()),
		control: a.cacheControl}}
	if a.pl != nil && a.pl.Cache != nil {
		rows = append(rows, settingsRowDef{label: i18n.T("Clear the Cache"), sub: i18n.T("Delete all the songs kept here"),
			control: a.clearCacheControl})
	}
	return rows
}

// cacheUsage says how much the cache takes, once measured.
func (a *app) cacheUsage() string {
	switch n := a.cacheUsed; {
	case n < 0:
		return i18n.T("Measuring the space used…")
	case n >= 1<<30:
		return i18n.Tf("%.1f GB used", float64(n)/(1<<30))
	default:
		return i18n.Tf("%d MB used", n>>20)
	}
}

// measureCache finds how much the cache takes, on a goroutine, for the
// settings to show.
func (a *app) measureCache() {
	if a.pl == nil || a.pl.Cache == nil {
		a.cacheUsed = 0
		return
	}
	cache := a.pl.Cache
	go func() {
		n := cache.Size()
		a.post(func() {
			a.cacheUsed = n
			a.bump()
		})
	}()
}

// clearCache deletes the songs kept, and measures the cache again.
func (a *app) clearCache() {
	cache := a.pl.Cache
	if cache == nil {
		return
	}
	a.cacheUsed = 0
	a.bump()
	go func() {
		cache.Clear()
		n := cache.Size()
		a.post(func() {
			a.cacheUsed = n
			a.bump()
		})
	}()
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

func (a *app) appIconControl(p palette) core.Node {
	var options []ui.Option[string]
	for _, ic := range appIcons {
		options = append(options, ui.Option[string]{Value: ic.id, Label: i18n.T(ic.label)})
	}
	return ui.Select[string]{Options: options, Value: func() string {
		if a.set.AppIcon == "" {
			return "dock-and-menubar"
		}
		return a.set.AppIcon
	}, OnChange: func(v string) {
		a.set.AppIcon = v
		a.set.save()
		a.applyAppIcon()
		a.bump()
	}, Label: i18n.T("App Icon"), Style: core.Style{Width: core.Pt(170)}}
}

func (a *app) qualityControl(p palette) core.Node {
	var options []ui.Option[string]
	for _, q := range qualityLevels {
		options = append(options, ui.Option[string]{Value: q.level, Label: i18n.T(q.label)})
	}
	return ui.Select[string]{Options: options, Value: func() string { return a.set.Quality }, OnChange: func(v string) {
		a.set.Quality = v
		a.set.save()
		a.bump()
	}, Label: i18n.T("Quality"), Style: core.Style{Width: core.Pt(140)}}
}

func (a *app) normalizeControl(p palette) core.Node {
	return ui.Switch{Checked: func() bool { return a.set.Normalize }, OnChange: func(on bool) {
		a.set.Normalize = on
		a.set.save()
		a.bump()
	}, Label: i18n.T("Even Out Loudness")}
}

func (a *app) smokeControl(p palette) core.Node {
	return ui.Switch{Checked: func() bool { return a.set.Smoke }, OnChange: func(on bool) {
		a.set.Smoke = on
		a.set.save()
		a.bump()
	}, Label: i18n.T("Smoke on the Progress Bar")}
}

func (a *app) fogControl(p palette) core.Node {
	return ui.Switch{Checked: func() bool { return a.set.Fog }, OnChange: func(on bool) {
		a.set.Fog = on
		a.set.save()
		a.bump()
	}, Label: i18n.T("Fog Backdrop")}
}

func (a *app) growFromCoverControl(p palette) core.Node {
	return ui.Switch{Checked: func() bool { return a.set.GrowFromCover }, OnChange: func(on bool) {
		a.set.GrowFromCover = on
		a.set.save()
		a.bump()
	}, Label: i18n.T("Grow From the Cover")}
}

// localFilesControl rescans the folders, or adds another.
func (a *app) localFilesControl(p palette) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
		a.settingsAction(p, i18n.T("Rescan"), "settings-rescan", a.local.Scanning(), func() {
			a.local.Rescan()
			a.bump()
		}),
		a.settingsAction(p, i18n.T("Add Folder…"), "settings-add-folder", false, func() {
			a.pickFolders()
			a.bump()
		}),
	}}
}

func (a *app) cacheControl(p palette) core.Node {
	var options []ui.Option[int]
	value := a.set.CacheMB
	sizes := cacheSizes
	if !slices.Contains(sizes, value) {
		sizes = append(append([]int{}, sizes...), value)
	}
	for _, mb := range sizes {
		options = append(options, ui.Option[int]{Value: mb, Label: cacheLabel(mb)})
	}
	return ui.Select[int]{Options: options, Value: func() int { return a.set.CacheMB }, OnChange: func(v int) {
		a.set.CacheMB = v
		a.set.save()
		if cache := a.pl.Cache; cache != nil {
			limit := int64(v) << 20
			go func() {
				cache.SetLimit(limit)
				n := cache.Size()
				a.post(func() {
					a.cacheUsed = n
					a.bump()
				})
			}()
		} else if v > 0 && a.dir != "" {
			a.pl.Cache = player.NewCache(filepath.Join(a.dir, "songs"), int64(v)<<20)
			a.cacheUsed = 0
		}
		a.bump()
	}, Label: i18n.T("Song Cache"), Style: core.Style{Width: core.Pt(130)}}
}

func (a *app) clearCacheControl(p palette) core.Node {
	empty := a.cacheUsed <= 0
	return a.settingsAction(p, i18n.T("Clear"), "settings-clear-cache", empty, a.clearCache)
}

// settingsAction is a small button at the end of a row, muted when it
// has nothing to do.
func (a *app) settingsAction(p palette, label, testID string, disabled bool, onPress func()) core.Node {
	fg := p.text
	style := core.Style{Padding: core.Symmetric(4, 10), CornerRadius: 7, Background: p.hover}
	if disabled {
		fg = p.faint
		style.Background = core.Color(0)
	} else {
		style.Hover = &core.Style{Background: p.press}
	}
	return base.Button{Label: label, TestID: testID, Disabled: func() bool { return disabled },
		OnPress: func(core.PressEvent) {
			if !disabled && onPress != nil {
				onPress()
			}
		}, Style: style, Children: []core.Node{text(label, 12, core.FontWeightMedium, fg)}}
}

// cacheLabel names a size of the cache.
func cacheLabel(mb int) string {
	switch {
	case mb == 0:
		return i18n.T("Off")
	case mb%1024 == 0:
		return i18n.T(fmt.Sprintf("%d GB", mb/1024))
	default:
		return i18n.Tf("%.1f GB", float64(mb)/1024)
	}
}
