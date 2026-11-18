package app

import (
	"strconv"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/player"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

func speedLabel(speed float64) string { return strconv.FormatFloat(speed, 'f', -1, 64) + "×" }

func (a *app) setPlaybackSpeed(speed float64) {
	a.pl.SetSpeed(speed)
	a.speedRev.Update(func(n int) int { return n + 1 })
	a.saveSession()
}

// Bind tempo independently so lyrics and artwork retain their mounted state.
func (a *app) playbackSpeedButton(id string, color core.Color, height core.Points) core.Node {
	return core.Show{When: func() bool {
		a.playRev.Get()
		return a.pl.Current() != nil && a.pl.CanSetSpeed()
	}, Then: func(cx *core.Scope) core.Node {
		p := a.palette(cx)
		var items []core.Node
		for _, speed := range player.Speeds() {
			label := speedLabel(speed)
			if speed == 1 {
				label += " · " + i18n.T("Normal")
			}
			items = append(items, base.MenuItem{
				TestID: id + "-" + speedLabel(speed), Label: label, Role: core.RoleMenuItemRadio,
				Checked:  func() bool { a.speedRev.Get(); return a.pl.Speed() == speed },
				OnSelect: func() { a.setPlaybackSpeed(speed) },
				Style: core.Style{MinHeight: core.Pt(40), Padding: core.Symmetric(6, 12), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, CornerRadius: 8,
					Hover: &core.Style{Background: p.hover}, Focus: &core.Style{Background: p.hover}},
				Indicator:      icon(lucideCheck, 14, p.text),
				IndicatorStyle: core.Style{Width: core.Pt(14), Height: core.Pt(14), Opacity: core.Alpha(0), Checked: &core.Style{Opacity: core.Alpha(1)}},
				Children:       []core.Node{text(label, 13, core.FontWeightMedium, p.text)},
			})
		}
		return base.Menu{TestID: id, Label: i18n.T("Playback Speed"), Placement: core.PlaceTop, Align: core.AlignEnd, Margin: 8, Offset: 6,
			Style: core.Style{MinWidth: core.Pt(52), Height: core.Pt(height), FlexShrink: 0, Padding: core.Symmetric(0, 8), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				CornerRadius: 10, Hover: &core.Style{Background: core.RGBA(0x888888, 0.12)}},
			Trigger: core.Text{TestID: id + "-value", ContentFunc: func() string { a.speedRev.Get(); a.playRev.Get(); return speedLabel(a.pl.Speed()) },
				Style: core.Style{FontSize: 12, FontWeight: core.FontWeightSemibold, Color: color}},
			PopupStyle: core.Style{MinWidth: core.Pt(170), Direction: core.Column, Padding: core.All(6), Background: p.sidebar, Color: p.text, CornerRadius: 12,
				BorderWidth: 0.5, BorderColor: p.hairline, ShadowColor: p.cardShade, ShadowRadius: 16, ShadowOffset: core.Point{Y: 6}},
			Children: append([]core.Node{
				core.View{Style: core.Style{Padding: core.Symmetric(8, 12), Gap: 4}, Children: []core.Node{
					text(i18n.T("Playback Speed"), 14, core.FontWeightSemibold, p.text),
					text(i18n.T("Keep original pitch"), 12, core.FontWeightNormal, p.muted),
				}},
			}, items...),
		}
	}}
}
