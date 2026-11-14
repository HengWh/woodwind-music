package app

import (
	"strings"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

// playlistScroll uses the original list's insets: the header sits twelve
// points outside the text of a song, aligned with the row's rounded edge.
func (a *app) playlistScroll(p palette, children ...core.Node) core.Node {
	scroll := a.pageScroll(p, children...).(core.Scroll)
	if !a.phoneLayout(a.cx) {
		content := scroll.Children[0].(core.View)
		content.Style.Padding.Left = pagePad - 12
		content.Style.Padding.Right = pagePad - 12
		scroll.Children[0] = content
	}
	return scroll
}

func (a *app) canEditPlaylistTracks(id string) bool {
	d := a.playlists[id]
	return d != nil && d.playlist != nil && a.playlistEditable(*d.playlist)
}

// playlistEditable follows the source's ownership and fixed-list flags.
func (a *app) playlistEditable(pl music.Playlist) bool {
	if pl.Fixed {
		return false
	}
	if pl.Mine {
		return true
	}
	for _, own := range a.mine {
		if own.ID == pl.ID && own.Mine {
			return !own.Fixed
		}
	}
	return false
}

func (a *app) playlistMenu(pl music.Playlist) []core.Node {
	if !a.playlistEditable(pl) {
		return nil
	}
	var items []core.Node
	if a.can("renamePlaylist") {
		items = append(items, ui.MenuItem{Text: i18n.T("Rename Playlist…"), TestID: "playlist-rename", OnSelect: func() { a.renamePlaylist(pl.ID, pl.Name) }})
	}
	if a.can("deletePlaylist") {
		if len(items) > 0 {
			items = append(items, ui.MenuSeparator{})
		}
		items = append(items, ui.MenuItem{Text: i18n.T("Delete Playlist"), TestID: "playlist-delete", Variant: ui.MenuItemDanger, OnSelect: func() { a.confirmDelete(pl.ID, pl.Name) }})
	}
	return items
}

func (a *app) playlistSaved(pl music.Playlist) bool {
	if pl.Saved {
		return true
	}
	for _, list := range a.mine {
		if list.ID == pl.ID && !list.Mine {
			return true
		}
	}
	return false
}

func (a *app) setPlaylistSaved(id string, saved bool) {
	src := a.source()
	a.sourceAction(src, func() error { return src.SavePlaylist(id, saved) }, nil)
}

// playlistHero ports MyGo's cover, metadata and inline actions together.
func (a *app) playlistHero(p palette, pl *music.Playlist) core.Node {
	var meta []string
	if pl.Owner != "" {
		meta = append(meta, pl.Owner)
	}
	meta = append(meta, i18n.Songs(len(pl.Tracks)))
	details := []core.Node{
		text(i18n.T("Playlist"), 12, core.FontWeightSemibold, p.accent),
		core.Text{TestID: "playlist-title", Content: pl.Name, MaxLines: 2, Style: core.Style{FontSize: 30, FontWeight: core.FontWeight(800), LetterSpacing: -0.6, LineHeightMultiple: 1.2, Color: p.text}},
		text(strings.Join(meta, " · "), 13, core.FontWeightNormal, p.muted),
	}
	if description := strings.TrimSpace(pl.Description); description != "" {
		details = append(details, core.Text{Content: strings.Join(strings.Fields(description), " "), MaxLines: 2,
			Style: core.Style{FontSize: 12.5, Color: p.muted, LineHeightMultiple: 1.45, Margin: core.Edges{Top: 2}}})
	}
	var actions []core.Node
	if len(pl.Tracks) > 0 || !a.phoneLayout(a.cx) {
		play := a.playButtons(p, pl.Tracks).(core.View)
		actions = append(actions, play.Children...)
	}
	own := pl.Mine
	for _, list := range a.mine {
		if list.ID == pl.ID && list.Mine {
			own = true
			break
		}
	}
	if a.can("savePlaylist") && !own {
		saved := a.playlistSaved(*pl)
		glyph, label := lucidePlus, i18n.TC("Save", "library")
		if saved {
			glyph, label = lucideCheck, i18n.T("Saved")
		}
		button := a.pillButton(p, glyph, label, blend(p.bg, p.text, 0.04), p.text, func() { a.setPlaylistSaved(pl.ID, !saved) }).(base.Button)
		button.TestID = "playlist-save"
		actions = append(actions, button)
	}
	if items := a.playlistMenu(*pl); len(items) > 0 {
		if a.phoneLayout(a.cx) {
			actions = append(actions, a.phoneIcon(p, "phone-playlist-more", i18n.T("More"), lucideMore, p.muted, func() { a.phonePlaylist = pl.ID; a.openPhoneSheet("playlist") }))
		} else {
			actions = append(actions, ui.Menu{TestID: "playlist-more", Trigger: ui.Button{Label: i18n.T("More"), Variant: ui.ButtonGhost, Side: 34},
				Face: icon(lucideMore, 18, p.muted), Children: items})
		}
	}
	if len(actions) > 0 {
		details = append(details, core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 10, Margin: core.Edges{Top: 10}}, Children: actions})
	}
	var pic core.Node = a.cover(p, pl.ImageURL, 212, 14)
	if pl.ImageURL == "" {
		pic = core.View{Style: core.Style{Width: core.Pt(212), Height: core.Pt(212), CornerRadius: 14, Background: p.hover}}
	}
	cover := core.View{TestID: "playlist-cover", Style: core.Style{Width: core.Pt(212), Height: core.Pt(212), FlexShrink: 0, CornerRadius: 14,
		Shadows: []core.BoxShadow{{Offset: core.Point{Y: 18}, Blur: 40, Spread: -10, Color: p.cardShade}}}, Children: []core.Node{pic}}
	if a.phoneLayout(a.cx) {
		cover.Style.AlignSelf = core.AlignCenter
		details = details[1:]
		details[0] = core.Text{TestID: "playlist-title", Content: pl.Name, MaxLines: 3, Style: core.Style{FontSize: phoneFontDisplay, FontWeight: core.FontWeightSemibold, Color: p.text}}
		return core.View{Style: core.Style{Gap: 12, Margin: core.Edges{Top: 8, Bottom: 16}}, Children: append([]core.Node{cover}, details...)}
	}
	return core.View{TestID: "playlist-header", Style: core.Style{Direction: core.Row, Gap: 28, AlignItems: core.AlignEnd, Padding: core.Edges{Bottom: 26}}, Children: []core.Node{
		cover,
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 8}, Children: details},
	}}
}
