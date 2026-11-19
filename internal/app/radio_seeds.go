package app

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

const radioSeedCount = 3

// radioSeeds reads explicit library choices only. Automatic playback caches
// and recent plays must not become positive preference signals.
// Called off the UI thread, through the sources' thread-safe read methods.
func (a *app) radioSeeds(src music.Source) ([]music.Track, error) {
	var seeds []music.Track
	seen := map[string]bool{}
	add := func(t music.Track, source string) {
		if t.Source == "" {
			t.Source = source
		}
		key := t.Source + "\x00" + t.ID
		if t.ID == "" || t.ShowID != "" || seen[key] {
			return
		}
		seen[key] = true
		seeds = append(seeds, t)
	}
	if a.local != nil {
		for offset := 0; ; {
			page, err := a.local.Search("", music.SearchTracks, offset)
			if err != nil {
				return nil, err
			}
			if page == nil {
				break
			}
			for _, t := range page.Tracks {
				add(t, a.local.Info().ID)
			}
			if !page.More || len(page.Tracks) == 0 {
				break
			}
			offset += len(page.Tracks)
		}
	}
	if a.disk != nil {
		for _, record := range a.disk.List() {
			if record.Pinned() {
				add(record.Track, record.Track.Source)
			}
		}
	}
	if src.Can("liked") {
		for offset := 0; ; {
			page, err := src.Liked(offset)
			if err != nil {
				// Keep explicit downloads usable when online likes cannot load.
				if len(seeds) == 0 {
					return nil, err
				}
				break
			}
			if page == nil {
				break
			}
			for _, t := range page.Tracks {
				add(t, src.Info().ID)
			}
			if !page.More || len(page.Tracks) == 0 {
				break
			}
			offset += len(page.Tracks)
		}
	}
	if len(seeds) == 0 {
		return nil, music.Fail(music.ErrUnavailable, i18n.T("Download or like some songs before starting this station."))
	}
	return seeds, nil
}

func pickRadioSeeds(seeds []music.Track) []music.Track {
	copy := slices.Clone(seeds)
	rand.Shuffle(len(copy), func(i, j int) { copy[i], copy[j] = copy[j], copy[i] })
	return copy[:min(radioSeedCount, len(copy))]
}

// Keep spelling variants of a track out of a single listening session too.
func radioTrackKey(t music.Track) string {
	name, artists := strings.ToLower(strings.TrimSpace(t.Name)), strings.ToLower(strings.TrimSpace(t.ArtistNames()))
	if name != "" && artists != "" {
		return name + "\x00" + artists
	}
	return t.Source + "\x00" + t.ID
}
