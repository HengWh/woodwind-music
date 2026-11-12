package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/player"
)

// featureCard leads a source's Home with the songs of its first section: the
// first cover over the card's colors, the day and the section's name, its
// first picks and the cover as a sleeve with its record out. A press opens
// all of them.
func (a *app) featureCard(p palette, sec music.Section, pageID string, at int, now time.Time) core.Node {
	return core.Dynamic(func(*core.Scope) core.Node {
		a.artRev.Get()
		return a.featureCardNow(p, sec, pageID, at, now)
	})
}

func (a *app) featureCardNow(p palette, sec music.Section, pageID string, at int, now time.Time) core.Node {
	tracks := sec.Tracks
	if len(tracks) == 0 {
		return core.Group{}
	}
	white := core.RGB(0xffffff)
	inner := a.cx.Env().Window().Width - sidebarWidth - 2*pagePad
	showStack, showPicks, showDay := inner >= 620, inner >= 1040, inner >= 900
	ours, playing := a.dailyPlaying(tracks)

	tint := a.coverTint(tracks[0].Album.ImageURL)
	deep, rich, glow := dailyColors(tint)
	artists := dailyArtists(tracks)
	title := sec.Title

	a.ensureDailyTicker()

	open := func(core.PressEvent) {
		if sec.MorePage != "" {
			a.open(route{page: pageBrowse, id: sec.MorePage})
			return
		}
		a.open(route{page: pageSection, id: pageID, query: strconv.Itoa(at)})
	}

	var children []core.Node
	if cover := tracks[0].Album.ImageURL; cover != "" {
		children = append(children, core.Image{Source: core.Src(cover), ResizeMode: core.Cover, BlurRadius: 34,
			Style: core.Style{Position: core.Absolute, Top: core.Pt(-30), Right: core.Pt(-30), Bottom: core.Pt(-30), Left: core.Pt(-30),
				Opacity: core.Alpha(0.24)}})
	}
	children = append(children,
		core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
			Gradients: []core.Gradient{{Kind: core.GradientLinear, Angle: 90, Stops: []core.ColorStop{
				{Color: deep.Faded(0.85), At: core.Pct(0)},
				{Color: deep.Faded(0), At: core.Pct(60)},
			}}}}},
		core.Canvas{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
			PointerEvents: core.PointerEventsNone}, Label: i18n.T("Recently Played"),
			Draw: func(d *core.Drawing, size core.Size) {
				a.dailyTick.Get()
				a.dailyPlay.Get()
				a.drawDailyBackdrop(d, size, tracks, glow, blend(rich, white, 0.06), showStack, now.Day(), showDay)
			}},
	)
	children = append(children, core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(-40), Bottom: core.Pt(-40),
		Left: core.Pt(-40), Width: core.Pct(40), PointerEvents: core.PointerEventsNone,
		Gradients: []core.Gradient{{Kind: core.GradientLinear, Angle: 100, Stops: []core.ColorStop{
			{Color: white.Faded(0), At: core.Pct(0)},
			{Color: white.Faded(0.13), At: core.Pct(50)},
			{Color: white.Faded(0), At: core.Pct(100)},
		}}},
		Transform: core.Transform{Rotate: 8, TranslateXPercent: -150},
		Transition: core.Transition{Duration: 900 * time.Millisecond, Easing: core.EaseInOut,
			Properties: core.TransitionTransform | core.TransitionOpacity},
		GroupStyle: &core.GroupStyle{Name: "daily", Hover: &core.Style{Transform: core.Transform{Rotate: 8, TranslateXPercent: 440}}},
	}})

	// The words, the first picks and the stack, from the left.
	var picks core.Node = core.Group{}
	if showPicks {
		picks = a.dailyPicks(tracks, ours)
	}
	var stack core.Node = core.Group{}
	if showStack {
		out := float32(0)
		if ours {
			out = 1
		}
		stack = a.dailyStack(tracks, out)
	}
	children = append(children, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 28,
		Height: core.Pct(100), Width: core.Pct(100), Padding: core.Symmetric(0, dailyPadX)}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 6}, Children: []core.Node{
			core.Text{Content: i18n.Today(now), MaxLines: 1, Style: core.Style{FontSize: 12, FontWeight: core.FontWeightSemibold,
				LetterSpacing: 0.3, Color: white.Faded(0.66)}},
			core.Text{Content: title, MaxLines: 2, Style: core.Style{FontSize: 32, FontWeight: core.FontWeight(800),
				LetterSpacing: -0.8, LineHeightMultiple: 1.15, Color: white}},
			core.Text{Content: artists, MaxLines: 1, Style: core.Style{FontSize: 13, Color: white.Faded(0.8)}},
			core.View{Style: core.Style{Direction: core.Row, Gap: 10, Margin: core.Edges{Top: 12}}, Children: []core.Node{
				a.dailyPlayButton(tracks, ours, playing),
			}},
		}},
		picks,
		stack,
	}})

	cardStyle := core.Style{
		Height: core.Pt(dailyHeight), Direction: core.Row, AlignItems: core.AlignCenter,
		CornerRadius: dailyRadius, Overflow: core.Hidden, Background: deep, Group: "daily",
		Gradients: []core.Gradient{{Kind: core.GradientLinear, Angle: 100, Stops: []core.ColorStop{
			{Color: deep, At: core.Pct(10)},
			{Color: rich, At: core.Pct(85)},
			{Color: blend(rich, deep, 0.3), At: core.Pct(100)},
		}}},
		Shadows:     []core.BoxShadow{{Offset: core.Point{Y: 14}, Blur: 34, Spread: -14, Color: p.cardShade}},
		BorderWidth: 1, BorderColor: white.Faded(0.1),
		Transition: core.Transition{Duration: 220 * time.Millisecond, Properties: core.TransitionShadow},
		Hover:      &core.Style{Shadows: []core.BoxShadow{{Offset: core.Point{Y: 22}, Blur: 46, Spread: -14, Color: p.cardShade}}},
	}
	return core.View{Style: cardStyle, Role: core.RoleButton, Label: title, TestID: "daily",
		OnPress: open,
		OnMouseEnter: func(core.MouseEvent) {
			a.dailyHover.Set(true)
			a.ensureDailyTicker()
		},
		OnMouseLeave: func(core.MouseEvent) {
			a.dailyHover.Set(false)
			a.ensureDailyTicker()
		},
		Children: children,
	}
}

// The daily card's measures; the stack of covers is laid out by hand.
const (
	dailyHeight = 212
	dailyRadius = 24
	dailyPadX   = 32
	stackWidth  = 300
	sleeveLeft  = 64
	sleeveSize  = 156
	recordSize  = 146
)

// recordOut is how far right of its sleeve's middle the record's is, as far
// out as out.
func recordOut(out float32) float32 { return 30 + 46*out }

// dailyPlaying reports whether songs are what plays, and whether they play
// now rather than paused.
func (a *app) dailyPlaying(tracks []music.Track) (ours, playing bool) {
	q, cur := a.pl.Queue(), a.pl.Current()
	if cur == nil || len(q) != len(tracks) || len(q) == 0 || q[0].ID != tracks[0].ID || q[len(q)-1].ID != tracks[len(tracks)-1].ID {
		return false, false
	}
	return true, a.pl.Playing()
}

// dailyColors returns the card's colors for the first cover: a deep and a
// rich shade of its hue, a little turned, and a bright one to glow, at depths
// that white reads over whatever the cover. A gray cover, or one still
// loading, takes the app's red.
func dailyColors(tint core.Color) (deep, rich, glow core.Color) {
	h, s := 356.0, 0.74
	if tint != 0 {
		r, g, b, _ := tint.Components()
		th, ts, _ := hsv(r, g, b)
		if ts > 0.15 {
			h, s = th, min(max(ts*1.2, 0.5), 0.78)
		}
	}
	return hsvColor(h, s, 0.24), hsvColor(math.Mod(h+16, 360), s*0.9, 0.5), hsvColor(math.Mod(h+32, 360), s*0.55, 1)
}

// dailyArtists names the first few singers of songs, and how many songs.
func dailyArtists(tracks []music.Track) string {
	var names []string
	seen := map[string]bool{}
	for i := range tracks {
		if len(tracks[i].Artists) == 0 {
			continue
		}
		n := tracks[i].Artists[0].Name
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		names = append(names, n)
		if len(names) == 3 {
			break
		}
	}
	if len(names) == 0 {
		return i18n.Songs(len(tracks))
	}
	return i18n.Tf("%s and more · %s", strings.Join(names, ", "), i18n.Songs(len(tracks)))
}

// dailyPlayButton plays the songs; while they play it says so, its bars
// dancing, and pauses them; paused, it goes on.
func (a *app) dailyPlayButton(tracks []music.Track, ours, playing bool) core.Node {
	ink := core.RGB(0x1d1d1f)
	label := func() string {
		a.dailyPlay.Get()
		hovered := a.dailyPlay.GetUntracked()
		switch {
		case playing && !hovered:
			return i18n.T("Playing")
		case playing:
			return i18n.T("Pause")
		case ours:
			return i18n.T("Resume")
		}
		return i18n.T("Play")
	}
	button := base.Button{LabelFunc: label,
		Style: core.Style{Height: core.Pt(38), MinWidth: core.Pt(112), Padding: core.Symmetric(0, 20), Gap: 8,
			Direction: core.Row, AlignItems: core.AlignCenter, CornerRadius: 999, Color: ink, Background: core.RGB(0xffffff),
			ShadowColor: core.RGBA(0x000000, 0.35), ShadowRadius: 16, ShadowOffset: core.Point{Y: 6},
			Transition: core.Transition{Duration: 120 * time.Millisecond, Properties: core.TransitionBackground},
			Hover:      &core.Style{Background: core.RGB(0xf2f2f4)}},
		OnPress: func(core.PressEvent) {
			if ours {
				a.pl.TogglePlay()
				a.heartbeat()
				return
			}
			a.playTracks(tracks, firstPlayable(tracks))
		},
		Children: []core.Node{
			core.Dynamic(func(*core.Scope) core.Node {
				a.playRev.Get()
				a.dailyPlay.Get()
				hovered := a.dailyPlay.GetUntracked()
				if playing && !hovered {
					return core.Canvas{Style: core.Style{Width: core.Pt(14), Height: core.Pt(13)},
						Draw: func(d *core.Drawing, size core.Size) {
							a.dailyTick.Get()
							a.playRev.Get()
							lv, live := a.pl.Levels()
							drawDailyBars(d, size, &a.dailyBars, lv, live)
						}}
				}
				if playing {
					return icon(lucidePause, 14, ink)
				}
				return icon(lucidePlay, 14, ink)
			}),
			core.Text{ContentFunc: label, Style: core.Style{FontSize: 14, FontWeight: core.FontWeight(650), Color: ink}},
		}}
	return core.View{OnMouseEnter: func(core.MouseEvent) { a.dailyPlay.Set(true) },
		OnMouseLeave: func(core.MouseEvent) { a.dailyPlay.Set(false) }, Children: []core.Node{button}}
}

// dailyPicks lists the first songs, numbered, the one playing marked; each
// plays the songs from it.
func (a *app) dailyPicks(tracks []music.Track, ours bool) core.Node {
	white := core.RGB(0xffffff)
	var rows []core.Node
	for i := 0; i < len(tracks) && i < 3; i++ {
		i, tr := i, tracks[i]
		rows = append(rows, base.Button{Label: tr.Name, OnPress: func(core.PressEvent) { a.playTracks(tracks, i) },
			Style: core.Style{Direction: core.Row, Gap: 12, Padding: core.Symmetric(7, 12), CornerRadius: 12,
				AlignItems: core.AlignCenter, Color: white,
				Transition: core.Transition{Duration: 120 * time.Millisecond, Properties: core.TransitionBackground},
				Hover:      &core.Style{Background: white.Faded(0.1)}},
			Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(18), Direction: core.Row, AlignItems: core.AlignCenter,
					JustifyContent: core.JustifyCenter}, Children: []core.Node{
					core.Dynamic(func(*core.Scope) core.Node {
						a.playRev.Get()
						cur := a.pl.Current()
						if ours && cur != nil && cur.ID == tr.ID {
							return icon(lucidePlay, 11, white)
						}
						return core.Text{Content: fmt.Sprintf("%02d", i+1), Style: core.Style{FontSize: 12,
							FontWeight: core.FontWeightBold, FontVariant: core.FontTabularNums, Color: white.Faded(0.45)}}
					}),
				}},
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 1},
					Children: []core.Node{
						core.Text{Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: 13, FontWeight: core.FontWeightSemibold,
							Color: white}},
						a.artistLine(palette{white: white}, tr, 11.5, white.Faded(0.6)),
					}},
			}})
	}
	return core.View{Style: core.Style{Width: core.Pt(236), FlexShrink: 0, Direction: core.Column, Gap: 2}, Children: rows}
}

// dailyStack is the first cover, as a sleeve with its record out, the two
// covers after it behind; under the pointer the covers fan and the record
// slides out.
func (a *app) dailyStack(tracks []music.Track, out float32) core.Node {
	white := core.RGB(0xffffff)
	var children []core.Node
	for i := min(2, len(tracks)-1); i >= 1; i-- {
		size := core.Points(sleeveSize - 18*i)
		left := core.Points(sleeveLeft - 30*i)
		i := i
		children = append(children, core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(left),
			Top: core.Pt((sleeveSize - size) / 2), Width: core.Pt(size), Height: core.Pt(size), CornerRadius: 10,
			Opacity:    core.Alpha(1 - 0.12*float32(i)),
			Shadows:    []core.BoxShadow{{Offset: core.Point{Y: 10}, Blur: 22, Spread: -6, Color: core.RGBA(0x000000, 0.45)}},
			Transform:  core.Transform{Rotate: -5 * float32(i), TranslateY: 2},
			Transition: core.Transition{Duration: 600 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform},
			GroupStyle: &core.GroupStyle{Name: "daily", Hover: &core.Style{Transform: core.Transform{
				Rotate: -9 * float32(i), TranslateX: core.Points(-10 * float32(i)), TranslateY: -2}}}},
			Children: []core.Node{a.cover(palette{sidebar: white.Faded(0.08), faint: white.Faded(0.3)}, tracks[i].Album.ImageURL, size, 10)}})
	}
	mid := core.Points(sleeveLeft + sleeveSize/2)
	hoverX := core.Points(28)
	if out >= 1 {
		hoverX = 46
	}
	children = append(children,
		core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(mid + core.Points(recordOut(0)) - recordSize/2),
			Top: core.Pt((sleeveSize - recordSize) / 2), Width: core.Pt(recordSize), Height: core.Pt(recordSize),
			Transform:  core.Transform{TranslateX: core.Points(46 * out)},
			Transition: core.Transition{Duration: 700 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform},
			GroupStyle: &core.GroupStyle{Name: "daily", Hover: &core.Style{Transform: core.Transform{TranslateX: hoverX}}}},
			Children: []core.Node{
				core.Canvas{Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)},
					Draw: func(d *core.Drawing, size core.Size) {
						a.dailyTick.Get()
						a.playRev.Get()
						a.drawRecord(d, size, tracks[0].Album.ImageURL, float32(a.dailySpin.angle))
					}},
				core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
					CornerRadius: recordSize / 2, BorderWidth: 1, BorderColor: white.Faded(0.1), PointerEvents: core.PointerEventsNone}},
			}})
	children = append(children,
		core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(sleeveLeft), Top: core.Pt(0),
			Width: core.Pt(sleeveSize), Height: core.Pt(sleeveSize), CornerRadius: 12,
			Shadows:    []core.BoxShadow{{Offset: core.Point{Y: 16}, Blur: 30, Spread: -8, Color: core.RGBA(0x000000, 0.5)}},
			Transform:  core.Transform{},
			Transition: core.Transition{Duration: 600 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform},
			GroupStyle: &core.GroupStyle{Name: "daily", Hover: &core.Style{Transform: core.Transform{Rotate: -2, TranslateY: -3}}}},
			Children: []core.Node{
				a.cover(palette{sidebar: white.Faded(0.08), faint: white.Faded(0.3)}, tracks[0].Album.ImageURL, sleeveSize, 12),
				core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
					CornerRadius: 12, BorderWidth: 1, BorderColor: white.Faded(0.14), PointerEvents: core.PointerEventsNone,
					Gradients: []core.Gradient{{Kind: core.GradientLinear, Angle: 145, Stops: []core.ColorStop{
						{Color: white.Faded(0.22), At: core.Pct(0)},
						{Color: white.Faded(0), At: core.Pct(42)},
						{Color: core.RGBA(0x000000, 0), At: core.Pct(70)},
						{Color: core.RGBA(0x000000, 0.18), At: core.Pct(100)},
					}}}}},
			}})
	return core.View{Style: core.Style{Width: core.Pt(stackWidth), Height: core.Pt(sleeveSize), FlexShrink: 0}, Children: children}
}

// drawDailyBackdrop paints the light behind the daily card: a glow where the
// record turns, and grooves rippling out from it.
func (a *app) drawDailyBackdrop(d *core.Drawing, size core.Size, tracks []music.Track, glow, dayColor core.Color, showStack bool, day int, showDay bool) {
	white := core.RGB(0xffffff)
	hovered := a.dailyHover.GetUntracked()
	ours, _ := a.dailyPlaying(tracks)
	out := float32(0)
	switch {
	case ours:
		out = 1
	case hovered:
		out = 0.6
	}
	cx := float32(size.Width) - dailyPadX - 120
	if showStack {
		cx = float32(size.Width) - dailyPadX - stackWidth + sleeveLeft + sleeveSize/2 + recordOut(out)
	}
	cy := float32(size.Height) / 2
	d.SetFillGradient(core.CanvasRadial(cx, cy, 0, cx, cy, 140, core.GradientStop{Offset: 0, Color: glow.Faded(0.32)},
		core.GradientStop{Offset: 1, Color: glow.Faded(0)}))
	d.BeginPath()
	d.Arc(cx, cy, 140, 0, 2*math.Pi, false)
	d.Fill()
	off := float32(math.Floor(a.dailySpin.ripple))
	const gap = 18
	for rr := float32(recordSize/2+6) + off; rr < float32(size.Width); rr += gap {
		alpha := 0.05 * (1 - (rr-recordSize/2)/(float32(size.Height)*1.6))
		if alpha <= 0.004 {
			break
		}
		alpha *= min(1, (rr-recordSize/2-6)/gap)
		d.BeginPath()
		d.Arc(cx, cy, rr, 0, 2*math.Pi, false)
		d.SetStrokeColor(white.Faded(alpha))
		d.SetLineWidth(1)
		d.Stroke()
	}
	if showDay {
		d.SetTextBaseline(core.BaselineTop)
		d.SetFillColor(dayColor)
		d.FillText(fmt.Sprint(day), dailyPadX+300, 30, core.Font{Size: 240, Weight: core.FontWeight(800)})
	}
}

// drawRecord paints a vinyl record of size, its label the cover at url,
// turned to angle: grooves catching the light, which stays where it is as the
// record turns, round a label that turns with it.
func (a *app) drawRecord(d *core.Drawing, size core.Size, url string, angle float32) {
	white := core.RGB(0xffffff)
	dim := float32(min(size.Width, size.Height))
	cx, cy := float32(size.Width)/2, float32(size.Height)/2
	d.Save()
	d.SetShadow(core.RGBA(0x000000, 0.5), 0, 8, 20)
	d.BeginPath()
	d.Arc(cx, cy, dim/2, 0, 2*math.Pi, false)
	d.SetFillColor(core.RGB(0x0e0e10))
	d.Fill()
	d.Restore()
	for i, rr := 0, dim*0.21; rr < dim*0.48; i, rr = i+1, rr+2.2 {
		alpha := float32(0.035)
		if i%3 == 0 {
			alpha = 0.07
		}
		d.BeginPath()
		d.Arc(cx, cy, rr, 0, 2*math.Pi, false)
		d.SetStrokeColor(white.Faded(alpha))
		d.SetLineWidth(0.7)
		d.Stroke()
	}
	d.SetFillGradient(core.CanvasConic(math.Pi/9, cx, cy,
		core.GradientStop{Offset: 0, Color: white.Faded(0)},
		core.GradientStop{Offset: 0.08, Color: white.Faded(0.13)},
		core.GradientStop{Offset: 0.2, Color: white.Faded(0)},
		core.GradientStop{Offset: 0.5, Color: white.Faded(0)},
		core.GradientStop{Offset: 0.58, Color: white.Faded(0.13)},
		core.GradientStop{Offset: 0.7, Color: white.Faded(0)},
		core.GradientStop{Offset: 1, Color: white.Faded(0)}))
	d.BeginPath()
	d.Arc(cx, cy, dim/2, 0, 2*math.Pi, false)
	d.Fill()
	d.BeginPath()
	d.Arc(cx, cy, dim/2, 0, 2*math.Pi, false)
	d.SetStrokeColor(white.Faded(0.1))
	d.SetLineWidth(1)
	d.Stroke()
	lr := dim * 0.18
	d.BeginPath()
	d.Arc(cx, cy, lr, 0, 2*math.Pi, false)
	d.SetFillColor(core.RGB(0xd8262d))
	d.Fill()
	if url != "" {
		d.Save()
		d.BeginPath()
		d.Arc(cx, cy, lr, 0, 2*math.Pi, false)
		d.Clip()
		d.Translate(cx, cy)
		d.Rotate(angle * math.Pi / 180)
		d.DrawImageIn(core.Src(url), -lr, -lr, 2*lr, 2*lr)
		d.Restore()
	}
	d.BeginPath()
	d.Arc(cx, cy, lr, 0, 2*math.Pi, false)
	d.SetStrokeColor(core.RGBA(0x000000, 0.35))
	d.SetLineWidth(1.5)
	d.Stroke()
	d.BeginPath()
	d.Arc(cx, cy, 2.5, 0, 2*math.Pi, false)
	d.SetFillColor(core.RGB(0x0e0e10))
	d.Fill()
}

// drawDailyBars dances the bars on the daily card's play button.
func drawDailyBars(d *core.Drawing, size core.Size, bars *[4]float32, levels [player.Bands]float32, live bool) {
	const n = 4
	per := player.Bands / n
	sw, sh := float32(size.Width), float32(size.Height)
	gap := sw / n * 0.38
	w := (sw - gap*(n-1)) / n
	for i := range n {
		var sum float32
		for b := i * per; b < (i+1)*per; b++ {
			sum += levels[b]
		}
		v := sum / float32(per)
		v = v * v * 1.15
		if !live {
			v = 0
		}
		bars[i] += (v - bars[i]) * 0.35
		h := max(2.5, min(sh, sh*bars[i]))
		x := float32(i) * (w + gap)
		d.BeginPath()
		d.RoundRect(x, sh-h, w, h, w/2)
		d.SetFillColor(core.RGBA(0xd8262d, 1))
		d.Fill()
	}
}

// spinState turns the daily card's record: its angle, in degrees, how far the
// grooves around it have rippled out, up to a groove, and how fast it turns,
// as a part of a turntable's speed, which eases up and glides to a stop.
type spinState struct {
	angle, ripple, speed float64
	last                 time.Time
}

// step turns the record on to now, its speed easing to to, and reports
// whether it still turns.
func (s *spinState) step(now time.Time, to float64) bool {
	dt := now.Sub(s.last).Seconds()
	if s.last.IsZero() || dt > 0.2 {
		dt = 1.0 / 60
	}
	s.last = now
	s.speed += (to - s.speed) * min(1, 2.2*dt)
	if math.Abs(to-s.speed) < 0.002 {
		s.speed = to
	}
	// 33⅓ turns a minute.
	s.angle = math.Mod(s.angle+s.speed*dt*200, 360)
	s.ripple = math.Mod(s.ripple+s.speed*dt*9/6, 9)
	return s.speed > 0
}

// ensureDailyTicker turns the daily card's record and redraws its grooves
// while it plays or the pointer rests on it, and until they stop.
func (a *app) ensureDailyTicker() {
	if a.dailyTicking || a.cx.Disposed() {
		return
	}
	playing := a.pl.Playing()
	hovered := a.dailyHover.GetUntracked()
	if !playing && !hovered && a.dailySpin.speed < 0.002 {
		return
	}
	a.dailyTicking = true
	var beat func()
	beat = func() {
		if a.cx.Disposed() {
			a.dailyTicking = false
			return
		}
		playing := a.pl.Playing()
		hovered := a.dailyHover.GetUntracked()
		if !playing && !hovered && a.dailySpin.speed < 0.002 {
			a.dailyTicking = false
			a.dailyTick.Update(func(n int) int { return n + 1 })
			return
		}
		to := 0.0
		switch {
		case playing:
			to = 1
		case hovered:
			to = 0.12
		}
		a.dailySpin.step(time.Now(), to)
		a.dailyTick.Update(func(n int) int { return n + 1 })
		a.cx.AfterFunc(16*time.Millisecond, beat)
	}
	a.cx.AfterFunc(16*time.Millisecond, beat)
}

// dayCover heads the page of the feature card's songs as the card is: the
// first cover blurred through its colors, the day's number set large over it.
func (a *app) dayCover(p palette, tracks []music.Track, now time.Time) core.Node {
	return core.Dynamic(func(*core.Scope) core.Node {
		a.artRev.Get()
		return a.dayCoverNow(p, tracks, now)
	})
}

func (a *app) dayCoverNow(p palette, tracks []music.Track, now time.Time) core.Node {
	white := core.RGB(0xffffff)
	var tint core.Color
	var cover string
	if len(tracks) > 0 {
		cover = tracks[0].Album.ImageURL
		tint = a.coverTint(cover)
	}
	deep, rich, _ := dailyColors(tint)
	var children []core.Node
	if cover != "" {
		children = append(children, core.Image{Source: core.Src(cover), ResizeMode: core.Cover, BlurRadius: 24,
			Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
				Opacity: core.Alpha(0.34)}})
	}
	children = append(children, core.Text{Content: fmt.Sprint(now.Day()), MaxLines: 1,
		Style: core.Style{FontSize: 112, FontWeight: core.FontWeight(800), LetterSpacing: -5, LineHeightMultiple: 1, Color: white}})
	return core.View{Style: core.Style{Width: core.Pt(212), Height: core.Pt(212), CornerRadius: 14, Overflow: core.Hidden,
		Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: deep,
		Gradients: []core.Gradient{{Kind: core.GradientLinear, Angle: 135, Stops: []core.ColorStop{
			{Color: deep, At: core.Pct(0)}, {Color: rich, At: core.Pct(100)},
		}}},
		Shadows:     []core.BoxShadow{{Offset: core.Point{Y: 18}, Blur: 40, Spread: -10, Color: p.cardShade}},
		BorderWidth: 1, BorderColor: white.Faded(0.1)}, Children: children}
}

// firstPlayable is the first of tracks that can play.
func firstPlayable(tracks []music.Track) int {
	for i := range tracks {
		if !tracks[i].Unavailable {
			return i
		}
	}
	return 0
}
