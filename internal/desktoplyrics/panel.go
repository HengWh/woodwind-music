package desktoplyrics

import (
	"errors"
	"math"
	"slices"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

const panelWidth, panelHeight core.Points = 800, 154

type panelWindow struct {
	panel        *desktop.Window
	cx           *core.Scope
	revision     *core.Signal[uint64]
	hover        *core.Signal[bool]
	frame        Frame
	events       []Event
	lastPosition core.Point
	positioned   bool
}

// Open uses NSGo's window host, which records surfaces without opening any
// native windows in headless tests. It belongs to the app scope, not its view.
func Open(cx *core.Scope, initial Frame) (Window, error) {
	if cx.Env().OS() != core.OSMacOS && cx.Env().OS() != core.OSWindows {
		return nil, errors.New("desktop lyrics unavailable on this platform")
	}
	w := &panelWindow{frame: initial}
	w.frame.Words = slices.Clone(initial.Words)
	target := initialFrame(initial, cx.Env().Displays())
	w.panel = desktop.OpenPanel(cx, desktop.PanelOptions{
		Width: target.Width, Height: target.Height, Transparent: true,
		NoActivate: true, PointerEvents: pointerMode(initial.Locked),
	}, w.view)
	if w.panel == nil {
		return nil, errors.New("desktop lyrics require a desktop window host")
	}
	w.panel.OnClosing(cx, func(e *desktop.WindowClosing) {
		e.Cancel()
		w.events = append(w.events, Event{Action: Hide})
	})
	// Show and place in one commit. Show's default placement must precede the
	// explicit frame, as panels otherwise return to their platform's default spot.
	w.panel.Show()
	desktop.SetWindowLevel(w.cx, desktop.WindowLevelFloating)
	desktop.SetWindowFrame(w.cx, target)
	return w, nil
}

func pointerMode(locked bool) desktop.WindowPointerEvents {
	if locked {
		return desktop.WindowPointerNone
	}
	return desktop.WindowPointerAuto
}

func (w *panelWindow) Update(f Frame) {
	if w.panel.Closed() {
		return
	}
	if f.Locked != w.frame.Locked {
		desktop.SetWindowPointerEvents(w.cx, pointerMode(f.Locked))
		if f.Locked {
			w.hover.Set(false)
		}
	}
	old := w.frame
	w.frame = f
	w.frame.Words = slices.Clone(f.Words)
	if old.FontSize != f.FontSize || old.Color != f.Color || old.Locked != f.Locked || old.Playing != f.Playing || old.Secondary != f.Secondary || !slices.Equal(old.Words, f.Words) {
		w.revision.Set(w.revision.GetUntracked() + 1)
	}
}

func (w *panelWindow) Poll() []Event {
	if w.panel.Closed() {
		return nil
	}
	f := desktop.Frame(w.cx)
	if f.Move && f.Width > 0 && f.Height > 0 {
		p := core.Point{X: f.X, Y: f.Y}
		if !w.positioned || p != w.lastPosition {
			w.events = append(w.events, Event{Moved: true, X: int(math.Round(float64(p.X))), Y: int(math.Round(float64(p.Y)))})
			w.lastPosition, w.positioned = p, true
		}
	}
	events := w.events
	w.events = nil
	return events
}

func (w *panelWindow) Close() { w.panel.Close(); w.events = nil }

func (w *panelWindow) view(cx *core.Scope) core.Node {
	w.cx = cx
	w.revision = core.NewSignal(cx, uint64(0))
	w.hover = core.NewSignal(cx, false)
	visible := func() bool { w.revision.Get(); return w.hover.Get() && !w.frame.Locked }
	return core.View{TestID: "desktop-lyrics-panel",
		Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn},
		OnMouseEnter: func(core.MouseEvent) {
			if !w.frame.Locked {
				w.hover.Set(true)
			}
		},
		OnMouseLeave: func(core.MouseEvent) { w.hover.Set(false) },
		StyleFunc: func() core.Style {
			bg := core.Transparent
			if visible() || !desktop.Transparent(cx) {
				bg = core.RGBA(0x101010, .8)
			}
			return core.Style{Width: core.Pct(100), Height: core.Pct(100), Background: bg}
		}, Children: []core.Node{
			core.Canvas{TestID: "desktop-lyrics-text", LabelFunc: func() string {
				w.revision.Get()
				text := ""
				for _, word := range w.frame.Words {
					text += word.Text
				}
				return text + "\n" + w.frame.Secondary
			}, Style: core.Style{Position: core.Absolute, Top: core.Pt(40), Left: core.Pt(0), Width: core.Pct(100), Height: core.Pt(110)},
				Draw: func(d *core.Drawing, size core.Size) { w.revision.Get(); drawLyrics(d, size, w.frame) }},
			core.Show{When: visible, Then: func(cx *core.Scope) core.Node {
				return core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Width: core.Pct(100), Height: core.Pt(34), Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
					w.button("previous", i18n.T("Previous"), lucide.SkipBack(), Previous),
					base.Button{TestID: "desktop-lyrics-play", LabelFunc: func() string {
						w.revision.Get()
						if w.frame.Playing {
							return i18n.T("Pause")
						}
						return i18n.T("Play")
					},
						OnPress: func(core.PressEvent) { w.events = append(w.events, Event{Action: TogglePlay}) },
						Style:   controlStyle(), Children: []core.Node{core.Icon{SVGFunc: func() *core.SVG {
							w.revision.Get()
							if w.frame.Playing {
								return lucide.Pause()
							}
							return lucide.Play()
						}, Style: core.Style{Width: core.Pt(18), Height: core.Pt(18), Color: core.RGB(0xffffff)}}}},
					w.button("next", i18n.T("Next"), lucide.SkipForward(), Next),
					w.button("lock", i18n.T("Lock Desktop Lyrics"), lucide.Lock(), Lock),
					w.button("close", i18n.T("Close"), lucide.X(), Hide),
				}}
			}},
		}}
}

func controlStyle() core.Style {
	return core.Style{Width: core.Pt(50), Height: core.Pt(34), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 6, Hover: &core.Style{Background: core.RGBA(0xffffff, .15)}}
}

func (w *panelWindow) button(id, label string, svg *core.SVG, action Action) core.Node {
	return base.Button{TestID: "desktop-lyrics-" + id, Label: label, Style: controlStyle(),
		OnPress:  func(core.PressEvent) { w.events = append(w.events, Event{Action: action}) },
		Children: []core.Node{core.Icon{SVG: svg, Style: core.Style{Width: core.Pt(18), Height: core.Pt(18), Color: core.RGB(0xffffff)}}}}
}

// initialFrame keeps the whole panel reachable after monitor changes. Windows
// used to save physical pixels; NSGo's display origins use the smallest scale,
// while coordinates inside a display use that display's own scale.
func initialFrame(f Frame, displays []core.DisplayInfo) desktop.WindowFrame {
	target := desktop.WindowFrame{Move: true, Width: panelWidth, Height: panelHeight, X: core.Points(f.X), Y: core.Points(f.Y)}
	if len(displays) == 0 {
		return target
	}
	chosen := displays[0]
	for _, d := range displays {
		if d.Main {
			chosen = d
			break
		}
	}
	if f.Positioned {
		if f.LegacyPixels {
			least := float32(math.Inf(1))
			for _, d := range displays {
				least = min(least, max(d.Scale, 1))
			}
			for _, d := range displays {
				scale := core.Points(max(d.Scale, 1))
				origin := core.Point{X: d.Frame.Origin.X * core.Points(least), Y: d.Frame.Origin.Y * core.Points(least)}
				pixels := core.Bounds{Origin: origin, Size: core.Size{Width: d.Frame.Size.Width * scale, Height: d.Frame.Size.Height * scale}}
				if pixels.Contains(core.Point{X: target.X, Y: target.Y}) {
					target.X = d.Frame.Origin.X + (target.X-origin.X)/scale
					target.Y = d.Frame.Origin.Y + (target.Y-origin.Y)/scale
					break
				}
			}
		}
		bounds := core.Bounds{Origin: core.Point{X: target.X, Y: target.Y}, Size: core.Size{Width: target.Width, Height: target.Height}}
		for _, d := range displays {
			area := d.VisibleFrame
			if bounds.Origin.X < area.Origin.X+area.Size.Width && bounds.Origin.X+bounds.Size.Width > area.Origin.X && bounds.Origin.Y < area.Origin.Y+area.Size.Height && bounds.Origin.Y+bounds.Size.Height > area.Origin.Y {
				chosen = d
				break
			}
		}
	}
	area := chosen.VisibleFrame
	if area.Size.Width <= 0 || area.Size.Height <= 0 {
		area = chosen.Frame
	}
	target.Width, target.Height = min(target.Width, area.Size.Width), min(target.Height, area.Size.Height)
	if !f.Positioned {
		target.X = area.Origin.X + (area.Size.Width-target.Width)/2
		target.Y = area.Origin.Y + area.Size.Height - target.Height - 40
	}
	target.X = max(area.Origin.X, min(target.X, area.Origin.X+area.Size.Width-target.Width))
	target.Y = max(area.Origin.Y, min(target.Y, area.Origin.Y+area.Size.Height-target.Height))
	return target
}
