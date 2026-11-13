package app

import (
	"fmt"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

// nowPlayingView covers the window with what plays, its lyrics and the
// queue.
func (a *app) nowPlayingView(cx *core.Scope, p palette) core.Node {
	white := core.RGB(0xffffff)
	body := core.Dynamic(func(cx *core.Scope) core.Node {
		a.playRev.Get()
		return a.nowPlayingBody(cx, p, white)
	})
	backdrop := core.Dynamic(func(cx *core.Scope) core.Node {
		a.playRev.Get()
		if t := a.pl.Current(); t != nil && t.Album.ImageURL != "" {
			return core.Image{Source: imageSource(t.Album.ImageURL), ResizeMode: core.Cover, BlurRadius: 90,
				Style: core.Style{Position: core.Absolute, Top: core.Pt(-60), Right: core.Pt(-60), Bottom: core.Pt(-60), Left: core.Pt(-60),
					Opacity: core.Alpha(0.16)}}
		}
		return core.Group{}
	})
	topbar := core.View{Style: core.Style{Height: core.Pt(40), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
		Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, Children: []core.Node{
			base.Button{Label: i18n.T("Close"), TestID: "np-close", OnPress: func(core.PressEvent) { a.closeNowPlaying() },
				Style: core.Style{Width: core.Pt(32), Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter,
					JustifyContent: core.JustifyCenter, CornerRadius: 16, Hover: &core.Style{Background: core.RGBA(0xffffff, 0.1)}},
				Children: []core.Node{icon(lucideChevronDown, 18, core.RGBA(0xffffff, 0.7))}},
		}}
	children := []core.Node{backdrop, topbar, body}
	fade := core.NewAnimatedValue(cx, 0)
	cx.StartAnimation(core.Timing(fade, 1, core.TimingConfig{Duration: 200 * time.Millisecond}), nil)
	return core.View{Style: core.Style{Animated: &core.AnimatedStyle{Opacity: fade}, Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
		ZIndex: 10, Direction: core.Column, Color: white, Background: core.RGB(0x0b0b0d)},
		StyleFunc: func() core.Style {
			a.playRev.Get()
			a.artRev.Get()
			tint := core.RGB(0x787882)
			if t := a.pl.Current(); t != nil {
				if c := a.coverTint(t.Album.ImageURL); c != 0 {
					tint = c
				}
			}
			top, foot := nowPlayingColors(tint)
			return core.Style{Gradients: []core.Gradient{core.LinearGradient(180, top, foot)}}
		}, Children: children}
}

// nowPlayingBody shares a bounded viewport between the cover and the lyrics.
// A zero flex basis is essential: a long lyric list must not size the window.
func (a *app) nowPlayingBody(cx *core.Scope, p palette, white core.Color) core.Node {
	tr := a.pl.Current()
	if tr == nil {
		return core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{text(i18n.T("Not Playing"), 22, core.FontWeightBold, core.RGBA(0xffffff, 0.5))}}
	}
	if a.npPane.GetUntracked() == 3 {
		return a.stageView(cx, p, white)
	}
	size := cx.Env().Window()
	pad := max(core.Points(72), (size.Width-1280)/2)
	gap := core.Points(72)
	coverSize := min(size.Width*0.36, max(140, size.Height-436), 440)
	sideWidth := max(coverSize, 280)
	mobile := a.phoneLayout(cx)
	direction := core.Row
	if mobile {
		pad, gap, direction = 16, 12, core.Column
		coverSize = min(size.Width-64, max(100, size.Height*0.18), 200)
		sideWidth = max(0, size.Width-2*pad)
	}
	coverPal := palette{sidebar: core.RGBA(0xffffff, 0.08), faint: core.RGBA(0xffffff, 0.3), hover: core.RGBA(0xffffff, 0.08)}
	side := core.View{TestID: "np-controls", Style: core.Style{Width: core.Pt(sideWidth), FlexShrink: 0,
		Direction: core.Column, JustifyContent: core.JustifyCenter}, Children: []core.Node{
		core.View{TestID: "np-cover", Style: core.Style{Width: core.Pt(coverSize), Height: core.Pt(coverSize), FlexShrink: 0, CornerRadius: 12,
			ShadowColor: core.RGBA(0x000000, 0.4), ShadowRadius: 22, ShadowOffset: core.Point{Y: 10}},
			Children: []core.Node{a.cover(coverPal, tr.Album.ImageURL, coverSize, 12)}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Margin: core.Edges{Top: 36, Bottom: 18}}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), Direction: core.Column, Gap: 4}, Children: []core.Node{
				core.Text{TestID: "np-title", Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: 22, FontWeight: core.FontWeightBold, LetterSpacing: -0.3, Color: white}},
				core.Text{Spans: a.artistSpans(palette{white: white}, *tr, 15, core.RGBA(0xffffff, 0.6)), MaxLines: 1,
					Style: core.Style{FontSize: 15, Color: core.RGBA(0xffffff, 0.6)}},
			}},
			a.npLikeButton(tr.ID),
			ui.Menu{Trigger: ui.Button{Label: i18n.T("More"), Variant: ui.ButtonGhost, Side: 30},
				Face: icon(lucideMore, 22, core.RGBA(0xffffff, 0.7)), Children: a.trackMenu(*tr, "", -1)},
		}},
		a.progress(palette{accent: white, press: core.RGBA(0xffffff, 0.18)}, true),
		core.View{Style: core.Style{Height: core.Pt(16), Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			core.Text{ContentFunc: func() string { a.tick.Get(); return clock(a.pl.Position()) }, Style: core.Style{FontSize: 11, Color: core.RGBA(0xffffff, 0.5)}},
			core.Text{ContentFunc: func() string {
				a.tick.Get()
				if a.pl.Loading() {
					return i18n.T("Buffering…")
				}
				return "-" + clock(a.pl.Duration()-a.pl.Position())
			}, Style: core.Style{FontSize: 11, Color: core.RGBA(0xffffff, 0.5)}},
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Margin: core.Edges{Top: 14, Bottom: 18}},
			Children: a.transport(palette{text: white, hover: core.RGBA(0xffffff, 0.10), press: core.RGBA(0xffffff, 0.16)},
				core.RGBA(0xffffff, 0.45), p.accent, 28)},
		core.View{TestID: "np-volume", Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 8}, Children: []core.Node{
			icon(lucideVolume, 14, core.RGBA(0xffffff, 0.55)),
			base.Slider{Value: func() base.SliderValue { a.tick.Get(); return base.SingleValue(a.pl.Volume() * 100) },
				Min: func() float64 { return 0 }, Max: func() float64 { return 100 },
				OnChange: func(v base.SliderValue) { a.set.Volume = v.End / 100; a.pl.SetVolume(a.set.Volume); a.set.save() }, Label: i18n.T("Volume"),
				Style: core.Style{Width: core.Pt(180), Height: core.Pt(18)}, TrackStyle: core.Style{Height: core.Pt(18)},
				IndicatorStyle: core.Style{Height: core.Pt(4), CornerRadius: 2, Margin: core.Edges{Top: 7}, Background: core.RGBA(0xffffff, 0.18)},
				RangeStyle:     core.Style{Height: core.Pt(4), CornerRadius: 2, Background: core.RGBA(0xffffff, 0.7)},
				ThumbStyle:     core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, Background: white}},
		}},
	}}
	pane := core.View{TestID: "np-pane", Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinWidth: core.Pt(0), MinHeight: core.Pt(0), Direction: core.Column},
		Children: []core.Node{core.Dynamic(func(cx *core.Scope) core.Node {
			a.npPane.Get()
			switch a.npPane.GetUntracked() {
			case 1:
				return a.queuePane(cx, white)
			case 2:
				return a.wallPane(cx, white)
			default:
				return a.lyricsPane(cx, white)
			}
		})}}
	return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(0), Direction: core.Column}, Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(28), FlexShrink: 0, Padding: core.Symmetric(0, pad)}, Children: []core.Node{a.paneTabs(nil, white)}},
		core.View{TestID: "np-body", Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(0), Direction: direction, Gap: gap,
			Padding: core.Edges{Top: 8, Right: pad, Bottom: 44, Left: pad}}, Children: []core.Node{side, pane}},
	}}
}

// npLikeButton is the heart of Now Playing.
func (a *app) npLikeButton(id string) core.Node {
	color := core.RGBA(0xffffff, 0.7)
	if a.likedSet[id] {
		color = core.RGBA(0xffffff, 0.95)
	}
	return base.Button{Label: i18n.T("Like"), OnPress: func(core.PressEvent) { a.toggleLike(id) },
		Style: core.Style{Width: core.Pt(30), Height: core.Pt(30), Direction: core.Row, AlignItems: core.AlignCenter,
			JustifyContent: core.JustifyCenter, CornerRadius: 15, Hover: &core.Style{Background: core.RGBA(0xffffff, 0.1)}},
		Children: []core.Node{icon(lucideHeart, 20, color)}}
}

// paneTabs are the tabs of Now Playing.
func (a *app) paneTabs(_ *core.Scope, white core.Color) core.Node {
	tabs := []struct {
		label string
		index int
	}{{i18n.T("Lyrics"), 0}, {i18n.T("Stage"), 3}, {i18n.T("Up Next"), 1}, {i18n.T("Wall"), 2}}
	var buttons []core.Node
	for _, t := range tabs {
		index := t.index
		selected := a.npPane.Get() == index
		bg := core.Color(0)
		fg := core.RGBA(0xffffff, 0.55)
		if selected {
			bg, fg = core.RGBA(0xffffff, 0.14), white
		}
		buttons = append(buttons, base.Button{Label: t.label, TestID: fmt.Sprintf("np-tab-%d", index), OnPress: func(core.PressEvent) { a.npPane.Set(index) },
			Style: core.Style{Padding: core.Symmetric(6, 14), CornerRadius: 999, Background: bg,
				Hover: &core.Style{Background: core.RGBA(0xffffff, 0.08)}},
			Children: []core.Node{text(t.label, 13, core.FontWeightSemibold, fg)}})
	}
	return core.View{Style: core.Style{Direction: core.Row, Gap: 6}, Children: buttons}
}

// queuePane lists the whole queue in play order, the track playing marked.
func (a *app) queuePane(cx *core.Scope, white core.Color) core.Node {
	order, at := a.pl.PlayOrder()
	queue := a.pl.Queue()
	list := &core.Ref{}
	last := -2
	cx.Effect(func() {
		a.tick.Get()
		_, now := a.pl.PlayOrder()
		if now != last {
			last = now
			if now >= 0 {
				list.ScrollToIndex(now, true)
			}
		}
	})
	return core.List{Ref: list, Count: func() int { return len(order) }, Key: func(i int) any { return i },
		EstimatedSize: 56, Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Gap: 2,
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			r := row.Index()
			i := order[r]
			tr := queue[i]
			playing := r == at
			item := base.Button{Label: tr.Name, OnPress: func(core.PressEvent) { a.pl.Jump(i) },
				StyleFunc: func() core.Style {
					s := core.Style{Height: core.Pt(54), Padding: core.Symmetric(0, 10), Gap: 12, Direction: core.Row, AlignItems: core.AlignCenter,
						CornerRadius: 10, Hover: &core.Style{Background: core.RGBA(0xffffff, 0.07)}}
					if playing {
						s.Background = core.RGBA(0xffffff, 0.12)
					}
					return s
				}, Children: []core.Node{
					a.cover(palette{faint: core.RGBA(0xffffff, 0.3), hover: core.RGBA(0xffffff, 0.08)}, tr.Album.ImageURL, 40, 6),
					core.View{Style: core.Style{FlexGrow: 1, MinWidth: core.Pt(0), Direction: core.Column, Gap: 2}, Children: []core.Node{
						core.Text{Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: 13.5, FontWeight: core.FontWeightMedium, Color: white}},
						core.Text{Content: tr.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: 12, Color: core.RGBA(0xffffff, 0.55)}},
					}},
					core.Text{Content: clock(tr.Length()), Style: core.Style{FontSize: 12, Color: core.RGBA(0xffffff, 0.4)}},
				}}
			return ui.ContextMenu{Trigger: item, Children: a.trackMenu(tr, "", -1)}
		}}
}

// wallPane is the queue's covers, a wall of what could play next: a press
// plays one.
func (a *app) wallPane(cx *core.Scope, white core.Color) core.Node {
	queue := a.pl.Queue()
	order, at := a.pl.PlayOrder()
	// The queue in the order it plays.
	rows := make([]music.Track, 0, len(order))
	for _, i := range order {
		if i >= 0 && i < len(queue) {
			rows = append(rows, queue[i])
		}
	}
	list := &core.Ref{}
	last := -2
	cx.Effect(func() {
		a.playRev.Get()
		_, now := a.pl.PlayOrder()
		if now != last {
			last = now
			if now >= 0 {
				list.ScrollToIndex(now, true)
			}
		}
	})
	cols := max(2, min(6, int((cx.Env().Window().Width-sidebarWidth)/210)))
	return core.List{Ref: list, Count: func() int { return len(rows) }, Key: func(i int) any { return rows[i].ID },
		NumColumns: cols, EstimatedSize: 190, ColumnGap: 14, Gap: 14,
		Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)},
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			i := row.Index()
			tr := rows[i]
			current := func() bool {
				a.playRev.Get()
				o, now := a.pl.PlayOrder()
				return now >= 0 && now < len(o) && o[now] == queueIndex(queue, tr.ID)
			}
			_ = at
			item := base.Button{Label: tr.Name, TestID: "wall-" + tr.ID, OnPress: func(core.PressEvent) {
				for qi, q := range queue {
					if q.ID == tr.ID {
						a.pl.Jump(qi)
						return
					}
				}
			}, StyleFunc: func() core.Style {
				s := core.Style{Direction: core.Column, Gap: 6, Padding: core.All(6), CornerRadius: 12,
					Hover: &core.Style{Background: core.RGBA(0xffffff, 0.07)}}
				if current() {
					s.Background = core.RGBA(0xffffff, 0.12)
					s.BorderWidth, s.BorderColor = 1.5, core.RGBA(0xffffff, 0.9)
				}
				if tr.Unavailable {
					s.Opacity = core.Alpha(0.4)
				}
				return s
			}, Children: []core.Node{
				func() core.Node {
					pic := a.cover(palette{faint: core.RGBA(0xffffff, 0.3), hover: core.RGBA(0xffffff, 0.08)}, tr.Album.ImageURL, 168, 10)
					return core.View{Style: core.Style{Width: core.Pct(100), AspectRatio: 1}, Children: []core.Node{pic}}
				}(),
				core.Text{Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: 12.5, FontWeight: core.FontWeightMedium, Color: white}},
				core.Text{Content: tr.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: 11.5, Color: core.RGBA(0xffffff, 0.5)}},
			}}
			return ui.ContextMenu{Trigger: item, Children: a.trackMenu(tr, "", -1)}
		}}
}

func queueIndex(queue []music.Track, id string) int {
	for i, t := range queue {
		if t.ID == id {
			return i
		}
	}
	return -1
}
