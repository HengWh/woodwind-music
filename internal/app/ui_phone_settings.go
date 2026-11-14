package app

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
)

func (a *app) phoneSettings(cx *core.Scope, p palette, cats []settingsCatDef) core.Node {
	if a.settingsPage == "" {
		a.settingsPage = cats[0].id
	}
	var tabs, groups []core.Node
	for _, category := range cats {
		cat := category
		color, bg := p.muted, p.sidebar
		if cat.id == a.settingsPage {
			color, bg = p.onAccent, p.accent
			for _, group := range cat.groups {
				groups = append(groups, a.settingsGroupCard(p, group))
			}
		}
		tabs = append(tabs, base.Button{TestID: "phone-settings-" + cat.id, Label: cat.label, OnPress: func(core.PressEvent) { a.settingsPage = cat.id; a.bump() },
			Style: core.Style{Height: core.Pt(48), FlexShrink: 0, Padding: core.Symmetric(0, 16), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				CornerRadius: 24, Background: bg}, Children: []core.Node{text(cat.label, 14, core.FontWeightMedium, color)}})
	}
	return core.View{TestID: "phone-settings", Style: core.Style{Width: core.Pct(100), Height: core.Pct(100), Direction: core.Column, Background: p.bg,
		PaddingEnv: core.EnvAll(core.EnvSafeArea)}, Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(56), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 12)}, Children: []core.Node{
			a.phoneIcon(p, "settings-back", i18n.T("Back"), lucideChevronLeft, p.text, a.closeSettings),
			text(i18n.T("Settings"), phoneFontTitle, core.FontWeightSemibold, p.text),
		}},
		core.Scroll{Horizontal: true, HideIndicator: true, Style: core.Style{FlexShrink: 0, Height: core.Pt(64), MinHeight: core.Pt(64)}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Gap: 8, Padding: core.Symmetric(8, 16)}, Children: tabs},
		}},
		core.Scroll{TestID: "settings-content", Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(0)}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Column, Padding: core.Edges{Top: 8, Left: 16, Right: 16, Bottom: 24}, Gap: 12}, Children: groups},
		}},
	}}
}
