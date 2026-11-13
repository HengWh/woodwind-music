package app

import (
	"strconv"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// settingsChoice uses the app's compact field and menu rather than the
// framework's form field. base.Select still owns keyboard and menu behavior.
type settingsChoice[T comparable] struct {
	palette  palette
	Touch    bool
	Options  []ui.Option[T]
	Value    func() T
	OnChange func(T)
	Label    string
	TestID   string
	Style    core.Style
}

func (s settingsChoice[T]) Render(cx *core.Scope) core.Node {
	p := s.palette
	chosen := -1
	for i, o := range s.Options {
		if o.Value == s.Value() {
			chosen = i
			break
		}
	}
	title := ""
	if chosen >= 0 {
		title = s.Options[chosen].Label
	}
	height := core.Points(26)
	if s.Touch {
		height = 48
	}
	return base.Select{Label: s.Label, TestID: s.TestID,
		Items: func() []base.SelectItem {
			items := make([]base.SelectItem, len(s.Options))
			for i, o := range s.Options {
				items[i] = base.SelectItem{Value: strconv.Itoa(i), Title: o.Label}
			}
			return items
		}, Value: func() *string {
			if chosen < 0 {
				return nil
			}
			v := strconv.Itoa(chosen)
			return &v
		}, OnChange: func(v *string) {
			if v != nil {
				if i, err := strconv.Atoi(*v); err == nil && i >= 0 && i < len(s.Options) && s.OnChange != nil {
					s.OnChange(s.Options[i].Value)
				}
			}
		},
		RootStyle: core.Style{FlexShrink: 0}.Merge(s.Style),
		Style: core.Style{Height: core.Pt(height), BorderWidth: 1, BorderColor: p.hairline, CornerRadius: 6, Background: p.hover,
			Hover: &core.Style{Background: p.press}, FocusVisible: &core.Style{OutlineWidth: 3, OutlineColor: ui.Alpha(p.accent, 0.25)}},
		Trigger: core.View{Style: core.Style{FlexGrow: 1, Direction: core.Row, AlignItems: core.AlignCenter,
			Padding: core.Symmetric(0, 9), Gap: 8}, Children: []core.Node{
			core.Text{Content: title, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: 13, Color: p.text}},
			icon(lucideChevronDown, 11, p.muted),
		}},
		PopupStyle: core.Style{Background: p.sidebar, BorderWidth: 1, BorderColor: p.hairline, CornerRadius: 8,
			Padding: core.All(4), ShadowColor: p.cardShade, ShadowRadius: 16, ShadowOffset: core.Point{Y: 6}},
		ListStyle: core.Style{MaxHeight: core.Pt(280)}, Margin: 8,
		ItemStyle: core.Style{MinHeight: core.Pt(height), Padding: core.Symmetric(3, 7), CornerRadius: 5,
			Focus: &core.Style{Background: p.accent, Color: p.onAccent}, Color: p.text},
		RenderItem: func(e base.SelectEntry) core.Node {
			check := core.Color(0)
			if e.Index == chosen {
				check = p.text
			}
			return core.View{TestID: s.TestID + "-" + strconv.Itoa(e.Index),
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 7}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(12)}, Children: []core.Node{
						core.Show{When: func() bool { return check != 0 }, Then: func(*core.Scope) core.Node {
							return core.Icon{SVG: lucideCheck, Style: core.Style{Width: core.Pt(12), Height: core.Pt(12)}}
						}},
					}},
					core.Text{Content: e.Title, MaxLines: 1, Style: core.Style{FontSize: 13}},
				}}
		},
	}
}

// settingsSwitch keeps the same accent, size and contrast in both appearances.
type settingsSwitch struct {
	palette  palette
	Touch    bool
	Checked  func() bool
	OnChange func(bool)
	Label    string
	TestID   string
}

func (s settingsSwitch) Render(cx *core.Scope) core.Node {
	p := s.palette
	minHeight := core.Points(0)
	if s.Touch {
		minHeight = 48
	}
	return base.Switch{Checked: s.Checked, OnChange: s.OnChange, Label: s.Label, TestID: s.TestID,
		Style: core.Style{MinHeight: core.Pt(minHeight), JustifyContent: core.JustifyCenter, FlexShrink: 0, Padding: core.Symmetric(4, 0),
			FocusVisible: &core.Style{OutlineWidth: 3, OutlineColor: ui.Alpha(p.accent, 0.25), CornerRadius: 12}},
		TrackStyle: core.Style{Width: core.Pt(32), Height: core.Pt(18), Padding: core.All(2), CornerRadius: 9,
			Background: p.press, Checked: &core.Style{Background: p.accent},
			Transition: core.Transition{Duration: 160 * time.Millisecond, Properties: core.TransitionBackground}},
		ThumbStyle: core.Style{Width: core.Pt(14), Height: core.Pt(14), CornerRadius: 7, Background: p.white,
			ShadowColor: core.RGBA(0x000000, 0.18), ShadowRadius: 1, ShadowOffset: core.Point{Y: 1},
			Checked:    &core.Style{Transform: core.Transform{TranslateX: 14}},
			Transition: core.Transition{Duration: 160 * time.Millisecond, Properties: core.TransitionTransform}},
	}
}
