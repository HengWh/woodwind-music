package app

import (
	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"time"
)

// Settings starts with a vertical directory. Each category has its own page.
func (a *app) phoneSettings(cx *core.Scope, p palette, cats []settingsCatDef) core.Node {
	title := i18n.T("Settings")
	var body []core.Node
	for _, cat := range cats {
		category := cat
		if a.settingsPage == "" {
			body = append(body, base.Button{TestID: "phone-settings-" + cat.id, Label: cat.label,
				OnPress:  func(core.PressEvent) { a.settingsPage = category.id; a.bump() },
				Style:    core.Style{MinHeight: core.Pt(64), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14, Padding: core.Symmetric(8, 14), CornerRadius: 12, Active: &core.Style{Background: p.hover}},
				Children: []core.Node{icon(cat.icon, 21, p.muted), core.Text{Content: cat.label, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: phoneFontSong, Color: p.text}}, icon(lucideChevronRight, 18, p.faint)},
			})
		} else if cat.id == a.settingsPage {
			title = cat.label
			for _, group := range cat.groups {
				body = append(body, a.settingsGroupCard(p, group))
			}
		}
	}
	key := "settings:" + a.settingsPage
	return core.View{TestID: "phone-settings", Style: core.Style{Width: core.Pct(100), Height: core.Pct(100), Direction: core.Column, Background: p.bg, PaddingEnv: core.EnvAll(core.EnvSafeArea)}, Children: []core.Node{
		core.View{Style: core.Style{MinHeight: core.Pt(56), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 12)}, Children: []core.Node{
			a.phoneIcon(p, "settings-back", i18n.T("Back"), lucideChevronLeft, p.text, a.backSettings),
			core.Text{Content: title, MaxLines: 1, Style: core.Style{FontSize: phoneFontTitle, FontWeight: core.FontWeightSemibold, Color: p.text, FlexShrink: 1, MinWidth: core.Pt(0)}},
		}},
		core.Scroll{TestID: "settings-content", ContentOffset: core.Point{Y: a.scroll[key]}, ScrollEventThrottle: 80 * time.Millisecond,
			OnScroll: func(e core.ScrollEvent) { a.scroll[key] = e.Offset.Y }, OnScrollEnd: func(e core.ScrollEvent) { a.scroll[key] = e.Offset.Y },
			Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(0)}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Column, Padding: core.Edges{Top: 8, Left: 16, Right: 16, Bottom: 24}, Gap: 8}, Children: body},
			}},
	}}
}
