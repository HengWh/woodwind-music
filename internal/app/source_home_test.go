package app

import (
	"github.com/HengWh/woodwind-music/internal/music"
	"testing"
	"time"
)

type slowHomeSource struct {
	music.Source
	allowHome <-chan struct{}
}

func (s *slowHomeSource) Info() music.Info { return music.Info{ID: "slow-home", Name: "Slow Home"} }
func (s *slowHomeSource) Home() (*music.Browse, error) {
	<-s.allowHome
	return &music.Browse{Title: "Ready", Sections: []music.Section{{Title: "Collection", Tracks: []music.Track{{ID: "saved", Source: "slow-home", Name: "Saved song"}}}}}, nil
}
func (s *slowHomeSource) Playlists() ([]music.Playlist, error) {
	return []music.Playlist{{ID: "saved-list", Name: "Saved playlist", Mine: true}}, nil
}
func (s *slowHomeSource) Liked(int) (*music.Page, error) {
	return &music.Page{Tracks: []music.Track{{ID: "saved", Source: "slow-home", Name: "Saved song"}}}, nil
}
func (s *slowHomeSource) Recent() ([]music.Track, error) {
	return []music.Track{{ID: "recent", Source: "slow-home", Name: "Recent song"}}, nil
}

func TestHomeRequestsDoNotBlockLibraryAndWarmSourceSwitch(t *testing.T) {
	ui, a := startPhone(t)
	permits := make(chan struct{}, 1)
	defer close(permits)
	src := &slowHomeSource{Source: a.local, allowHome: permits}
	a.post(func() { a.sources.items[src.Info().ID] = src; a.switchSource(src.Info().ID) })
	ui.WaitUntil(time.Second, func() bool { return !a.playlistsBusy && !a.likedBusy && len(a.recent) == 1 })
	if !a.homeBusy || a.home != nil || len(a.mine) != 1 || len(a.liked) != 1 {
		t.Fatal("a pending home request blocked independent collection data")
	}
	ui.Press("id:phone-tab-library")
	ui.Find("id:playlist-saved-list")
	permits <- struct{}{}
	ui.WaitUntil(time.Second, func() bool { return !a.homeBusy && a.home != nil })
	a.post(func() { a.switchSource("local"); a.switchSource(src.Info().ID) })
	ui.Settle()
	if !a.homeBusy || a.home == nil || a.home.Title != "Ready" || len(a.mine) != 1 {
		t.Fatal("returning to a source discarded its warm content")
	}
	ui.Press("id:phone-tab-library")
	ui.Find("id:playlist-saved-list")
	a.post(func() { a.switchSource("local") })
	ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
	if a.home != nil && a.home.Title == "Ready" || len(a.mine) > 0 || len(a.liked) > 0 {
		t.Fatal("cached source content leaked into local music")
	}
}
