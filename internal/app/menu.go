package app

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/player"
)

// menu sets the application menu: the app's, the pages', playback's and
// the window's.
func (a *app) menu(cx *core.Scope) {
	appMenu := desktop.MenuItem{Label: appName, Items: []desktop.MenuItem{
		{Label: i18n.Tf("About %s", appName), Role: desktop.MenuAbout},
		{Separator: true},
		{Label: i18n.T("Settings…"), Shortcut: "mod+,", OnSelect: a.openSettings},
		{Separator: true},
		{Role: desktop.MenuHide},
		{Role: desktop.MenuHideOthers},
		{Role: desktop.MenuShowAll},
		{Separator: true},
		{Label: i18n.Tf("Quit %s", appName), Shortcut: "mod+q", OnSelect: a.quit},
	}}
	file := desktop.MenuItem{Label: i18n.T("File"), Items: []desktop.MenuItem{
		{Label: i18n.T("New Playlist"), Shortcut: "mod+n", OnSelect: a.newPlaylist},
		{Label: i18n.T("Add Folder…"), Shortcut: "mod+o", OnSelect: a.pickFolders},
		{Separator: true},
		{Label: i18n.T("Search"), Shortcut: "mod+f", OnSelect: func() { a.searchFieldFocus(a.cx) }},
	}}
	edit := desktop.MenuItem{Label: i18n.T("Edit"), Items: []desktop.MenuItem{
		{Role: desktop.MenuUndo},
		{Role: desktop.MenuRedo},
		{Separator: true},
		{Role: desktop.MenuCut},
		{Role: desktop.MenuCopy},
		{Role: desktop.MenuPaste},
		{Role: desktop.MenuSelectAll},
	}}
	play := desktop.MenuItem{Label: i18n.T("Playback"), Items: []desktop.MenuItem{
		{Label: i18n.T("Play/Pause"), Shortcut: "space", When: core.MenuNotEditing, OnSelect: a.toggle},
		{Label: i18n.T("Next"), Shortcut: "mod+right", OnSelect: a.pl.Next},
		{Label: i18n.T("Previous"), Shortcut: "mod+left", OnSelect: a.pl.Prev},
		{Separator: true},
		{Label: i18n.T("Shuffle"), Checkable: true, Checked: a.pl.Shuffle(), OnSelect: a.toggleShuffle},
		{Label: i18n.T("Repeat"), Checkable: true, Checked: a.pl.Repeat() != player.RepeatOff, OnSelect: a.cycleRepeat},
		{Label: i18n.T("Now Playing"), Shortcut: "mod+shift+p", OnSelect: a.openNowPlaying},
	}}
	view := desktop.MenuItem{Label: i18n.T("View"), Items: []desktop.MenuItem{
		{Label: i18n.T("Back"), Shortcut: "mod+[", OnSelect: a.back},
		{Label: i18n.T("Forward"), Shortcut: "mod+]", OnSelect: a.goForward},
		{Separator: true},
		{Label: i18n.T("Toggle Full Screen"), Role: desktop.MenuToggleFullScreen},
	}}
	window := desktop.MenuItem{Label: i18n.T("Window"), Items: []desktop.MenuItem{
		{Role: desktop.MenuMinimize},
		{Role: desktop.MenuZoom},
		{Separator: true},
		{Role: desktop.MenuClose},
	}}
	desktop.SetApplicationMenu(cx, []desktop.MenuItem{appMenu, file, edit, play, view, window})
}
