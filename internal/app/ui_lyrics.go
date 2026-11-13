package app

import (
	"fmt"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/lyrics"
)

// lyricsPane keeps the sung line a third of the way down. Browsing the lyrics
// temporarily suspends following; seeking a line returns to following at once.
func (a *app) lyricsPane(cx *core.Scope, white core.Color) core.Node {
	lines, loaded := a.pl.Lyrics()
	if !loaded {
		var rows []core.Node
		for i, width := range []float32{76, 92, 62, 84, 70, 88} {
			rows = append(rows, core.View{Style: core.Style{Width: core.Pct(width), Height: core.Pt(24), CornerRadius: 7,
				Background: core.RGBA(0xffffff, 0.10), Animation: core.KeyframeAnimation{Keyframes: core.KeyframesPulse, Delay: time.Duration(i) * 90 * time.Millisecond}}})
		}
		return core.View{Label: i18n.T("Loading…"), TestID: "lyrics-loading", Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0),
			MinHeight: core.Pt(0), Gap: 30, Padding: core.Edges{Top: 24, Left: 12, Right: 12}}, Children: rows}
	}
	if len(lines) == 0 {
		return core.View{TestID: "lyrics-empty", Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(0),
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{text(i18n.T("No lyrics. Enjoy the music."), a.headingSize(24, phoneFontSection), a.headingWeight(), core.RGBA(0xffffff, 0.4))}}
	}
	list := new(core.Ref)
	frame := core.NewSignal(cx, 0)
	height := core.NewSignal(cx, max(core.Points(0), cx.Env().Window().Height-120))
	current := core.NewMemo(cx, func() int {
		a.tick.Get()
		frame.Get()
		return lyrics.Index(lines, a.pl.Position())
	})
	// Only this pane asks for word frames; the rest of the application retains
	// its slower playback clock. Pausing or disposing the pane stops the loop.
	running := false
	var beat func()
	beat = func() {
		frame.Update(func(n int) int { return n + 1 })
		if a.pl.Playing() {
			cx.AfterFunc(40*time.Millisecond, beat)
		} else {
			running = false
		}
	}
	cx.Effect(func() {
		a.tick.Get()
		if !running && a.pl.Playing() {
			running = true
			cx.AfterFunc(40*time.Millisecond, beat)
		}
	})
	last := -2
	browsing := core.NewSignal(cx, false)
	var release func() bool
	resume := func() {
		if release != nil {
			release()
			release = nil
		}
		last = -2
		browsing.Set(false)
		frame.Update(func(n int) int { return n + 1 })
	}
	startBrowse := func(core.ScrollEvent) {
		if release != nil {
			release()
			release = nil
		}
		browsing.Set(true)
	}
	endBrowse := func(core.ScrollEvent) {
		if release != nil {
			release()
		}
		release = cx.AfterFunc(4*time.Second, resume)
	}
	cx.Effect(func() {
		cur := current.Get()
		frame.Get()
		if browsing.Get() {
			return
		}
		if _, ready := list.Bounds(); !ready || cur == last {
			return
		}
		animated := last >= 0 && !cx.Env().ReduceMotion()
		last = cur
		if cur < 0 {
			list.ScrollTo(core.Point{}, animated)
		} else {
			list.ScrollToIndexAt(cur, 0.30, 0, animated)
		}
	})
	mask := core.LinearGradient(180, white, white)
	mask.Stops = []core.ColorStop{{Color: core.Transparent, At: core.Pct(0)}, {Color: white, At: core.Pt(18)},
		{Color: white, At: core.Pct(90)}, {Color: core.Transparent, At: core.Pct(100)}}
	return core.List{Ref: list, TestID: "lyrics-list", Count: func() int { return len(lines) }, Key: func(i int) any { return i },
		EstimatedSize: 70, Gap: 6, HideIndicator: true,
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(0), Mask: &mask},
		OnLayout: func(b core.Bounds) {
			height.Set(b.Size.Height)
			last = -2
			frame.Update(func(n int) int { return n + 1 })
		},
		OnScrollDragStart: startBrowse, OnScrollDragEnd: endBrowse,
		OnMomentumStart: startBrowse, OnMomentumEnd: endBrowse,
		Header: core.View{Style: core.Style{Height: core.Pt(18)}},
		Footer: core.View{StyleFunc: func() core.Style { return core.Style{Height: core.Pt(height.Get() * 0.70)} }},
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			// A virtualized cell can move to another line. Rebuild its bindings
			// with that line, rather than retaining the first index it displayed.
			return core.Dynamic(func(cx *core.Scope) core.Node {
				i := row.Index()
				return a.lyricLine(cx, lines[i], i, current, frame, func() {
					a.pl.Seek(lines[i].At)
					resume()
					a.heartbeat()
				})
			})
		}}
}

func (a *app) lyricLine(cx *core.Scope, line lyrics.Line, index int, current *core.Memo[int], frame *core.Signal[int], seek func()) core.Node {
	fontSize, translationSize := core.Points(30), core.Points(17)
	if a.phoneLayout(cx) {
		fontSize, translationSize = phoneFontLyrics, phoneFontTranslation
	}
	weight, translationWeight := core.FontWeight(800), core.FontWeightSemibold
	padding := core.Symmetric(10, 12)
	lineHeight, spacing, minHeight := float32(1.25), core.Points(-0.4), core.Points(0)
	if a.phoneLayout(cx) {
		weight, translationWeight = core.FontWeightSemibold, core.FontWeightNormal
		lineHeight, spacing, minHeight = 1.35, 0, 48
		padding = core.Symmetric(8, 8)
	}
	hover := core.NewSignal(cx, false)
	active := core.NewMemo(cx, func() bool { return current.Get() == index })
	scale := core.NewAnimatedValue(cx, 0.94)
	cx.Effect(func() {
		to := 0.94
		if active.Get() {
			to = 1
		}
		if cx.Env().ReduceMotion() {
			scale.Set(to)
		} else {
			cx.StartAnimation(core.Spring(scale, to, core.SpringConfig{Stiffness: 160, Damping: 22, Mass: 1}), nil)
		}
	})
	return core.View{OnMouseEnter: func(core.MouseEvent) { hover.Set(true) }, OnMouseLeave: func(core.MouseEvent) { hover.Set(false) }, Children: []core.Node{
		base.Button{TestID: fmt.Sprintf("lyric-line-%d", index), Label: line.Text, OnPress: func(core.PressEvent) { seek() },
			Style: core.Style{MinHeight: core.Pt(minHeight), Padding: padding, CornerRadius: 12, Direction: core.Column, AlignItems: core.AlignStart,
				TransformOrigin: core.ObjectPosition{X: core.PositionAxis{Offset: core.Pt(0)}, Y: core.PositionAxis{Offset: core.Pct(50)}},
				Animated:        &core.AnimatedStyle{Scale: scale}, Hover: &core.Style{Background: core.RGBA(0xffffff, 0.06)},
				FocusVisible: &core.Style{OutlineWidth: 2, OutlineColor: core.RGBA(0xffffff, 0.4)}},
			Children: []core.Node{core.View{StyleFunc: func() core.Style {
				alpha := float32(0.28)
				if active.Get() {
					alpha = 1
				} else if hover.Get() {
					alpha = 0.55
				}
				duration := 380 * time.Millisecond
				if cx.Env().ReduceMotion() {
					duration = 0
				}
				return core.Style{Direction: core.Column, AlignItems: core.AlignStart, Gap: 6, Opacity: core.Alpha(alpha),
					Transition: core.Transition{Duration: duration, Properties: core.TransitionOpacity}}
			}, Children: []core.Node{
				core.Text{TestID: fmt.Sprintf("lyric-text-%d", index), SpansFunc: func() []core.Span {
					if !active.Get() || len(line.Words) == 0 {
						return []core.Span{{Content: line.Text}}
					}
					frame.Get()
					a.tick.Get()
					pos := a.pl.Position()
					spans := make([]core.Span, len(line.Words))
					for i, w := range line.Words {
						lit := min(max(float64(pos-w.Start)/float64(max(50*time.Millisecond, w.End-w.Start)), 0), 1)
						spans[i] = core.Span{Content: w.Text, Style: core.Style{Color: core.RGBA(0xffffff, float32(0.38+0.62*lit))}}
					}
					return spans
				}, Style: core.Style{FontSize: fontSize, FontWeight: weight, LineHeightMultiple: lineHeight, LetterSpacing: spacing, Color: core.RGB(0xffffff)}},
				core.Show{When: func() bool { return line.Translation != "" }, Then: func(*core.Scope) core.Node {
					return core.Text{Content: line.Translation, Style: core.Style{FontSize: translationSize, FontWeight: translationWeight,
						Color: core.RGBA(0xffffff, 0.7), LineHeightMultiple: 1.4}}
				}},
			}}}},
	}}
}
