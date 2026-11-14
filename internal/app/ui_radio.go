package app

import (
	"strings"
	"time"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// Keep the source and station with the player's radio, including in sessions.
func radioName(src string, st music.Station) string { return src + "\x00" + st.ID + "\x00" + st.Name }

func radioParts(name string) (src, id, label string) {
	parts := strings.SplitN(name, "\x00", 3)
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2]
	}
	return "", "", ""
}

func (a *app) cancelRadioStart() {
	a.radioRequest++
	a.radioStarting = ""
}

// A source can return a batch of unavailable songs. Ask up to three times,
// as upstream does, before leaving the current queue alone.
func (a *app) radioFeed(name string) func(func([]music.Track)) {
	return func(got func([]music.Track)) {
		id, station, _ := radioParts(name)
		src := a.sourceByID(id)
		if src == nil {
			got(nil)
			return
		}
		go func() {
			var tracks []music.Track
			var err error
			for tries := 0; tries < 3 && len(tracks) == 0; tries++ {
				var batch []music.Track
				batch, err = src.StationNext(station)
				if err != nil {
					break
				}
				for _, tr := range batch {
					if !tr.Unavailable {
						tracks = append(tracks, tr)
					}
				}
			}
			a.post(func() {
				if a.sourceByID(id) != src {
					got(nil)
					return
				}
				if err != nil && (a.radioStarting == name || a.pl.Radio() == name) {
					a.say(err.Error())
				}
				got(tracks)
				a.playRev.Update(func(n int) int { return n + 1 })
			})
		}()
	}
}

func (a *app) playStation(station music.Station) {
	src := a.source()
	account := a.accountState(src)
	if src.Can("signIn") && account.asked && account.account == nil {
		a.signInSource(src)
		return
	}
	name := radioName(src.Info().ID, station)
	if a.pl.Radio() == name {
		a.cancelRadioStart()
		if !a.pl.Playing() {
			a.toggle()
		}
		a.bump()
		return
	}
	if a.radioStarting == name {
		return
	}
	a.cancelFocusResume()
	a.cancelRadioStart()
	request, current := a.radioRequest, a.pl.Current()
	a.radioStarting = name
	a.bump()
	feed := a.radioFeed(name)
	feed(func(tracks []music.Track) {
		if request != a.radioRequest {
			return
		}
		a.radioStarting = ""
		if src != a.source() || a.sourceByID(src.Info().ID) != src || a.pl.Current() != current {
			a.bump()
			return
		}
		if len(tracks) == 0 {
			a.say(i18n.T("This station can't play right now. Try again later."))
		} else {
			a.pl.PlayRadio(name, tracks, feed)
			a.heartbeat()
		}
		a.playRev.Update(func(n int) int { return n + 1 })
		a.bump()
	})
}

func (a *app) exitRadio() {
	_, _, label := radioParts(a.pl.Radio())
	a.cancelRadioStart()
	a.pl.EndRadio()
	a.playRev.Update(func(n int) int { return n + 1 })
	a.saveSession()
	a.say(i18n.Tf("Left %s", label))
}

const maxSwitchedStations = 4

func (a *app) stationsTitle() string {
	for _, nav := range a.source().Info().Navigation {
		if nav.Builtin == "stations" && nav.Title != "" {
			return nav.Title
		}
	}
	return i18n.T("Radio")
}

func (a *app) stationsPage(cx *core.Scope, p palette, r route) core.Node {
	d := a.extra[r]
	if d == nil || d.busy && !d.loaded {
		return a.loadingPage(p)
	}
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.playRev.Get()
		a.artRev.Get()
		few := len(d.stations) > 1 && len(d.stations) <= maxSwitchedStations
		src, playing, _ := radioParts(a.pl.Radio())
		_, starting, _ := radioParts(a.radioStarting)
		for i, st := range d.stations {
			if a.radioStarting != "" && st.ID == starting || a.radioStarting == "" && src == a.source().Info().ID && st.ID == playing {
				d.stationAt = i
			}
		}
		d.stationAt = max(0, min(d.stationAt, len(d.stations)-1))
		heading := []core.Node{core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0)}, Children: []core.Node{
			a.titleBlock(p, a.stationsTitle(), i18n.T("Music without end")),
		}}}
		if few {
			heading = append(heading, a.stationSwitch(p, d))
		}
		direction := core.Row
		if a.phoneLayout(cx) || cx.Env().Window().Width < 1000 {
			direction = core.Column
		}
		body := []core.Node{core.View{Style: core.Style{Direction: direction, AlignItems: core.AlignStretch, Gap: 8}, Children: heading}}
		if d.err != "" {
			body = append(body, text(d.err, 14, core.FontWeightNormal, p.accent),
				a.settingsAction(p, i18n.T("Retry"), "source-retry", d.busy, func() { d.loaded = false; a.loadExtra(r, false); a.bump() }))
		}
		if len(d.stations) == 0 && d.err == "" {
			body = append(body, a.emptyNote(p, i18n.T("No stations yet"), ""))
		}
		if len(d.stations) > 0 {
			body = append(body, a.radioCard(cx, p, d.stations[d.stationAt]))
			if len(d.stations) > maxSwitchedStations {
				body = append(body, core.View{Style: core.Style{Margin: core.Edges{Top: 28, Bottom: 14}}, Children: []core.Node{text(i18n.T("Stations"), 20, core.FontWeightBold, p.text)}},
					a.cardGrid(p, len(d.stations), func(i int) core.Node { return a.stationCard(p, d.stations[i]) }))
			}
		}
		return a.pageScroll(p, body...)
	})
}

func (a *app) stationSwitch(p palette, d *extraData) core.Node {
	var buttons []core.Node
	for i, st := range d.stations {
		bg, fg := core.Color(0), p.muted
		if i == d.stationAt {
			bg, fg = p.bg, p.text
		}
		buttons = append(buttons, base.Button{TestID: "station-" + st.ID, Label: st.Name,
			OnPress: func(core.PressEvent) {
				d.stationAt = i
				src, _, _ := radioParts(a.pl.Radio())
				if src == a.source().Info().ID || a.radioStarting != "" {
					a.playStation(st)
				}
				a.bump()
			}, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Padding: core.Symmetric(8, 12), CornerRadius: 8, Background: bg,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Hover: &core.Style{Background: p.hover}},
			Children: []core.Node{core.Text{Content: st.Name, MaxLines: 1, Style: core.Style{FontSize: 13, FontWeight: core.FontWeightSemibold, Color: fg}}}})
	}
	return core.View{TestID: "station-switch", Style: core.Style{Direction: core.Row, AlignSelf: core.AlignStart, Padding: core.All(3),
		CornerRadius: 11, Background: p.sidebar, Margin: core.Edges{Bottom: 22}, MaxWidth: core.Pct(100)}, Children: buttons}
}

// Match MyGo's 320-point card and 232-point cover, stacking vertically on phones.
func (a *app) radioCard(cx *core.Scope, p palette, first music.Station) core.Node {
	white, dark := core.RGB(0xffffff), core.RGB(0x1d1d1f)
	ink := palette{sidebar: white.Faded(.12), faint: white.Faded(.3), hover: white.Faded(.1), press: white.Faded(.16)}
	src, _, label := radioParts(a.pl.Radio())
	var tr *music.Track
	if src == a.source().Info().ID {
		tr = a.pl.Current()
	}
	if tr == nil {
		label = first.Name
	}
	phone := a.phoneLayout(cx)
	height, coverSize, pad, gap, titleSize := core.Points(320), core.Points(232), core.Points(40), core.Points(40), core.Points(30)
	direction := core.Row
	if phone {
		height, coverSize, pad, gap, titleSize, direction = 0, min(232, cx.Env().Window().Width-120), 24, 26, 26, core.Column
	} else if cx.Env().Window().Width < 1050 {
		coverSize, pad, gap = 200, 28, 24
	}
	children := []core.Node{}
	if tr != nil && tr.Album.ImageURL != "" {
		children = append(children, a.sourceImage(core.Image{Source: imageSource(tr.Album.ImageURL), ResizeMode: core.Cover, BlurRadius: 60, FadeDuration: 300 * time.Millisecond,
			Style: core.Style{Position: core.Absolute, Top: core.Pt(-40), Left: core.Pt(-40), Right: core.Pt(-40), Bottom: core.Pt(-40), Opacity: core.Alpha(.7)}}))
	}
	children = append(children, core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0),
		Gradients: []core.Gradient{core.LinearGradient(90, core.RGBA(0x000000, .35), core.RGBA(0x000000, .1))}, PointerEvents: core.PointerEventsNone}})
	info := []core.Node{core.View{Style: core.Style{Direction: core.Row, Gap: 6, AlignItems: core.AlignCenter}, Children: []core.Node{
		icon(lucide.Radio(), 14, white.Faded(.75)), text(label, 12, core.FontWeightSemibold, white.Faded(.75)),
	}}}
	name := i18n.T("Hear something new")
	if tr != nil {
		name = tr.Name
	}
	info = append(info, core.Text{TestID: "radio-title", Content: name, MaxLines: 2,
		Style: core.Style{FontSize: titleSize, FontWeight: core.FontWeight(800), LetterSpacing: -.6, LineHeightMultiple: 1.2, Color: white}})
	if tr == nil {
		if first.Description != "" {
			info = append(info, core.Text{Content: first.Description, MaxLines: 2, Style: core.Style{FontSize: 13.5, Color: white.Faded(.75)}})
		}
		start := i18n.T("Start Listening")
		if a.radioStarting != "" {
			start = i18n.T("Tuning in…")
		}
		button := a.pillButton(p, transportPlay, start, white, dark, func() { a.playStation(first) }).(base.Button)
		button.TestID, button.Disabled = "radio-start", func() bool { return a.radioStarting != "" }
		info = append(info, core.View{Style: core.Style{Margin: core.Edges{Top: 18}, AlignItems: core.AlignStart}, Children: []core.Node{button}})
	} else {
		status := core.Text{Spans: a.artistSpans(p, *tr, 14, white.Faded(.7)), MaxLines: 1, Style: core.Style{FontSize: 14, Color: white.Faded(.7)}}
		if a.pl.Loading() {
			status.Content, status.Spans = i18n.T("Buffering…"), nil
		} else if a.pl.Err() != "" {
			status.Content, status.Spans = a.pl.Err(), nil
		}
		info = append(info, status)
		glyph := transportPlay
		if a.pl.Playing() {
			glyph = transportPause
		}
		controls := []core.Node{}
		if a.can("like") {
			color, heart := white.Faded(.8), lucideHeart
			if a.isTrackLiked(*tr) {
				color, heart = white, filledHeart
			}
			controls = append(controls, base.Button{TestID: "radio-like", Label: i18n.T("Like"), OnPress: func(core.PressEvent) { a.toggleTrackLike(*tr) },
				Style:    core.Style{Width: core.Pt(44), Height: core.Pt(48), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 24, Hover: &core.Style{Background: ink.hover}},
				Children: []core.Node{icon(heart, 20, color)}})
		}
		controls = append(controls, base.Button{TestID: "radio-play", Label: i18n.T("Play/Pause"), OnPress: func(core.PressEvent) { a.toggle() },
			Style: core.Style{Width: core.Pt(56), Height: core.Pt(56), CornerRadius: 28, Background: white, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				Transition: core.Transition{Duration: 180 * time.Millisecond, Properties: core.TransitionTransform}, Hover: &core.Style{Transform: core.Transform{ScaleX: 1.05, ScaleY: 1.05}}, Active: &core.Style{Transform: core.Transform{ScaleX: .92, ScaleY: .92}}},
			Children: []core.Node{icon(glyph, 22, dark)}},
			base.Button{TestID: "radio-next", Label: i18n.T("Next"), OnPress: func(core.PressEvent) { a.cancelFocusResume(); a.pl.Next(); a.heartbeat() },
				Style:    core.Style{Width: core.Pt(44), Height: core.Pt(48), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 24, Hover: &core.Style{Background: ink.hover}},
				Children: []core.Node{icon(transportNext, 20, white.Faded(.8))}})
		info = append(info, core.View{Style: core.Style{Direction: core.Row, Gap: 14, AlignItems: core.AlignCenter, Margin: core.Edges{Top: 20, Left: -8}}, Children: controls})
	}
	infoStyle := core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 8}
	if phone {
		infoStyle.Width = core.Pct(100)
	}
	contentStyle := core.Style{Direction: direction, AlignItems: core.AlignCenter, Padding: core.Symmetric(36, pad), Gap: gap}
	if !phone {
		contentStyle.Height = core.Pct(100)
	}
	children = append(children, core.View{Style: contentStyle, Children: []core.Node{
		a.radioCovers(ink, tr, coverSize), core.View{Style: infoStyle, Children: info},
	}})
	style := core.Style{CornerRadius: 24, Overflow: core.Hidden, Background: core.RGB(0x232328),
		Gradients: []core.Gradient{core.LinearGradient(135, core.RGB(0x793e43), core.RGB(0x39284f))},
		Shadows:   []core.BoxShadow{{Offset: core.Point{Y: 18}, Blur: 44, Spread: -16, Color: p.cardShade}}}
	if !phone {
		style.Height = core.Pt(height)
	}
	return core.View{TestID: "radio-card", Style: style, Children: children}
}

var filledHeart = transportIcon(`<path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0l-1 1-1-1a5.5 5.5 0 0 0-7.8 7.8l1 1L12 21l7.8-7.6 1-1a5.5 5.5 0 0 0 0-7.8z"/>`)

func (a *app) radioCovers(p palette, tr *music.Track, size core.Points) core.Node {
	var covers []core.Node
	if tr != nil {
		covers = append(covers, core.Dynamic(func(cx *core.Scope) core.Node {
			a.modeRev.Get()
			order, at := a.pl.PlayOrder()
			if at < 0 || at+1 >= len(order) {
				return core.Group{}
			}
			next := a.pl.Queue()[order[at+1]]
			return core.View{TestID: "radio-next-cover", Style: core.Style{Position: core.Absolute, Top: core.Pt(16), Left: core.Pt(36),
				Width: core.Pt(size - 32), Height: core.Pt(size - 32), CornerRadius: 14, Opacity: core.Alpha(.55), Transform: core.Transform{Rotate: 6, TranslateX: 14},
				Shadows: []core.BoxShadow{{Offset: core.Point{Y: 10}, Blur: 24, Spread: -8, Color: core.RGBA(0x000000, .4)}}}, Children: []core.Node{a.cover(p, next.Album.ImageURL, size-32, 14)}}
		}))
	}
	front := core.View{TestID: "radio-cover", Style: core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: 16, Background: p.sidebar,
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Transition: core.Transition{Duration: 600 * time.Millisecond, Properties: core.TransitionTransform},
		Shadows: []core.BoxShadow{{Offset: core.Point{Y: 20}, Blur: 44, Spread: -10, Color: core.RGBA(0x000000, .55)}}}}
	if tr == nil {
		front.Children = []core.Node{icon(lucide.Radio(), 64, core.RGBA(0xffffff, .85))}
	} else {
		front.Children = []core.Node{a.cover(p, tr.Album.ImageURL, size, 16)}
		if !a.pl.Playing() {
			front.Style.Transform = core.Transform{ScaleX: .94, ScaleY: .94}
		}
	}
	covers = append(covers, front)
	return core.View{Style: core.Style{Width: core.Pt(size + 36), Height: core.Pt(size), FlexShrink: 0}, Children: covers}
}

func (a *app) radioChip() core.Node {
	_, _, label := radioParts(a.pl.Radio())
	if label == "" {
		return core.Group{}
	}
	white := core.RGB(0xffffff)
	return base.Button{TestID: "radio-leave", Label: i18n.Tf("Leave %s", label), OnPress: func(core.PressEvent) { a.exitRadio() },
		Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, AlignSelf: core.AlignStart, Gap: 6, Padding: core.Symmetric(6, 10),
			Margin: core.Edges{Bottom: 8}, CornerRadius: 999, Background: white.Faded(.12), Hover: &core.Style{Background: white.Faded(.2)}},
		Children: []core.Node{icon(lucide.Radio(), 12, white.Faded(.8)), text(label, 12, core.FontWeightMedium, white.Faded(.8)), icon(lucideX, 12, white.Faded(.6))}}
}
