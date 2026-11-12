package app

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/player"
)

const playerBarHeight core.Points = 84

// playerBar is the strip along the foot: what plays, the transport, its
// progress and the volume.
func (a *app) playerBar(cx *core.Scope, p palette) core.Node {
	left := core.Dynamic(func(cx *core.Scope) core.Node {
		a.playRev.Get()
		return a.barTrack(p)
	})
	controls := core.Dynamic(func(cx *core.Scope) core.Node {
		a.playRev.Get()
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2}, Children: a.transport(p, p.muted, p.accent, 18)}
	})
	progress := a.progress(p, false)
	var right []core.Node
	right = append(right, core.Dynamic(func(cx *core.Scope) core.Node {
		a.playRev.Get()
		if a.pl.Current() == nil {
			return core.Group{}
		}
		return core.Group{
			base.Button{Label: i18n.T("Lyrics"), TestID: "bar-lyrics", OnPress: func(core.PressEvent) {
				a.openNowPlaying()
				a.npPane.Set(0)
			}, Style: core.Style{Width: core.Pt(30), Height: core.Pt(30), Direction: core.Row, AlignItems: core.AlignCenter,
				JustifyContent: core.JustifyCenter, CornerRadius: 8, Hover: &core.Style{Background: p.hover}},
				Children: []core.Node{icon(lucideMic, 16, p.muted)}},
			base.Button{Label: i18n.T("Up Next"), TestID: "bar-queue", OnPress: func(core.PressEvent) {
				a.openNowPlaying()
				a.npPane.Set(1)
			}, Style: core.Style{Width: core.Pt(30), Height: core.Pt(30), Direction: core.Row, AlignItems: core.AlignCenter,
				JustifyContent: core.JustifyCenter, CornerRadius: 8, Hover: &core.Style{Background: p.hover}},
				Children: []core.Node{icon(lucideList, 16, p.muted)}},
		}
	}))
	right = append(right, base.Button{Label: i18n.T("Volume"), TestID: "volume-icon", OnPress: func(core.PressEvent) {
		a.mute()
	}, Style: core.Style{Width: core.Pt(24), Height: core.Pt(24), Direction: core.Row, AlignItems: core.AlignCenter,
		JustifyContent: core.JustifyCenter, CornerRadius: 6, Hover: &core.Style{Background: p.hover}},
		Children: []core.Node{icon(lucideVolume, 16, p.muted)}})
	right = append(right, a.volumeSlider(p))
	return core.View{TestID: "player-bar", Style: core.Style{Height: core.Pt(playerBarHeight), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter,
		Padding: core.Symmetric(0, 18), Gap: 16, Background: p.bg, Borders: core.Edges{Top: 1},
		BorderColor: p.hairline}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(150)}, Children: []core.Node{left}},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(192), Direction: core.Column, AlignItems: core.AlignCenter, Gap: 1}, Children: []core.Node{
			controls,
			core.View{Style: core.Style{Width: core.Pct(100), MaxWidth: core.Pt(620), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8},
				Children: []core.Node{
					core.Text{ContentFunc: func() string { a.tick.Get(); return clock(a.pl.Position()) },
						Style: core.Style{Width: core.Pt(36), TextAlign: core.TextAlignEnd, FontSize: 10.5, Color: p.faint}},
					core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0)}, Children: []core.Node{progress}},
					core.Text{ContentFunc: func() string { a.tick.Get(); return clock(a.pl.Duration()) },
						Style: core.Style{Width: core.Pt(36), FontSize: 10.5, Color: p.faint}},
				}},
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(194), JustifyContent: core.JustifyEnd},
			Children: right},
	}}
}

// barTrack is the cover, title and artist in the player bar, which opens
// Now Playing.
func (a *app) barTrack(p palette) core.Node {
	tr := a.pl.Current()
	if tr == nil {
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(52), Height: core.Pt(52), CornerRadius: 8, Background: p.hover,
				Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{icon(lucideNote, 20, p.faint)}},
			text(i18n.T("Not Playing"), 13, core.FontWeightNormal, p.muted),
		}}
	}
	open := func(core.PressEvent) { a.openNowPlaying() }
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
		base.Button{Label: i18n.T("Now Playing"), TestID: "bar-open", OnPress: open, Style: core.Style{Width: core.Pt(52), Height: core.Pt(52), CornerRadius: 8},
			Children: []core.Node{a.cover(p, tr.Album.ImageURL, 52, 8)}},
		core.View{Style: core.Style{FlexShrink: 1, MinWidth: core.Pt(0), MaxWidth: core.Pt(170), Direction: core.Column, Gap: 3}, Children: []core.Node{
			base.Button{Label: tr.Name, TestID: "bar-open-title", OnPress: open, Style: core.Style{Padding: core.Symmetric(0, 0)},
				Children: []core.Node{core.Text{Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: 13.5, FontWeight: core.FontWeightSemibold, Color: p.text}}}},
			core.Text{Spans: a.artistSpans(p, *tr, 12, p.muted), MaxLines: 1, Style: core.Style{FontSize: 12, Color: p.muted}},
		}},
		a.likeButton(p, tr.ID, 15, p.muted),
	}}
}

// artistSpans is a track's artists as links.
func (a *app) artistSpans(p palette, tr music.Track, size core.Points, color core.Color) []core.Span {
	var spans []core.Span
	for i, ar := range tr.Artists {
		if i > 0 {
			spans = append(spans, core.Span{Content: ", "})
		}
		if ar.Name == "" {
			continue
		}
		id := ar.ID
		spans = append(spans, core.Span{Content: ar.Name, Style: core.Style{Color: color},
			OnPress: func(core.PressEvent) {
				if id != "" {
					a.open(route{page: pageArtist, id: id})
				}
			}})
	}
	return spans
}

// transport is the buttons that play: shuffle, previous, play or pause,
// next and repeat.
func (a *app) transport(p palette, fg, accent core.Color, size core.Points) []core.Node {
	pl := a.pl
	playing := pl.Playing()
	shuffle := pl.Shuffle()
	repeat := pl.Repeat()
	shuffleColor := fg
	if shuffle {
		shuffleColor = accent
	}
	repeatColor := fg
	repeatIcon := lucideRepeat
	if repeat != player.RepeatOff {
		repeatColor = accent
	}
	if repeat == player.RepeatOne {
		repeatIcon = lucideRepeatOne
	}
	glyph := transportPlay
	if playing {
		glyph = transportPause
	}
	return []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: size * 0.5}, Children: []core.Node{
		a.roundButton(nil, p, size*0.62, shuffleColor, i18n.T("Shuffle"), lucideShuffle, a.toggleShuffle),
		a.roundButton(nil, p, size*0.85, p.text, i18n.T("Previous"), transportPrev, pl.Prev),
		base.Button{Label: i18n.T("Play/Pause"), TestID: "play-pause", OnPress: func(core.PressEvent) { a.toggle() },
			Style: core.Style{Width: core.Pt(size*1.25 + 16), Height: core.Pt(size*1.25 + 16), CornerRadius: 999,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Hover: &core.Style{Background: p.hover}, Active: &core.Style{Background: p.press}},
			Children: []core.Node{icon(glyph, size*1.25, p.text)}},
		a.roundButton(nil, p, size*0.85, p.text, i18n.T("Next"), transportNext, pl.Next),
		a.roundButton(nil, p, size*0.62, repeatColor, i18n.T("Repeat"), repeatIcon, a.cycleRepeat),
	}}}
}

// progress is the bar of what has played. On the Now Playing view it is
// drawn in white; in the bar, in the accent.
func (a *app) progress(p palette, nowPlaying bool) core.Node {
	track := p.accent
	if nowPlaying {
		track = core.RGB(0xffffff)
	}
	rest := p.press
	if nowPlaying {
		rest = core.RGBA(0xffffff, 0.18)
	}
	pl := a.pl
	return base.Slider{
		Value: func() base.SliderValue {
			a.tick.Get()
			return base.SingleValue(pl.Position().Seconds())
		},
		Min: func() float64 { return 0 },
		Max: func() float64 {
			a.tick.Get()
			return max(1, pl.Duration().Seconds())
		},
		OnChangeEnd: func(v base.SliderValue) { pl.Seek(time.Duration(v.End * float64(time.Second))) },
		Label:       i18n.T("Progress"),
		TestID:      "progress",
		ValueText:   func(v float64) string { return clock(time.Duration(v * float64(time.Second))) },
		Style:       core.Style{Height: core.Pt(18)},
		TrackStyle:  core.Style{Height: core.Pt(18)},
		IndicatorStyle: core.Style{Height: core.Pt(4), CornerRadius: 2,
			Margin: core.Edges{Top: 7, Left: 0, Right: 0}, Background: rest},
		RangeStyle: core.Style{Height: core.Pt(4), CornerRadius: 2, Background: track},
		ThumbStyle: core.Style{Width: core.Pt(0), Height: core.Pt(0)},
	}
}

// volumeSlider is the volume in the player bar.
func (a *app) volumeSlider(p palette) core.Node {
	pl := a.pl
	return base.Slider{
		Value: func() base.SliderValue { a.tick.Get(); return base.SingleValue(pl.Volume() * 100) },
		Min:   func() float64 { return 0 },
		Max:   func() float64 { return 100 },
		OnChange: func(v base.SliderValue) {
			a.set.Volume = v.End / 100
			pl.SetVolume(a.set.Volume)
			a.set.save()
		},
		Label:      i18n.T("Volume"),
		TestID:     "bar-volume",
		Style:      core.Style{Width: core.Pt(92), Height: core.Pt(18)},
		TrackStyle: core.Style{Height: core.Pt(18)},
		IndicatorStyle: core.Style{Height: core.Pt(4), CornerRadius: 2,
			Margin: core.Edges{Top: 7, Left: 0, Right: 0}, Background: p.press},
		RangeStyle: core.Style{Height: core.Pt(4), CornerRadius: 2, Background: p.muted},
		ThumbStyle: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, Background: p.text},
	}
}
