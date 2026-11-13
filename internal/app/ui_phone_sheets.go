package app

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

func (a *app) openPhoneSheet(kind string) { a.cx.DismissKeyboard(); a.phoneSheet.Set(kind) }
func (a *app) openPhoneTrack(tr music.Track, playlist string, index int) {
	a.phoneTrack, a.phonePlaylist, a.phoneTrackIndex = tr, playlist, index
	a.openPhoneSheet("track")
}

func (a *app) phoneSheets(cx *core.Scope, p palette) core.Node {
	last := ""
	return base.Sheet{TestID: "phone-sheet", Label: i18n.T("More"), Open: func() bool { return a.phoneSheet.Get() != "" },
		OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				a.phoneSheet.Set("")
			}
		},
		BackdropStyle: core.Style{Background: core.RGBA(0x000000, 0.45)},
		PopupStyleFunc: func() core.Style {
			return core.Style{Direction: core.Column, Background: p.bg,
				CornerRadii: core.Corners{TopLeft: 24, TopRight: 24}, Padding: core.Edges{Top: 10, Left: 20, Right: 20, Bottom: cx.Env().SafeArea().Bottom + 12},
				MaxHeight: core.Pt(cx.Env().Window().Height - cx.Env().SafeArea().Top - 24)}
		},
		Children: []core.Node{
			// Keep the pan on the grip: Android cannot hand a scroll drag
			// back from a sheet-wide pan to the list inside it.
			core.View{TestID: "phone-sheet-grip", PanActivation: core.PanActivation{ActiveOffsetY: 12, FailOffsetX: 16},
				OnPan: func(e core.PanEvent) {
					if e.Phase == core.GestureEnded && (e.Translation.Y > 60 || e.Velocity.Y > 800) {
						a.phoneSheet.Set("")
					}
				}, Style: core.Style{Height: core.Pt(28), FlexShrink: 0, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(4), CornerRadius: 2, Background: p.press}}}},
			core.Dynamic(func(cx *core.Scope) core.Node {
				kind := a.phoneSheet.Get()
				a.rev.Get()
				a.playRev.Get()
				if kind != "" {
					last = kind
				}
				if last == "queue" {
					return a.phoneQueue(cx, p)
				}
				return a.phoneActions(cx, p, last)
			}),
		}}
}

func (a *app) phoneSheetAction(p palette, id, label string, svg *core.SVG, action func()) core.Node {
	return base.Button{TestID: id, Label: label, OnPress: func(core.PressEvent) { action() },
		Style: core.Style{Height: core.Pt(56), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16,
			CornerRadius: 10, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
			icon(svg, 22, p.muted), core.Text{Content: label, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: 16, Color: p.text}},
		}}
}

func (a *app) phoneQueue(cx *core.Scope, p palette) core.Node {
	order, at := a.pl.PlayOrder()
	tracks := a.pl.Queue()
	heading := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, FlexShrink: 0}, Children: []core.Node{
		core.Text{Content: i18n.T("Queue") + fmt.Sprintf(" · %d", len(order)), Style: core.Style{FlexGrow: 1, FontSize: 21, FontWeight: core.FontWeightBold, Color: p.text}},
		a.phoneIcon(p, "phone-sheet-close", i18n.T("Close"), lucideX, p.muted, func() { a.phoneSheet.Set("") }),
	}}
	if len(order) == 0 {
		return core.View{Children: []core.Node{heading, a.emptyNote(p, i18n.T("Not Playing"), i18n.T("Choose a song to start listening"))}}
	}
	list := new(core.Ref)
	cx.AfterFunc(0, func() {
		if at >= 0 {
			list.ScrollToIndex(at, false)
		}
	})
	return core.View{TestID: "phone-queue", Style: core.Style{Direction: core.Column, MinHeight: core.Pt(0)}, Children: []core.Node{
		heading,
		core.List{Ref: list, TestID: "phone-queue-list", Count: func() int { return len(order) }, Key: func(i int) any { return order[i] }, EstimatedSize: 64,
			Style: core.Style{Height: core.Pt(min(420, cx.Env().Window().Height*0.52)), MinHeight: core.Pt(0)},
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				return core.Dynamic(func(cx *core.Scope) core.Node {
					index := order[row.Index()]
					tr := tracks[index]
					color := p.text
					if row.Index() == at {
						color = p.accent
					}
					return base.Button{TestID: fmt.Sprintf("phone-queue-%d", index), Label: tr.Name, OnPress: func(core.PressEvent) { a.cancelFocusResume(); a.pl.Jump(index); a.heartbeat() },
						Style: core.Style{Height: core.Pt(64), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, CornerRadius: 10, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
							a.cover(p, tr.Album.ImageURL, 44, 8),
							core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 4}, Children: []core.Node{
								core.Text{Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: 15, FontWeight: core.FontWeightMedium, Color: color}},
								core.Text{Content: tr.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: 12, Color: p.muted}},
							}}, icon(func() *core.SVG {
								if row.Index() == at {
									return lucideVolume
								}
								return lucideNote
							}(), 18, color),
						}}
				})
			}},
	}}
}

func (a *app) phoneActions(cx *core.Scope, p palette, kind string) core.Node {
	tr := a.phoneTrack
	close := func(fn func()) func() { return func() { a.phoneSheet.Set(""); fn() } }
	var body []core.Node
	if kind == "playlist" {
		var list music.Playlist
		for _, pl := range a.mine {
			if pl.ID == a.phonePlaylist {
				list = pl
				break
			}
		}
		body = append(body, text(list.Name, 20, core.FontWeightBold, p.text),
			a.phoneSheetAction(p, "phone-playlist-rename", i18n.T("Rename Playlist…"), lucidePencil, close(func() { a.renamePlaylist(list.ID, list.Name) })),
			a.phoneSheetAction(p, "phone-playlist-delete", i18n.T("Delete Playlist"), lucideTrash, close(func() { a.confirmDelete(list.ID, list.Name) })))
	} else {
		body = append(body, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Edges{Top: 8, Bottom: 16}, Borders: core.Edges{Bottom: 1}, BorderColor: p.hairline}, Children: []core.Node{
			a.cover(p, tr.Album.ImageURL, 52, 10),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{
				core.Text{Content: tr.Name, MaxLines: 2, Style: core.Style{FontSize: 17, FontWeight: core.FontWeightSemibold, Color: p.text}},
				core.Text{Content: tr.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: 13, Color: p.muted}},
			}},
		}})
		switch kind {
		case "add":
			body = append(body, text(i18n.T("Add to Playlist"), 19, core.FontWeightBold, p.text))
			for _, list := range a.mine {
				pl := list
				body = append(body, a.phoneSheetAction(p, "phone-add-to-"+pl.ID, pl.Name, lucideListPlus, close(func() { a.addToPlaylist(pl.ID, []string{tr.ID}) })))
			}
			body = append(body, a.phoneSheetAction(p, "phone-add-new-playlist", i18n.T("New Playlist…"), lucidePlus, close(func() {
				pl, err := a.local.CreatePlaylist(tr.Name)
				if err != nil {
					a.say(err.Error())
					return
				}
				a.addToPlaylist(pl.ID, []string{tr.ID})
				a.reload()
			})))
		case "info":
			body = append(body, text(i18n.T("Song Info"), 19, core.FontWeightBold, p.text),
				text(i18n.T("Album")+" · "+tr.Album.Name, 15, core.FontWeightNormal, p.text),
				text(i18n.T("Artists")+" · "+tr.ArtistNames(), 15, core.FontWeightNormal, p.text),
				text(i18n.T("Duration")+" · "+clock(tr.Length()), 15, core.FontWeightNormal, p.muted),
				a.phoneSheetAction(p, "phone-copy-title", i18n.T("Copy Title"), lucideNote, close(func() { a.cx.SetClipboard(tr.Name) })))
		default:
			label := i18n.T("Like")
			if a.likedSet[tr.ID] {
				label = i18n.T("Unlike")
			}
			body = append(body,
				a.phoneSheetAction(p, "phone-track-like", label, lucideHeart, close(func() { a.toggleLike(tr.ID) })),
				a.phoneSheetAction(p, "phone-track-add", i18n.T("Add to Playlist"), lucideListPlus, func() { a.phoneSheet.Set("add") }))
			if tr.Album.ID != "" {
				body = append(body, a.phoneSheetAction(p, "phone-track-album", i18n.Tf("Go to Album: %s", tr.Album.Name), lucideDisc, close(func() { a.open(route{page: pageAlbum, id: tr.Album.ID}) })))
			}
			for _, ar := range tr.Artists {
				artist := ar
				if artist.ID != "" {
					body = append(body, a.phoneSheetAction(p, "phone-track-artist-"+artist.ID, i18n.Tf("Go to Artist: %s", artist.Name), lucideUser, close(func() { a.open(route{page: pageArtist, id: artist.ID}) })))
				}
			}
			body = append(body, a.phoneSheetAction(p, "phone-track-info", i18n.T("Song Info"), lucideNote, func() { a.phoneSheet.Set("info") }))
			if a.phonePlaylist != "" {
				playlist, index := a.phonePlaylist, a.phoneTrackIndex
				move := func(to int) func() {
					return close(func() {
						if err := a.local.MovePlaylistTrack(playlist, index, to); err != nil {
							a.say(err.Error())
							return
						}
						a.loadPlaylist(playlist, true)
					})
				}
				if index > 0 {
					body = append(body, a.phoneSheetAction(p, "phone-track-up", i18n.T("Move Up"), lucideArrowUp, move(index-1)))
				}
				if data := a.playlists[playlist]; data != nil && data.playlist != nil && index+1 < len(data.playlist.Tracks) {
					body = append(body, a.phoneSheetAction(p, "phone-track-down", i18n.T("Move Down"), lucideArrowDown, move(index+1)))
				}
				body = append(body, a.phoneSheetAction(p, "phone-track-remove", i18n.T("Remove from Playlist"), lucideTrash, close(func() { a.removeFromPlaylist(playlist, []string{tr.ID}) })))
			}
		}
	}
	return core.View{Style: core.Style{Direction: core.Column, MinHeight: core.Pt(0)}, Children: []core.Node{
		core.Scroll{TestID: "phone-actions", Style: core.Style{MaxHeight: core.Pt(cx.Env().Window().Height * 0.65), MinHeight: core.Pt(0)}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Column, Gap: 4}, Children: body},
		}},
		a.phoneSheetAction(p, "phone-sheet-close", i18n.T("Close"), lucideX, func() { a.phoneSheet.Set("") }),
	}}
}
