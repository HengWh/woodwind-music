package app

import (
	_ "embed"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"

	"nsgo-music/internal/i18n"
)

// trayTemplate is the app's icon in the menu bar: the record and tonearm,
// black on transparent for macOS to tint.
//
//go:embed tray/tray-template.png
var trayTemplate []byte

// makeTray adds the app's icon to the menu bar or the notification area,
// which opens its window and plays.
func (a *app) makeTray(cx *core.Scope) *desktop.Tray {
	return desktop.NewTray(cx, desktop.TrayOptions{Icon: trayTemplate, Template: true, Tooltip: "MyGo Music",
		Menu: func() []desktop.MenuItem {
			return []desktop.MenuItem{
				{Label: i18n.T("Open MyGo Music"), OnSelect: a.showMain},
				{Separator: true},
				{Label: i18n.T("Play/Pause"), OnSelect: func() { a.pl.TogglePlay(); a.heartbeat() }},
				{Label: i18n.T("Next"), OnSelect: a.pl.Next},
				{Label: i18n.T("Previous"), OnSelect: a.pl.Prev},
				{Label: i18n.T("Like/Unlike"), OnSelect: func() {
					if t := a.pl.Current(); t != nil {
						a.toggleLike(t.ID)
					}
				}},
				{Separator: true},
				{Label: i18n.T("Now Playing"), OnSelect: func() { a.showMain(); a.openNowPlaying() }},
				{Label: i18n.T("Settings…"), OnSelect: a.openSettings},
				{Separator: true},
				{Label: i18n.Tf("Quit %s", "MyGo Music"), OnSelect: func() { a.saveSession(); a.cx.Quit() }},
			}
		}})
}

// showMain opens the app's window, or brings it forward.
func (a *app) showMain() {
	if w := desktop.MainWindow(a.cx); w != nil && !w.Closed() {
		w.Show()
		w.Focus()
		return
	}
	if a.openMain != nil {
		a.openMain()
	}
}
