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
		dialog := ui.Dialog{Open: func() bool { return a.naming }, OnOpenChange: func(open bool, _ base.Reason) {
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
		return a.fitPhoneDialog(cx, p, dialog)
	})
}

// deleteDialog asks whether to delete a playlist.
func (a *app) deleteDialog(cx *core.Scope, p palette) core.Node {
	return core.Dynamic(func(cx *core.Scope) core.Node {
		a.rev.Get()
		dialog := ui.Dialog{Open: func() bool { return a.deleting }, OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				a.deleting = false
				a.bump()
			}
		}, Title: i18n.T("Delete Playlist"), Description: i18n.Tf("“%s” will be deleted. The songs stay in your library.", a.deletingName),
			Footer: []core.Node{
				ui.Button{Text: i18n.T("Cancel"), Variant: ui.ButtonSecondary, OnPress: func() { a.deleting = false; a.bump() }},
				ui.Button{Text: i18n.T("Delete"), TestID: "delete-confirm", Color: func() core.Color { return core.RGB(0xd92b2b) }, OnPress: a.deletePlaylist},
			}}
		return a.fitPhoneDialog(cx, p, dialog)
	})
}

// Phone dialogs follow the keyboard and compact their actions on short screens.
func (a *app) fitPhoneDialog(cx *core.Scope, p palette, dialog ui.Dialog) core.Node {
	if !a.phoneLayout(cx) {
		return dialog
	}
	dialog.Label = dialog.Title
	head := []core.Node{text(dialog.Title, phoneFontTitle, core.FontWeightSemibold, p.text)}
	if dialog.Description != "" {
		head = append(head, text(dialog.Description, phoneFontBody, core.FontWeightNormal, p.muted))
	}
	dialog.Title, dialog.Description = "", ""
	for i, child := range dialog.Children {
		if input, ok := child.(base.Input); ok {
			input.Style.Height, input.Style.FontSize = core.Pt(48), phoneFontBody
			submit := input.OnSubmit
			input.OnSubmit = func(value string) {
				cx.DismissKeyboard()
				if submit != nil {
					submit(value)
				}
			}
			dialog.Children[i] = input
		}
	}
	dialog.Children = append(head, dialog.Children...)
	for i, child := range dialog.Footer {
		if button, ok := child.(ui.Button); ok {
			color, background := p.text, p.hover
			if button.Variant != ui.ButtonSecondary {
				color, background = p.onAccent, p.accent
			}
			if button.Color != nil {
				background = button.Color()
				color = p.onAccent
			}
			dialog.Footer[i] = base.Button{TestID: button.TestID, Label: button.Text,
				OnPress:  func(core.PressEvent) { cx.DismissKeyboard(); button.OnPress() },
				Style:    core.Style{MinHeight: core.Pt(48), Padding: core.Symmetric(10, 16), CornerRadius: 12, Background: background, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{text(button.Text, phoneFontBody, core.FontWeightMedium, color)}}
		}
	}
	footer := core.View{Style: core.Style{Direction: core.ColumnReverse, Gap: 8, Margin: core.Edges{Top: 8}}, StyleFunc: func() core.Style {
		if cx.Env().Window().Height-cx.Env().KeyboardHeight() < 360 {
			return core.Style{Direction: core.RowReverse}
		}
		return core.Style{Direction: core.ColumnReverse}
	}, Children: dialog.Footer}
	for i, child := range footer.Children {
		if button, ok := child.(base.Button); ok {
			button.Style.FlexGrow = 1
			button.Style.FlexBasis = core.Pt(0)
			footer.Children[i] = button
		}
	}
	body := append(dialog.Children, footer)
	return base.Sheet{Open: dialog.Open, Label: dialog.Label, TestID: dialog.TestID,
		OnOpenChange: func(open bool, reason base.Reason) {
			if !open {
				cx.DismissKeyboard()
			}
			dialog.OnOpenChange(open, reason)
		},
		BackdropStyle: core.Style{Background: core.RGBA(0x000000, .45)},
		PopupStyleFunc: func() core.Style {
			return core.Style{Bottom: core.Pt(cx.Env().KeyboardHeight()), Background: p.bg, CornerRadii: core.Corners{TopLeft: 24, TopRight: 24}}
		},
		Children: []core.Node{core.View{TestID: "phone-dialog-scroll", Style: core.Style{Gap: 12}, StyleFunc: func() core.Style {
			bottom := core.Points(16)
			if cx.Env().KeyboardHeight() == 0 {
				bottom += cx.Env().SafeArea().Bottom
			}
			return core.Style{Padding: core.Edges{Top: 16, Left: 16, Right: 16, Bottom: bottom}}
		}, Children: body}},
	}
}
