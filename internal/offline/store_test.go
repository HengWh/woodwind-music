package offline

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
)

type testSource struct {
	music.Source
	url    string
	calls  atomic.Int32
	lyrics atomic.Int32
	bad    bool
}

func TestWaitingNetworkResumesWithoutDownloadingTwice(t *testing.T) {
	b := audio(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	defer srv.Close()
	s := store(t, t.TempDir())
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	var network atomic.Bool
	s.Network(true, network.Load)
	if err := s.Pin(track(), "high", false); err != nil {
		t.Fatal(err)
	}
	taskUntil(t, s, func(tasks []Task) bool { return len(tasks) == 1 && tasks[0].State == "waiting-network" })
	if src.calls.Load() != 0 {
		t.Fatal("resolved stream before permitted network")
	}
	network.Store(true)
	s.ResumeWaiting()
	taskUntil(t, s, func(tasks []Task) bool { return len(tasks) == 1 && tasks[0].State == "saved" })
	if src.calls.Load() != 1 {
		t.Fatal("download was not resumed exactly once")
	}
}
func taskUntil(t *testing.T, s *Store, predicate func([]Task) bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		if predicate(s.Tasks()) {
			return
		}
		select {
		case <-deadline:
			t.Fatal("download task did not reach expected state", s.Tasks())
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestUpgradeFailureKeepsPinnedOriginal(t *testing.T) {
	b := audio(t)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Write(b)
		} else {
			io.WriteString(w, "<html>membership expired</html>")
		}
	}))
	defer srv.Close()
	s := store(t, t.TempDir())
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	if err := s.Pin(track(), "high", false); err != nil {
		t.Fatal(err)
	}
	taskUntil(t, s, func(tasks []Task) bool { return len(tasks) == 1 && tasks[0].State == "saved" })
	before, _ := s.Get(track())
	if err := s.Pin(track(), "lossless", false); err != nil {
		t.Fatal(err)
	}
	taskUntil(t, s, func(tasks []Task) bool { return slicesContainsState(tasks, "failed") })
	after, _ := s.Get(track())
	if len(after.Variants) != 1 || !after.Pinned() || before.Variants[0].Path != after.Variants[0].Path {
		t.Fatal("upgrade damaged old copy")
	}
}
func slicesContainsState(tasks []Task, state string) bool {
	for _, t := range tasks {
		if t.State == state {
			return true
		}
	}
	return false
}

func TestCapacityRespectsLeasesAndDoesNotResurrectLegacy(t *testing.T) {
	b := audio(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	defer srv.Close()
	dir := t.TempDir()
	s := store(t, dir)
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	first := track()
	h, e := s.Acquire(first, "high")
	if e != nil {
		t.Fatal(e)
	}
	wait(t, h)
	if e = s.Pin(first, "", false); e != nil {
		t.Fatal(e)
	}
	h.Close()
	second := first
	second.ID = "two"
	h, e = s.Acquire(second, "high")
	if e != nil {
		t.Fatal(e)
	}
	wait(t, h)
	s.Configure(1, true)
	if !s.Has(second) {
		t.Fatal("evicted active file")
	}
	h.Close()
	if s.Has(second) || !s.Has(first) {
		t.Fatal("did not evict unleased automatic copy")
	}
	s.Close()
	s = store(t, dir)
	if len(s.List()) != 1 || !s.List()[0].Pinned() {
		t.Fatal("restart lost pin or resurrected cache")
	}
}

func (s *testSource) Info() music.Info {
	return music.Info{ID: "test", Name: "Test source", Playback: music.PlaybackStream}
}
func (s *testSource) Can(name string) bool { return name == "lyrics" || name == "stream" }
func (s *testSource) Stream(id, q string) (*music.Stream, error) {
	s.calls.Add(1)
	return &music.Stream{URL: s.url, Type: "mp3", Quality: "high", Headers: map[string]string{"Authorization": "secret"}}, nil
}
func (s *testSource) Lyrics(id string) (*music.Lyrics, error) {
	s.lyrics.Add(1)
	return &music.Lyrics{Lines: []music.LyricLine{{Start: 100, End: 500, Text: "hello", Translation: "你好", Words: []music.LyricWord{{Start: 100, End: 500, Text: "hello"}}}}}, nil
}
func audio(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../player/testdata/seek.mp3")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func store(t *testing.T, dir string) *Store {
	t.Helper()
	s, e := Open(dir, 64<<20, true)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s
}
func wait(t *testing.T, h *Handle) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		p := h.Snapshot()
		if p.Done {
			if p.Err != nil {
				t.Fatal(p.Err)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("transfer did not finish")
		case <-p.Changed:
		}
	}
}
func track() music.Track {
	return music.Track{ID: "one", Source: "test", Name: "Song", Album: music.Album{ID: "album", Name: "Album"}, Artists: []music.Artist{{ID: "artist", Name: "Artist"}}}
}

func TestLibrarySurvivesPluginRemovalAndRestart(t *testing.T) {
	b := audio(t)
	var cover bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	_ = png.Encode(&cover, img)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cover" {
			w.Write(cover.Bytes())
		} else {
			w.Write(b)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	s := store(t, dir)
	src := &testSource{url: srv.URL + "/audio?token=secret"}
	s.SetSource(func(string) music.Source { return src })
	tr := track()
	tr.Album.ImageURL = srv.URL + "/cover?token=secret"
	if e := s.Remember(tr, src.Info().Name); e != nil {
		t.Fatal(e)
	}
	h, e := s.Acquire(tr, "high")
	if e != nil {
		t.Fatal(e)
	}
	wait(t, h)
	<-s.Fill(tr, src, false)
	if e = s.Pin(tr, "", false); e != nil {
		t.Fatal(e)
	}
	r, _ := s.Get(tr)
	if !r.Pinned() || r.Cover == "" || r.Lyrics == nil || len(r.Lyrics.Lines[0].Words) != 1 {
		t.Fatalf("incomplete saved record: %+v", r)
	}
	h.Close()
	s.RemoveSource("test")
	s.Close()
	srv.Close()
	s = store(t, dir)
	r, _ = s.Get(tr)
	local := s.Track(r)
	h, e = s.Acquire(local, "lossless")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	st, e := h.Info(context.Background())
	if e != nil || st.Quality != "high" {
		t.Fatalf("local quality fallback: %+v %v", st, e)
	}
	got := make([]byte, len(b))
	if _, e = h.ReadAt(got, 0); e != nil || !bytes.Equal(got, b) {
		t.Fatal("local copy differs", e)
	}
	meta, e := os.ReadFile(filepath.Join(dir, "media", "records", r.ID+".json"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(meta), "secret") || strings.Contains(string(meta), "Authorization") {
		t.Fatal("credentials persisted")
	}
	if src.calls.Load() != 1 {
		t.Fatal("local playback asked source again")
	}
	if e = s.ClearAutomatic(); e != nil {
		t.Fatal(e)
	}
	if len(s.List()) != 1 {
		t.Fatal("clearing cache deleted pinned song")
	}
}

func TestTransferSharesPlaybackAndPin(t *testing.T) {
	b := audio(t)
	release := make(chan struct{})
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write(b[:1024])
		w.(http.Flusher).Flush()
		<-release
		w.Write(b[1024:])
	}))
	defer srv.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	s := store(t, t.TempDir())
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	tr := track()
	h, e := s.Acquire(tr, "high")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = h.Info(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = s.Pin(tr, "high", false); e != nil {
		t.Fatal(e)
	}
	close(release)
	wait(t, h)
	h.Close()
	r, _ := s.Get(tr)
	if !r.Pinned() || requests.Load() != 1 || src.calls.Load() != 1 {
		t.Fatalf("not shared/pinned: %+v, calls %d/%d", r, requests.Load(), src.calls.Load())
	}
}

func TestClearCannotResurrectInflightCache(t *testing.T) {
	b := audio(t)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(b[:1024])
		w.(http.Flusher).Flush()
		<-release
		w.Write(b[1024:])
	}))
	defer srv.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	s := store(t, t.TempDir())
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	h, e := s.Acquire(track(), "high")
	if e != nil {
		t.Fatal(e)
	}
	h.Info(context.Background())
	if e = s.ClearAutomatic(); e != nil {
		t.Fatal(e)
	}
	close(release)
	wait(t, h)
	h.Close()
	if len(s.List()) != 0 {
		t.Fatal("a completed old transfer resurrected cleared cache")
	}
}

func TestNativeRangeUsesOneUpstreamAndSurvivesDeletion(t *testing.T) {
	b := audio(t)
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write(b) }))
	defer srv.Close()
	s := store(t, t.TempDir())
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	h, e := s.Acquire(track(), "high")
	if e != nil {
		t.Fatal(e)
	}
	wait(t, h)
	url, unroute, e := h.URL()
	if e != nil {
		t.Fatal(e)
	}
	defer unroute()
	defer h.Close()
	r, _ := s.Get(track())
	if e = s.Delete(r.ID); e != nil {
		t.Fatal(e)
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Range", "bytes=100-199")
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	got, e := io.ReadAll(res.Body)
	if e != nil || res.StatusCode != 206 || !bytes.Equal(got, b[100:200]) {
		t.Fatalf("range: %d, %v, %d", res.StatusCode, e, len(got))
	}
	if requests.Load() != 1 || len(s.List()) != 0 {
		t.Fatal("range redownloaded source or deletion failed")
	}
}

func TestLegacyMigrationKeepsUnidentifiedAudio(t *testing.T) {
	dir := t.TempDir()
	if e := os.MkdirAll(filepath.Join(dir, "songs"), 0o700); e != nil {
		t.Fatal(e)
	}
	b := audio(t)
	tr := track()
	id := Key(tr.Source, tr.ID)
	base := filepath.Join(dir, "songs", id+"-high")
	os.WriteFile(base+".mp3", b, 0o600)
	st, _ := json.Marshal(music.Stream{Type: "mp3", Quality: "high", Size: int64(len(b))})
	os.WriteFile(base+".json", st, 0o600)
	s := store(t, dir)
	records := s.List()
	if len(records) != 1 || !records[0].Legacy {
		t.Fatal("opaque old file was not listed")
	}
	s.AttachLegacy([]music.Track{tr})
	r, _ := s.Get(tr)
	if r.Track.Name != "Song" || r.Legacy {
		t.Fatal("queue did not identify legacy cache")
	}
	s.Close()
	s = store(t, dir)
	if len(s.List()) != 1 {
		t.Fatal("migration duplicated audio")
	}
	// A deleted legacy file may still be leased by a player when the app exits.
	r, _ = s.Get(tr)
	h, err := s.Acquire(s.Track(r), "high")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err = s.Delete(r.ID); err != nil {
		t.Fatal(err)
	}
	restarted := store(t, dir)
	if len(restarted.List()) != 0 {
		t.Fatal("leased legacy audio was resurrected after deletion")
	}
}

func TestOfflineNeverFallsBackAndInvalidBodyIsNotSaved(t *testing.T) {
	s := store(t, t.TempDir())
	tr := track()
	tr.LocalOnly = true
	if _, e := s.Acquire(tr, "high"); e != ErrMissing {
		t.Fatal(e)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "<html>not audio</html>") }))
	defer srv.Close()
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	tr.LocalOnly = false
	h, e := s.Acquire(tr, "high")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		p := h.Snapshot()
		if p.Done {
			if p.Err == nil {
				t.Fatal("HTML passed audio validation")
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out")
		case <-p.Changed:
		}
	}
	if len(s.List()) != 0 {
		t.Fatal("invalid audio was indexed")
	}
}

func TestDisabledAutomaticCacheStillAllowsPinnedDownloads(t *testing.T) {
	b := audio(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	defer srv.Close()
	s := store(t, t.TempDir())
	s.Configure(0, false)
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	if e := s.Pin(track(), "high", false); e != nil {
		t.Fatal(e)
	}
	deadline := time.After(5 * time.Second)
	for {
		r, ok := s.Get(track())
		if ok && r.Pinned() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("pin did not finish with cache disabled")
		case <-time.After(5 * time.Millisecond):
		}
	}
	s.Configure(0, false)
	if len(s.List()) != 1 {
		t.Fatal("disabled setting deleted pinned audio")
	}
}

func TestBrokenRecordRecoversAudioWithoutPlugin(t *testing.T) {
	b := audio(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	defer srv.Close()
	dir := t.TempDir()
	s := store(t, dir)
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	h, err := s.Acquire(track(), "high")
	if err != nil {
		t.Fatal(err)
	}
	wait(t, h)
	h.Close()
	r, _ := s.Get(track())
	s.Close()
	if err = os.WriteFile(filepath.Join(dir, "media", "records", r.ID+".json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	recovered := store(t, dir)
	records := recovered.List()
	if len(records) != 1 || !records[0].Legacy || !records[0].Pinned() {
		t.Fatal("lost audio when its record was damaged")
	}
	h, err = recovered.Acquire(recovered.Track(records[0]), "lossless")
	if err != nil {
		t.Fatal(err)
	}
	h.Close()
}

func TestMissingPinnedAudioCreatesRepairTask(t *testing.T) {
	b := audio(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(b) }))
	defer srv.Close()
	dir := t.TempDir()
	s := store(t, dir)
	src := &testSource{url: srv.URL}
	s.SetSource(func(string) music.Source { return src })
	h, err := s.Acquire(track(), "high")
	if err != nil {
		t.Fatal(err)
	}
	wait(t, h)
	if err = s.Pin(track(), "", false); err != nil {
		t.Fatal(err)
	}
	h.Close()
	r, _ := s.Get(track())
	s.Close()
	if err = os.Remove(filepath.Join(dir, r.Variants[0].Path)); err != nil {
		t.Fatal(err)
	}
	restarted := store(t, dir)
	tasks := restarted.Tasks()
	if len(restarted.List()) != 0 || len(tasks) != 1 || tasks[0].State != "failed" {
		t.Fatal("missing saved audio was still reported as available", tasks)
	}
}
