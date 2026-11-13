package app

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/player"
)

// The cover and lyrics occupy the same stage; controls stay within thumb reach.
func (a *app) phoneNowPlaying(cx *core.Scope, p palette) core.Node {
	lyrics := core.NewSignal(cx, false)
	white := core.RGB(0xffffff)
	ink := palette{bg: core.RGB(0x202024), text: white, muted: core.RGBA(0xffffff, 0.6), faint: core.RGBA(0xffffff, 0.3),
		sidebar: core.RGBA(0xffffff, 0.1), hover: core.RGBA(0xffffff, 0.1), press: core.RGBA(0xffffff, 0.16), accent: white, onAccent: p.bg}
	fade, slide := core.NewAnimatedValue(cx, 1), core.NewAnimatedValue(cx, 0)
	if !cx.Env().ReduceMotion() {
		fade.Set(0)
		slide.Set(48)
		cx.StartAnimation(core.Parallel(core.Timing(fade, 1, core.TimingConfig{Duration: 220 * time.Millisecond}),
			core.Timing(slide, 0, core.TimingConfig{Duration: 260 * time.Millisecond})), nil)
	}
	return core.View{TestID: "phone-now-playing", Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0),
		ZIndex: 10, Direction: core.Column, Background: ink.bg, PaddingEnv: core.EnvAll(core.EnvSafeArea),
		Animated: &core.AnimatedStyle{Opacity: fade, TranslateY: slide}}, StyleFunc: func() core.Style {
		a.playRev.Get()
		a.artRev.Get()
		tint := core.RGB(0x525363)
		if tr := a.pl.Current(); tr != nil {
			if c := a.coverTint(tr.Album.ImageURL); c != 0 {
				tint = c
			}
		}
		top, bottom := nowPlayingColors(tint)
		return core.Style{Gradients: []core.Gradient{core.LinearGradient(165, top, bottom)}}
	}, Children: []core.Node{
		core.View{TestID: "phone-player-header", PanActivation: core.PanActivation{ActiveOffsetY: 12, FailOffsetX: 24}, OnPan: func(e core.PanEvent) {
			if e.Phase == core.GestureEnded && (e.Translation.Y > 70 || e.Velocity.Y > 800) {
				a.closeNowPlaying()
			}
		}, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(56), FlexShrink: 0, Padding: core.Symmetric(0, 12)}, Children: []core.Node{
			a.phoneIcon(ink, "np-close", i18n.T("Close"), lucideChevronDown, white, a.closeNowPlaying),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), AlignItems: core.AlignCenter}, Children: []core.Node{
				text(i18n.T("Now Playing"), 13, core.FontWeightMedium, ink.muted),
			}},
			a.phoneIcon(ink, "np-more", i18n.T("More"), lucideMore, white, func() {
				if tr := a.pl.Current(); tr != nil {
					a.openPhoneTrack(*tr, "", -1)
				}
			}),
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			a.playRev.Get()
			tr := a.pl.Current()
			if tr == nil {
				return a.emptyNote(ink, i18n.T("Not Playing"), i18n.T("Choose a song to start listening"))
			}
			safe, size := cx.Env().SafeArea(), cx.Env().Window()
			coverSize := min(size.Width-safe.Left-safe.Right-64, max(100, size.Height-safe.Top-safe.Bottom-370-core.Points(max(0, cx.Env().FontScale()-1))*100), 340)
			return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), Padding: core.Edges{Left: 24, Right: 24, Bottom: 8}, Direction: core.Column}, Children: []core.Node{
				core.View{TestID: "phone-player-stage", Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(0), Overflow: core.Hidden}, Children: []core.Node{
					core.Show{When: lyrics.Get, Then: func(cx *core.Scope) core.Node { return a.lyricsPane(cx, white) }, Else: func(cx *core.Scope) core.Node {
						return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
							PanActivation: core.PanActivation{ActiveOffsetX: 24, FailOffsetY: 16}, OnPan: func(e core.PanEvent) {
								if e.Phase == core.GestureEnded && (e.Translation.X > 60 || e.Translation.X < -60) {
									lyrics.Set(true)
								}
							}, Children: []core.Node{
								base.Button{TestID: "np-cover", Label: i18n.T("Lyrics"), OnPress: func(core.PressEvent) { lyrics.Set(true) },
									Style: core.Style{Width: core.Pt(coverSize), Height: core.Pt(coverSize), CornerRadius: 20,
										ShadowColor: core.RGBA(0x000000, 0.35), ShadowRadius: 30, ShadowOffset: core.Point{Y: 16}}, Children: []core.Node{a.cover(ink, tr.Album.ImageURL, coverSize, 20)}},
							}}
					}},
				}},
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, FlexShrink: 0, Padding: core.Edges{Top: 12, Bottom: 8}}, Children: []core.Node{
					core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 6}, Children: []core.Node{
						core.Text{TestID: "np-title", Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: phoneFontTitle, FontWeight: core.FontWeightSemibold, Color: white}},
						core.Text{Content: tr.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: phoneFontTranslation, Color: ink.muted}},
					}},
					a.phoneIcon(ink, "np-like", i18n.T("Like"), lucideHeart, func() core.Color {
						a.rev.Get()
						if a.likedSet[tr.ID] {
							return p.accent
						}
						return white
					}(), func() { a.toggleLike(tr.ID) }),
				}},
				a.phoneProgress(ink),
				core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween, FlexShrink: 0}, Children: []core.Node{
					core.Text{ContentFunc: func() string { a.tick.Get(); return clock(a.pl.Position()) }, Style: core.Style{FontSize: 11, Color: ink.muted}},
					core.Text{ContentFunc: func() string { a.tick.Get(); return clock(a.pl.Duration()) }, Style: core.Style{FontSize: 11, Color: ink.muted}},
				}},
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, FlexShrink: 0, Padding: core.Edges{Top: 12, Bottom: 12}}, Children: []core.Node{
					a.phoneIcon(ink, "np-shuffle", i18n.T("Shuffle"), lucideShuffle, func() core.Color {
						if a.pl.Shuffle() {
							return white
						}
						return ink.faint
					}(), a.toggleShuffle),
					a.phoneIcon(ink, "np-previous", i18n.T("Previous"), lucidePrev, white, a.pl.Prev),
					a.phonePlayButton(ink, "np-play-pause", true),
					a.phoneIcon(ink, "np-next", i18n.T("Next"), lucideNext, white, a.pl.Next),
					a.phoneIcon(ink, "np-repeat", i18n.T("Repeat"), func() *core.SVG {
						if a.pl.Repeat() == player.RepeatOne {
							return lucideRepeatOne
						}
						return lucideRepeat
					}(),
						func() core.Color {
							if a.pl.Repeat() != player.RepeatOff {
								return white
							}
							return ink.faint
						}(), a.cycleRepeat),
				}},
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, FlexShrink: 0}, Children: []core.Node{
					base.Button{TestID: "phone-toggle-lyrics", Label: i18n.T("Lyrics"), OnPress: func(core.PressEvent) { lyrics.Update(func(v bool) bool { return !v }) },
						Style: core.Style{Height: core.Pt(48), MinWidth: core.Pt(80), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{icon(lucideMic, 21, ink.muted),
							core.Text{ContentFunc: func() string {
								if lyrics.Get() {
									return i18n.T("Cover")
								}
								return i18n.T("Lyrics")
							}, Style: core.Style{FontSize: 13, Color: ink.muted}}}},
					core.Text{TestID: "np-status", ContentFunc: func() string {
						a.tick.Get()
						if e := a.pl.Err(); e != "" {
							return e
						}
						if a.pl.Loading() {
							return i18n.T("Buffering…")
						}
						return ""
					},
						MaxLines: 1, Style: core.Style{FlexShrink: 1, MinWidth: core.Pt(0), FontSize: 11, Color: ink.muted}},
					a.phoneIcon(ink, "np-queue", i18n.T("Up Next"), lucideList, white, func() { a.openPhoneSheet("queue") }),
				}},
			}}
		}),
	}}
}

func (a *app) phoneProgress(p palette) core.Node {
	// A large invisible track makes a thin progress line easy to drag.
	return base.Slider{TestID: "progress", Label: i18n.T("Progress"),
		Value: func() base.SliderValue { a.tick.Get(); return base.SingleValue(a.pl.Position().Seconds()) },
		Min:   func() float64 { return 0 }, Max: func() float64 { a.tick.Get(); return max(1, a.pl.Duration().Seconds()) },
		OnChangeEnd: func(v base.SliderValue) { a.pl.Seek(time.Duration(v.End * float64(time.Second))) },
		ValueText:   func(v float64) string { return clock(time.Duration(v * float64(time.Second))) },
		Style:       core.Style{Height: core.Pt(36), FlexShrink: 0}, TrackStyle: core.Style{Height: core.Pt(36)},
		IndicatorStyle: core.Style{Height: core.Pt(3), Margin: core.Edges{Top: 16}, CornerRadius: 2, Background: p.faint},
		RangeStyle:     core.Style{Height: core.Pt(3), CornerRadius: 2, Background: p.text},
		ThumbStyle:     core.Style{Width: core.Pt(9), Height: core.Pt(9), CornerRadius: 5, Background: p.text},
	}
}
