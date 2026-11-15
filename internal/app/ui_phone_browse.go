package app

import (
	"fmt"
	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

func (a *app) phoneBrowseSection(p palette, sec music.Section, pageID string, index int) []core.Node {
	var body []core.Node
	if sec.Title != "" {
		var more func()
		if sec.MorePage != "" {
			more = func() { a.openPhoneSection(pageID, index, sec.MorePage) }
		}
		body = append(body, a.phoneSectionHeading(p, sec.Title, more))
	}
	if len(sec.Tracks) > 0 {
		body = append(body, a.trackList(p, sec.Tracks, true, false, a.currentID()))
	}
	if len(sec.Albums) > 0 {
		body = append(body, a.cardGrid(p, len(sec.Albums), func(i int) core.Node { return a.albumCard(p, sec.Albums[i]) }))
	}
	if len(sec.Artists) > 0 {
		body = append(body, a.cardGrid(p, len(sec.Artists), func(i int) core.Node { return a.artistCard(p, sec.Artists[i]) }))
	}
	if len(sec.Playlists) > 0 {
		body = append(body, a.cardGrid(p, len(sec.Playlists), func(i int) core.Node { return a.playlistCard(p, sec.Playlists[i]) }))
	}
	if len(sec.Shows) > 0 {
		body = append(body, a.cardGrid(p, len(sec.Shows), func(i int) core.Node { return a.showCard(p, sec.Shows[i]) }))
	}
	if len(sec.Stations) > 0 {
		body = append(body, a.cardGrid(p, len(sec.Stations), func(i int) core.Node { return a.stationCard(p, sec.Stations[i]) }))
	}
	for i, l := range sec.Links {
		link := l
		var leading core.Node = core.View{Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), CornerRadius: 12, Background: p.sidebar, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideNote, 20, p.muted)}}
		if link.ImageURL != "" {
			leading = a.cover(p, link.ImageURL, 56, 12)
		}
		body = append(body, base.Button{Label: link.Title, OnPress: func(core.PressEvent) { a.open(route{page: pageBrowse, id: link.Page}) }, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(76), Padding: core.Symmetric(8, 0), CornerRadius: 12, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
			core.Text{Content: fmt.Sprintf("%02d", i+1), Style: core.Style{Width: core.Pt(23), FontSize: phoneFontCaption, Color: p.faint}}, leading,
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{
				core.Text{Content: link.Title, MaxLines: 2, Style: core.Style{FontSize: phoneFontSong, FontWeight: core.FontWeightMedium, Color: p.text}}, core.Text{Content: link.Subtitle, MaxLines: 1, Style: core.Style{FontSize: phoneFontCaption, Color: p.muted}},
			}}, icon(lucideChevronRight, 17, p.faint),
		}})
	}
	if len(body) == 0 && sec.Empty != "" {
		body = append(body, a.emptyNote(p, sec.Empty, ""))
	}
	return body
}

func (a *app) phoneSearchHistory(p palette) core.Node {
	body := []core.Node{core.View{Style: core.Style{Gap: 8, Margin: core.Edges{Top: 18, Bottom: 18}}, Children: []core.Node{
		text(i18n.Tf("Search %s", a.sourceName()), phoneFontCaption, core.FontWeightMedium, p.muted),
		core.Text{Content: i18n.T("What would you like to hear?"), Style: core.Style{FontSize: phoneFontDisplay, FontWeight: core.FontWeightSemibold, Color: p.text, LineHeightMultiple: 1.2}},
	}}}
	if len(a.set.SearchHistory) > 0 {
		body = append(body, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(48)}, Children: []core.Node{
			core.Text{Content: i18n.T("Recent Searches"), Style: core.Style{FlexGrow: 1, FontSize: phoneFontBody, FontWeight: core.FontWeightMedium, Color: p.text}},
			a.phoneIcon(p, "clear-history", i18n.T("Clear"), lucideTrash, p.muted, func() { a.set.SearchHistory = nil; a.set.save(); a.bump() }),
		}})
		var chips []core.Node
		for _, q := range a.set.SearchHistory {
			query := q
			chips = append(chips, base.Button{Label: query, OnPress: func(core.PressEvent) { a.search(query) }, Style: core.Style{MinHeight: core.Pt(48), MaxWidth: core.Pct(100), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 7, Padding: core.Symmetric(8, 13), CornerRadius: 12, Background: p.sidebar, Active: &core.Style{Background: p.press}}, Children: []core.Node{
				icon(lucide.Clock3(), 15, p.muted), core.Text{Content: query, MaxLines: 1, Style: core.Style{FlexShrink: 1, MinWidth: core.Pt(0), FontSize: phoneFontBody, Color: p.text}},
			}})
		}
		body = append(body, core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8}, Children: chips})
	} else {
		body = append(body, text(i18n.T("Find songs, albums and artists."), phoneFontBody, core.FontWeightNormal, p.muted))
	}
	return a.pageScroll(p, body...)
}
