package app

import (
	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

func (a *app) phoneSkeletonRows(p palette, count int) core.Node {
	var rows []core.Node
	for range count {
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Height: core.Pt(68)}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), CornerRadius: 10, Background: p.hover}},
			core.View{Style: core.Style{FlexGrow: 1, Gap: 9}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pct(66), Height: core.Pt(12), CornerRadius: 6, Background: p.hover}},
				core.View{Style: core.Style{Width: core.Pct(40), Height: core.Pt(9), CornerRadius: 5, Background: p.hover}},
			}},
		}})
	}
	return core.View{TestID: "phone-content-skeleton", Label: i18n.T("Loading…"), Style: core.Style{Gap: 2}, Children: rows}
}
func (a *app) phoneSkeleton(p palette, featured bool) core.Node {
	var body []core.Node
	if featured {
		body = append(body, core.View{Style: core.Style{Height: core.Pt(156), CornerRadius: 20, Background: p.hover, Margin: core.Edges{Top: 12, Bottom: 22}}})
	}
	body = append(body, a.phoneSkeletonRows(p, 4))
	return a.pageScroll(p, body...)
}
func (a *app) phoneNotice(p palette, id, title, detail string, retry func()) core.Node {
	// Full service errors belong in a detail sheet. The page keeps a short,
	// actionable status without replacing its already loaded music.
	return core.View{TestID: id + "-notice", Style: core.Style{Padding: core.All(12), Gap: 6, CornerRadius: 12, Background: p.sidebar, Margin: core.Edges{Bottom: 12}}, Children: []core.Node{
		text(title, phoneFontBody, core.FontWeightMedium, p.text),
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			base.Button{TestID: id + "-details", Label: i18n.T("Details"), OnPress: func(core.PressEvent) { a.phoneMessage = detail; a.openPhoneSheet("message") }, Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(48), JustifyContent: core.JustifyCenter}, Children: []core.Node{text(i18n.T("Details"), phoneFontCaption, core.FontWeightNormal, p.muted)}},
			base.Button{TestID: id, Label: i18n.T("Retry"), OnPress: func(core.PressEvent) { retry() }, Style: core.Style{MinHeight: core.Pt(48), Padding: core.Symmetric(0, 14), CornerRadius: 10, Background: p.hover, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{text(i18n.T("Retry"), phoneFontBody, core.FontWeightMedium, p.text)}},
		}},
	}}
}
func (a *app) phoneAccountSheet(p palette) core.Node {
	src := a.source()
	close := func(action func()) func() { return func() { a.phoneSheet.Set(""); action() } }
	rows := []core.Node{text(i18n.T("Account"), phoneFontTitle, core.FontWeightSemibold, p.text)}
	if a.account != nil {
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(14, 0)}, Children: []core.Node{a.accountAvatar(p, a.account, 44), text(a.account.Name, phoneFontSection, core.FontWeightSemibold, p.text)}})
		if a.account.Page != "" {
			rows = append(rows, a.phoneSheetAction(p, "phone-account-my-page", i18n.T("My Page"), lucideUser, close(func() { a.openMyPage(src, a.account) })))
		}
	} else if a.can("signIn") {
		rows = append(rows, a.phoneSheetAction(p, "phone-account-signin", i18n.T("Sign In"), lucideUser, close(func() { a.signInSource(src) })))
	}
	rows = append(rows, a.phoneSheetAction(p, "phone-account-settings", i18n.T("Settings"), lucideSettings, close(a.openSettings)), a.phoneSheetAction(p, "phone-account-sources", i18n.T("Music Sources"), lucideNote, func() { a.phoneSheet.Set("sources") }))
	if a.home != nil && a.home.Subtitle != "" {
		rows = append(rows, core.Text{Content: a.home.Subtitle, Style: core.Style{FontSize: phoneFontCaption, Color: p.muted, Padding: core.Symmetric(8, 0)}})
	}
	if a.account != nil && a.can("signOut") {
		rows = append(rows, a.phoneSheetAction(p, "phone-account-signout", i18n.T("Sign Out"), lucide.LogOut(), close(func() { a.signOutSource(src) })))
	}
	return core.View{Style: core.Style{Gap: 4}, Children: rows}
}

func (a *app) phoneMessageSheet(p palette) core.Node {
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		text(i18n.T("Details"), phoneFontTitle, core.FontWeightSemibold, p.text), core.Scroll{Style: core.Style{MaxHeight: core.Pt(280)}, Children: []core.Node{core.Text{Content: a.phoneMessage, Style: core.Style{FontSize: phoneFontBody, Color: p.muted}}}},
		a.phoneSheetAction(p, "phone-sheet-close", i18n.T("Close"), lucideX, func() { a.phoneSheet.Set("") }),
	}}
}

func (a *app) phonePlaybackIssue(p palette) core.Node {
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		text(i18n.T("Playback unavailable"), phoneFontTitle, core.FontWeightSemibold, p.text),
		core.Scroll{Style: core.Style{MaxHeight: core.Pt(180)}, Children: []core.Node{core.Text{Content: a.phoneMessage, Style: core.Style{FontSize: phoneFontBody, Color: p.muted}}}},
		a.phoneSheetAction(p, "phone-playback-retry", i18n.T("Retry"), lucidePlay, func() { a.phoneSheet.Set(""); a.toggle() }),
		a.phoneSheetAction(p, "phone-playback-sources", i18n.T("Music Sources"), lucideNote, func() { a.phoneSheet.Set("sources") }),
		a.phoneSheetAction(p, "phone-sheet-close", i18n.T("Close"), lucideX, func() { a.phoneSheet.Set("") }),
	}}
}
