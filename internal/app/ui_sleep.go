package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HengWh/woodwind-music/internal/i18n"
)

func (a *app) sleepCountdown() string {
	a.sleepRev.Get()
	a.tick.Get()
	d := a.pl.SleepRemaining()
	if d <= 0 {
		return ""
	}
	return clock(((d + time.Second - 1) / time.Second) * time.Second)
}

func (a *app) sleepButton(p palette, id string, size core.Points, countdown bool) core.Node {
	children := []core.Node{core.Icon{SVG: lucideTimer, Style: core.Style{Width: core.Pt(17), Height: core.Pt(17)}, StyleFunc: func() core.Style {
		a.sleepRev.Get()
		if a.pl.SleepRemaining() > 0 {
			return core.Style{Color: p.accent}
		}
		return core.Style{Color: p.muted}
	}}}
	if countdown {
		children = append(children, core.Text{ContentFunc: a.sleepCountdown, Style: core.Style{FontSize: 11, Color: p.muted}})
	}
	return base.Button{TestID: id, LabelFunc: func() string {
		if remaining := a.sleepCountdown(); remaining != "" {
			return i18n.Tf("Sleep timer · %s", remaining)
		}
		return i18n.T("Sleep Timer")
	}, OnPress: func(core.PressEvent) {
		a.sleepMinutes = ""
		a.sleepError.Set("")
		a.sleepOpen.Set(true)
	}, Style: core.Style{MinWidth: core.Pt(size), Height: core.Pt(size), FlexShrink: 0, Padding: core.Symmetric(0, 6),
		Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 4,
		CornerRadius: 8, Hover: &core.Style{Background: p.hover}, Active: &core.Style{Background: p.press}},
		Children: children}
}

func (a *app) startSleep(minutes int) {
	a.pl.SleepAfter(time.Duration(minutes) * time.Minute)
	a.sleepOpen.Set(false)
	a.cx.DismissKeyboard()
}

func (a *app) startCustomSleep() {
	minutes, err := strconv.Atoi(strings.TrimSpace(a.sleepMinutes))
	if err != nil || minutes < 1 || minutes > 1440 {
		a.sleepError.Set(i18n.T("Enter 1–1440 whole minutes"))
		return
	}
	a.startSleep(minutes)
}

func (a *app) sleepDialog(cx *core.Scope, p palette) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.sleepRev.Get()
		var choices []core.Node
		for _, row := range [][]int{{15, 30, 45}, {60, 90, 120}} {
			var buttons []core.Node
			for _, minutes := range row {
				label := i18n.Tf("%d min", minutes)
				buttons = append(buttons, base.Button{TestID: fmt.Sprintf("sleep-%d", minutes), Label: label,
					OnPress: func(core.PressEvent) { a.startSleep(minutes) },
					Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(44), CornerRadius: 10,
						Background: p.hover, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
					Children: []core.Node{text(label, 14, core.FontWeightMedium, p.text)}})
			}
			choices = append(choices, core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: buttons})
		}
		children := []core.Node{
			core.Text{TestID: "sleep-status", ContentFunc: func() string {
				if remaining := a.sleepCountdown(); remaining != "" {
					return i18n.Tf("Playback pauses in %s", remaining)
				}
				return i18n.T("Timer is off")
			}, Style: core.Style{FontSize: 14, Color: p.muted}},
			core.View{Style: core.Style{Gap: 8}, Children: choices},
			base.Input{TestID: "sleep-minutes", Label: i18n.T("Custom minutes"), Placeholder: i18n.T("Custom minutes (1–1440)"),
				Value: func() string { return a.sleepMinutes }, OnChange: func(v string) { a.sleepMinutes = v; a.sleepError.Set("") },
				OnSubmit: func(string) { a.startCustomSleep() },
				Style: core.Style{Height: core.Pt(36), Padding: core.Symmetric(0, 10), CornerRadius: 8,
					Background: p.hover, Color: p.text, PlaceholderColor: p.faint, FontSize: 14}},
			core.Text{TestID: "sleep-error", ContentFunc: a.sleepError.Get, Style: core.Style{FontSize: 12, Color: p.accent}},
		}
		footer := []core.Node{ui.Button{Text: i18n.T("Close"), Variant: ui.ButtonSecondary, TestID: "sleep-close", OnPress: func() { a.sleepOpen.Set(false) }}}
		if a.pl.SleepRemaining() > 0 {
			footer = append(footer, ui.Button{Text: i18n.T("Cancel Timer"), Variant: ui.ButtonSecondary, TestID: "sleep-cancel", OnPress: func() {
				a.pl.SleepAfter(0)
				a.sleepOpen.Set(false)
			}})
		}
		footer = append(footer, ui.Button{Text: i18n.T("Start Timer"), TestID: "sleep-start", OnPress: a.startCustomSleep})
		panel := a.fitPhoneDialog(cx, p, ui.Dialog{TestID: "sleep-dialog", Title: i18n.T("Sleep Timer"),
			Description: i18n.T("Pause after a set time, even while paused or buffering. The timer resets when the app exits."),
			Open:        a.sleepOpen.Get, OnOpenChange: func(open bool, _ base.Reason) { a.sleepOpen.Set(open) },
			Children: children, Footer: footer})
		if sheet, ok := panel.(base.Sheet); ok {
			// Long timers add an extra action. Keep every action reachable
			// on a short screen, with larger text or the keyboard open.
			sheet.Children = []core.Node{core.Scroll{TestID: "sleep-scroll", Style: core.Style{MinHeight: core.Pt(0)},
				StyleFunc: func() core.Style {
					return core.Style{MaxHeight: core.Pt(max(0, cx.Env().Window().Height-cx.Env().KeyboardHeight()-cx.Env().SafeArea().Top-16))}
				}, Children: sheet.Children}}
			return sheet
		}
		return panel
	})
}
