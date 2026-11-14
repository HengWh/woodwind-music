package app

import (
	"runtime"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

// Account state belongs to the source even when another source is on screen.
type sourceAccount struct {
	source                  music.Source
	account                 *music.Account
	asked, busy, signingOut bool
	generation              int
}

func (a *app) accountState(src music.Source) *sourceAccount {
	if a.accounts == nil {
		a.accounts = map[string]*sourceAccount{}
	}
	id := src.Info().ID
	s := a.accounts[id]
	if s == nil || s.source != src {
		s = &sourceAccount{source: src}
		a.accounts[id] = s
	}
	return s
}

func (a *app) invalidateAccount(src music.Source) {
	s := a.accountState(src)
	s.generation++
	s.busy = false
}

func (a *app) setAccount(src music.Source, account *music.Account) {
	a.invalidateAccount(src)
	s := a.accountState(src)
	s.account, s.asked = account, true
	if a.source() == src {
		a.account = account
	}
}

func (a *app) loadAccount(src music.Source) {
	s := a.accountState(src)
	if !src.Can("account") || s.busy || s.asked || s.signingOut || a.signingIn && a.signSource == src {
		return
	}
	s.busy = true
	gen := s.generation
	go func() {
		account, err := src.Account()
		a.post(func() {
			if a.sourceByID(src.Info().ID) != src || gen != s.generation {
				return
			}
			s.busy, s.asked = false, true
			if err == nil {
				s.account = account
				if a.source() == src {
					a.account = account
				}
			}
			a.bump()
		})
	}()
}

func (a *app) signOutSource(src music.Source) {
	s := a.accountState(src)
	if s.signingOut || a.signingIn || !src.Can("signOut") {
		return
	}
	a.invalidateAccount(src)
	s.signingOut = true
	a.bump()
	go func() {
		err := src.SignOut()
		a.post(func() {
			if a.sourceByID(src.Info().ID) != src {
				return
			}
			s.signingOut = false
			if err != nil {
				s.asked = true
				a.say(err.Error())
			} else {
				a.setAccount(src, nil)
				if a.source() == src {
					a.reload()
				}
			}
			a.bump()
		})
	}()
}

func (a *app) accountAvatar(p palette, account *music.Account, size core.Points) core.Node {
	if account.ImageURL != "" {
		return a.sourceImage(core.Image{TestID: "account-avatar", Source: imageSource(account.ImageURL), ResizeMode: core.Cover,
			Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), FlexShrink: 0, CornerRadius: size / 2, Overflow: core.Hidden, Background: p.press}})
	}
	return core.View{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), FlexShrink: 0,
		CornerRadius: size / 2, Background: p.press, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Children: []core.Node{icon(lucideUser, size*0.54, p.muted)}}
}

func (a *app) openMyPage(src music.Source, account *music.Account) {
	if account == nil || account.Page == "" {
		return
	}
	a.open(route{page: pageBrowse, id: account.Page, src: src.Info().ID})
	a.loadBrowse(account.Page, true)
}

// MyGo's footer is Settings for sources without accounts, Sign In when signed
// out, and an avatar/name menu when signed in.
func (a *app) accountRow(p palette) core.Node {
	src := a.source()
	s := a.accountState(src)
	if a.account == nil {
		label, id, action := i18n.T("Settings"), "open-settings", a.openSettings
		glyph := lucideSettings
		unknown := src.Can("account") && !s.asked
		if unknown {
			label, id, glyph = i18n.T("Loading…"), "account-loading", lucideUser
		} else if src.Can("signIn") {
			label, id, glyph = i18n.Tf("Sign in to %s", a.sourceName()), "account-signin", lucideUser
			action = func() { a.signInSource(src) }
		}
		return base.Button{Label: label, TestID: id, Disabled: func() bool { return unknown || a.signingIn },
			OnPress: func(core.PressEvent) { action() },
			Style: core.Style{Height: core.Pt(40), Padding: core.Symmetric(0, 8), Gap: 10, Direction: core.Row,
				AlignItems: core.AlignCenter, CornerRadius: 10, Hover: &core.Style{Background: p.hover}, Active: &core.Style{Background: p.press}},
			Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(26), Height: core.Pt(26), AlignItems: core.AlignCenter,
				JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(glyph, 15, p.muted)}},
				core.Text{Content: label, MaxLines: 1, Style: core.Style{FlexShrink: 1, MinWidth: core.Pt(0), FontSize: 13, FontWeight: core.FontWeightMedium, Color: p.muted}}}}
	}
	account := a.account
	row := core.View{Style: core.Style{Height: core.Pt(40), Direction: core.Row, AlignItems: core.AlignCenter,
		Padding: core.Symmetric(0, 8), Gap: 10, CornerRadius: 10, Hover: &core.Style{Background: p.hover}}, Children: []core.Node{
		a.accountAvatar(p, account, 26),
		core.Text{TestID: "account-name", Content: account.Name, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1,
			MinWidth: core.Pt(0), FontSize: 13, FontWeight: core.FontWeightSemibold, Color: p.text}},
		icon(lucide.ChevronsUpDown(), 13, p.faint),
	}}
	var items []core.Node
	if account.Page != "" {
		items = append(items, a.accountMenuItem(p, "account-my-page", i18n.T("My Page"), "", lucideUser,
			func() { a.openMyPage(src, account) }))
	}
	shortcut := "Ctrl+,"
	if runtime.GOOS == "darwin" {
		shortcut = "⌘,"
	}
	items = append(items, a.accountMenuItem(p, "account-settings", i18n.T("Settings…"), shortcut, lucideSettings, a.openSettings))
	if src.Can("signOut") {
		items = append(items, ui.MenuSeparator{}, base.MenuItem{TestID: "account-signout", Label: i18n.T("Sign Out"),
			Disabled: func() bool { return s.signingOut || a.signingIn }, OnSelect: func() { a.signOutSource(src) },
			Style: a.accountMenuItemStyle(p), Children: []core.Node{icon(lucide.LogOut(), 15, p.muted), text(i18n.T("Sign Out"), 13, core.FontWeightNormal, p.text)}})
	}
	menu := base.Menu{TestID: "account-menu", Label: account.Name, Trigger: row, Placement: core.PlaceTop, Align: core.AlignStart, Offset: 6, Margin: 8,
		PopupStyle: core.Style{Width: core.Pt(sidebarWidth - 28), Direction: core.Column, Padding: core.All(5), Gap: 1,
			Background: p.bg, Color: p.text, CornerRadius: 12, BorderWidth: 0.5, BorderColor: p.hairline,
			ShadowColor: p.cardShade, ShadowRadius: 24, ShadowOffset: core.Point{Y: 8}}, Children: items}
	if a.phoneLayout(a.cx) {
		menu.Presentation = base.PresentSheet
		menu.PopupStyle.Width = core.Pct(100)
		menu.PopupStyle.Padding = core.Edges{Top: 12, Right: 16, Bottom: a.cx.Env().SafeArea().Bottom + 12, Left: 16}
		menu.BackdropStyle.Background = core.RGBA(0x000000, 0.45)
	}
	return menu
}

func (a *app) accountMenuItemStyle(p palette) core.Style {
	return core.Style{Height: core.Pt(a.headingSize(30, 48)), Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 8), Gap: 9,
		CornerRadius: 7, Hover: &core.Style{Background: p.press}, Focus: &core.Style{Background: p.press}}
}

func (a *app) accountMenuItem(p palette, id, label, shortcut string, svg *core.SVG, action func()) core.Node {
	return base.MenuItem{TestID: id, Label: label, OnSelect: action, Style: a.accountMenuItemStyle(p), Children: []core.Node{
		icon(svg, 15, p.muted), core.Text{Content: label, Style: core.Style{FlexGrow: 1, FontSize: 13, Color: p.text}},
		text(shortcut, 12, core.FontWeightNormal, p.faint),
	}}
}
