package local

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/HengWh/woodwind-music/internal/music"
)

type companionSource struct {
	music.Source
	tracks   []music.Track
	words    *music.Lyrics
	err      error
	searches atomic.Int32
	lyrics   atomic.Int32
	entered  chan struct{}
	release  chan struct{}
}

func (s *companionSource) Info() music.Info     { return music.Info{ID: "test-provider"} }
func (s *companionSource) Can(what string) bool { return what == "search" || what == "lyrics" }
func (s *companionSource) Search(string, music.SearchKind, int) (*music.SearchResult, error) {
	s.searches.Add(1)
	if s.entered != nil {
		s.entered <- struct{}{}
		<-s.release
	}
	return &music.SearchResult{Tracks: s.tracks}, s.err
}
func (s *companionSource) Lyrics(string) (*music.Lyrics, error) {
	s.lyrics.Add(1)
	return s.words, s.err
}

func companionFixture(t *testing.T) (*Source, music.Track, string) {
	t.Helper()
	folder := t.TempDir()
	path := write(t, filepath.Join(folder, "阴天.mp3"), mp3With(t, [2]string{"TIT2", "阴天"}, [2]string{"TPE1", "莫文蔚"}, [2]string{"TALB", "你可以"}, [2]string{"USLT", "[00:01invalid embedded lyrics"}))
	s := New(t.TempDir())
	s.AddFolders(folder)
	wait(t, s)
	return s, songNamed(t, s, "阴天"), path
}

func TestCompanionMatch(t *testing.T) {
	original := music.Track{ID: "local", Name: "阴天", Artists: []music.Artist{{Name: "莫文蔚"}}, Album: music.Album{Name: "你可以"}, Duration: 240000}
	for _, tc := range []struct {
		name string
		edit func(*music.Track)
		want bool
	}{
		{"exact", func(*music.Track) {}, true},
		{"small duration difference", func(t *music.Track) { t.Duration += 2500 }, true},
		{"live", func(t *music.Track) { t.Name += " (Live)" }, false},
		{"different artist", func(t *music.Track) { t.Artists = []music.Artist{{Name: "其他歌手"}} }, false},
		{"different album", func(t *music.Track) { t.Album.Name = "演唱会" }, false},
		{"wrong duration", func(t *music.Track) { t.Duration += 15000 }, false},
		{"album without duration", func(t *music.Track) { t.Duration = 0 }, true},
		{"no album or duration", func(t *music.Track) { t.Duration = 0; t.Album.Name = "" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := original
			candidate.ID = "online"
			tc.edit(&candidate)
			_, ok := companionMatch(original, []music.Track{candidate})
			if ok != tc.want {
				t.Fatalf("matched = %v", ok)
			}
		})
	}
	duplicate := original
	duplicate.ID = "other"
	if _, ok := companionMatch(original, []music.Track{original, duplicate}); ok {
		t.Fatal("ambiguous version matched")
	}
	original.Artists = nil
	if _, ok := companionMatch(original, []music.Track{original}); ok {
		t.Fatal("title alone matched")
	}
}

func TestCompanionsPersistAndLocalFilesWin(t *testing.T) {
	s, tr, path := companionFixture(t)
	before, _ := os.ReadFile(path)
	var data bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&data, im); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write(data.Bytes()) }))
	defer server.Close()
	candidate := tr
	candidate.ID = "remote"
	candidate.Album.ImageURL = server.URL + "/cover.png"
	words := &music.Lyrics{Lines: []music.LyricLine{{Start: 1000, End: 3000, Text: "歌词", Translation: "translation", Words: []music.LyricWord{{Start: 1000, End: 3000, Text: "歌词"}}}}}
	src := &companionSource{tracks: []music.Track{candidate}, words: words}
	completed, l := s.Complete(tr, []music.Source{src})
	wait(t, s)
	if completed.Album.ImageURL == "" || !reflect.DeepEqual(l, words) {
		t.Fatalf("not completed: %+v, %+v", completed, l)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, before) {
		t.Fatal("original audio changed")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "阴天.lrc")); !os.IsNotExist(err) {
		t.Fatal("wrote into music folder")
	}
	restarted := New(s.dir)
	cached := songNamed(t, restarted, "阴天")
	if cached.Album.ImageURL != completed.Album.ImageURL {
		t.Fatal("cover missing after restart")
	}
	if b, err := music.Fetch(cached.Album.ImageURL); err != nil || len(b) == 0 {
		t.Fatal("cover not saved", err)
	}
	_, l = restarted.Complete(cached, []music.Source{src})
	if !reflect.DeepEqual(l, words) || src.searches.Load() != 1 || requests.Load() != 1 {
		t.Fatal("cached data was requested again or word timing was lost")
	}
	write(t, filepath.Join(filepath.Dir(path), "阴天.lrc"), []byte("[00:01.00]本地歌词"))
	write(t, filepath.Join(filepath.Dir(path), "阴天.png"), data.Bytes())
	restarted.Rescan()
	wait(t, restarted)
	local := songNamed(t, restarted, "阴天")
	_, l = restarted.Complete(local, []music.Source{src})
	if l.LRC != "[00:01.00]本地歌词" || local.Album.ImageURL == completed.Album.ImageURL {
		t.Fatal("download replaced local companions")
	}
	// Replacing the audio at its path invalidates its downloaded companions.
	os.Remove(filepath.Join(filepath.Dir(path), "阴天.lrc"))
	os.Remove(filepath.Join(filepath.Dir(path), "阴天.png"))
	write(t, path, append(before, 0))
	restarted.Rescan()
	wait(t, restarted)
	changed := songNamed(t, restarted, "阴天")
	l, _ = restarted.Lyrics(changed.ID)
	if l.LRC != "" || len(l.Lines) > 0 || changed.Album.ImageURL != "" {
		t.Fatal("replacement inherited old companions")
	}
}

func TestCompanionsFailureBackoffAndPartialCompletion(t *testing.T) {
	s, tr, path := companionFixture(t)
	write(t, filepath.Join(filepath.Dir(path), "阴天.lrc"), []byte("empty or invalid lyrics"))
	src := &companionSource{err: errors.New("offline")}
	s.Complete(tr, []music.Source{src})
	s.Complete(tr, []music.Source{src})
	if src.searches.Load() != 1 {
		t.Fatal("failed request repeated on replay")
	}
	// A provider becoming available after the retry delay can fill just lyrics.
	s.assetsMu.Lock()
	clear(s.assetRetry)
	s.assetsMu.Unlock()
	candidate := tr
	candidate.ID = "online"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("not an image")) }))
	defer server.Close()
	candidate.Album.ImageURL = server.URL
	src.err = nil
	src.tracks = []music.Track{candidate}
	src.words = &music.Lyrics{LRC: "[00:01.00]歌词"}
	completed, l := s.Complete(tr, []music.Source{src})
	if completed.Album.ImageURL != "" || l.LRC != src.words.LRC {
		t.Fatal("one failed companion blocked the other")
	}
	l, _ = New(s.dir).Lyrics(tr.ID)
	if l.LRC != src.words.LRC {
		t.Fatal("partial completion was not persisted")
	}
}

func TestCompanionsKeepExistingLyricsAndDeduplicate(t *testing.T) {
	s, tr, path := companionFixture(t)
	write(t, filepath.Join(filepath.Dir(path), "阴天.lrc"), []byte("[00:01.00]已有歌词"))
	candidate := tr
	candidate.ID = "remote"
	src := &companionSource{tracks: []music.Track{candidate}, words: &music.Lyrics{LRC: "[00:01.00]替换"}, entered: make(chan struct{}, 1), release: make(chan struct{})}
	done := make(chan struct{})
	go func() { defer close(done); s.Complete(tr, []music.Source{src}) }()
	<-src.entered
	second := make(chan struct{})
	go func() { defer close(second); s.Complete(tr, []music.Source{src}) }()
	close(src.release)
	<-done
	<-second
	l, _ := s.Lyrics(tr.ID)
	if src.searches.Load() != 1 || src.lyrics.Load() != 0 || l.LRC != "[00:01.00]已有歌词" {
		t.Fatal("duplicate request or overwritten lyrics")
	}
}

func TestCompanionsKeepExistingCover(t *testing.T) {
	s, tr, path := companionFixture(t)
	var art bytes.Buffer
	if err := png.Encode(&art, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	cover := write(t, filepath.Join(filepath.Dir(path), "阴天.png"), art.Bytes())
	s.Rescan()
	wait(t, s)
	tr = songNamed(t, s, "阴天")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	remote := tr
	remote.ID = "remote"
	remote.Album.ImageURL = server.URL
	src := &companionSource{tracks: []music.Track{remote}, words: &music.Lyrics{LRC: "[00:01.00]歌词"}}
	completed, l := s.Complete(tr, []music.Source{src})
	if completed.Album.ImageURL != music.FileURL(cover) || requests.Load() != 0 || l.LRC != src.words.LRC {
		t.Fatal("existing cover was fetched or replaced instead of filling only lyrics")
	}
}
