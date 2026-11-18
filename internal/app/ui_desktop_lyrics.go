package app

import (
	"fmt"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

func (a *app) desktopLyricsButton(p palette) core.Node {
	return base.Button{TestID: "bar-desktop-lyrics", Label: i18n.T("Desktop Lyrics"), OnPress: func(core.PressEvent) { a.toggleDesktopLyrics() },
		Selected: func() bool { a.rev.Get(); return a.set.DesktopLyrics.Enabled },
		Style:    core.Style{Width: core.Pt(30), Height: core.Pt(30), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 8, Hover: &core.Style{Background: p.hover}, Selected: &core.Style{Background: p.press}},
		Children: []core.Node{icon(lucide.Monitor(), 16, p.muted)}}
}

func (a *app) desktopLyricsRows() []settingsRowDef {
	return []settingsRowDef{
		{label: i18n.T("Desktop Lyrics"), sub: i18n.T("Show synchronized lyrics above other apps. Drag the lyrics to move them."), control: func(p palette) core.Node {
			return settingsSwitch{palette: p, Label: i18n.T("Desktop Lyrics"), TestID: "desktop-lyrics-enabled", Checked: func() bool { return a.set.DesktopLyrics.Enabled }, OnChange: a.setDesktopLyricsEnabled}
		}},
		{label: i18n.T("Lock Desktop Lyrics"), sub: i18n.T("Clicks pass through locked lyrics. Unlock here or from the tray menu."), control: func(p palette) core.Node {
			return settingsSwitch{palette: p, Label: i18n.T("Lock Desktop Lyrics"), TestID: "desktop-lyrics-locked", Checked: func() bool { return a.set.DesktopLyrics.Locked }, OnChange: func(v bool) { a.set.DesktopLyrics.Locked = v; a.saveDesktopLyricsStyle() }}
		}},
		{label: i18n.T("Lyric Font Size"), control: func(p palette) core.Node {
			var options []ui.Option[int]
			for _, size := range []int{24, 30, 36, 42, 48} {
				options = append(options, ui.Option[int]{Value: size, Label: fmt.Sprint(size)})
			}
			return settingsChoice[int]{palette: p, Label: i18n.T("Lyric Font Size"), TestID: "desktop-lyrics-size", Value: func() int { return a.set.DesktopLyrics.FontSize }, Options: options, OnChange: func(v int) { a.set.DesktopLyrics.FontSize = v; a.saveDesktopLyricsStyle() }}
		}},
		{label: i18n.T("Lyric Highlight Color"), control: func(p palette) core.Node {
			return settingsChoice[string]{palette: p, Label: i18n.T("Lyric Highlight Color"), TestID: "desktop-lyrics-color", Value: func() string { return a.set.DesktopLyrics.Color }, Options: []ui.Option[string]{{Value: "gold", Label: i18n.T("Gold")}, {Value: "cyan", Label: i18n.T("Cyan")}, {Value: "pink", Label: i18n.T("Pink")}}, OnChange: func(v string) { a.set.DesktopLyrics.Color = v; a.saveDesktopLyricsStyle() }}
		}},
		{label: i18n.T("Show Lyric Translation"), sub: i18n.T("Show the next line when a translation is unavailable."), control: func(p palette) core.Node {
			return settingsSwitch{palette: p, Label: i18n.T("Show Lyric Translation"), TestID: "desktop-lyrics-translation", Checked: func() bool { return a.set.DesktopLyrics.Translation }, OnChange: func(v bool) { a.set.DesktopLyrics.Translation = v; a.saveDesktopLyricsStyle() }}
		}},
		{label: i18n.T("Lyric Window Position"), sub: i18n.T("Return the lyrics to the bottom of the main screen."), control: func(p palette) core.Node {
			return a.settingsAction(p, i18n.T("Reset Position"), "desktop-lyrics-reset", false, a.resetDesktopLyricsPosition)
		}},
	}
}
