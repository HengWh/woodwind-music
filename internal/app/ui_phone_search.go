package app

import (
	"time"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// A results list keeps its cells and native scroll view as pages arrive.
func (a *app) phoneSearchResults(cx *core.Scope, p palette, r route, d *searchData) core.Node {
	if d.revision == nil {
		d.revision = core.NewSignal(a.cx, 0)
		d.busy = core.NewSignal(a.cx, d.loading)
	}
	phase := core.NewMemo(cx, func() int {
		d.revision.Get()
		if d.loaded {
			return 2
		}
		if d.err != "" {
			return 1
		}
		return 0
	})
	return core.Dynamic(func(cx *core.Scope) core.Node {
		switch phase.Get() {
		case 0:
			return a.pageScroll(p, a.searchHeader(p, r), a.phoneSkeletonRows(p, 5))
		case 1:
			return a.pageScroll(p, a.searchHeader(p, r), a.phoneNotice(p, "phone-search-retry", i18n.T("The search failed"), d.err, func() { a.loadSearch(r, true) }))
		}
		columns := 1
		estimate := a.trackRowHeight()
		if r.kind != music.SearchTracks {
			columns = 2
			estimate = (cx.Env().Window().Width-46)/2 + 72
		}
		key := phoneScrollKey(r)
		save := func(e core.ScrollEvent) { a.scroll[key] = e.Offset.Y }
		return core.List{TestID: "phone-page-scroll", Count: func() int { d.revision.Get(); return searchResultCount(d.res, r.kind) },
			Key: func(i int) any {
				switch r.kind {
				case music.SearchTracks:
					return d.res.Tracks[i].Source + "\x00" + d.res.Tracks[i].ID
				case music.SearchAlbums:
					return d.res.Albums[i].ID
				case music.SearchArtists:
					return d.res.Artists[i].ID
				case music.SearchPlaylists:
					return d.res.Playlists[i].ID
				default:
					return d.res.Shows[i].ID
				}
			},
			NumColumns: columns, EstimatedSize: estimate, ColumnGap: 14, Gap: func() core.Points {
				if columns == 2 {
					return 20
				}
				return 0
			}(),
			ContentOffset: core.Point{Y: a.scroll[key]}, OnScroll: save, OnScrollEnd: save, ScrollEventThrottle: 64 * time.Millisecond,
			Behavior: core.ScrollBehavior{KeyboardDismissMode: core.KeyboardDismissOnDrag},
			OnLoadMore: func() {
				if d.more && !d.loading && d.err == "" {
					a.loadMoreSearch(r)
				}
			}, LoadMoreThreshold: .5,
			Style:  core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(0), Padding: core.Edges{Top: 8, Left: 16, Right: 16, Bottom: 24}},
			Header: a.searchHeader(p, r),
			Empty:  a.emptyNote(p, i18n.T("No results found"), i18n.T("Try another search.")),
			Footer: core.Dynamic(func(*core.Scope) core.Node {
				d.revision.Get()
				d.busy.Get()
				if d.err != "" {
					return a.phoneNotice(p, "phone-search-more-retry", i18n.T("Couldn't load more results"), d.err, func() {
						if d.more {
							a.loadMoreSearch(r)
						} else {
							a.loadSearch(r, true)
						}
					})
				}
				if !d.more {
					return core.View{}
				}
				label := i18n.T("Load More")
				if d.loading {
					label = i18n.T("Loading…")
				}
				return base.Button{TestID: "search-load-more", Label: label, Disabled: func() bool { return d.busy.Get() }, OnPress: func(core.PressEvent) { a.loadMoreSearch(r) }, Style: core.Style{MinHeight: core.Pt(48), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Margin: core.Edges{Top: 12}, CornerRadius: 12, Background: p.sidebar}, Children: []core.Node{text(label, phoneFontBody, core.FontWeightNormal, p.muted)}}
			}),
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				if r.kind == music.SearchTracks {
					return a.phoneTrackCell(cx, p, func() []music.Track { d.revision.Get(); return d.res.Tracks }, row, "")
				}
				return core.Dynamic(func(*core.Scope) core.Node {
					d.revision.Get()
					i := row.Index()
					switch r.kind {
					case music.SearchAlbums:
						return a.albumCard(p, d.res.Albums[i])
					case music.SearchArtists:
						return a.artistCard(p, d.res.Artists[i])
					case music.SearchPlaylists:
						return a.playlistCard(p, d.res.Playlists[i])
					default:
						return a.showCard(p, d.res.Shows[i])
					}
				})
			},
		}
	})
}

func searchResultCount(res music.SearchResult, kind music.SearchKind) int {
	switch kind {
	case music.SearchTracks:
		return len(res.Tracks)
	case music.SearchAlbums:
		return len(res.Albums)
	case music.SearchArtists:
		return len(res.Artists)
	case music.SearchPlaylists:
		return len(res.Playlists)
	case music.SearchShows:
		return len(res.Shows)
	}
	return 0
}
