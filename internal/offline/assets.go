package offline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"net/url"
	"path/filepath"

	"github.com/HengWh/woodwind-music/internal/music"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Fill obtains missing companions once, outside playback. Existing data wins
// over failed refreshes; a removed record/source cannot be recreated by a late response.
func (s *Store) Fill(t music.Track, src music.Source, force bool) <-chan struct{} {
	idle := make(chan struct{})
	close(idle)
	if src == nil || t.LocalOnly {
		return idle
	}
	id := t.StoredID
	if id == "" {
		id = Key(t.Source, t.ID)
	}
	s.mu.Lock()
	r := s.records[id]
	if r != nil && t.Album.ImageURL == "" {
		t.Album.ImageURL = r.CoverURL
	}
	if pending := s.meta[id]; pending != nil {
		s.mu.Unlock()
		return pending
	}
	if s.closed || s.blocked[t.Source] || r == nil {
		s.mu.Unlock()
		return idle
	}
	cover := r.Cover == "" && (force || r.CoverState == "pending" || r.CoverState == "")
	lyric := r.Lyrics == nil && (force || r.LyricsState == "pending" || r.LyricsState == "")
	if !cover && !lyric {
		s.mu.Unlock()
		return idle
	}
	done := make(chan struct{})
	s.meta[id] = done
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer func() { s.mu.Lock(); delete(s.meta, id); close(done); s.mu.Unlock(); s.notify() }()
		// A bounded cover request cannot hold up audio playback.
		s.metaSem <- struct{}{}
		defer func() { <-s.metaSem }()
		s.mu.Lock()
		active := !s.closed && !s.blocked[t.Source] && s.records[id] != nil
		s.mu.Unlock()
		if !active {
			return
		}
		var art []byte
		var artErr error
		var l *music.Lyrics
		var lyricErr error
		if cover && force && t.Album.ImageURL == "" && t.Album.ID != "" && src.Can("album") {
			if al, err := src.Album(t.Album.ID); err == nil && al != nil {
				t.Album.ImageURL = al.Album.ImageURL
			}
		}
		if cover && t.Album.ImageURL != "" {
			art, artErr = music.Fetch(music.SizedImage(t.Album.ImageURL, 600))
			if artErr == nil {
				art, artErr = coverImage(art)
			}
		}
		if lyric {
			if src.Can("lyrics") {
				l, lyricErr = src.Lyrics(t.ID)
			} else {
				lyricErr = music.ErrUnsupported
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		r := s.records[id]
		if s.closed || s.blocked[t.Source] || r == nil {
			return
		}
		next := clone(r)
		if cover && next.Cover == "" {
			switch {
			case artErr != nil:
				next.CoverState = "failed"
			case len(art) == 0:
				next.CoverState = "unavailable"
			default:
				sum := sha256.Sum256(art)
				rel := filepath.Join("media", "artwork", hex.EncodeToString(sum[:])+".image")
				if err := s.copyFile(rel, art); err == nil {
					next.Cover = rel
					next.CoverState = "saved"
				} else {
					next.CoverState = "failed"
					s.lastError = err
				}
			}
		}
		if lyric && next.Lyrics == nil {
			switch {
			case errors.Is(lyricErr, music.ErrUnsupported) || errors.Is(lyricErr, music.ErrUnavailable):
				next.LyricsState = "unavailable"
			case lyricErr != nil:
				next.LyricsState = "failed"
			case l == nil || l.LRC == "" && len(l.Lines) == 0:
				next.LyricsState = "unavailable"
			default:
				next.Lyrics = l
				next.LyricsState = "saved"
			}
		}
		if s.save(&next) == nil {
			*r = next
			s.prune("")
		}
	}()
	return done
}

func safeArtworkURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	for k := range u.Query() {
		switch k {
		case "width", "height", "size", "format", "quality":
		default:
			return ""
		}
	}
	u.Fragment = ""
	return u.String()
}
func coverImage(b []byte) ([]byte, error) {
	if len(b) > 8<<20 {
		return nil, errors.New("cover image is too large")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 {
		return nil, errors.New("cover dimensions are too large")
	}
	im, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	w, h := config.Width, config.Height
	if max(w, h) > 600 {
		ratio := 600.0 / float64(max(w, h))
		w = max(1, int(float64(w)*ratio))
		h = max(1, int(float64(h)*ratio))
		scaled := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), im, im.Bounds(), draw.Src, nil)
		im = scaled
	}
	var out bytes.Buffer
	err = png.Encode(&out, im)
	return out.Bytes(), err
}

// Lyrics returns a stable snapshot, retaining true word timing and translations.
func (s *Store) Lyrics(t music.Track) (*music.Lyrics, bool) {
	r, ok := s.Get(t)
	if !ok {
		return nil, false
	}
	return r.Lyrics, r.Lyrics != nil
}

// Reconcile reports externally removed audio without silently redownloading it.
func (s *Store) Reconcile() {
	s.mu.Lock()
	for _, r := range s.records {
		next := clone(r)
		out := next.Variants[:0]
		for _, v := range next.Variants {
			if s.valid(v) {
				out = append(out, v)
			}
		}
		next.Variants = out
		if len(out) != len(r.Variants) {
			_ = s.save(&next)
			*r = next
		}
	}
	s.mu.Unlock()
	s.notify()
}

// RememberPlayed is intentionally separate from download or prefetch completion.
func (s *Store) RememberPlayed(t music.Track) { s.Touch(t); s.notify() }
