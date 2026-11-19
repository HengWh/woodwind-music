package app

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/local"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/offline"
)

type libraryRadioSource struct {
	*radioTestSource
	likedPage func(int) (*music.Page, error)
	nextFrom  func(string, []music.Track) ([]music.Track, error)
}

func (s *libraryRadioSource) UsesLibrarySeeds(id string) bool { return id == "a" }
func (s *libraryRadioSource) StationNextFrom(id string, seeds []music.Track) ([]music.Track, error) {
	return s.nextFrom(id, seeds)
}
func (s *libraryRadioSource) Liked(offset int) (*music.Page, error) {
	return s.likedPage(offset)
}

func TestLibraryRadioSeedsUseDownloadsAndAllLikes(t *testing.T) {
	dir := t.TempDir()
	stored := seedStored(t, dir) // first explicitly pinned, second automatic
	disk, err := offline.Open(dir, 64<<20, true)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	files := t.TempDir()
	writeToneWAV(t, filepath.Join(files, "Imported.wav"))
	library := local.New(t.TempDir())
	library.AddFolders(files)
	deadline := time.Now().Add(2 * time.Second)
	for library.Scanning() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if library.Scanning() {
		t.Fatal("local scan did not finish")
	}
	a := &app{local: library, disk: disk}
	var offsets []int
	src := &libraryRadioSource{radioTestSource: &radioTestSource{Source: library}, likedPage: func(offset int) (*music.Page, error) {
		offsets = append(offsets, offset)
		if offset == 0 {
			return &music.Page{Tracks: []music.Track{stored[0], {ID: "liked-one", Name: "Liked"}}, More: true}, nil
		}
		return &music.Page{Tracks: []music.Track{{ID: "liked-two", Name: "Later page"}, {ID: "podcast", ShowID: "show"}}}, nil
	}}
	seeds, err := a.radioSeeds(src)
	if err != nil || len(seeds) != 4 || !slices.Equal(offsets, []int{0, 2}) {
		t.Fatalf("seeds: %+v, pages %v, error %v", seeds, offsets, err)
	}
	if slices.ContainsFunc(seeds, func(t music.Track) bool { return t.Source == stored[1].Source || t.ShowID != "" }) {
		t.Fatal("automatic caches or podcasts became preference seeds")
	}
	if !slices.ContainsFunc(seeds, func(t music.Track) bool { return t.Name == "Imported" && t.Source == local.ID }) || !slices.ContainsFunc(seeds, func(t music.Track) bool { return t.ID == "liked-two" && t.Source == src.Info().ID }) {
		t.Fatal("missing local import or source-scoped likes", seeds)
	}
	before := slices.Clone(seeds)
	if picked := pickRadioSeeds(seeds); len(picked) != radioSeedCount || len(seeds) != len(before) {
		t.Fatal("unexpected seed batch", picked)
	}
	for i := range seeds {
		if seeds[i].ID != before[i].ID {
			t.Fatal("sampling reordered the library")
		}
	}
	// An unavailable online likes list does not invalidate explicit downloads.
	src.likedPage = func(int) (*music.Page, error) { return nil, errors.New("offline") }
	if seeds, err := a.radioSeeds(src); err != nil || len(seeds) != 2 {
		t.Fatalf("download-only fallback: %+v, %v", seeds, err)
	}
}

func TestLibraryRadioFeedsAndDeduplicatesOnDesktopAndPhone(t *testing.T) {
	for _, phone := range []bool{false, true} {
		name := "desktop"
		if phone {
			name = "phone"
		}
		t.Run(name, func(t *testing.T) {
			ui, a, base := startRadioTest(t, phone)
			calls := make(chan []music.Track, 8)
			src := &libraryRadioSource{radioTestSource: base, likedPage: func(int) (*music.Page, error) {
				return &music.Page{Tracks: []music.Track{{ID: "liked", Name: "Favorite"}}}, nil
			}}
			src.nextFrom = func(id string, seeds []music.Track) ([]music.Track, error) {
				calls <- seeds
				tracks := radioTestTracks("recommended")
				if len(calls) > 1 {
					tracks = append(tracks, music.Track{ID: "fifth", Source: src.Info().ID, Name: "New Fifth"})
				}
				return append(tracks, tracks[0], music.Track{ID: "bad", Unavailable: true}), nil
			}
			base.next = func(string) ([]music.Track, error) { return nil, errors.New("legacy FM must not be used") }
			a.post(func() { a.sources.items[src.Info().ID] = src; a.active = src })
			ui.Settle()
			ui.Press("id:radio-start")
			ui.WaitUntil(2*time.Second, func() bool { return a.pl.Ready() && len(a.pl.Queue()) == 4 })
			if len(calls) != 1 {
				t.Fatal("unexpected initial calls", len(calls))
			}
			a.post(func() { a.saveSession(); a.pl.Clear(); a.restoreSession() })
			ui.WaitUntil(time.Second, func() bool { return a.pl.Ready() && a.pl.Radio() != "" })
			if a.pl.Playing() || a.pl.Current().ID != "recommended1" {
				t.Fatal("library FM did not restore paused at the same song")
			}
			ui.Press("id:radio-next")
			ui.WaitUntil(time.Second, a.pl.Ready)
			ui.Press("id:radio-next")
			ui.WaitUntil(2*time.Second, func() bool { return len(a.pl.Queue()) == 5 })
			if a.pl.Queue()[4].ID != "fifth" {
				t.Fatal("refill did not filter songs already in this session")
			}
			for len(calls) > 0 {
				seeds := <-calls
				if len(seeds) != 1 || seeds[0].ID != "liked" || seeds[0].Source != src.Info().ID {
					t.Fatal("FM did not use explicit, source-scoped likes", seeds)
				}
			}
		})
	}
}

func TestLibraryRadioRetriesAreBoundedAndCancelledRequestsStayOut(t *testing.T) {
	ui, a, base := startRadioTest(t, false)
	src := &libraryRadioSource{radioTestSource: base, likedPage: func(int) (*music.Page, error) {
		return &music.Page{Tracks: []music.Track{{ID: "liked"}}}, nil
	}}
	calls := 0
	src.nextFrom = func(string, []music.Track) ([]music.Track, error) { calls++; return nil, music.ErrUnavailable }
	a.post(func() { a.sources.items[src.Info().ID] = src; a.active = src })
	ui.Settle()
	ui.Press("id:radio-start")
	ui.WaitUntil(time.Second, func() bool { return a.radioStarting == "" && a.toast.GetUntracked() != "" })
	if calls != 3 || a.pl.Radio() != "" {
		t.Fatal("seed errors did not stop after three attempts", calls)
	}
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	src.nextFrom = func(string, []music.Track) ([]music.Track, error) {
		close(started)
		<-release
		return radioTestTracks("late"), nil
	}
	base.next = func(id string) ([]music.Track, error) { return radioTestTracks(id), nil }
	ui.Press("id:radio-start")
	ui.WaitUntil(time.Second, func() bool { return a.radioStarting != "" })
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("seed request did not start")
	}
	ui.Press("id:station-b")
	ui.WaitUntil(time.Second, func() bool { return a.pl.Ready() && a.pl.Current().ID == "b1" })
	revision := a.playRev.GetUntracked()
	close(release)
	ui.WaitUntil(time.Second, func() bool { return a.playRev.GetUntracked() > revision })
	if a.pl.Current().ID != "b1" {
		t.Fatal("cancelled seeded response replaced the new station")
	}
}

func TestLibraryRadioEmptySeedsAndFailuresKeepCurrentQueue(t *testing.T) {
	ui, a, base := startRadioTest(t, false)
	src := &libraryRadioSource{radioTestSource: base, likedPage: func(int) (*music.Page, error) { return &music.Page{}, nil }}
	src.nextFrom = func(string, []music.Track) ([]music.Track, error) {
		return nil, errors.New("must not request without seeds")
	}
	a.post(func() {
		a.sources.items[src.Info().ID] = src
		a.active = src
		a.pl.Cue(radioTestTracks("existing"))
	})
	ui.Settle()
	// Call directly because the existing track makes the card show transport.
	a.post(func() { a.playStation(src.stations[0]) })
	ui.WaitUntil(time.Second, func() bool {
		return a.radioStarting == "" && strings.Contains(a.toast.GetUntracked(), "Download or like")
	})
	if a.pl.Current().ID != "existing1" || len(a.pl.Queue()) != 4 || a.pl.Radio() != "" {
		t.Fatal("empty seeds replaced the existing queue")
	}
	src.likedPage = func(int) (*music.Page, error) { return &music.Page{Tracks: []music.Track{{ID: "liked"}}}, nil }
	src.nextFrom = func(string, []music.Track) ([]music.Track, error) { return nil, errors.New("seed service failed") }
	a.post(func() { a.playStation(src.stations[0]) })
	ui.WaitUntil(time.Second, func() bool { return a.radioStarting == "" && a.toast.GetUntracked() == "seed service failed" })
	if a.pl.Current().ID != "existing1" {
		t.Fatal("failed recommendation replaced the current song")
	}
}
