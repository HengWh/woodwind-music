package app

import (
	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// A ListRow changes its index as its native cell is recycled. Keep the
// buttons and text views mounted, and bind their contents and actions to that
// index. Only the artwork subtree changes when the actual cover URL changes.
func (a *app) phoneTrackCell(cx *core.Scope, p palette, tracks func() []music.Track, row core.ListRow, playlist string) core.Node {
	track := func() music.Track {
		list, i := tracks(), row.Index()
		if i < 0 || i >= len(list) {
			return music.Track{Unavailable: true}
		}
		return list[i]
	}
	artwork := core.NewMemo(cx, func() string { return track().Album.ImageURL })
	height := a.trackRowHeight()
	return core.View{TestID: "phone-row-cell", Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinWidth: core.Pt(0), Height: core.Pt(height)}, Children: []core.Node{
		base.Button{TestID: "phone-track-play", LabelFunc: func() string { return track().Name }, Disabled: func() bool { return track().Unavailable },
			OnPress: func(core.PressEvent) {
				list, i := tracks(), row.Index()
				if i >= 0 && i < len(list) && !list[i].Unavailable {
					a.playTracks(list, i)
				}
			},
			Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(height), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(8, 0), CornerRadius: 10, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node { return a.cover(p, artwork.Get(), 44, 9) }),
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{
					core.Text{ContentFunc: func() string { return track().Name }, MaxLines: 1, Style: core.Style{FontSize: phoneFontSong, FontWeight: core.FontWeightNormal}, StyleFunc: func() core.Style {
						a.playRev.Get()
						tr := track()
						color := p.text
						if a.isCurrentTrack(tr) {
							color = p.accent
						}
						if tr.Unavailable {
							color = p.faint
						}
						return core.Style{Color: color}
					}},
					core.Text{ContentFunc: func() string { return phoneTrackSubtitle(track()) }, MaxLines: 1, Style: core.Style{FontSize: phoneFontCaption, Color: p.muted}},
				}},
			}},
		base.Button{TestID: "phone-track-more", LabelFunc: func() string { return i18n.Tf("More: %s", track().Name) }, OnPress: func(core.PressEvent) {
			list, i := tracks(), row.Index()
			if i >= 0 && i < len(list) {
				a.openPhoneTrack(list[i], playlist, i)
			}
		},
			Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 24, Active: &core.Style{Background: p.press}}, Children: []core.Node{icon(lucideMore, 23, p.muted)}},
	}}
}
