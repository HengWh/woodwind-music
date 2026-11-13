package app

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

// Layout follows the available window, independently of the playback backend.
func (a *app) phoneLayout(cx *core.Scope) bool {
	width := cx.Env().Window().Width
	return width > 0 && width < 720
}

type phoneTabState struct {
	route            route
	history, forward []route
}

// Each primary destination retains its own detail stack and scroll positions.
func (a *app) selectPhoneTab(tab int) {
	if tab == a.phoneTab {
		return
	}
	a.cx.DismissKeyboard()
	a.phoneTabs[a.phoneTab] = phoneTabState{a.route.GetUntracked(), a.history, a.forward}
	a.phoneTab = tab
	saved := a.phoneTabs[tab]
	a.history, a.forward = saved.history, saved.forward
	a.show(saved.route)
}

func (a *app) phoneView(cx *core.Scope, p palette) core.Node {
	return core.View{TestID: "app-root", Style: core.Style{Width: core.Pct(100), Height: core.Pct(100),
		Direction: core.Column, Background: p.bg, Color: p.text, FontSize: phoneFontBody, PaddingEnv: core.EnvAll(core.EnvSafeArea)}, Children: []core.Node{
		a.phoneHeader(cx, p),
		core.View{TestID: "app-content", Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0), Background: p.bg}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node { a.rev.Get(); return a.page(cx, p) }),
		}},
		core.Show{When: func() bool { return cx.Env().KeyboardHeight() == 0 }, Then: func(cx *core.Scope) core.Node {
			return core.Group{a.phonePlayerBar(cx, p), a.phoneNavigation(cx, p)}
		}},
		core.Show{When: a.np.Get, Then: func(cx *core.Scope) core.Node { return a.phoneNowPlaying(cx, p) }},
		core.Show{When: func() bool { a.rev.Get(); return a.settingsInMain }, Then: func(cx *core.Scope) core.Node {
			return core.View{TestID: "settings-in-main", Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0), ZIndex: 20}, Children: []core.Node{a.settingsView(cx)}}
		}},
		a.phoneSheets(cx, p), a.nameDialog(cx, p), a.deleteDialog(cx, p),
		core.Show{When: func() bool { return a.toast.Get() != "" }, Then: a.toastView},
	}}
}

func (a *app) phoneIcon(p palette, id, label string, svg *core.SVG, color core.Color, action func()) core.Node {
	return base.Button{TestID: id, Label: label, OnPress: func(core.PressEvent) { action() },
		Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: 24, Active: &core.Style{Background: p.press}}, Children: []core.Node{icon(svg, 23, color)}}
}

func (a *app) phoneHeader(cx *core.Scope, p palette) core.Node {
	return core.View{TestID: "phone-header", Style: core.Style{FlexShrink: 0, Padding: core.Symmetric(4, 12)}, Children: []core.Node{
		core.Show{When: func() bool { return a.route.Get().page == pageSearch }, Then: func(cx *core.Scope) core.Node { return a.phoneSearchField(p) },
			Else: func(cx *core.Scope) core.Node {
				return core.Dynamic(func(cx *core.Scope) core.Node {
					a.rev.Get()
					r := a.route.Get()
					title := i18n.T("Home")
					if a.phoneTab == 1 || r.page == pageLibrary {
						title = i18n.T("Music Library")
					}
					switch r.page {
					case pageAlbum:
						title = i18n.T("Album")
					case pageArtist:
						title = i18n.T("Artist")
					case pagePlaylist:
						title = i18n.T("Playlist")
					case pageLiked:
						title = i18n.T("Liked Songs")
					case pageBrowse:
						if data := a.browses[r.id]; data != nil && data.browse != nil {
							title = data.browse.Title
						}
					}
					var children []core.Node
					if len(a.history) > 0 {
						children = append(children, a.phoneIcon(p, "phone-back", i18n.T("Back"), lucideChevronLeft, p.text, a.back))
					} else {
						children = append(children, core.View{Style: core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 14, Background: p.accent,
							AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideNote, 21, p.onAccent)}})
					}
					children = append(children, core.Text{Content: title, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: phoneFontTitle, FontWeight: core.FontWeightSemibold, Color: p.text}})
					children = append(children, a.phoneIcon(p, "phone-add-folder", i18n.T("Add Folder…"), lucideFolder, p.text, a.pickFolders))
					if a.phoneTab == 1 {
						children = append(children, a.phoneIcon(p, "open-settings", i18n.T("Settings"), lucideSettings, p.text, a.openSettings))
					} else {
						children = append(children, a.phoneIcon(p, "phone-search", i18n.T("Search"), lucideSearch, p.text, func() { a.selectPhoneTab(2) }))
					}
					return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: children}
				})
			}},
	}}
}

func (a *app) phoneSearchField(p palette) core.Node {
	return core.View{TestID: "search-box", Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(48),
		Background: p.sidebar, CornerRadius: 24, Padding: core.Edges{Left: 16}, Gap: 10}, Children: []core.Node{
		icon(lucideSearch, 21, p.muted),
		core.Input{Ref: a.searchRef, TestID: "search-input", Label: i18n.T("Search"), Placeholder: i18n.T("Search your library"), Value: a.query.Get,
			NoAutoCorrect: true, ReturnKeyType: core.ReturnKeySearch, SubmitBehavior: core.SubmitBehaviorSubmit,
			OnChange: a.query.Set, OnSubmit: func(q string) { a.search(q); a.cx.DismissKeyboard() },
			Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(48), FontSize: phoneFontBody, Color: p.text, PlaceholderColor: p.muted, Background: core.Color(0)}},
		a.phoneIcon(p, "clear-search", i18n.T("Clear Search"), lucideX, p.muted, func() {
			a.query.Set("")
			a.open(route{page: pageSearch})
			a.searchRef.Focus()
		}),
	}}
}

func (a *app) phoneNavigation(cx *core.Scope, p palette) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.route.Get()
		a.rev.Get()
		var tabs []core.Node
		for i, tab := range []struct {
			id, label string
			svg       *core.SVG
		}{
			{"phone-tab-home", i18n.T("Home"), lucideHome}, {"phone-tab-library", i18n.T("Music Library"), lucideList}, {"phone-tab-search", i18n.T("Search"), lucideSearch},
		} {
			index := i
			color := p.muted
			if i == a.phoneTab {
				color = p.accent
			}
			tabs = append(tabs, base.Button{TestID: tab.id, Label: tab.label, Selected: func() bool { return index == a.phoneTab },
				OnPress: func(core.PressEvent) { a.selectPhoneTab(index) }, Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Height: core.Pt(60),
					Direction: core.Column, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 4, Active: &core.Style{Background: p.hover}},
				Children: []core.Node{icon(tab.svg, 23, color), text(tab.label, 11, core.FontWeightMedium, color)}})
		}
		return core.View{TestID: "phone-navigation", Style: core.Style{Direction: core.Row, FlexShrink: 0, Background: p.bg, Borders: core.Edges{Top: 1}, BorderColor: p.hairline}, Children: tabs}
	})
}

func (a *app) phonePlayerBar(cx *core.Scope, p palette) core.Node {
	return core.View{TestID: "player-bar", Style: core.Style{FlexShrink: 0, Direction: core.Column, Background: p.sidebar}, Children: []core.Node{
		core.View{TestID: "mini-progress", StyleFunc: func() core.Style {
			a.tick.Get()
			percent := float32(0)
			if d := a.pl.Duration(); d > 0 {
				percent = float32(a.pl.Position()) / float32(d) * 100
			}
			return core.Style{Width: core.Pct(min(100, max(0, percent))), Height: core.Pt(2), Background: p.accent}
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			a.playRev.Get()
			tr := a.pl.Current()
			title, art := i18n.T("Not Playing"), ""
			if tr != nil {
				title, art = tr.Name, tr.Album.ImageURL
			}
			return core.View{Style: core.Style{Height: core.Pt(phoneTwoLineHeight(cx, phoneFontBody)), Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 12), Gap: 4}, Children: []core.Node{
				base.Button{TestID: "bar-open", Label: i18n.T("Now Playing"), OnPress: func(core.PressEvent) {
					if tr != nil {
						a.openNowPlaying()
					}
				},
					Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(phoneTwoLineHeight(cx, phoneFontBody) - 8), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
						a.cover(p, art, 44, 22),
						core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 3}, Children: []core.Node{
							core.Text{Content: title, MaxLines: 1, Style: core.Style{FontSize: phoneFontBody, FontWeight: core.FontWeightMedium, Color: p.text}},
							core.Text{TestID: "playback-status", ContentFunc: func() string {
								a.tick.Get()
								a.rev.Get()
								if err := a.pl.Err(); err != "" {
									return err
								}
								if a.importing {
									return i18n.T("Importing music…")
								}
								if a.pl.Loading() {
									return i18n.T("Buffering…")
								}
								if tr != nil {
									return tr.ArtistNames()
								}
								return i18n.T("Choose a song to start listening")
							}, MaxLines: 1, Style: core.Style{FontSize: 12, Color: p.muted}},
						}},
					}},
				a.phonePlayButton(p, "play-pause", false),
				a.phoneIcon(p, "bar-queue", i18n.T("Up Next"), lucideList, p.text, func() { a.openPhoneSheet("queue") }),
			}}
		}),
	}}
}

func (a *app) phonePlayButton(p palette, id string, large bool) core.Node {
	svg := lucidePlay
	if a.pl.Playing() {
		svg = lucidePause
	}
	side, glyph, bg, fg := core.Points(48), core.Points(24), core.Color(0), p.text
	if large {
		side, glyph, bg, fg = 68, 32, p.text, p.bg
	}
	return base.Button{TestID: id, Label: i18n.T("Play/Pause"), Disabled: func() bool { return a.pl.Current() == nil }, OnPress: func(core.PressEvent) { a.toggle() },
		Style: core.Style{Width: core.Pt(side), Height: core.Pt(side), FlexShrink: 0, CornerRadius: side / 2, Background: bg,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Active: &core.Style{Opacity: core.Alpha(0.6)}}, Children: []core.Node{icon(svg, glyph, fg)}}
}

func (a *app) phoneTrackRow(p palette, tracks []music.Track, i int, playlist string) core.Node {
	tr := tracks[i]
	return core.View{TestID: "phone-row-" + tr.ID, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinWidth: core.Pt(0), Height: core.Pt(a.trackRowHeight())}, Children: []core.Node{
		base.Button{TestID: "track-" + tr.ID, Label: tr.Name, Disabled: func() bool { return tr.Unavailable }, OnPress: func(core.PressEvent) { a.playTracks(tracks, i) },
			Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(a.trackRowHeight()), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12,
				Padding: core.Symmetric(8, 0), CornerRadius: 10, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
				a.cover(p, tr.Album.ImageURL, 44, 9),
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{
					core.Text{Content: tr.Name, MaxLines: 1, StyleFunc: func() core.Style {
						a.playRev.Get()
						color := p.text
						if a.currentID() == tr.ID {
							color = p.accent
						}
						return core.Style{FontSize: phoneFontSong, FontWeight: core.FontWeightMedium, Color: color}
					}},
					core.Text{Content: tr.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: phoneFontCaption, Color: p.muted}},
				}},
			}},
		a.phoneIcon(p, "track-more-"+tr.ID, i18n.T("More"), lucideMore, p.muted, func() { a.openPhoneTrack(tr, playlist, i) }),
	}}
}

func phoneScrollKey(r route) string {
	return fmt.Sprintf("phone:%d:%s:%s:%s", r.page, r.id, r.query, r.kind)
}

func (a *app) trackRowHeight() core.Points {
	if a.phoneLayout(a.cx) {
		return phoneTwoLineHeight(a.cx, phoneFontSong)
	}
	return 54
}
