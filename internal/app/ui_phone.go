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
		core.View{TestID: "app-content", Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0), Background: p.bg, Overflow: core.Hidden}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node { a.rev.Get(); return a.page(cx, p) }),
		}},
		core.Show{When: func() bool { return cx.Env().KeyboardHeight() == 0 }, Then: func(cx *core.Scope) core.Node {
			return core.Group{core.Show{When: func() bool {
				a.playRev.Get()
				src, _, _ := radioParts(a.pl.Radio())
				return a.route.Get().page != pageStations || src != a.source().Info().ID
			}, Then: func(cx *core.Scope) core.Node { return a.phonePlayerBar(cx, p) }}, a.phoneNavigation(cx, p)}
		}},
		core.Show{When: a.np.Get, Then: func(cx *core.Scope) core.Node { return a.phoneNowPlaying(cx, p) }},
		core.Show{When: func() bool { a.rev.Get(); return a.settingsInMain }, Then: func(cx *core.Scope) core.Node {
			return core.View{TestID: "settings-in-main", Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0), ZIndex: 20}, Children: []core.Node{a.settingsView(cx)}}
		}},
		a.phoneSheets(cx, p), a.nameDialog(cx, p), a.deleteDialog(cx, p), a.storedDeleteDialog(cx, p),
		core.Show{When: func() bool { return a.toast.Get() != "" }, Then: a.toastView},
	}}
}

func (a *app) phoneIcon(p palette, id, label string, svg *core.SVG, color core.Color, action func()) core.Node {
	return base.Button{TestID: id, Label: label, OnPress: func(core.PressEvent) { action() },
		Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
			CornerRadius: 24, Active: &core.Style{Background: p.press}}, Children: []core.Node{icon(svg, 23, color)}}
}

func (a *app) phoneSourceButton(p palette, id string) core.Node {
	width := core.Points(118)
	page := a.route.GetUntracked().page
	if page != pageHome && page != pageLibrary && page != pageSearch {
		width = 88
	}
	return base.Button{TestID: id, Label: a.sourceName(), OnPress: func(core.PressEvent) { a.openPhoneSheet("sources") },
		Style: core.Style{Height: core.Pt(48), Width: core.Pt(width), MaxWidth: core.Pt(width), FlexShrink: 1, MinWidth: core.Pt(72), JustifyContent: core.JustifyCenter}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pct(100), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 5, Padding: core.Symmetric(7, 10), CornerRadius: 14, Background: p.sidebar}, Children: []core.Node{
				core.Text{Content: a.sourceName(), MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: 10.5, FontWeight: core.FontWeightMedium, Color: p.muted}}, icon(lucideChevronDown, 13, p.muted),
			}},
		}}
}

func (a *app) phoneHeader(cx *core.Scope, p palette) core.Node {
	return core.View{TestID: "phone-header", Style: core.Style{FlexShrink: 0, ZIndex: 1, Background: p.bg, Padding: core.Symmetric(4, 16)}, Children: []core.Node{
		core.Show{When: func() bool { return a.route.Get().page == pageSearch }, Then: func(*core.Scope) core.Node { return a.phoneSearchField(p) }, Else: func(cx *core.Scope) core.Node {
			return core.Dynamic(func(cx *core.Scope) core.Node {
				a.rev.Get()
				r := a.route.Get()
				title := i18n.T("Discover")
				if a.phoneTab == 1 || r.page == pageLibrary {
					title = i18n.T("Music Library")
				}
				switch r.page {
				case pageStored:
					title = i18n.T("Saved Music")
				case pageAlbum:
					title = i18n.T("Album")
				case pageArtist:
					title = i18n.T("Artist")
				case pagePlaylist:
					title = i18n.T("Playlist")
				case pageLiked:
					title = i18n.T("Liked Songs")
				case pageShows:
					title = i18n.T("Podcasts")
				case pageComments:
					title = i18n.T("Comments")
				case pageSavedAlbums:
					title = i18n.T("Saved Albums")
				case pageStations:
					title = a.stationsTitle()
				case pageBrowse:
					if d := a.browses[r.id]; d != nil && d.browse != nil {
						title = d.browse.Title
					}
				}
				var children []core.Node
				if len(a.history) > 0 {
					children = append(children, a.phoneIcon(p, "phone-back", i18n.T("Back"), lucideChevronLeft, p.text, a.back))
				}
				children = append(children, core.Text{Content: title, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: phoneFontHeader, FontWeight: core.FontWeightSemibold, Color: p.text}})
				id := "phone-source-switcher"
				if r.page == pageLibrary {
					id = "phone-sources"
				}
				children = append(children, a.phoneSourceButton(p, id))
				if a.source() == a.local {
					if r.page == pageHome {
						children = append(children, a.phoneIcon(p, "phone-add-folder", i18n.T("Add Folder…"), lucideFolder, p.muted, a.pickFolders))
					}
				} else {
					var avatar core.Node = core.View{Style: core.Style{Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 16, Background: p.sidebar, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideUser, 18, p.muted)}}
					if a.account != nil {
						avatar = a.accountAvatar(p, a.account, 32)
						if a.account.ImageURL != "" {
							avatar = a.sourceImage(core.Image{TestID: "phone-account-avatar", Source: imageSource(a.account.ImageURL), ResizeMode: core.Cover, Style: core.Style{Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 16, Overflow: core.Hidden, Background: p.sidebar}})
						}
					}
					children = append(children, base.Button{TestID: "phone-account", Label: i18n.T("Account"), OnPress: func(core.PressEvent) { a.openPhoneSheet("account") }, Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{avatar}})
				}
				if r.page == pageLibrary {
					children = append(children, a.phoneIcon(p, "open-settings", i18n.T("Settings"), lucideSettings, p.muted, a.openSettings))
				}
				return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(52), Gap: 6}, Children: children}
			})
		}},
	}}
}

func (a *app) phoneSearchField(p palette) core.Node { return phoneSearchBox{a, p} }

type phoneSearchBox struct {
	a *app
	p palette
}

func (s phoneSearchBox) Render(cx *core.Scope) core.Node {
	a, p := s.a, s.p
	anchor := new(core.Ref)
	focused, composing := core.NewSignal(cx, false), core.NewSignal(cx, false)
	close := func() { a.searchOpen.Set(false) }
	submit := func(q string) {
		if composing.GetUntracked() {
			return
		}
		a.search(q)
		a.searchRef.Blur()
		a.cx.DismissKeyboard()
	}
	return core.Group{
		core.View{Ref: anchor, TestID: "search-box", Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(48), Background: p.sidebar, CornerRadius: 14, Padding: core.Edges{Left: 16}, Gap: 10}, Children: []core.Node{
			icon(lucideSearch, 21, p.muted),
			core.Input{Ref: a.searchRef, TestID: "search-input", Label: i18n.T("Search"), Placeholder: i18n.T("Songs, artists, albums"), Value: a.query.Get,
				NoAutoCorrect: true, ReturnKeyType: core.ReturnKeySearch, SubmitBehavior: core.SubmitBehaviorSubmit,
				OnFocus: func() { focused.Set(true); a.searchOpen.Set(true) }, OnBlur: func() { focused.Set(false); close() },
				OnCompositionChange: func(c core.Composition) { composing.Set(c.Composing) },
				OnChange:            func(q string) { a.query.Set(q); a.searchOpen.Set(true) }, OnSubmit: submit,
				Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(48), FontSize: phoneFontBody, Color: p.text, PlaceholderColor: p.muted, Background: core.Color(0)}},
			a.phoneIcon(p, "clear-search", i18n.T("Clear Search"), lucideX, p.muted, func() { a.query.Set(""); a.open(route{page: pageSearch}); a.searchRef.Focus(); a.searchOpen.Set(true) }),
		}},
		core.Show{When: func() bool {
			return focused.Get() && !composing.Get() && a.searchOpen.Get() && len(a.searchSuggestions()) > 0
		}, Then: func(cx *core.Scope) core.Node {
			return core.Overlay{Anchor: anchor, TestID: "phone-search-suggestions", Placement: core.PlaceBottom, Align: core.AlignStart, Offset: 6, Margin: 8, OnCancel: close, OnPressOutside: func(core.Point) { close() },
				Style: core.Style{Width: core.Pt(cx.Env().Window().Width - 32), Background: p.bg, CornerRadius: 14, Padding: core.All(6), ShadowColor: p.cardShade, ShadowRadius: 12},
				Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
					var rows []core.Node
					for i, item := range a.searchSuggestions() {
						if i >= 6 {
							break
						}
						suggestion := item
						rows = append(rows, base.Button{TestID: fmt.Sprintf("phone-suggestion-%d", i), Label: item.title, NoFocus: true, OnPress: func(core.PressEvent) {
							if suggestion.clear {
								a.set.SearchHistory = nil
								a.set.save()
								close()
								a.bump()
							} else {
								submit(suggestion.query)
							}
						},
							Style: core.Style{MinHeight: core.Pt(48), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(8, 10), CornerRadius: 8, Active: &core.Style{Background: p.hover}}, Children: []core.Node{icon(lucideSearch, 17, p.muted), core.Text{Content: item.title, MaxLines: 2, Style: core.Style{FlexShrink: 1, MinWidth: core.Pt(0), FontSize: phoneFontBody, Color: p.text}}}})
					}
					return core.Scroll{Style: core.Style{MaxHeight: core.Pt(min(320, max(96, cx.Env().Window().Height-cx.Env().KeyboardHeight()-cx.Env().SafeArea().Top-88)))}, Children: rows}
				})},
			}
		}},
	}
}

func (a *app) phoneNavigation(cx *core.Scope, p palette) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.route.Get()
		a.navigationRev.Get()
		var tabs []core.Node
		for i, tab := range []struct {
			id, label string
			svg       *core.SVG
		}{
			{"phone-tab-home", i18n.T("Discover"), lucideHome}, {"phone-tab-library", i18n.T("Music Library"), lucideList}, {"phone-tab-search", i18n.T("Search"), lucideSearch},
		} {
			index := i
			color := p.muted
			if i == a.phoneTab {
				color = p.accent
			}
			tabs = append(tabs, base.Button{TestID: tab.id, Label: tab.label, Selected: func() bool { return index == a.phoneTab },
				OnPress: func(core.PressEvent) { a.selectPhoneTab(index) }, Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Height: core.Pt(58),
					Direction: core.Column, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 4, Active: &core.Style{Background: p.hover}},
				Children: []core.Node{icon(tab.svg, 21, color), text(tab.label, 10, core.FontWeightMedium, color)}})
		}
		return core.View{TestID: "phone-navigation", Style: core.Style{Direction: core.Row, FlexShrink: 0, Background: p.bg, Borders: core.Edges{Top: 1}, BorderColor: p.hairline}, Children: tabs}
	})
}

func (a *app) phonePlayerBar(cx *core.Scope, p palette) core.Node {
	return core.View{TestID: "player-bar", Style: core.Style{FlexShrink: 0, ZIndex: 1, Padding: core.Edges{Top: 3, Left: 12, Right: 12, Bottom: 3}, Background: p.bg}, Children: []core.Node{
		core.View{Style: core.Style{CornerRadius: 14, Background: p.sidebar, BorderWidth: .5, BorderColor: p.hairline, Overflow: core.Hidden}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node {
				a.playRev.Get()
				tr := a.pl.Current()
				title, art := i18n.T("Not Playing"), ""
				if tr != nil {
					title, art = tr.Name, tr.Album.ImageURL
				}
				return core.View{Style: core.Style{Height: core.Pt(60), Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 8), Gap: 2}, Children: []core.Node{
					base.Button{TestID: "bar-open", Label: i18n.T("Now Playing"), OnPress: func(core.PressEvent) {
						if tr != nil {
							if err := a.pl.Err(); err != "" {
								a.phoneMessage = err
								a.openPhoneSheet("playback-error")
							} else {
								a.openNowPlaying()
							}
						}
					}, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(56), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
						a.cover(p, art, 40, 10), core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 4}, Children: []core.Node{
							core.Text{Content: title, MaxLines: 1, Style: core.Style{FontSize: phoneFontBody, FontWeight: core.FontWeightMedium, Color: p.text}},
							core.Text{TestID: "playback-status", ContentFunc: func() string {
								a.tick.Get()
								a.rev.Get()
								if a.pl.Err() != "" {
									return i18n.T("Playback unavailable")
								}
								if a.importing {
									return i18n.T("Importing music…")
								}
								if a.pl.Loading() {
									return i18n.T("Buffering…")
								}
								if tr != nil {
									label := tr.ArtistNames()
									if tr.Source != "" && tr.Source != a.source().Info().ID {
										if src := a.sourceByID(tr.Source); src != nil {
											label += " · " + src.Info().Name
										}
									}
									return label
								}
								return i18n.T("Choose a song to start listening")
							}, MaxLines: 1, Style: core.Style{FontSize: 10.5, Color: p.muted}},
						}},
					}}, a.phonePlayButton(p, "play-pause", false), a.phoneIcon(p, "bar-queue", i18n.T("Up Next"), lucideList, p.muted, func() { a.openPhoneSheet("queue") }),
				}}
			}),
			core.View{TestID: "mini-progress", StyleFunc: func() core.Style {
				a.tick.Get()
				percent := float32(0)
				if d := a.pl.Duration(); d > 0 {
					percent = float32(a.pl.Position()) / float32(d) * 100
				}
				return core.Style{Position: core.Absolute, Left: core.Pt(0), Bottom: core.Pt(0), Width: core.Pct(min(100, max(0, percent))), Height: core.Pt(2), Background: p.accent.Faded(.75)}
			}},
		}},
	}}
}

func (a *app) phonePlayButton(p palette, id string, large bool) core.Node {
	side, glyph, bg, fg := core.Points(48), core.Points(24), core.Color(0), p.text
	if large {
		side, glyph, bg, fg = 68, 32, p.text, p.bg
	}
	return base.Button{TestID: id, Label: i18n.T("Play/Pause"), Disabled: func() bool { return a.pl.Current() == nil }, OnPress: func(core.PressEvent) { a.toggle() },
		Style: core.Style{Width: core.Pt(side), Height: core.Pt(side), FlexShrink: 0, CornerRadius: side / 2, Background: bg,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Active: &core.Style{Opacity: core.Alpha(0.6)}}, Children: []core.Node{core.Icon{SVGFunc: func() *core.SVG {
			a.playRev.Get()
			if a.pl.Playing() {
				return lucidePause
			}
			return lucidePlay
		}, Style: core.Style{Width: core.Pt(glyph), Height: core.Pt(glyph), Color: fg}}}}
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
						if a.isCurrentTrack(tr) {
							color = p.accent
						}
						if tr.Unavailable {
							color = p.faint
						}
						return core.Style{FontSize: phoneFontSong, FontWeight: core.FontWeightNormal, Color: color}
					}},
					core.Text{Content: phoneTrackSubtitle(tr), MaxLines: 1, Style: core.Style{FontSize: phoneFontCaption, Color: p.muted}},
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

func phoneTrackSubtitle(tr music.Track) string {
	if tr.Unavailable {
		return i18n.T("Playback unavailable") + " · " + tr.ArtistNames()
	}
	return tr.ArtistNames()
}

func (a *app) isCurrentTrack(tr music.Track) bool {
	current := a.pl.Current()
	if current == nil || current.ID != tr.ID {
		return false
	}
	src, playing := tr.Source, current.Source
	if src == "" {
		src = a.source().Info().ID
	}
	if playing == "" {
		playing = a.source().Info().ID
	}
	return src == playing
}
