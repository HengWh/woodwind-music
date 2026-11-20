package app

import (
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

const radioSeedCount = 10

// Read the complete source collection before sampling, off the UI thread.
// Downloads, local imports and recent plays are not preference seeds.
func (a *app) radioSeeds(src music.Source) ([]music.Track, error) {
	tracks, err := allLiked(src)
	if err != nil {
		return nil, err
	}
	seeds := slices.DeleteFunc(tracks, func(t music.Track) bool { return t.ShowID != "" })
	if len(seeds) == 0 {
		return nil, music.Fail(music.ErrUnavailable, i18n.T("Like some songs before starting this station."))
	}
	return seeds, nil
}

func pickRadioSeeds(seeds []music.Track) []music.Track {
	copy := slices.Clone(seeds)
	rand.Shuffle(len(copy), func(i, j int) { copy[i], copy[j] = copy[j], copy[i] })
	return copy[:min(radioSeedCount, len(copy))]
}

func radioTrackKey(t music.Track) string {
	return "id:" + t.Source + "\x00" + t.ID
}

// Keep different source IDs for the same song out of one listening session.
func radioSongKey(t music.Track) string {
	name, artists := strings.ToLower(strings.TrimSpace(t.Name)), strings.ToLower(strings.TrimSpace(t.ArtistNames()))
	if name != "" && artists != "" {
		return "song:" + name + "\x00" + artists
	}
	return radioTrackKey(t)
}
