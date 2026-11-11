package app

import (
	"path/filepath"
	"slices"

	"nsgo-music/internal/player"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"

	"nsgo-music/internal/i18n"
)

// settingsView is the settings window: the app's languages and look, what
// plays, where the music is, and what the cache may take.
func (a *app) settingsView(cx *core.Scope) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		p := a.palette(cx)
		general := a.settingsSection(p, i18n.T("General"), []core.Node{a.languageRow(p), a.appearanceRow(p)})
		playback := a.settingsSection(p, i18n.T("Playback"), []core.Node{
			a.switchRow(p, i18n.T("Normalize volume"), i18n.T("Adjust each song's volume so they sound as loud as each other"), a.set.Normalize, func(on bool) {
				a.set.Normalize = on
				a.set.save()
			}),
		})
		library := a.settingsSection(p, i18n.T("Library"), a.folderRows(p))
		cache := a.settingsSection(p, i18n.T("Cache"), []core.Node{a.cacheRow(p)})
		column := core.View{Style: core.Style{Direction: core.Column, Padding: core.All(24), Gap: 26, MaxWidth: core.Pt(640)},
			Children: []core.Node{general, playback, library, cache}}
		head := core.View{Style: core.Style{Height: core.Pt(56), Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 24),
			BorderColors: core.EdgeColors{Bottom: p.hairline}, BorderWidth: 1, BorderColor: p.hairline},
			Children: []core.Node{text(i18n.T("Settings"), 20, core.FontWeightBold, p.text)}}
		scroll := core.Scroll{Style: core.Style{FlexGrow: 1}, Children: []core.Node{column}}
		return core.View{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100), Background: p.bg, Color: p.text, Direction: core.Column},
			Children: []core.Node{head, scroll}}
	})
}

func (a *app) settingsSection(p palette, title string, rows []core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Column, Gap: 2}, Children: []core.Node{
		core.View{Style: core.Style{Padding: core.Edges{Bottom: 8}}, Children: []core.Node{text(title, 13, core.FontWeightSemibold, p.muted)}},
		core.View{Style: core.Style{Direction: core.Column, Background: p.sidebar, CornerRadius: 12, Padding: core.All(6),
			BorderWidth: 1, BorderColor: p.hairline}, Children: rows},
	}}
}

func (a *app) settingsRow(p palette, label, sub string, control core.Node) core.Node {
	children := []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 2}, Children: []core.Node{
			text(label, 13.5, core.FontWeightMedium, p.text),
			core.Show{When: func() bool { return sub != "" }, Then: func(*core.Scope) core.Node {
				return core.Text{Content: sub, Style: core.Style{FontSize: 12, Color: p.muted}}
			}},
		}},
	}
	if control != nil {
		children = append(children, control)
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(10, 10),
		CornerRadius: 8, Hover: &core.Style{Background: p.hover}}, Children: children}
}

func (a *app) languageRow(p palette) core.Node {
	options := []ui.Option[string]{{Value: i18n.System, Label: i18n.T("System")}}
	for _, l := range i18n.Languages {
		options = append(options, ui.Option[string]{Value: l.Code, Label: l.Name})
	}
	return a.settingsRow(p, i18n.T("Language"), "", ui.Select[string]{Options: options, Value: func() string { return a.set.Language },
		OnChange: func(v string) {
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
		}, Label: i18n.T("Language"), Style: core.Style{Width: core.Pt(170)}})
}

func (a *app) appearanceRow(p palette) core.Node {
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
	return a.settingsRow(p, i18n.T("Appearance"), "", core.View{Style: core.Style{Direction: core.Row, Gap: 2,
		Background: p.hover, Padding: core.All(2), CornerRadius: 8}, Children: buttons})
}

func (a *app) switchRow(p palette, label, sub string, value bool, onChange func(bool)) core.Node {
	return a.settingsRow(p, label, sub, ui.Switch{Checked: func() bool { return value }, OnChange: func(on bool) {
		onChange(on)
		a.bump()
	}, Label: label})
}

func (a *app) folderRows(p palette) []core.Node {
	var rows []core.Node
	folders := a.local.Folders()
	for _, f := range folders {
		path := f
		rows = append(rows, a.settingsRow(p, path, "", base.Button{Label: i18n.T("Remove"),
			OnPress: func(core.PressEvent) { a.removeFolder(path); a.bump() },
			Style: core.Style{Padding: core.Symmetric(4, 10), CornerRadius: 7, Background: p.hover,
				Hover: &core.Style{Background: p.press}}, Children: []core.Node{text(i18n.T("Remove"), 12, core.FontWeightMedium, p.muted)}}))
	}
	if len(folders) == 0 {
		rows = append(rows, a.settingsRow(p, i18n.T("No folders yet"), i18n.T("Add the folders where your music lives"), nil))
	}
	rows = append(rows, a.settingsRow(p, i18n.T("Add Folder…"), "", base.Button{Label: i18n.T("Add Folder…"),
		OnPress: func(core.PressEvent) { a.pickFolders(); a.bump() },
		Style: core.Style{Padding: core.Symmetric(4, 10), CornerRadius: 7, Background: p.hover,
			Hover: &core.Style{Background: p.press}}, Children: []core.Node{text(i18n.T("Add"), 12, core.FontWeightMedium, p.text)}}))
	return rows
}

func (a *app) cacheRow(p palette) core.Node {
	options := []ui.Option[int]{{Value: 0, Label: i18n.T("Off")}}
	values := []int{0, 512, 1024, 2048, 4096, 10240}
	for _, v := range values[1:] {
		options = append(options, ui.Option[int]{Value: v, Label: cacheLabel(v)})
	}
	if !slices.Contains(values, a.set.CacheMB) {
		options = append(options, ui.Option[int]{Value: a.set.CacheMB, Label: cacheLabel(a.set.CacheMB)})
	}
	return a.settingsRow(p, i18n.T("Keep songs on disk"), i18n.T("Songs played again open at once, and play offline"),
		ui.Select[int]{Options: options, Value: func() int { return a.set.CacheMB }, OnChange: func(v int) {
			a.set.CacheMB = v
			a.set.save()
			if v > 0 && a.pl.Cache == nil && a.dir != "" {
				a.pl.Cache = player.NewCache(filepath.Join(a.dir, "songs"), int64(v)<<20)
			} else if a.pl.Cache != nil {
				a.pl.Cache.SetLimit(int64(v) << 20)
			}
			a.bump()
		}, Label: i18n.T("Cache"), Style: core.Style{Width: core.Pt(140)}})
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
