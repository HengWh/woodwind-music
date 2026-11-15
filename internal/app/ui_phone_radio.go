package app

import (
	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

func (a *app) phoneRadioPage(cx *core.Scope, p palette, d *extraData, r route) core.Node {
	var body []core.Node
	if d.err != "" {
		body = append(body, a.phoneNotice(p, "source-retry", i18n.T("This station can't play right now. Try again later."), d.err, func() { d.loaded = false; a.loadExtra(r, false); a.bump() }))
	}
	if len(d.stations) == 0 {
		if d.err == "" {
			body = append(body, a.emptyNote(p, i18n.T("No stations yet"), ""))
		}
		return a.pageScroll(p, body...)
	}
	if len(d.stations) > 1 {
		var tabs []core.Node
		for i, station := range d.stations {
			index, st := i, station
			fg, bg := p.muted, core.Color(0)
			if i == d.stationAt {
				fg, bg = p.accent, p.sidebar
			}
			tabs = append(tabs, base.Button{TestID: "station-" + st.ID, Label: st.Name, Selected: func() bool { return index == d.stationAt },
				OnPress: func(core.PressEvent) {
					d.stationAt = index
					src, _, _ := radioParts(a.pl.Radio())
					if src == a.source().Info().ID || a.radioStarting != "" {
						a.playStation(st)
					}
					a.bump()
				},
				Style:    core.Style{MinHeight: core.Pt(48), FlexShrink: 0, Padding: core.Symmetric(8, 14), CornerRadius: 16, Background: bg, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{text(st.Name, phoneFontBody, core.FontWeightMedium, fg)},
			})
		}
		body = append(body, core.Scroll{TestID: "station-switch", Horizontal: true, HideIndicator: true, Style: core.Style{Height: core.Pt(48), FlexShrink: 0, Margin: core.Edges{Bottom: 8}}, Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, Gap: 4}, Children: tabs}}})
	}
	st := d.stations[d.stationAt]
	src, _, _ := radioParts(a.pl.Radio())
	var tr *music.Track
	if src == a.source().Info().ID {
		tr = a.pl.Current()
	}
	safe, size := cx.Env().SafeArea(), cx.Env().Window()
	mini := core.Points(66)
	if tr != nil {
		mini = 0
	}
	channel := core.Points(0)
	if len(d.stations) > 1 {
		channel = 56
	}
	coverSize := min(size.Width-safe.Left-safe.Right-88, 280, max(88, size.Height-safe.Top-safe.Bottom-64-58-mini-channel-184-core.Points(max(0, cx.Env().FontScale()-1))*92))
	white := core.RGB(0xffffff)
	ink := palette{bg: core.RGB(0x292937), text: white, muted: white.Faded(.65), faint: white.Faded(.3), sidebar: white.Faded(.1), hover: white.Faded(.12), press: white.Faded(.18), accent: white, onAccent: p.bg}
	title, by, art := i18n.T("Hear something new"), st.Description, ""
	if by == "" {
		by = i18n.T("Music without end")
	}
	if tr != nil {
		title, by, art = tr.Name, tr.ArtistNames(), tr.Album.ImageURL
	}
	var controls []core.Node
	if tr == nil {
		label := i18n.T("Start Listening")
		if a.radioStarting != "" {
			label = i18n.T("Tuning in…")
		}
		controls = append(controls, base.Button{TestID: "radio-start", Label: label, Disabled: func() bool { return a.radioStarting != "" }, OnPress: func(core.PressEvent) { a.playStation(st) }, Style: core.Style{MinHeight: core.Pt(52), Padding: core.Symmetric(8, 24), CornerRadius: 26, Background: white, Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 10}, Children: []core.Node{icon(lucidePlay, 22, ink.bg), text(label, phoneFontBody, core.FontWeightMedium, ink.bg)}})
	} else {
		like := core.Node(core.View{Style: core.Style{Width: core.Pt(48), Height: core.Pt(48)}})
		if a.can("like") {
			glyph := lucideHeart
			if a.isTrackLiked(*tr) {
				glyph = filledHeart
			}
			like = base.Button{TestID: "radio-like", Label: i18n.T("Like"), Selected: func() bool { return a.isTrackLiked(*tr) }, OnPress: func(core.PressEvent) { a.toggleTrackLike(*tr) }, Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 24, Active: &core.Style{Background: ink.press}}, Children: []core.Node{icon(glyph, 24, white)}}
		}
		play := a.phonePlayButton(ink, "radio-play", true)
		controls = append(controls, like, play, a.phoneIcon(ink, "radio-next", i18n.T("Next"), lucideNext, white, func() { a.cancelFocusResume(); a.pl.Next(); a.heartbeat() }))
	}
	deep, rich := nowPlayingColors(core.RGB(0x62556f))
	if art != "" {
		if tint := a.coverTint(art); tint != 0 {
			deep, rich = nowPlayingColors(tint)
		}
	}
	stage := []core.Node{
		base.Button{TestID: "radio-cover", Label: i18n.T("Now Playing"), Disabled: func() bool { return tr == nil }, OnPress: func(core.PressEvent) { a.openNowPlaying() }, Style: core.Style{Width: core.Pt(coverSize), Height: core.Pt(coverSize), FlexShrink: 0, CornerRadius: 18}, Children: []core.Node{a.cover(ink, art, coverSize, 18)}},
		core.View{Style: core.Style{Width: core.Pct(100), Gap: 5, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.Text{TestID: "radio-title", Content: title, MaxLines: 2, Style: core.Style{Width: core.Pct(100), TextAlign: core.TextAlignCenter, FontSize: phoneFontTitle, FontWeight: core.FontWeightSemibold, Color: white, LineHeightMultiple: 1.2}},
			core.Text{Content: by, MaxLines: 1, Style: core.Style{Width: core.Pct(100), TextAlign: core.TextAlignCenter, FontSize: phoneFontCaption, Color: ink.muted}},
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 24, FlexShrink: 0}, Children: controls},
	}
	if tr != nil && (a.pl.Loading() || a.pl.Err() != "") {
		status := i18n.T("Buffering…")
		if a.pl.Err() != "" {
			status = i18n.T("Playback unavailable")
		}
		stage = append(stage, base.Button{TestID: "radio-status", Label: status, OnPress: func(core.PressEvent) {
			if err := a.pl.Err(); err != "" {
				a.phoneMessage = err
				a.openPhoneSheet("playback-error")
			}
		}, Style: core.Style{MinHeight: core.Pt(48), JustifyContent: core.JustifyCenter}, Children: []core.Node{text(status, phoneFontCaption, core.FontWeightNormal, ink.muted)}})
	}
	body = append(body, core.View{TestID: "radio-card", Style: core.Style{AlignItems: core.AlignCenter, Gap: 16, Padding: core.Symmetric(20, 16), CornerRadius: 20, Gradients: []core.Gradient{core.LinearGradient(160, deep, rich)}}, Children: stage})
	return a.pageScroll(p, body...)
}
