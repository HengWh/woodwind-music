package local

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/HengWh/woodwind-music/internal/lyrics"
	"github.com/HengWh/woodwind-music/internal/music"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/text/unicode/norm"
)

// Companions belong to the application, never to the listener's music folder.
// File identity prevents a replacement at the same path inheriting old lyrics.
type companion struct {
	Size   int64         `json:"size"`
	Mod    int64         `json:"mod"`
	Cover  bool          `json:"cover,omitempty"`
	Lyrics *music.Lyrics `json:"lyrics,omitempty"`
}

func (s *Source) companionsPath() string { return filepath.Join(s.dir, "companions", "index.json") }
func (s *Source) companionCover(id string) string {
	return filepath.Join(s.dir, "companions", id+".png")
}

func (s *Source) loadCompanions() {
	s.assets = map[string]companion{}
	s.assetBusy = map[string]chan struct{}{}
	s.assetRetry = map[string]time.Time{}
	if b, err := os.ReadFile(s.companionsPath()); err == nil {
		// A damaged cache must not stop the local library from opening.
		var saved map[string]companion
		if json.Unmarshal(b, &saved) == nil && saved != nil {
			s.assets = saved
		}
	}
}

func (s *Source) companionFor(it *item) companion {
	s.assetsMu.Lock()
	defer s.assetsMu.Unlock()
	c := s.assets[it.id]
	if c.Size != it.Size || c.Mod != it.Mod {
		return companion{}
	}
	return c
}

func (s *Source) companionLyrics(it *item) *music.Lyrics {
	if l := s.companionFor(it).Lyrics; l != nil {
		return l
	}
	return &music.Lyrics{}
}

func (s *Source) buildCatalog(entries []entry) *catalog {
	s.assetsMu.Lock()
	defer s.assetsMu.Unlock()
	for i := range entries {
		e := &entries[i]
		e.SupplementCover = ""
		id := makeID("track", e.Path)
		if c := s.assets[id]; c.Cover && c.Size == e.Size && c.Mod == e.Mod {
			path := s.companionCover(id)
			if _, err := os.Stat(path); err == nil {
				e.SupplementCover = path
			}
		}
	}
	return buildCatalog(entries)
}

func (s *Source) companionTrack(it *item) music.Track {
	t := it.public()
	if t.Album.ImageURL == "" && s.companionFor(it).Cover {
		path := s.companionCover(it.id)
		if _, err := os.Stat(path); err == nil {
			t.Album.ImageURL = music.FileURL(path)
		}
	}
	return t
}

// Refresh presentation from the existing entries without walking the music
// folders again. Scan publication takes the same lock and sees saved assets.
func (s *Source) refreshCompanions() {
	s.catalogMu.Lock()
	before := s.cat.Load()
	entries := make([]entry, len(before.items))
	for i, it := range before.items {
		entries[i] = it.entry
	}
	s.cat.Store(s.buildCatalog(entries))
	s.catalogMu.Unlock()
	s.notify()
}

// Complete runs on a background goroutine after playback starts. It only asks
// installed sources, only accepts a precise match, and deduplicates concurrent
// requests. Missing results are retried later rather than on every replay.
func (s *Source) Complete(t music.Track, providers []music.Source) (music.Track, *music.Lyrics) {
	it := s.cat.Load().byID[t.ID]
	if it == nil || t.LocalOnly {
		return t, nil
	}
	t = s.companionTrack(it)
	l, _ := s.Lyrics(t.ID)
	needLyrics := len(lyrics.FromSource(l)) == 0
	needCover := t.Album.ImageURL == ""
	if !needLyrics && !needCover {
		return t, l
	}
	var ids []string
	var sources []music.Source
	for _, src := range providers {
		if src != nil && src.Info().ID != ID && src.Can("search") {
			ids = append(ids, src.Info().ID+":"+src.Info().Version)
			sources = append(sources, src)
		}
	}
	if len(sources) == 0 {
		return t, l
	}
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%s", t.ID, it.Size, it.Mod, strings.Join(ids, "\x00"))
	s.assetsMu.Lock()
	if pending := s.assetBusy[key]; pending != nil {
		s.assetsMu.Unlock()
		<-pending
		if current := s.cat.Load().byID[t.ID]; current != nil {
			t = s.companionTrack(current)
			l, _ = s.Lyrics(t.ID)
		}
		return t, l
	}
	if time.Now().Before(s.assetRetry[key]) {
		s.assetsMu.Unlock()
		return t, l
	}
	done := make(chan struct{})
	s.assetBusy[key] = done
	s.assetsMu.Unlock()
	defer func() {
		s.assetsMu.Lock()
		delete(s.assetBusy, key)
		s.assetRetry[key] = time.Now().Add(30 * time.Minute)
		close(done)
		s.assetsMu.Unlock()
	}()
	var art []byte
	var found *music.Lyrics
	for _, src := range sources {
		result, err := src.Search(strings.TrimSpace(t.Name+" "+t.ArtistNames()), music.SearchTracks, 0)
		if err != nil || result == nil {
			continue
		}
		candidate, ok := companionMatch(t, result.Tracks)
		if !ok {
			continue
		}
		if needLyrics && found == nil && src.Can("lyrics") {
			if words, err := src.Lyrics(candidate.ID); err == nil && len(lyrics.FromSource(words)) > 0 {
				found = words
			}
		}
		if needCover && len(art) == 0 {
			raw := candidate.Album.ImageURL
			if raw == "" && candidate.Album.ID != "" && src.Can("album") {
				if album, err := src.Album(candidate.Album.ID); err == nil && album != nil {
					raw = album.Album.ImageURL
				}
			}
			if u, err := url.Parse(raw); err == nil && u.User == nil && (u.Scheme == "https" || u.Scheme == "http") {
				if b, err := music.Fetch(music.SizedImage(raw, 600)); err == nil {
					art = companionImage(b)
				}
			}
		}
		if (!needLyrics || found != nil) && (!needCover || len(art) > 0) {
			break
		}
	}
	if len(art) == 0 && found == nil {
		return t, l
	}
	current := s.cat.Load().byID[t.ID]
	if current == nil || current.Size != it.Size || current.Mod != it.Mod {
		return t, l
	}
	// A sidecar may have appeared while a source was searching. It keeps priority.
	if existing, _ := s.Lyrics(t.ID); len(lyrics.FromSource(existing)) > 0 {
		found = nil
		l = existing
	}
	s.assetsMu.Lock()
	c := s.assets[t.ID]
	if c.Size != it.Size || c.Mod != it.Mod {
		c = companion{Size: it.Size, Mod: it.Mod}
	}
	if len(art) > 0 && current.public().Album.ImageURL == "" {
		if err := writeAtomic(s.companionCover(t.ID), art); err == nil {
			c.Cover = true
		}
	}
	if found != nil {
		c.Lyrics = found
	}
	next := make(map[string]companion, len(s.assets)+1)
	for id, v := range s.assets {
		next[id] = v
	}
	next[t.ID] = c
	b, err := json.Marshal(next)
	if err == nil {
		err = writeAtomic(s.companionsPath(), b)
	}
	if err == nil {
		s.assets = next
	}
	s.assetsMu.Unlock()
	if err != nil {
		log.Printf("local: saving companions: %v", err)
		return t, l
	}
	if found != nil {
		l = found
	}
	s.refreshCompanions()
	if latest := s.cat.Load().byID[t.ID]; latest != nil {
		t = s.companionTrack(latest)
	}
	return t, l
}

// Keep version words (Live, remix, accompaniment, etc.) in the comparison.
// A title alone is insufficient: require every artist plus album or duration.
func companionMatch(t music.Track, candidates []music.Track) (music.Track, bool) {
	artists := companionArtists(t)
	if companionName(t.Name) == "" || len(artists) == 0 {
		return music.Track{}, false
	}
	var matches []music.Track
	for _, c := range candidates {
		if c.ID == "" || companionName(c.Name) != companionName(t.Name) || !slices.Equal(artists, companionArtists(c)) {
			continue
		}
		sameAlbum := companionName(t.Album.Name) != "" && companionName(t.Album.Name) == companionName(c.Album.Name)
		if t.Album.Name != "" && c.Album.Name != "" && !sameAlbum {
			continue
		}
		if t.Duration > 0 && c.Duration > 0 {
			tolerance := math.Min(5000, math.Max(3000, float64(t.Duration)/100))
			if math.Abs(float64(t.Duration-c.Duration)) > tolerance {
				continue
			}
		} else if !sameAlbum {
			continue
		}
		matches = append(matches, c)
	}
	if len(matches) != 1 {
		return music.Track{}, false
	}
	return matches[0], true
}

func companionName(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, norm.NFKC.String(s))
}
func companionArtists(t music.Track) []string {
	var names []string
	for _, a := range t.Artists {
		if n := companionName(a.Name); n != "" {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func companionImage(b []byte) []byte {
	if len(b) > 8<<20 {
		return nil
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || config.Width <= 0 || config.Height <= 0 || max(config.Width, config.Height) > 4096 {
		return nil
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	if max(config.Width, config.Height) > 600 {
		ratio := 600.0 / float64(max(config.Width, config.Height))
		resized := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(config.Width)*ratio)), max(1, int(float64(config.Height)*ratio))))
		draw.CatmullRom.Scale(resized, resized.Bounds(), im, im.Bounds(), draw.Src, nil)
		im = resized
	}
	var out bytes.Buffer
	if png.Encode(&out, im) != nil {
		return nil
	}
	return out.Bytes()
}
