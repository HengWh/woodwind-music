package app

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

type radioTestSource struct {
	music.Source
	mu       sync.Mutex
	stations []music.Station
	next     func(string) ([]music.Track, error)
	liked    map[string]bool
	stream   string
}

func (s *radioTestSource) Info() music.Info {
	return music.Info{ID: "radio-test", Name: "Radio Test", Navigation: []music.NavItem{{Builtin: "stations", Title: "私人 FM"}}}
}
func (s *radioTestSource) Can(name string) bool {
	return name == "stations" || name == "stationNext" || name == "like" || name == "liked"
}
func (s *radioTestSource) Stations() ([]music.Station, error)           { return s.stations, nil }
func (s *radioTestSource) StationNext(id string) ([]music.Track, error) { return s.next(id) }
func (s *radioTestSource) Stream(string, string) (*music.Stream, error) {
	return &music.Stream{URL: s.stream, Type: "wav"}, nil
}
func (s *radioTestSource) Like(id string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.liked[id] = on
	return nil
}
func (s *radioTestSource) Liked(int) (*music.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	page := &music.Page{}
	for id, on := range s.liked {
		if on {
			page.Tracks = append(page.Tracks, music.Track{ID: id, Source: s.Info().ID})
		}
	}
	return page, nil
}

func radioTestTracks(id string) []music.Track {
	var tracks []music.Track
	for _, n := range []string{"1", "2", "3", "4"} {
		tracks = append(tracks, music.Track{ID: id + n, Source: "radio-test", Name: "晚风 · " + id + n,
			Artists: []music.Artist{{Name: "木听风"}}, Duration: 60000})
	}
	return tracks
}

func startRadioTest(t *testing.T, phone bool) (*nsgotest.App, *app, *radioTestSource) {
	t.Helper()
	t.Setenv("WOODWIND_MUSIC_DATA_DIR", t.TempDir())
	path := filepath.Join(t.TempDir(), "radio.wav")
	writeToneWAV(t, path)
	wav, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	wav = append(wav[:44:44], bytes.Repeat(wav[44:], 60)...)
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(wav)-44))
	if err := os.WriteFile(path, wav, 0600); err != nil {
		t.Fatal(err)
	}
	src := &radioTestSource{liked: map[string]bool{}, stream: "file://" + path,
		stations: []music.Station{{ID: "a", Name: "私人 FM", Description: "听见更多好音乐"}, {ID: "b", Name: "探索"}, {ID: "c", Name: "漫游"}}}
	src.next = func(id string) ([]music.Track, error) { return radioTestTracks(id), nil }
	var a *app
	ui := nsgotest.Start(t, func(cx *core.Scope) core.Node { a = newApp(cx); return a.view(cx) }, nsgotest.Size(1180, 780))
	ui.WaitUntil(time.Second, func() bool { return !a.homeBusy })
	a.post(func() {
		src.Source = a.local
		a.sources.items[src.Info().ID] = src
		a.switchSource(src.Info().ID)
	})
	ui.WaitUntil(time.Second, func() bool { return !a.homeBusy && a.source() == src })
	if phone {
		ui.Resize(390, 844)
	}
	a.post(func() { a.open(route{page: pageStations}) })
	ui.WaitUntil(time.Second, func() bool { return ui.Exists("id:radio-start") })
	t.Cleanup(func() { a.post(a.pl.Clear); ui.Settle() })
	return ui, a, src
}

// Choosing a channel waits for Start; while listening it switches immediately,
// and the card's transport, likes and leave action operate on that radio.
func TestRadioCardInteractions(t *testing.T) {
	for _, phone := range []bool{false, true} {
		name := "desktop"
		if phone {
			name = "phone"
		}
		t.Run(name, func(t *testing.T) {
			ui, a, src := startRadioTest(t, phone)
			ui.Press("id:station-b")
			if a.pl.Radio() != "" {
				t.Fatal("selecting a channel started playback")
			}
			ui.Press("id:radio-start")
			ui.WaitUntil(2*time.Second, func() bool { return a.pl.Radio() == radioName(src.Info().ID, src.stations[1]) && a.pl.Ready() })
			ui.Press("id:radio-play")
			if a.pl.Playing() {
				t.Fatal("card did not pause playback")
			}
			ui.Press("id:radio-play")
			if !a.pl.Playing() {
				t.Fatal("card did not resume playback")
			}
			ui.Press("id:radio-next")
			ui.WaitUntil(time.Second, func() bool { return a.pl.Current().ID == "b2" && a.pl.Ready() })
			ui.Press("id:radio-like")
			ui.WaitUntil(time.Second, func() bool { return a.isTrackLiked(*a.pl.Current()) && !a.homeBusy && ui.Exists("id:station-c") })
			ui.Press("id:station-c")
			ui.WaitUntil(time.Second, func() bool {
				return a.pl.Radio() == radioName(src.Info().ID, src.stations[2]) && a.pl.Current().ID == "c1"
			})
			a.post(a.openNowPlaying)
			ui.Settle()
			ui.Press("id:radio-leave")
			if a.pl.Radio() != "" || a.pl.Current().ID != "c1" || len(a.pl.Queue()) != 1 {
				t.Fatal("leaving the radio did not keep only the current song")
			}
			if ui.Exists("id:radio-leave") {
				t.Fatal("leave chip stayed after leaving")
			}
		})
	}
}

// Unavailable batches must be retried only three times, and a failed tuning
// attempt must leave the existing queue and a usable Start button.
func TestRadioUnavailableAndErrors(t *testing.T) {
	ui, a, src := startRadioTest(t, false)
	calls := 0
	src.next = func(string) ([]music.Track, error) {
		calls++
		return []music.Track{{ID: "blocked", Unavailable: true}}, nil
	}
	ui.Press("id:radio-start")
	ui.WaitUntil(time.Second, func() bool { return a.radioStarting == "" && a.toast.GetUntracked() != "" })
	if calls != 3 || a.pl.Radio() != "" || a.pl.Current() != nil {
		t.Fatal("unavailable radio did not stop after three attempts", calls)
	}
	src.next = func(string) ([]music.Track, error) { return nil, errors.New("offline") }
	ui.Press("id:radio-start")
	ui.WaitUntil(time.Second, func() bool { return a.radioStarting == "" })
	if a.pl.Radio() != "" || a.pl.Current() != nil {
		t.Fatal("failed tuning replaced playback")
	}
	src.next = func(id string) ([]music.Track, error) { return radioTestTracks(id), nil }
	ui.Press("id:radio-start")
	ui.WaitUntil(time.Second, func() bool { return a.pl.Radio() != "" })
}

// Restoring FM keeps its source and channel, paused, and reconnects the feed.
// Subsequent batches append new songs without duplicating those already queued.
func TestRadioRestoresAndRefills(t *testing.T) {
	ui, a, src := startRadioTest(t, false)
	ui.Press("id:station-b")
	ui.Press("id:radio-start")
	ui.WaitUntil(time.Second, func() bool { return a.pl.Ready() && a.pl.Radio() != "" })
	a.post(func() { a.saveSession(); a.pl.Clear(); a.restoreSession() })
	ui.WaitUntil(time.Second, func() bool { return a.pl.Ready() && a.pl.Radio() != "" })
	if a.pl.Radio() != radioName(src.Info().ID, src.stations[1]) || a.pl.Playing() || a.pl.Current().ID != "b1" {
		t.Fatal("FM session did not restore its channel and paused track")
	}
	requested := make(chan string, 1)
	src.next = func(id string) ([]music.Track, error) {
		requested <- id
		tracks := radioTestTracks(id)
		return append(tracks[3:], music.Track{ID: id + "5", Source: src.Info().ID, Name: "More"}), nil
	}
	ui.Press("id:radio-next")
	ui.WaitUntil(time.Second, a.pl.Ready)
	ui.Press("id:radio-next")
	ui.WaitUntil(time.Second, func() bool { return len(a.pl.Queue()) == 5 })
	if id := <-requested; id != "b" {
		t.Fatal("restored feed requested the wrong channel", id)
	}
	if a.pl.Queue()[4].ID != "b5" {
		t.Fatal("feed did not deduplicate the existing song")
	}
}

// Starting a second channel while the first is waiting must use the latest
// selection even when both responses finish together.
func TestRadioLatestChannelWins(t *testing.T) {
	ui, a, src := startRadioTest(t, false)
	started := make(chan string, 2)
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	src.next = func(id string) ([]music.Track, error) {
		started <- id
		<-release
		return radioTestTracks(id), nil
	}
	ui.Press("id:radio-start")
	ui.WaitUntil(time.Second, func() bool { return len(started) == 1 })
	ui.Press("id:station-b")
	ui.WaitUntil(time.Second, func() bool { return len(started) == 2 })
	close(release)
	ui.WaitUntil(time.Second, func() bool {
		return a.radioStarting == "" && a.pl.Ready() && a.pl.Radio() == radioName(src.Info().ID, src.stations[1])
	})
	if a.pl.Current().ID != "b1" {
		t.Fatal("earlier channel replaced the latest selection")
	}
}

// FM screenshots cover the original desktop card and the stacked phone card.
func TestRadioShots(t *testing.T) {
	if os.Getenv("WOODWIND_RADIO_SHOTS") == "" {
		t.Skip("set WOODWIND_RADIO_SHOTS=1 for visual review")
	}
	ui, a, src := startRadioTest(t, false)
	_, file, _, _ := runtime.Caller(0)
	art := "file://" + filepath.Join(filepath.Dir(file), "../../resources/icon.png")
	ui.Screenshot("radio-desktop-idle")
	a.post(func() {
		tracks := radioTestTracks("a")
		for i := range tracks {
			tracks[i].Album.ImageURL = art
		}
		a.pl.Cue(tracks)
		a.pl.Hold(30 * time.Second)
		a.pl.AttachRadio(radioName(src.Info().ID, src.stations[0]), func(func([]music.Track)) {})
		a.playRev.Update(func(n int) int { return n + 1 })
	})
	ui.Settle()
	ui.Screenshot("radio-desktop-playing")
	ui.Resize(880, 580)
	ui.Screenshot("radio-desktop-narrow")
	ui.Resize(390, 844)
	ui.Screenshot("radio-phone-playing")
	ui.Resize(320, 568)
	ui.Screenshot("radio-phone-small")
	ui.Scroll("id:phone-page-scroll", 400)
	ui.Screenshot("radio-phone-small-controls")
	ui.Resize(1180, 780)
	a.post(func() { a.cx.SetAppearance(core.AppearanceLight); a.bump() })
	ui.Settle()
	ui.Screenshot("radio-desktop-light")
}
