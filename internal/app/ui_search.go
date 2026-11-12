package app

import (
	"strings"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
)

func (a *app) searchField(p palette) core.Node { return searchBox{a, p} }

type searchSuggestion struct {
	query, title string
	clear        bool
}

// searchSuggestions are commands, rather than values that replace the
// input with a selected title. The input always shows the user's query.
func (a *app) searchSuggestions() []searchSuggestion {
	a.rev.Get()
	q := strings.TrimSpace(a.query.Get())
	var items []searchSuggestion
	if q != "" {
		items = append(items, searchSuggestion{query: q, title: i18n.Tf("Search for “%s”", q)})
	}
	for _, h := range a.set.SearchHistory {
		if q == "" || !strings.EqualFold(h, q) && strings.Contains(strings.ToLower(h), strings.ToLower(q)) {
			items = append(items, searchSuggestion{query: h, title: h})
		}
	}
	if q == "" && len(items) > 0 {
		items = append(items, searchSuggestion{title: i18n.T("Clear Recent Searches"), clear: true})
	}
	return items
}

type searchBox struct {
	a *app
	p palette
}

func (s searchBox) Render(cx *core.Scope) core.Node {
	a, p := s.a, s.p
	anchor := new(core.Ref)
	focused, composing := core.NewSignal(cx, false), core.NewSignal(cx, false)
	selected := core.NewSignal(cx, -1)
	close := func() { a.searchOpen.Set(false); selected.Set(-1) }
	choose := func(item searchSuggestion) {
		if item.clear {
			a.set.SearchHistory = nil
			a.set.save()
			close()
			a.bump()
			return
		}
		a.search(item.query)
		selected.Set(-1)
	}
	move := func(delta int) {
		items := a.searchSuggestions()
		if len(items) == 0 {
			return
		}
		at := selected.GetUntracked()
		if !a.searchOpen.GetUntracked() || at < 0 {
			at = 0
			if delta < 0 {
				at = len(items) - 1
			}
		} else {
			at = (at + delta + len(items)) % len(items)
		}
		selected.Set(at)
		a.searchOpen.Set(true)
	}
	return core.Group{
		core.View{Ref: anchor, TestID: "search-box", OnPress: func(core.PressEvent) { a.searchRef.Focus() },
			Style: core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Padding: core.Symmetric(0, 8),
				CornerRadius: 8, Background: p.hover, BorderWidth: 1, BorderColor: p.hairline, Margin: core.Edges{Top: 4, Bottom: 8}},
			Children: []core.Node{
				icon(lucideSearch, 14, p.muted),
				core.Input{Ref: a.searchRef, TestID: "search-input", Label: i18n.T("Search"), Placeholder: i18n.T("Search"),
					Value: a.query.Get, NoAutoCorrect: true, Role: core.RoleCombobox, Expanded: a.searchOpen.Get,
					SubmitBehavior:      core.SubmitBehaviorSubmit,
					OnFocus:             func() { focused.Set(true); selected.Set(-1); a.searchOpen.Set(true) },
					OnBlur:              func() { focused.Set(false); close() },
					OnChange:            func(q string) { a.query.Set(q); selected.Set(-1); a.searchOpen.Set(true) },
					OnCompositionChange: func(c core.Composition) { composing.Set(c.Composing) },
					OnSubmit: func(q string) {
						if composing.GetUntracked() {
							return
						}
						items, at := a.searchSuggestions(), selected.GetUntracked()
						if a.searchOpen.GetUntracked() && at >= 0 && at < len(items) {
							choose(items[at])
						} else {
							a.search(q)
						}
					},
					Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(24), Background: core.Color(0), Color: p.text,
						PlaceholderColor: p.faint, FontSize: 12.5, BorderWidth: 0, OutlineWidth: 0}},
				core.Show{When: func() bool { return a.query.Get() != "" }, Then: func(*core.Scope) core.Node {
					return base.Button{TestID: "clear-search", Label: i18n.T("Clear Search"), NoFocus: true,
						OnPress: func(core.PressEvent) { a.closeSearch(); selected.Set(-1) },
						Style: core.Style{Width: core.Pt(18), Height: core.Pt(18), Direction: core.Row, AlignItems: core.AlignCenter,
							JustifyContent: core.JustifyCenter, CornerRadius: 9, Background: p.faint},
						Children: []core.Node{icon(lucideX, 10, p.bg)}}
				}},
			}},
		core.Show{When: func() bool { return focused.Get() && !composing.Get() }, Then: func(cx *core.Scope) core.Node {
			cx.OnKeyWith("down", core.KeyOptions{When: core.KeyAlways}, func() { move(1) })
			cx.OnKeyWith("up", core.KeyOptions{When: core.KeyAlways}, func() { move(-1) })
			cx.OnKeyWith("escape", core.KeyOptions{When: core.KeyAlways}, close)
			return nil
		}},
		core.Show{When: func() bool { return a.searchOpen.Get() && len(a.searchSuggestions()) > 0 }, Then: func(*core.Scope) core.Node {
			return core.Overlay{Anchor: anchor, Placement: core.PlaceBottom, Align: core.AlignStart, Offset: 4, Margin: 8,
				TestID: "search-suggestions", Role: core.RoleListbox, Label: i18n.T("Suggestions"),
				OnCancel: close, OnPressOutside: func(core.Point) { close() },
				Style: core.Style{Width: core.Pt(sidebarWidth - 28), Background: p.bg, BorderWidth: 1, BorderColor: p.hairline,
					CornerRadius: 10, Padding: core.All(4), ShadowColor: p.cardShade, ShadowRadius: 18, ShadowOffset: core.Point{Y: -8}},
				Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
					var rows []core.Node
					for i, item := range a.searchSuggestions() {
						rows = append(rows, base.Button{Label: item.title, Role: core.RoleOption, NoFocus: true,
							Selected: func() bool { return selected.Get() == i },
							OnPress:  func(core.PressEvent) { choose(item) },
							StyleFunc: func() core.Style {
								bg := core.Color(0)
								if selected.Get() == i {
									bg = p.hover
								}
								return core.Style{MinHeight: core.Pt(28), Padding: core.Symmetric(5, 8), CornerRadius: 6,
									Background: bg, Hover: &core.Style{Background: p.hover}}
							}, Children: []core.Node{text(item.title, 12.5, core.FontWeightNormal, p.text)}})
					}
					return core.Scroll{Style: core.Style{MaxHeight: core.Pt(300)}, Children: rows}
				})}}
		}},
	}
}
