package app

import (
	_ "embed"
	"runtime"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"

	"nsgo-music/internal/i18n"
)

// trayTemplate is the app's icon in the menu bar: the record and tonearm,
// black on transparent for macOS to tint.
//
//go:embed tray/tray-template.png
var trayTemplate []byte

// appIcons are where the app's icon shows on macOS, with their names: the
// Dock and the menu bar, the Dock alone, or the menu bar alone.
var appIcons = []struct{ id, label string }{
	{"dock-and-menubar", i18n.N("Dock and Menu Bar")},
	{"dock", i18n.N("Dock Only")},
	{"menubar", i18n.N("Menu Bar Only")},
}

// applyAppIcon shows the app's icon where the settings say: in the menu
// bar or the notification area, and in the Dock on macOS. The Dock icon
// stays while a window is open, so the app can be switched to.
func (a *app) applyAppIcon() {
	show := runtime.GOOS != "darwin" || a.set.AppIcon != "dock"
	switch {
	case show && a.tray == nil:
		a.tray = a.makeTray(a.cx)
	case !show && a.tray != nil:
		a.tray.Remove()
		a.tray = nil
	}
	if runtime.GOOS == "darwin" {
		desktop.SetDockVisible(a.cx, a.set.AppIcon != "menubar" || a.windowOpen())
	}
}

// windowOpen tells whether the main window or the settings window is
// open.
func (a *app) windowOpen() bool {
	if w := desktop.MainWindow(a.cx); w != nil && w.Visible() {
		return true
	}
	return a.settingsWin != nil && !a.settingsWin.Closed() && a.settingsWin.Visible()
}

// makeTray adds the app's icon to the menu bar or the notification area,
// which opens its window and plays.
func (a *app) makeTray(cx *core.Scope) *desktop.Tray {
	return desktop.NewTray(cx, desktop.TrayOptions{Icon: trayTemplate, Template: true, Tooltip: appName,
		Menu: func() []desktop.MenuItem {
			return []desktop.MenuItem{
				{Label: i18n.Tf("Open %s", appName), OnSelect: a.showMain},
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
				{Label: i18n.Tf("Quit %s", appName), OnSelect: func() { a.saveSession(); a.cx.Quit() }},
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
	a.applyAppIcon()
}
