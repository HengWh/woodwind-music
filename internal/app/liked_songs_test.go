package app

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func favoriteTracks(count int) []music.Track {
	tracks := make([]music.Track, count)
	for i := range tracks {
		tracks[i] = music.Track{ID: fmt.Sprintf("HASH_%d_0", i+1), Name: fmt.Sprintf("Favorite %03d", i+1)}
	}
	return tracks
}

func favoritePage(tracks []music.Track, offset int) *music.Page {
	end := min(offset+100, len(tracks))
	return &music.Page{Tracks: slices.Clone(tracks[offset:end]), More: end < len(tracks)}
}

func TestAllLikedPaginationAndSampling(t *testing.T) {
	for _, count := range []int{0, 7, 10, 100, 310, 521} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			tracks := favoriteTracks(count)
			var offsets []int
			src := &libraryRadioSource{radioTestSource: &radioTestSource{}, likedPage: func(offset int) (*music.Page, error) {
				offsets = append(offsets, offset)
				return favoritePage(tracks, offset), nil
			}}
			got, err := allLiked(src)
			if err != nil || len(got) != count {
				t.Fatalf("collection: %d tracks, %v", len(got), err)
			}
			var wantOffsets []int
			for n := 0; n < max(count, 1); n += 100 {
				wantOffsets = append(wantOffsets, n)
			}
			if !slices.Equal(offsets, wantOffsets) {
				t.Fatalf("offsets %v, want %v", offsets, wantOffsets)
			}
			seeds, err := (&app{}).radioSeeds(src)
			if count == 0 {
				if !errors.Is(err, music.ErrUnavailable) || len(seeds) != 0 {
					t.Fatal("empty collection must not seed FM", seeds, err)
				}
				return
			}
			if err != nil || len(seeds) != count {
				t.Fatalf("seed collection: %d, %v", len(seeds), err)
			}
			seen := map[string]bool{}
			// Repeated real sampling checks the entire population, including
			// every song after the first page. Missing one after 2048 draws
			// of ten from 521 has probability below 4e-15.
			for range 2048 {
				batch := pickRadioSeeds(seeds)
				if len(batch) != min(10, count) {
					t.Fatal("wrong batch size", len(batch))
				}
				batchIDs := map[string]bool{}
				for _, tr := range batch {
					if batchIDs[tr.ID] || tr.Source != src.Info().ID {
						t.Fatal("duplicate or unscoped seed", tr)
					}
					batchIDs[tr.ID], seen[tr.ID] = true, true
				}
			}
			for i, tr := range seeds {
				if tr.ID != tracks[i].ID || !seen[tr.ID] {
					t.Fatalf("collection reordered or song excluded from sampling: %s", tr.ID)
				}
			}
		})
	}
}

func TestAllLikedRejectsPartialCollections(t *testing.T) {
	failure := errors.New("favorite page failed")
	for _, mode := range []string{"first error", "later error", "nil page", "empty continuation", "repeated continuation"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			src := &libraryRadioSource{radioTestSource: &radioTestSource{}, likedPage: func(offset int) (*music.Page, error) {
				calls++
				if calls > 3 {
					t.Fatal("pagination did not stop")
				}
				if mode == "first error" || mode == "later error" && offset > 0 {
					return nil, failure
				}
				if offset == 0 || mode == "repeated continuation" {
					return &music.Page{Tracks: favoriteTracks(100), More: true}, nil
				}
				if mode == "nil page" {
					return nil, nil
				}
				return &music.Page{More: true}, nil
			}}
			tracks, err := (&app{}).radioSeeds(src)
			if err == nil || len(tracks) != 0 {
				t.Fatalf("partial favorites escaped: %d, %v", len(tracks), err)
			}
			if (mode == "first error" || mode == "later error") && !errors.Is(err, failure) {
				t.Fatal("lost source error", err)
			}
		})
	}
}

func TestAllLikedDeduplicatesWithoutChangingOffsets(t *testing.T) {
	var offsets []int
	track := music.Track{ID: "a"}
	src := &libraryRadioSource{radioTestSource: &radioTestSource{}, likedPage: func(offset int) (*music.Page, error) {
		offsets = append(offsets, offset)
		if offset == 0 {
			return &music.Page{Tracks: []music.Track{track, track}, More: true}, nil
		}
		return &music.Page{Tracks: []music.Track{track, {ID: "b"}}}, nil
	}}
	tracks, err := allLiked(src)
	if err != nil || len(tracks) != 2 || !slices.Equal(offsets, []int{0, 2}) {
		t.Fatalf("deduplication: %+v, %v, %v", tracks, offsets, err)
	}
}

func TestLikedCollectionPublishesAllSongsOnDesktopAndPhone(t *testing.T) {
	for _, width := range []core.Points{1180, 390} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			t.Setenv("WOOD_WIND_MUSIC_DATA_DIR", t.TempDir())
			var a *app
			ui := nsgotest.Start(t, func(cx *core.Scope) core.Node {
				a = newApp(cx)
				return core.Dynamic(func(cx *core.Scope) core.Node {
					a.rev.Get()
					scroll := a.likedPage(cx, a.palette(cx)).(core.Scroll)
					scroll.TestID = "liked-scroll"
					return scroll
				})
			}, nsgotest.Size(width, 780))
			ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
			tracks := favoriteTracks(310)
			src := &libraryRadioSource{radioTestSource: &radioTestSource{Source: a.local}, likedPage: func(offset int) (*music.Page, error) {
				return favoritePage(tracks, offset), nil
			}}
			a.post(func() { a.active = src; a.sources.items[src.Info().ID] = src; a.reload() })
			ui.WaitUntil(2*time.Second, func() bool { return !a.homeBusy && !a.likedBusy })
			ui.Find("text:" + i18n.Songs(310))
			if len(a.liked) != 310 || len(a.likedSet) != 310 {
				t.Fatalf("collection and hearts: %d, %d", len(a.liked), len(a.likedSet))
			}
			for i, track := range tracks {
				if a.liked[i].ID != track.ID || !a.isTrackLiked(a.liked[i]) || !a.trackLikes[src.Info().ID][track.ID] {
					t.Fatal("missing song or heart", track.ID)
				}
			}
			scroll := ui.Find("id:liked-scroll")
			header := scroll.Content.Height - core.Points(310)*a.trackRowHeight()
			ui.ScrollTo("id:liked-scroll", core.Point{Y: header + 99*a.trackRowHeight()})
			ui.Find(`label:"Favorite 101"`)
			scroll = ui.Find("id:liked-scroll")
			ui.ScrollTo("id:liked-scroll", core.Point{Y: scroll.Content.Height - scroll.Frame.Size.Height})
			ui.Find(`label:"Favorite 310"`)

			// A failed later page preserves the last complete snapshot.
			src.likedPage = func(offset int) (*music.Page, error) {
				if offset > 0 {
					return nil, errors.New("later page failed")
				}
				return favoritePage(tracks, offset), nil
			}
			a.post(a.reload)
			ui.WaitUntil(time.Second, func() bool { return !a.homeBusy && a.toast.GetUntracked() == "later page failed" })
			if len(a.liked) != 310 || len(a.likedSet) != 310 {
				t.Fatal("failed reload replaced the complete collection")
			}
			src.likedPage = func(int) (*music.Page, error) { return &music.Page{}, nil }
			a.post(a.reload)
			ui.WaitUntil(time.Second, func() bool { return !a.homeBusy && !a.likedBusy })
			if len(a.liked) != 0 || len(a.likedSet) != 0 || len(a.trackLikes[src.Info().ID]) != 0 {
				t.Fatal("empty favorites retained stale songs or hearts")
			}
			ui.Find("text:" + i18n.Songs(0))
		})
	}
}
