package app

import (
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

func (a *app) phoneHome(cx *core.Scope, p palette) core.Node {
	if a.source() != a.local {
		return a.homePage(cx, p)
	}
	if a.home == nil && a.homeBusy {
		return a.loadingPage(p)
	}
	if a.home == nil || len(a.home.Sections) == 0 {
		return a.emptyHome(p)
	}
	body := []core.Node{core.View{Style: core.Style{Gap: 4, Margin: core.Edges{Top: 8, Bottom: 16}}, Children: []core.Node{
		text(i18n.Today(a.localTime()), 12, core.FontWeightMedium, p.muted),
		text(greeting(a.localTime()), phoneFontDisplay, core.FontWeightSemibold, p.text),
	}}}
	body = append(body, core.Dynamic(func(cx *core.Scope) core.Node {
		a.playRev.Get()
		tr := a.pl.Current()
		label := i18n.T("Continue listening")
		if tr == nil && len(a.recent) > 0 {
			tr = &a.recent[0]
		}
		if tr == nil {
			label = i18n.T("From your library")
			for _, sec := range a.home.Sections {
				if len(sec.Tracks) > 0 {
					t := sec.Tracks[0]
					tr = &t
					break
				}
			}
		}
		if tr == nil {
			return core.Group{}
		}
		track := *tr
		return base.Button{TestID: "phone-continue", Label: label, OnPress: func(core.PressEvent) {
			if a.currentID() != track.ID {
				a.playTracks([]music.Track{track}, 0)
			} else if !a.pl.Playing() {
				a.toggle()
			}
			a.openNowPlaying()
		}, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.All(16), Gap: 12, CornerRadius: 22,
			Background: p.accent, Margin: core.Edges{Bottom: 16}, Active: &core.Style{Opacity: core.Alpha(0.85)}}, Children: []core.Node{
			a.cover(p, track.Album.ImageURL, 80, 14),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 8}, Children: []core.Node{
				text(label, 12, core.FontWeightMedium, core.RGBA(0xffffff, 0.75)),
				core.Text{Content: track.Name, MaxLines: 2, Style: core.Style{FontSize: phoneFontSection, FontWeight: core.FontWeightSemibold, Color: p.onAccent}},
				core.Text{Content: track.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: 12, Color: core.RGBA(0xffffff, 0.8)}},
			}},
			icon(lucideChevronRight, 22, p.onAccent),
		}}
	}))
	for index, sec := range a.home.Sections {
		if len(sec.Tracks) > 0 {
			sectionIndex := index
			body = append(body, a.phoneSectionHeading(p, sec.Title, func() { a.open(route{page: pageSection, id: "", query: strconv.Itoa(sectionIndex)}) }))
			// Keep the home page short; the full section retains the complete queue.
			for i := range min(4, len(sec.Tracks)) {
				body = append(body, a.phoneTrackRow(p, sec.Tracks, i, ""))
			}
		}
		if len(sec.Albums) > 0 {
			body = append(body, a.phoneSectionHeading(p, sec.Title, func() { a.open(route{page: pageBrowse, id: "albums"}) }))
			cards := make([]core.Node, 0, len(sec.Albums))
			for _, album := range sec.Albums {
				cards = append(cards, core.View{Style: core.Style{Width: core.Pt(144), FlexShrink: 0}, Children: []core.Node{a.albumCard(p, album)}})
			}
			body = append(body, a.phoneRail(cards))
		}
		if len(sec.Playlists) > 0 {
			body = append(body, a.phoneSectionHeading(p, sec.Title, func() { a.selectPhoneTab(1) }))
			var cards []core.Node
			for _, list := range sec.Playlists {
				cards = append(cards, core.View{Style: core.Style{Width: core.Pt(144), FlexShrink: 0}, Children: []core.Node{a.playlistCard(p, list)}})
			}
			body = append(body, a.phoneRail(cards))
		}
	}
	return a.pageScroll(p, body...)
}

func (a *app) phoneRail(cards []core.Node) core.Node {
	return core.Scroll{Horizontal: true, HideIndicator: true, Style: core.Style{Margin: core.Edges{Bottom: 12}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 14, Padding: core.Edges{Bottom: 8}}, Children: cards},
	}}
}

func (a *app) phoneSectionHeading(p palette, title string, more func()) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Margin: core.Edges{Top: 8, Bottom: 8}}, Children: []core.Node{
		core.Text{Content: title, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: phoneFontSection, FontWeight: core.FontWeightSemibold, Color: p.text}},
		a.phoneIcon(p, "", i18n.T("See All"), lucideChevronRight, p.muted, more),
	}}
}

func (a *app) phoneLibrary(cx *core.Scope, p palette) core.Node {
	if a.source() != a.local {
		return a.pluginPhoneLibrary(p)
	}
	body := []core.Node{core.View{Style: core.Style{Gap: 6, Margin: core.Edges{Top: 8, Bottom: 16}}, Children: []core.Node{
		text(i18n.T("Your music, always with you"), phoneFontDisplay, core.FontWeightSemibold, p.text),
		text(i18n.Songs(a.local.TrackCount()), 13, core.FontWeightNormal, p.muted),
	}}}
	body = append(body, a.phoneLibraryRow(p, "phone-sources", i18n.T("Music Sources"), a.sourceName(), lucideNote, func() { a.openPhoneSheet("sources") }))
	for _, item := range []struct {
		id, title, sub string
		svg            *core.SVG
		r              route
	}{
		{"phone-liked", i18n.T("Liked Songs"), i18n.Songs(len(a.liked)), lucideHeart, route{page: pageLiked}},
		{"phone-songs", i18n.T("Songs"), i18n.Songs(a.local.TrackCount()), lucideNote, route{page: pageBrowse, id: "songs"}},
		{"phone-albums", i18n.T("Albums"), "", lucideDisc, route{page: pageBrowse, id: "albums"}},
		{"phone-artists", i18n.T("Artists"), "", lucideUser, route{page: pageBrowse, id: "artists"}},
	} {
		r := item.r
		body = append(body, a.phoneLibraryRow(p, item.id, item.title, item.sub, item.svg, func() { a.open(r) }))
	}
	body = append(body, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Margin: core.Edges{Top: 24, Bottom: 8}}, Children: []core.Node{
		core.Text{Content: i18n.T("My Playlists"), Style: core.Style{FlexGrow: 1, FontSize: phoneFontSection, FontWeight: core.FontWeightSemibold, Color: p.text}},
		a.phoneIcon(p, "new-playlist", i18n.T("New Playlist"), lucidePlus, p.text, a.newPlaylist),
	}})
	if len(a.mine) == 0 {
		body = append(body, text(i18n.T("Create a playlist for your favorite moments"), 13, core.FontWeightNormal, p.muted))
	}
	for _, list := range a.mine {
		pl := list
		body = append(body, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter}, Children: []core.Node{
			base.Button{TestID: "playlist-" + pl.ID, Label: pl.Name, OnPress: func(core.PressEvent) { a.open(route{page: pagePlaylist, id: pl.ID}) },
				Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(10, 0)}, Children: []core.Node{
					a.cover(p, pl.ImageURL, 56, 12),
					core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{
						core.Text{Content: pl.Name, MaxLines: 1, Style: core.Style{FontSize: phoneFontSong, FontWeight: core.FontWeightMedium, Color: p.text}},
						text(playlistLine(pl), 12, core.FontWeightNormal, p.muted),
					}},
				}},
			a.phoneIcon(p, "playlist-more-"+pl.ID, i18n.T("More"), lucideMore, p.muted, func() { a.phonePlaylist = pl.ID; a.openPhoneSheet("playlist") }),
		}})
	}
	body = append(body, core.View{Style: core.Style{Margin: core.Edges{Top: 24}}, Children: []core.Node{
		a.phoneLibraryRow(p, "phone-library-import", i18n.T("Add Folder…"), i18n.T("Import local music"), lucideFolder, a.pickFolders),
	}})
	return a.pageScroll(p, body...)
}

func (a *app) phoneLibraryRow(p palette, id, title, sub string, svg *core.SVG, action func()) core.Node {
	return base.Button{TestID: id, Label: title, OnPress: func(core.PressEvent) { action() },
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(phoneTwoLineHeight(a.cx, phoneFontSong)), Gap: 14, Padding: core.Symmetric(6, 12),
			Background: p.sidebar, CornerRadius: 16, Margin: core.Edges{Bottom: 8}, Active: &core.Style{Background: p.press}}, Children: []core.Node{
			icon(svg, 25, p.accent),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 4}, Children: []core.Node{
				core.Text{Content: title, MaxLines: 1, Style: core.Style{FontSize: phoneFontSong, FontWeight: core.FontWeightMedium, Color: p.text}},
				core.Text{Content: sub, MaxLines: 1, Style: core.Style{FontSize: 12, Color: p.muted}},
			}}, icon(lucideChevronRight, 19, p.faint),
		}}
}
