package app

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HengWh/woodwind-music/internal/i18n"
)

// nameDialog names a new playlist, or renames one.
func (a *app) nameDialog(cx *core.Scope, p palette) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		title := i18n.T("New Playlist")
		if !a.namingNew {
			title = i18n.T("Rename Playlist")
		}
		return ui.Dialog{Open: func() bool { return a.naming }, OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				a.naming = false
				a.bump()
			}
		}, Title: title, Children: []core.Node{
			base.Input{Value: func() string { return a.namingText }, OnChange: func(v string) { a.namingText = v }, TestID: "playlist-name",
				OnSubmit: func(string) { a.saveName() }, Placeholder: i18n.T("Name"), AutoFocus: true,
				Style: core.Style{Height: core.Pt(30), Padding: core.Symmetric(0, 10), CornerRadius: 8, FontSize: 13,
					Background: p.hover, Color: p.text, PlaceholderColor: p.faint, BorderWidth: 0}},
		}, Footer: []core.Node{
			ui.Button{Text: i18n.T("Cancel"), Variant: ui.ButtonSecondary, TestID: "name-cancel", OnPress: func() { a.naming = false; a.bump() }},
			ui.Button{Text: i18n.T("Save"), TestID: "name-save", OnPress: a.saveName},
		}}
	})
}

// deleteDialog asks whether to delete a playlist.
func (a *app) deleteDialog(cx *core.Scope, p palette) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		return ui.Dialog{Open: func() bool { return a.deleting }, OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				a.deleting = false
				a.bump()
			}
		}, Title: i18n.T("Delete Playlist"), Description: i18n.Tf("“%s” will be deleted. The songs stay in your library.", a.deletingName),
			Footer: []core.Node{
				ui.Button{Text: i18n.T("Cancel"), Variant: ui.ButtonSecondary, OnPress: func() { a.deleting = false; a.bump() }},
				ui.Button{Text: i18n.T("Delete"), TestID: "delete-confirm", Color: func() core.Color { return core.RGB(0xd92b2b) }, OnPress: a.deletePlaylist},
			}}
	})
}
