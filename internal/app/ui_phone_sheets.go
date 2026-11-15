package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

func (a *app) openPhoneSheet(kind string) {
	a.cx.DismissKeyboard()
	if kind == "queue" && a.phoneSheet.GetUntracked() != "queue" {
		a.phoneQueueVisited = false
		a.phoneQueueOffset = core.Point{}
	}
	a.phoneSheet.Set(kind)
}
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
				CornerRadii: core.Corners{TopLeft: 24, TopRight: 24}, Padding: core.Edges{Top: 10, Left: 16, Right: 16, Bottom: cx.Env().SafeArea().Bottom + 12},
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
				if kind != "" {
					last = kind
				}
				if last == "sources" {
					return a.phoneSources(p)
				}
				if last == "account" {
					return a.phoneAccountSheet(p)
				}
				if last == "playback-error" {
					return a.phonePlaybackIssue(p)
				}
				if last == "message" {
					return a.phoneMessageSheet(p)
				}
				if last == "queue" {
					return a.phoneQueue(cx, p)
				}
				a.playRev.Get()
				return a.phoneActions(cx, p, last)
			}),
		}}
}

func (a *app) phoneSheetAction(p palette, id, label string, svg *core.SVG, action func()) core.Node {
	return base.Button{TestID: id, Label: label, OnPress: func(core.PressEvent) { action() },
		Style: core.Style{MinHeight: core.Pt(52), FlexShrink: 0, Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16,
			CornerRadius: 10, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
			icon(svg, 22, p.muted), core.Text{Content: label, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), FontSize: phoneFontBody, Color: p.text}},
		}}
}

func (a *app) phoneQueue(cx *core.Scope, p palette) core.Node {
	// Pause, likes and progress must not replace the list being browsed.
	contents := core.NewMemo(cx, func() string {
		a.playRev.Get()
		a.modeRev.Get()
		order, _ := a.pl.PlayOrder()
		var key strings.Builder
		for _, i := range order {
			tr := a.pl.Queue()[i]
			fmt.Fprintf(&key, "%d:%s:%s:%s\x00", i, tr.Source, tr.ID, tr.Name)
		}
		return key.String()
	})
	current := core.NewMemo(cx, func() int { a.playRev.Get(); a.modeRev.Get(); _, at := a.pl.PlayOrder(); return at })
	return core.Dynamic(func(cx *core.Scope) core.Node {
		contents.Get()
		order, _ := a.pl.PlayOrder()
		tracks := a.pl.Queue()
		list := new(core.Ref)
		heading := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, FlexShrink: 0}, Children: []core.Node{
			core.Text{Content: i18n.T("Queue") + fmt.Sprintf(" · %d", len(order)), Style: core.Style{FlexGrow: 1, FontSize: phoneFontTitle, FontWeight: core.FontWeightSemibold, Color: p.text}},
			a.phoneIcon(p, "phone-queue-current", i18n.T("Current song"), lucideVolume, p.accent, func() {
				if at := current.Get(); at >= 0 {
					list.ScrollToIndexAt(at, .5, 0, !cx.Env().ReduceMotion())
				}
			}),
			a.phoneIcon(p, "phone-sheet-close", i18n.T("Close"), lucideX, p.muted, func() { a.phoneSheet.Set("") }),
		}}
		if len(order) == 0 {
			return core.View{Children: []core.Node{heading, a.emptyNote(p, i18n.T("Not Playing"), i18n.T("Choose a song to start listening"))}}
		}
		initial := 0
		if !a.phoneQueueVisited {
			initial = max(0, current.GetUntracked())
			a.phoneQueueVisited = true
		}
		save := func(e core.ScrollEvent) { a.phoneQueueOffset = e.Offset }
		return core.View{TestID: "phone-queue", Style: core.Style{Direction: core.Column, MinHeight: core.Pt(0)}, Children: []core.Node{
			heading,
			core.List{Ref: list, TestID: "phone-queue-list", Count: func() int { return len(order) }, Key: func(i int) any { return order[i] }, EstimatedSize: phoneTwoLineHeight(cx, phoneFontSong),
				InitialScrollIndex: initial, ContentOffset: a.phoneQueueOffset, OnScroll: save, OnScrollEnd: save, ScrollEventThrottle: 80 * time.Millisecond,
				Style: core.Style{Height: core.Pt(min(420, cx.Env().Window().Height*.52)), MinHeight: core.Pt(0)},
				Row: func(cx *core.Scope, row core.ListRow) core.Node {
					return core.Dynamic(func(*core.Scope) core.Node {
						index := order[row.Index()]
						at := row.Index()
						tr := tracks[index]
						color := func() core.Color {
							if current.Get() == at {
								return p.accent
							}
							if tr.Unavailable {
								return p.faint
							}
							return p.text
						}
						return base.Button{TestID: fmt.Sprintf("phone-queue-%d", index), Label: tr.Name, Disabled: func() bool { return tr.Unavailable }, OnPress: func(core.PressEvent) { a.cancelFocusResume(); a.pl.Jump(index); a.heartbeat() },
							Style: core.Style{Height: core.Pt(phoneTwoLineHeight(cx, phoneFontSong)), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, CornerRadius: 10, Active: &core.Style{Background: p.hover}}, Children: []core.Node{
								a.cover(p, tr.Album.ImageURL, 44, 8), core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 4}, Children: []core.Node{
									core.Text{Content: tr.Name, MaxLines: 1, Style: core.Style{FontSize: phoneFontSong}, StyleFunc: func() core.Style { return core.Style{Color: color()} }},
									core.Text{Content: phoneTrackSubtitle(tr), MaxLines: 1, Style: core.Style{FontSize: phoneFontCaption, Color: p.muted}},
								}}, core.Icon{SVGFunc: func() *core.SVG {
									if current.Get() == at {
										return lucideVolume
									}
									return lucideNote
								}, Style: core.Style{Width: core.Pt(18), Height: core.Pt(18)}, StyleFunc: func() core.Style { return core.Style{Color: color()} }},
							}}
					})
				},
			},
		}}
	})
}

func (a *app) phoneActions(cx *core.Scope, p palette, kind string) core.Node {
	tr := a.phoneTrack
	src := a.sourceByID(tr.Source)
	if tr.Source == "" {
		src = a.source()
	}
	can := func(cap string) bool { return src != nil && src.Can(cap) }
	canAdd := src == a.source() && can("addToPlaylist")
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
		body = append(body, text(list.Name, phoneFontTitle, core.FontWeightSemibold, p.text))
		if a.playlistEditable(list) {
			if a.can("renamePlaylist") {
				body = append(body, a.phoneSheetAction(p, "phone-playlist-rename", i18n.T("Rename Playlist…"), lucidePencil, close(func() { a.renamePlaylist(list.ID, list.Name) })))
			}
			if a.can("deletePlaylist") {
				body = append(body, a.phoneSheetAction(p, "phone-playlist-delete", i18n.T("Delete Playlist"), lucideTrash, close(func() { a.confirmDelete(list.ID, list.Name) })))
			}
		}
	} else {
		body = append(body, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Edges{Top: 8, Bottom: 16}, Borders: core.Edges{Bottom: 1}, BorderColor: p.hairline}, Children: []core.Node{
			a.cover(p, tr.Album.ImageURL, 52, 10),
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 5}, Children: []core.Node{
				core.Text{Content: tr.Name, Style: core.Style{FontSize: phoneFontSong, FontWeight: core.FontWeightMedium, Color: p.text}},
				core.Text{Content: tr.ArtistNames(), MaxLines: 1, Style: core.Style{FontSize: phoneFontBody, Color: p.muted}},
			}},
		}})
		switch kind {
		case "add":
			body = append(body, text(i18n.T("Add to Playlist"), phoneFontSection, core.FontWeightSemibold, p.text))
			for _, list := range a.mine {
				pl := list
				if !canAdd || !a.playlistEditable(pl) {
					continue
				}
				body = append(body, a.phoneSheetAction(p, "phone-add-to-"+pl.ID, pl.Name, lucideListPlus, close(func() { a.addToPlaylist(pl.ID, []string{tr.ID}) })))
			}
			if canAdd && a.can("createPlaylist") {
				body = append(body, a.phoneSheetAction(p, "phone-add-new-playlist", i18n.T("New Playlist…"), lucidePlus, close(func() {
					a.playlistForTrack(tr)
				})))
			}
		case "info":
			body = append(body, text(i18n.T("Song Info"), phoneFontSection, core.FontWeightSemibold, p.text),
				text(i18n.T("Album")+" · "+tr.Album.Name, phoneFontBody, core.FontWeightNormal, p.text),
				text(i18n.T("Artists")+" · "+tr.ArtistNames(), phoneFontBody, core.FontWeightNormal, p.text),
				text(i18n.T("Duration")+" · "+clock(tr.Length()), phoneFontBody, core.FontWeightNormal, p.muted),
				a.phoneSheetAction(p, "phone-copy-title", i18n.T("Copy Title"), lucideNote, close(func() { a.cx.SetClipboard(tr.Name) })))
			if src := a.sourceByID(tr.Source); src != nil {
				name := src.Info().Name
				if src == a.local {
					name = i18n.T("Local Files")
				}
				body = append(body, text(i18n.T("Music source")+" · "+name, phoneFontBody, core.FontWeightNormal, p.muted))
			}
			if current := a.pl.Current(); current != nil && current.ID == tr.ID && current.Source == tr.Source {
				if stream := a.pl.Stream(); stream != nil {
					label := strings.ToUpper(stream.Type)
					if stream.BitRate > 0 {
						label += fmt.Sprintf(" · %d kbps", stream.BitRate)
					}
					body = append(body, text(label, phoneFontBody, core.FontWeightNormal, p.muted))
				}
			}

		default:
			if src := a.sourceByID(tr.Source); src != nil && src.Can("comments") {
				body = append(body, a.phoneSheetAction(p, "phone-comments", i18n.T("Comments"), lucideNote, close(func() { a.openTrackPage(tr, pageComments, tr.ID) })))
			}
			label := i18n.T("Like")
			if a.isTrackLiked(tr) {
				label = i18n.T("Unlike")
			}
			if can("like") || tr.ShowID != "" && can("likeEpisode") {
				body = append(body, a.phoneSheetAction(p, "phone-track-like", label, lucideHeart, close(func() { a.toggleTrackLike(tr) })))
			}
			if canAdd {
				body = append(body, a.phoneSheetAction(p, "phone-track-add", i18n.T("Add to Playlist"), lucideListPlus, func() { a.phoneSheet.Set("add") }))
			}
			if tr.Album.ID != "" && can("album") {
				body = append(body, a.phoneSheetAction(p, "phone-track-album", i18n.Tf("Go to Album: %s", tr.Album.Name), lucideDisc, close(func() { a.openTrackPage(tr, pageAlbum, tr.Album.ID) })))
			}
			for _, ar := range tr.Artists {
				artist := ar
				if artist.ID != "" && can("artist") {
					body = append(body, a.phoneSheetAction(p, "phone-track-artist-"+artist.ID, i18n.Tf("Go to Artist: %s", artist.Name), lucideUser, close(func() { a.openTrackPage(tr, pageArtist, artist.ID) })))
				}
			}
			body = append(body, a.phoneSheetAction(p, "phone-track-info", i18n.T("Song Info"), lucideNote, func() { a.phoneSheet.Set("info") }))
			if a.phonePlaylist != "" && src == a.source() && a.canEditPlaylistTracks(a.phonePlaylist) {
				playlist, index := a.phonePlaylist, a.phoneTrackIndex
				move := func(to int) func() {
					return close(func() {
						a.movePlaylistTrack(playlist, index, to)
					})
				}
				if index > 0 && can("movePlaylistTrack") {
					body = append(body, a.phoneSheetAction(p, "phone-track-up", i18n.T("Move Up"), lucideArrowUp, move(index-1)))
				}
				if data := a.playlists[playlist]; can("movePlaylistTrack") && data != nil && data.playlist != nil && index+1 < len(data.playlist.Tracks) {
					body = append(body, a.phoneSheetAction(p, "phone-track-down", i18n.T("Move Down"), lucideArrowDown, move(index+1)))
				}
				if can("removeFromPlaylist") {
					body = append(body, a.phoneSheetAction(p, "phone-track-remove", i18n.T("Remove from Playlist"), lucideTrash, close(func() { a.removeFromPlaylist(playlist, []string{tr.ID}) })))
				}
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
