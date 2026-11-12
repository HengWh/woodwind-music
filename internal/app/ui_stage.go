package app

import (
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/lyrics"
)

// The stage sets a song's lyrics as Folia's visualizers do: Flow, its classic,
// where a line's words spring in as they are sung; Steps, which stacks a line
// in staggered rows; and Tilt, which breaks a line into a few large lines, one
// of them leaning. It follows playback alone, so seeking shows the frame the
// song would.

var stageStyles = []struct{ id, label string }{
	{"", i18n.N("Flow")},
	{"partita", i18n.N("Steps")},
	{"tilt", i18n.N("Tilt")},
}

func stageStyleLabel(id string) string {
	label := stageStyles[0].label
	for _, s := range stageStyles {
		if s.id == id {
			label = s.label
		}
	}
	return i18n.T(label)
}

// ensureStageTicker redraws the stage while it shows and plays. The canvas
// follows the signal; pausing stops the ticker until the next tap.
func (a *app) ensureStageTicker() {
	if a.stageTicking || a.cx.Disposed() || !a.nowPlaying || a.npPane.GetUntracked() != 3 || !a.pl.Playing() {
		return
	}
	a.stageTicking = true
	var beat func()
	beat = func() {
		if a.cx.Disposed() || !a.nowPlaying || a.npPane.GetUntracked() != 3 || !a.pl.Playing() {
			a.stageTicking = false
			a.stageTick.Update(func(n int) int { return n + 1 })
			return
		}
		a.stageTick.Update(func(n int) int { return n + 1 })
		a.cx.AfterFunc(33*time.Millisecond, beat)
	}
	a.cx.AfterFunc(33*time.Millisecond, beat)
}

// stageView is the stage as the whole view: its tabs, the lyrics, and the
// controls along the foot.
func (a *app) stageView(cx *core.Scope, p palette, white core.Color) core.Node {
	lines, loaded := a.pl.Lyrics()
	a.ensureStageTicker()
	header := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
		Padding: core.Edges{Left: 44, Right: 44, Top: 6, Bottom: 10}}, Children: []core.Node{
		a.paneTabs(cx, white),
		core.View{Style: core.Style{FlexGrow: 1}},
		core.Show{When: func() bool { return len(lines) > 0 }, Then: func(*core.Scope) core.Node {
			return a.stageStyleMenu(white)
		}},
	}}

	var content core.Node
	switch {
	case !loaded:
		content = core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{text(i18n.T("Loading…"), 15, core.FontWeightNormal, core.RGBA(0xffffff, 0.4))}}
	case len(lines) == 0:
		content = core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{text(i18n.T("No lyrics. Enjoy the music."), 28, core.FontWeightBold, core.RGBA(0xffffff, 0.4))}}
	default:
		// The canvas paints the stage; the hidden texts beside it are what
		// assistive technology and tests read, a line at a time.
		content = core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Direction: core.Column}, Children: []core.Node{
			core.Canvas{Draw: func(d *core.Drawing, size core.Size) {
				// Subscribe to the tick and the track, and read the live clock.
				a.stageTick.Get()
				a.playRev.Get()
				a.drawStage(d, size, lines, p.accent, white)
			}, Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Width: core.Pct(100)}, Label: i18n.T("Stage")},
			core.Dynamic(func(cx *core.Scope) core.Node {
				a.stageTick.Get()
				a.playRev.Get()
				cur := lyrics.Index(lines, a.pl.Position())
				var texts []core.Node
				if cur >= 0 {
					texts = append(texts, core.Text{Content: lines[cur].Text, Style: core.Style{Opacity: core.Alpha(0)}})
					if lines[cur].Translation != "" {
						texts = append(texts, core.Text{Content: lines[cur].Translation, Style: core.Style{Opacity: core.Alpha(0)}})
					}
				}
				if cur+1 < len(lines) {
					texts = append(texts, core.Text{Content: lines[cur+1].Text, Style: core.Style{Opacity: core.Alpha(0)}})
				}
				return core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
					Direction: core.Column, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Opacity: core.Alpha(0),
					PointerEvents: core.PointerEventsNone}, Children: texts}
			}),
		}}
	}

	return core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Direction: core.Column}, Children: []core.Node{
		header,
		core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0), Direction: core.Column, Padding: core.Edges{Left: 44, Right: 44}},
			Children: []core.Node{content}},
		a.stageBar(cx, p, white),
	}}
}

// stageStyleMenu names the style and offers the others, with sparks on or off.
func (a *app) stageStyleMenu(white core.Color) core.Node {
	label := stageStyleLabel(a.set.StageStyle)
	face := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.Symmetric(5, 8)},
		Children: []core.Node{
			text(label, 13, core.FontWeightSemibold, core.RGBA(0xffffff, 0.65)),
			icon(lucideChevronDown, 12, core.RGBA(0xffffff, 0.45)),
		}}
	var items []core.Node
	for _, s := range stageStyles {
		id := s.id
		items = append(items, ui.MenuItem{Text: i18n.T(s.label), Checked: func() bool { return a.set.StageStyle == id },
			OnSelect: func() {
				a.set.StageStyle = id
				a.set.save()
				a.stageTick.Update(func(n int) int { return n + 1 })
				a.bump()
			}})
	}
	items = append(items, ui.MenuSeparator{}, ui.MenuItem{Text: i18n.T("Sparks"), Checked: func() bool { return a.set.StageSparks },
		OnSelect: func() {
			a.set.StageSparks = !a.set.StageSparks
			a.set.save()
			a.stageTick.Update(func(n int) int { return n + 1 })
			a.bump()
		}})
	return ui.Menu{Trigger: ui.Button{Label: i18n.T("Stage Style"), Variant: ui.ButtonGhost, Side: 30}, Face: face,
		Placement: core.PlaceBottom, Align: core.AlignEnd, Children: items, Label: i18n.T("Stage Style"), TestID: "stage-style"}
}

// stageBar is the slim transport along the foot of the stage.
func (a *app) stageBar(cx *core.Scope, p palette, white core.Color) core.Node {
	tr := a.pl.Current()
	dark := palette{accent: white, press: core.RGBA(0xffffff, 0.18), text: white, muted: core.RGBA(0xffffff, 0.5),
		faint: core.RGBA(0xffffff, 0.35), hover: core.RGBA(0xffffff, 0.1), onAccent: p.onAccent}
	coverPal := palette{sidebar: core.RGBA(0xffffff, 0.08), faint: core.RGBA(0xffffff, 0.3),
		hover: core.RGBA(0xffffff, 0.08)}
	var cover core.Node = core.Group{}
	var title, artist string
	if tr != nil {
		cover = a.cover(coverPal, tr.Album.ImageURL, 44, 8)
		title, artist = tr.Name, tr.ArtistNames()
	}
	return core.View{Style: core.Style{FlexShrink: 0, Direction: core.Column, Gap: 10, Padding: core.Edges{Left: 48, Right: 48, Bottom: 28}},
		Children: []core.Node{
			a.progress(dark, true),
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 14}, Children: []core.Node{
				core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12},
					Children: []core.Node{
						core.View{Style: core.Style{Width: core.Pt(44), Height: core.Pt(44), FlexShrink: 0}, Children: []core.Node{cover}},
						core.View{Style: core.Style{FlexShrink: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 2}, Children: []core.Node{
							core.Text{Content: title, MaxLines: 1, Style: core.Style{FontSize: 14, FontWeight: core.FontWeightBold, Color: white}},
							core.Text{Content: artist, MaxLines: 1, Style: core.Style{FontSize: 12, Color: core.RGBA(0xffffff, 0.55)}},
						}},
					}},
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter},
					Children: a.transport(palette{accent: p.accent, hover: core.RGBA(0xffffff, 0.1), press: core.RGBA(0xffffff, 0.16),
						text: white, muted: core.RGBA(0xffffff, 0.5), onAccent: p.onAccent}, core.RGBA(0xffffff, 0.6), p.accent, 22)},
				core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), Direction: core.Row, JustifyContent: core.JustifyEnd},
					Children: []core.Node{core.Text{ContentFunc: func() string {
						a.tick.Get()
						return clock(a.pl.Position()) + " / " + clock(a.pl.Duration())
					}, Style: core.Style{FontSize: 11.5, Color: core.RGBA(0xffffff, 0.5)}}}},
			}},
		}}
}

// drawStage draws the current line, and the line before it as it goes, in the
// chosen style.
func (a *app) drawStage(d *core.Drawing, size core.Size, lines []lyrics.Line, accent, white core.Color) {
	pos := a.pl.Position()
	cur := lyrics.Index(lines, pos)
	w, h := float32(size.Width), float32(size.Height)
	f := min(64, max(36, w*0.045))
	if cur < 0 {
		a.drawInterlude(d, pos, 0, lines[0].At, w, h, white)
		return
	}
	if cur > 0 {
		if gone := float32((pos - lines[cur].At).Seconds()) / 0.4; gone < 1 {
			a.drawStageLine(d, &lines[cur-1], pos, f, w, h, 1-stageEaseOutCubic(gone), accent, white, true)
		}
	}
	a.drawStageLine(d, &lines[cur], pos, f, w, h, 1, accent, white, false)
	if lines[cur].Translation != "" {
		d.SetTextAlign(core.TextAlignCenter)
		d.SetTextBaseline(core.BaselineMiddle)
		d.SetFillColor(core.RGBA(0xffffff, 0.65))
		d.FillText(lines[cur].Translation, w/2, h-52, core.Font{Size: 16, Weight: core.FontWeightMedium})
	}
	if cur+1 < len(lines) {
		d.SetTextAlign(core.TextAlignCenter)
		d.SetTextBaseline(core.BaselineMiddle)
		d.SetFillColor(core.RGBA(0xffffff, 0.30))
		d.FillText(lines[cur+1].Text, w/2, h-24, core.Font{Size: 14, Weight: core.FontWeightNormal})
	}
}

func (a *app) drawStageLine(d *core.Drawing, l *lyrics.Line, pos time.Duration, f, w, h, fade float32, accent, white core.Color, previous bool) {
	switch a.set.StageStyle {
	case "tilt":
		a.drawTiltLine(d, l, pos, w, h, fade, accent, white)
	case "partita":
		a.drawPartitaLine(d, l, pos, f, w, h, fade, accent, white)
	default:
		a.drawFlowLine(d, l, pos, f, w, h, fade, accent, white, previous)
	}
}

func (a *app) drawInterlude(d *core.Drawing, pos, from, to time.Duration, w, h float32, white core.Color) {
	progress := stageClamp01(float32((pos - from).Seconds() / max(0.1, (to-from).Seconds())))
	breath := 1 + 0.12*float32(math.Sin(float64(time.Now().UnixMilli())/420))
	for i := range 3 {
		lit := stageClamp01(progress*3 - float32(i))
		alpha := 0.2 + 0.7*lit
		r := 8 * breath
		d.SetFillColor(core.RGBA(0xffffff, alpha))
		d.Ellipse(w/2+float32(i-1)*34, h/2, r, r)
		d.Fill()
	}
}

// stageShape is where a word sits and leans, the same for a line each time.
type stageShape struct {
	x, y, rotate, scale, drift float32
	margin                     float32
	last                       bool
}

// stageWordPlace is a word of a laid-out line.
type stageWordPlace struct {
	word       lyrics.Word
	shape      stageShape
	x, y, size float32
}

type stageRowPlace struct {
	words []stageWordPlace
	width float32
}

func stageRandom(seed, offset float64) float32 {
	x := math.Sin(seed+offset) * 10000
	return float32(x - math.Floor(x))
}

func stageSpringStep(t float32) float32 {
	if t <= 0 {
		return 0
	}
	e := float32(math.Exp(-10 * float64(t)))
	return 1 - e*(float32(math.Cos(10*float64(t)))+float32(math.Sin(10*float64(t))))
}

func stageClamp01(v float32) float32    { return min(1, max(0, v)) }
func stageLerp(a, b, t float32) float32 { return a + (b-a)*t }
func stageEaseOutCubic(t float32) float32 {
	t = stageClamp01(t)
	return 1 - (1-t)*(1-t)*(1-t)
}

func stageTextWidth(d *core.Drawing, s string, f float32, weight core.FontWeight) float32 {
	if s == "" {
		return 0
	}
	return d.MeasureText(s, core.Font{Size: core.Points(f), Weight: weight})
}

// stageShapes lays out the words of a line at font size f.
func stageShapes(d *core.Drawing, l *lyrics.Line, f float32) []stageShape {
	seed := float64(l.At / time.Millisecond)
	shapes := make([]stageShape, len(l.Words))
	widths := make([]float32, len(l.Words))
	for i, word := range l.Words {
		widths[i] = stageTextWidth(d, word.Text, f, core.FontWeightBold)
	}
	for i := range l.Words {
		ws := seed + float64(i)
		shapes[i] = stageShape{
			x:      (stageRandom(ws, 1) - 0.5) * 36,
			y:      (stageRandom(ws, 2) - 0.5) * 36,
			rotate: (stageRandom(ws, 3) - 0.5) * 10,
			scale:  1.1 + stageRandom(ws, 4)*0.2,
			drift:  (stageRandom(ws, 5) - 0.5) * 10,
		}
	}
	for i := range shapes {
		grow := widths[i] * (shapes[i].scale*1.4 - 1) / 2
		next, toward := float32(0), float32(0)
		if i+1 < len(shapes) {
			next = widths[i+1] * (shapes[i+1].scale*1.4 - 1) / 2
			toward = shapes[i].x - shapes[i+1].x
		}
		shapes[i].margin = max(2*f*0.08, grow+next+toward+f*0.08)
	}
	if len(shapes) > 0 {
		shapes[len(shapes)-1].last = true
	}
	return shapes
}

func layoutFlow(d *core.Drawing, l *lyrics.Line, f, maxW float32) []stageRowPlace {
	shapes := stageShapes(d, l, f)
	var rows []stageRowPlace
	var row stageRowPlace
	for i, word := range l.Words {
		width := stageTextWidth(d, word.Text, f, core.FontWeightBold)
		margin := float32(0)
		if i < len(shapes) {
			margin = shapes[i].margin
		}
		gap := float32(0)
		if len(row.words) > 0 {
			gap = margin
		}
		if len(row.words) > 0 && row.width+gap+width > maxW {
			rows = append(rows, row)
			row = stageRowPlace{}
			gap = 0
		}
		x := row.width + gap
		row.words = append(row.words, stageWordPlace{word: word, shape: shapes[i], x: x, y: 0, size: width})
		row.width += gap + width
	}
	if len(row.words) > 0 {
		rows = append(rows, row)
	}
	// A line leans to one side or sits in the middle, a little up or down.
	seed := float64(l.At / time.Millisecond)
	justify := int(seed) % 3
	for i := range rows {
		switch justify {
		case 0:
		case 1:
			shift := (maxW - rows[i].width) / 2
			for j := range rows[i].words {
				rows[i].words[j].x += shift
			}
		default:
			shift := maxW - rows[i].width
			for j := range rows[i].words {
				rows[i].words[j].x += shift
			}
		}
	}
	return rows
}

func (a *app) drawFlowLine(d *core.Drawing, l *lyrics.Line, pos time.Duration, f, w, h, fade float32, accent, white core.Color, previous bool) {
	maxW := w * 0.80
	rows := layoutFlow(d, l, f, maxW)
	if len(rows) == 0 {
		return
	}
	seed := float64(l.At / time.Millisecond)
	lift := []float32{-0.14, 0, 0.12}[int(seed*2)%3] * h
	lineH := f * 1.38
	blockH := float32(len(rows)) * lineH
	originX := (w - maxW) / 2
	originY := h/2 - blockH/2 + lineH/2 + lift
	if previous {
		originY -= 8
	}
	for ri, row := range rows {
		for _, place := range row.words {
			a.drawFlowWord(d, place, pos, f, originX, originY+float32(ri)*lineH, fade, accent, white)
		}
	}
}

func (a *app) drawFlowWord(d *core.Drawing, place stageWordPlace, pos time.Duration, f, originX, originY, fade float32, accent, white core.Color) {
	word, shape := place.word, place.shape
	const lookahead = 80 * time.Millisecond
	since := float32((pos - (word.Start - lookahead)).Seconds())
	if since <= 0 {
		return
	}
	end := max(word.End, word.Start+150*time.Millisecond)
	after := float32((pos - end).Seconds())
	singing := float32(max(0.08, (end - word.Start).Seconds()))
	x := stageLerp(shape.x+float32(math.Sin(float64(shape.y)))*100, shape.x, stageSpringStep(since))
	y := stageLerp(shape.y+float32(math.Cos(float64(shape.x)))*50, shape.y, stageSpringStep(since))
	scale := stageLerp(0.5, shape.scale*1.4, stageSpringStep(since))
	rotate := stageLerp(shape.rotate+20, shape.rotate, stageSpringStep(since))
	alpha := stageClamp01(since / 0.1)
	colorAlpha := stageLerp(0.5, 1, stageClamp01(since/singing))
	glow := stageClamp01(since / max(0.08, 0.3*singing))
	if after > 0 {
		q := stageEaseOutCubic(after / 0.5)
		scale = stageLerp(shape.scale*1.4, shape.scale, q)
		alpha = stageLerp(1, 0.82, q)
		rotate = shape.rotate + shape.drift*stageClamp01(after/5)
		colorAlpha = stageLerp(1, 0.5, stageEaseOutCubic(after/0.8))
		glow = 1 - stageClamp01(after/0.9)
	}
	cx := originX + place.x + place.size/2 + x
	cy := originY + y
	font := core.Font{Size: core.Points(f), Weight: core.FontWeightBold}
	d.Save()
	d.SetTextAlign(core.TextAlignCenter)
	d.SetTextBaseline(core.BaselineMiddle)
	if glow > 0.01 {
		d.SetAlpha(0.85 * glow * fade * alpha)
		d.SetFillColor(accent)
		d.SetShadow(core.RGBA(0xffffff, 0.35*glow*fade), 0, 0, f*0.18)
		d.FillText(word.Text, cx, cy, font)
		d.SetShadow(core.RGBA(0x000000, 0), 0, 0, 0)
	}
	d.SetAlpha(alpha * fade)
	d.SetFillColor(core.RGBA(0xffffff, colorAlpha))
	d.Translate(cx, cy)
	d.Rotate(rotate * math.Pi / 180)
	d.Scale(scale, scale)
	d.FillText(word.Text, 0, 0, font)
	d.Restore()
	if a.set.StageSparks {
		stageSparks(d, cx, cy, place.size, f, since, float64(word.Start/time.Millisecond), shape.last, accent, fade*alpha)
	}
}

// stageSparks throws sparks off a word as it lands.
func stageSparks(d *core.Drawing, cx, cy, w, h, since float32, seed float64, last bool, accent core.Color, fade float32) {
	const sparkTime = 0.6
	if since < 0 || since >= sparkTime {
		return
	}
	t := since / sparkTime
	n, reach := 9, max(w, h)*0.75
	if last {
		n, reach = 16, reach*1.6
	}
	out := 1 - (1-t)*(1-t)*(1-t)
	fade2 := (1 - t) * (1 - t) * fade
	d.Save()
	d.SetLineWidth(1.4)
	d.SetAlpha(0.9 * fade2)
	d.SetStrokeColor(accent)
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * (float64(i) + float64(stageRandom(seed, float64(i)))*0.7) / float64(n)
		dx, dy := float32(math.Cos(angle)), float32(math.Sin(angle))
		edge := w/2*0.7*stageAbs(dx) + h/2*0.7*stageAbs(dy)
		head := edge + reach*(0.55+0.45*stageRandom(seed, float64(i)+50))*out
		tail := max(edge, head-reach*0.3*(1-t))
		d.BeginPath()
		d.MoveTo(cx+dx*tail, cy+dy*tail)
		d.LineTo(cx+dx*head, cy+dy*head)
		d.Stroke()
	}
	d.Restore()
}

func stageAbs(v float32) float32 { return max(v, -v) }

// stageUnits splits words into the units Steps and Tilt break a line between.
func stageUnits(words []lyrics.Word) [][2]int {
	var units [][2]int
	phrase := func(from, to int) {
		n := (to - from + 3) / 4
		if to-from <= 6 {
			n = 1
		}
		for k := 0; k < n; k++ {
			units = append(units, [2]int{from + (to-from)*k/n, from + (to-from)*(k+1)/n})
		}
	}
	from := 0
	for i, word := range words {
		if i == from && len(units) > 0 && strings.TrimFunc(word.Text, stageIsMark) == "" {
			units[len(units)-1][1] = i + 1
			from = i + 1
			continue
		}
		last, _ := utf8.DecodeLastRuneInString(word.Text)
		if i+1 == len(words) || stageIsMark(last) && !strings.ContainsRune("'’-", last) {
			phrase(from, i+1)
			from = i + 1
		}
	}
	return units
}

func stageIsMark(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) }

func stageWordsLength(words []lyrics.Word) int {
	n := 0
	for _, word := range words {
		n += utf8.RuneCountInString(strings.TrimSpace(word.Text))
	}
	return n
}

// stagePart is a part of a line on Tilt.
type stagePart struct {
	from, to   int
	lean, tail bool
}

func stageSplitParts(words []lyrics.Word, units [][2]int, n int) []stagePart {
	total := float32(stageWordsLength(words))
	before := make([]float32, len(units)+1)
	for i, u := range units {
		before[i+1] = before[i] + float32(stageWordsLength(words[u[0]:u[1]]))
	}
	parts := make([]stagePart, 0, n)
	from := 0
	for k := 1; k < n; k++ {
		at, best := -1, float32(0)
		for cut := from + 1; cut <= len(units)-(n-k); cut++ {
			dist := float32(math.Abs(float64(before[cut] - total*float32(k)/float32(n))))
			if last, _ := utf8.DecodeLastRuneInString(strings.TrimSpace(words[units[cut-1][1]-1].Text)); unicode.IsPunct(last) {
				dist -= total * 0.15
			}
			if at < 0 || dist < best {
				at, best = cut, dist
			}
		}
		parts = append(parts, stagePart{from: units[from][0], to: units[at-1][1]})
		from = at
	}
	return append(parts, stagePart{from: units[from][0], to: units[len(units)-1][1]})
}

func stageTiltParts(l *lyrics.Line) []stagePart {
	units := stageUnits(l.Words)
	if len(units) == 0 {
		return nil
	}
	seed := float64(l.At/time.Millisecond) / 1000
	chars := stageWordsLength(l.Words)
	score := math.Log(float64(chars)+4) / math.Log(24) * float64(stageRandom(seed, 1)*0.6+0.7) * 0.75
	n := 4
	for i, at := range []float64{0.45, 1.05, 1.7} {
		if score < at {
			n = i + 1
			break
		}
	}
	n = min(n, len(units))
	parts := stageSplitParts(l.Words, units, n)
	for i := range parts {
		if stageRandom(seed, 100+float64(i)) < 0.35 {
			parts[i].lean = true
		}
	}
	if k := len(parts); k > 1 && !parts[k-1].lean {
		last, before := stageWordsLength(l.Words[parts[k-1].from:parts[k-1].to]), stageWordsLength(l.Words[parts[k-2].from:parts[k-2].to])
		parts[k-1].tail = last <= 2 && last*2 <= before
	}
	return parts
}

func (a *app) drawTiltLine(d *core.Drawing, l *lyrics.Line, pos time.Duration, w, h, fade float32, accent, white core.Color) {
	parts := stageTiltParts(l)
	if len(parts) == 0 {
		return
	}
	size := min(w*0.06875, 90)
	avail := w * 0.85
	widthOf := func(part stagePart) float32 {
		text := ""
		for _, word := range l.Words[part.from:part.to] {
			text += word.Text
		}
		text = strings.TrimSpace(text)
		spacing := float32(0.08)
		if part.lean {
			spacing = 0.15
		}
		return stageTextWidth(d, text, size, core.FontWeightNormal) + float32(utf8.RuneCountInString(text))*size*spacing
	}
	widest := float32(0)
	for _, part := range parts {
		widest = max(widest, widthOf(part))
	}
	if widest > avail {
		size *= max(0.5, avail/widest)
	}
	lineH := size * 1.35
	blockH := float32(len(parts))*lineH + float32(len(parts)-1)*14
	y := h/2 - blockH/2 + lineH/2
	for pi, part := range parts {
		a.drawTiltPart(d, l, part, pos, size, w, y+float32(pi)*(lineH+14), fade, accent, white, pi == len(parts)-1)
	}
}

func (a *app) drawTiltPart(d *core.Drawing, l *lyrics.Line, part stagePart, pos time.Duration, size, w, y, fade float32, accent, white core.Color, last bool) {
	words := l.Words[part.from:part.to]
	if len(words) == 0 {
		return
	}
	since := float32((pos - (words[0].Start - 250*time.Millisecond)).Seconds())
	in := stageEaseOutCubic(since / 0.55)
	if in <= 0 {
		return
	}
	weight, spacing, gap := core.FontWeightNormal, float32(0.08), float32(0.25)
	color := white
	if part.lean {
		weight, spacing, gap = core.FontWeightLight, 0.15, 0.35
		color = accent
	}
	rowFont := core.Font{Size: core.Points(size), Weight: weight}
	text := ""
	for _, word := range words {
		text += word.Text
	}
	text = strings.TrimSpace(text)
	rowW := stageTextWidth(d, text, size, weight) + float32(utf8.RuneCountInString(text))*size*spacing
	if part.tail {
		size *= 1.18
		rowFont.Size = core.Points(size)
		rowW *= 1.18
	}
	x := (w - rowW) / 2
	rise := 20 * (1 - in)
	d.Save()
	d.SetTextBaseline(core.BaselineMiddle)
	d.SetTextAlign(core.TextAlignLeft)
	k := 0
	for wi, word := range words {
		text := strings.TrimRightFunc(word.Text, unicode.IsSpace)
		n := utf8.RuneCountInString(text)
		each := (word.End - word.Start) / time.Duration(max(n, 1))
		wStart := x
		i := 0
		for _, r := range text {
			if unicode.IsSpace(r) {
				x += size * gap
				i++
				continue
			}
			start := word.Start + time.Duration(i)*each
			dur := float32(min(max(each.Seconds(), 0.2), 0.9))
			t := float32((pos - start).Seconds())
			swell := float32(0)
			switch {
			case t < 0:
			case t <= dur:
				swell = float32(math.Sin(float64(t / dur * math.Pi)))
			default:
				swell = 0.25 * stageClamp01((t-dur)/(dur*1.2))
			}
			pulse := float32(0.15)
			if part.lean {
				pulse = 0.18
			}
			scale := 1 + swell*pulse
			shown := stageEaseOutCubic((since - float32(k)*0.04) / 0.5)
			alpha := in * shown * fade
			wRune := stageTextWidth(d, string(r), size, weight)
			cx := x + wRune/2
			cy := y + rise
			if part.lean {
				if k%2 == 0 {
					cy -= size / 6 * (2 - in)
				} else {
					cy += size / 6 * (2 - in)
				}
			}
			d.Save()
			d.SetAlpha(alpha)
			d.SetFillColor(color)
			d.Translate(cx, cy)
			d.Scale(scale, scale)
			font := rowFont
			font.Italic = part.lean
			d.FillText(string(r), -wRune/2, 0, font)
			d.Restore()
			x += wRune + size*spacing
			i++
			k++
		}
		if a.set.StageSparks {
			t := float32((pos - words[wi].Start).Seconds())
			stageSparks(d, (wStart+x)/2, y+rise, x-wStart, size, t, float64(words[wi].Start/time.Millisecond), last && wi == len(words)-1, accent, fade*in)
		}
		if text != word.Text && wi < len(words)-1 {
			x += size * gap
		}
	}
}

// drawPartitaLine stacks a line in a few staggered rows, each with a guide.
func (a *app) drawPartitaLine(d *core.Drawing, l *lyrics.Line, pos time.Duration, f, w, h, fade float32, accent, white core.Color) {
	units := stageUnits(l.Words)
	if len(units) == 0 {
		return
	}
	count := min(len(units), max(1, int(h*0.65/100)))
	rows := make([][2]int, 0, count)
	left, unit := len(units), 0
	for i := 0; i < count; i++ {
		n := left
		if i < count-1 {
			avg := float32(left) / float32(count-i)
			n = min(int(math.Ceil(float64(avg*1.5))), max(1, int(math.Round(float64(avg)))))
		}
		n = max(1, min(n, left-(count-i-1)))
		rows = append(rows, [2]int{unit, unit + n})
		unit += n
		left -= n
	}
	if stageWordsLength(l.Words) > 40 {
		f *= 0.8
	}
	f *= 0.8
	grow := 1 + 0.05*(1-fade)
	lineH := f * 1.5
	totalH := float32(len(rows)) * lineH
	seed := float64(l.At/time.Millisecond) / 1000
	for ri, row := range rows {
		fromWord, toWord := units[row[0]][0], units[row[1]-1][1]
		words := l.Words[fromWord:toWord]
		if len(words) == 0 {
			continue
		}
		text := ""
		for _, word := range words {
			text += word.Text
		}
		rowW := stageTextWidth(d, text, f, core.FontWeightBold)
		stagger := 20 + stageRandom(seed, float64(ri)+1)*80
		x := (w-rowW)/2 + stagger
		if ri%2 == 0 {
			x = (w-rowW)/2 - stagger
		}
		since := float32((pos - (words[0].Start - 80*time.Millisecond)).Seconds())
		e := stageSpringStep(since)
		side := float32(40)
		if ri%2 == 0 {
			side = -40
		}
		x += side * (1 - e)
		y := h/2 - totalH/2 + lineH/2 + float32(ri)*lineH - 10*(1-fade)
		// The guide lights while the row is sung.
		end := float32((pos - max(words[len(words)-1].End, words[len(words)-1].Start+150*time.Millisecond)).Seconds())
		guide := stageEaseOutCubic(since / 0.4)
		if guide > 0 {
			gx := x
			if ri%2 == 0 {
				gx = x + rowW + 8
			} else {
				gx = x - 8
			}
			alpha := float32(0.14)
			if end > 0 {
				alpha = 0.22
			}
			d.SetStrokeColor(core.RGBA(0xffffff, alpha*grow*fade*guide))
			d.SetLineWidth(1)
			d.BeginPath()
			d.MoveTo(gx, y+lineH/2)
			d.LineTo(gx, y+lineH/2-32*guide)
			if ri%2 == 0 {
				d.MoveTo(gx-8, y+lineH/2-16)
				d.LineTo(gx-8+(rowW+36)*guide, y+lineH/2-16)
			} else {
				d.MoveTo(gx+8, y+lineH/2-16)
				d.LineTo(gx+8-(rowW+36)*guide, y+lineH/2-16)
			}
			d.Stroke()
		}
		for wi, word := range words {
			shape := stageShape{x: (stageRandom(seed+float64(wi), 1) - 0.5) * 12,
				y:      (stageRandom(seed+float64(wi), 2) - 0.5) * 12,
				rotate: (stageRandom(seed+float64(wi), 3) - 0.5) * 6,
				scale:  0.8 + stageRandom(seed+float64(wi), 4)*0.9,
				drift:  (stageRandom(seed+float64(wi), 8) - 0.5) * 20}
			wd := stageTextWidth(d, word.Text, f, core.FontWeightBold)
			place := stageWordPlace{word: word, shape: shape, x: x, y: 0, size: wd}
			a.drawFlowWord(d, place, pos, f, 0, y, fade*grow, accent, white)
			x += wd + f*0.08
		}
	}
}
