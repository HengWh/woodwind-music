package app

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"nsgo-music/internal/i18n"
	"nsgo-music/internal/music"
	"nsgo-music/internal/player"
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
	return core.View{Style: core.Style{Height: core.Pt(playerBarHeight), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter,
		Padding: core.Symmetric(0, 18), Gap: 16, Background: p.bg, BorderColors: core.EdgeColors{Top: p.hairline}, BorderWidth: 1,
		BorderColor: p.hairline}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(280), FlexShrink: 0}, Children: []core.Node{left}},
		core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), Direction: core.Column, AlignItems: core.AlignCenter, Gap: 1}, Children: []core.Node{
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
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Width: core.Pt(250), JustifyContent: core.JustifyEnd},
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
	var play core.Node
	if playing {
		play = icon(lucidePause, size+6, p.onAccent)
	} else {
		play = core.View{Style: core.Style{Margin: core.Edges{Left: 2}}, Children: []core.Node{icon(lucidePlay, size+6, p.onAccent)}}
	}
	return []core.Node{
		a.roundButton(nil, p, size-4, shuffleColor, i18n.T("Shuffle"), lucideShuffle, a.toggleShuffle),
		a.roundButton(nil, p, size, fg, i18n.T("Previous"), lucidePrev, pl.Prev),
		base.Button{Label: i18n.T("Play/Pause"), TestID: "play-pause", OnPress: func(core.PressEvent) { a.toggle() },
			Style: core.Style{Width: core.Pt(size + 20), Height: core.Pt(size + 20), CornerRadius: 999, Background: p.accent,
				Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Hover: &core.Style{Background: blend(p.accent, core.RGB(0xffffff), 0.15)}, Active: &core.Style{Background: blend(p.accent, core.RGB(0x000000), 0.15)}},
			Children: []core.Node{play}},
		a.roundButton(nil, p, size, fg, i18n.T("Next"), lucideNext, pl.Next),
		a.roundButton(nil, p, size-4, repeatColor, i18n.T("Repeat"), repeatIcon, a.cycleRepeat),
	}
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
		Style:      core.Style{Width: core.Pt(92), Height: core.Pt(18)},
		TrackStyle: core.Style{Height: core.Pt(18)},
		IndicatorStyle: core.Style{Height: core.Pt(4), CornerRadius: 2,
			Margin: core.Edges{Top: 7, Left: 0, Right: 0}, Background: p.press},
		RangeStyle: core.Style{Height: core.Pt(4), CornerRadius: 2, Background: p.muted},
		ThumbStyle: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, Background: p.text},
	}
}
