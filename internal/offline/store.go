// Package offline owns saved audio and its description independently of plugins.
package offline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
)

var ErrMissing = errors.New("saved audio is not available on this device")
var ErrSource = errors.New("the original music plugin is not available")

type Variant struct {
	Hash    string       `json:"sha256,omitempty"`
	Path    string       `json:"path"`
	Stream  music.Stream `json:"stream"`
	Pinned  bool         `json:"pinned,omitempty"`
	Created int64        `json:"created"`
}

type Record struct {
	CoverURL    string        `json:"coverUrl,omitempty"`
	Version     int           `json:"version"`
	ID          string        `json:"id"`
	Track       music.Track   `json:"track"`
	SourceName  string        `json:"sourceName"`
	Variants    []Variant     `json:"variants"`
	Cover       string        `json:"cover,omitempty"`
	Lyrics      *music.Lyrics `json:"lyrics,omitempty"`
	CoverState  string        `json:"coverState,omitempty"`
	LyricsState string        `json:"lyricsState,omitempty"`
	Created     int64         `json:"created"`
	Played      int64         `json:"played,omitempty"`
	Legacy      bool          `json:"legacy,omitempty"`
}

func (r Record) Pinned() bool {
	return slices.ContainsFunc(r.Variants, func(v Variant) bool { return v.Pinned })
}
func (r Record) Size() int64 {
	var n int64
	for _, v := range r.Variants {
		n += v.Stream.Size
	}
	return n
}

type Usage struct{ Automatic, Pinned, Total, Temporary int64 }

// Store serializes record changes. Audio readers hold path leases, so deletion
// never closes a file belonging to a player (including on Windows).
type Store struct {
	deletedLegacy map[string]bool
	mu            sync.Mutex
	dir           string
	records       map[string]*Record
	jobs          map[string]*job
	tasks         map[string]*Task
	leases        map[string]int
	garbage       map[string]bool
	blocked       map[string]bool
	meta          map[string]chan struct{}
	limit         int64
	enabled       bool
	generation    uint64
	closed        bool
	changed       func()
	sem           chan struct{}
	metaSem       chan struct{}
	wg            sync.WaitGroup
	// Network returns true when online and unmetered; nil is a desktop network.
	network   func() bool
	wifiOnly  bool
	server    *localServer
	source    func(string) music.Source
	lastError error
}

func Key(source, id string) string {
	sum := sha256.Sum256([]byte(source + "\x00" + id))
	return hex.EncodeToString(sum[:16])
}

func Open(dir string, limit int64, enabled bool) (*Store, error) {
	s := &Store{dir: dir, records: map[string]*Record{}, jobs: map[string]*job{}, tasks: map[string]*Task{}, leases: map[string]int{}, garbage: map[string]bool{}, blocked: map[string]bool{}, meta: map[string]chan struct{}{}, limit: limit, enabled: enabled, sem: make(chan struct{}, 2), metaSem: make(chan struct{}, 2)}
	for _, part := range []string{"records", "audio", "artwork", "temp"} {
		if err := os.MkdirAll(filepath.Join(dir, "media", part), 0o700); err != nil {
			return nil, err
		}
	}
	// macOS exposes temporary directories through /var -> /private/var.
	// Compare canonical roots so legitimate files do not look like escapes.
	if canonical, err := filepath.EvalSymlinks(dir); err == nil {
		dir = canonical
		s.dir = canonical
	}
	s.deletedLegacy = map[string]bool{}
	if b, err := os.ReadFile(filepath.Join(dir, "media", "deleted-legacy.json")); err == nil {
		if json.Unmarshal(b, &s.deletedLegacy) != nil {
			return nil, errors.New("could not read legacy cache cleanup journal")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	recoveryIDs := map[string]bool{}
	var missing []Task
	files, err := filepath.Glob(filepath.Join(dir, "media", "records", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var r Record
		if json.Unmarshal(b, &r) != nil {
			id := strings.TrimSuffix(filepath.Base(p), ".json")
			if !safeID(id) {
				return nil, errors.New("invalid saved music record")
			}
			s.records[id] = &Record{Version: 1, ID: id, Track: music.Track{ID: id, StoredID: id, Name: "Unidentified cached song"}, SourceName: "Unknown source", Legacy: true, Created: time.Now().UnixMilli()}
			recoveryIDs[id] = true
			continue
		}
		if r.Version != 1 || !safeID(r.ID) || filepath.Base(p) != r.ID+".json" {
			return nil, errors.New("unsupported saved music record version or identity")
		}
		r.CoverURL = safeArtworkURL(r.CoverURL)
		r.Track = cleanTrack(r.Track)
		r.Track.StoredID = r.ID
		for _, v := range r.Variants {
			if v.Pinned && !s.valid(v) {
				q := v.Stream.Quality
				if q == "" {
					q = "high"
				}
				missing = append(missing, Task{ID: Key(r.ID, q), RecordID: r.ID, Track: r.Track, Quality: q, State: "failed", Error: "The saved audio file is missing or incomplete", Created: time.Now().UnixMilli()})
			}
		}
		r.Variants = slices.DeleteFunc(r.Variants, func(v Variant) bool { return !s.valid(v) })
		if filepath.Dir(r.Cover) != filepath.Join("media", "artwork") || !s.exists(r.Cover) {
			r.Cover = ""
			if r.CoverState == "saved" {
				r.CoverState = "failed"
			}
		}
		s.records[r.ID] = &r
	}
	var tasks []Task
	if b, err := os.ReadFile(filepath.Join(dir, "media", "tasks.json")); err == nil {
		if json.Unmarshal(b, &tasks) != nil {
			return nil, errors.New("could not read offline download tasks")
		}
		for _, t := range tasks {
			t.Track = cleanTrack(t.Track)
			if !safeID(t.ID) || !safeID(t.RecordID) {
				continue
			}
			if t.State == "queued" || t.State == "resolving" || t.State == "downloading" {
				t.State = "interrupted"
			}
			s.tasks[t.RecordID+"/"+t.Quality] = &t
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for _, task := range missing {
		s.tasks[task.RecordID+"/"+task.Quality] = &task
	}
	if len(missing) > 0 {
		s.saveTasks()
	}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	for rel := range s.deletedLegacy {
		referenced := false
		for _, r := range s.records {
			if slices.ContainsFunc(r.Variants, func(v Variant) bool { return v.Path == rel }) {
				referenced = true
				break
			}
		}
		if !referenced && filepath.Dir(rel) == "songs" && knownType(strings.TrimPrefix(filepath.Ext(rel), ".")) {
			s.unlink(rel)
		}
	}
	// Transfers never resume byte ranges without proving resource identity.
	parts, _ := filepath.Glob(filepath.Join(dir, "media", "temp", "*"))
	for _, p := range parts {
		_ = os.Remove(p)
	}
	referenced := map[string]bool{}
	for _, r := range s.records {
		for _, v := range r.Variants {
			referenced[v.Path] = true
		}
	}
	files, _ = filepath.Glob(filepath.Join(dir, "media", "audio", "*"))
	for _, p := range files {
		rel, _ := filepath.Rel(dir, p)
		if !referenced[rel] {
			id, _, _ := strings.Cut(filepath.Base(p), "-")
			r := s.records[id]
			if r != nil && (recoveryIDs[id] || len(r.Variants) == 0) {
				f, e := os.Open(p)
				if e == nil {
					info, e := f.Stat()
					if e == nil {
						typ, e := DetectAudio(f, info.Size())
						if e == nil {
							v := Variant{Path: rel, Stream: music.Stream{Type: typ, Size: info.Size()}, Pinned: r.Legacy, Created: time.Now().UnixMilli()}
							for _, task := range s.tasks {
								if task.RecordID == id {
									v.Pinned = true
								}
							}
							r.Variants = append(r.Variants, v)
							f.Close()
							if e = s.save(r); e != nil {
								return nil, e
							}
							continue
						}
					}
					f.Close()
				}
			}
			_ = os.Remove(p)
		}
	}
	s.gcArtwork()
	return s, nil
}

func safeID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func (s *Store) path(rel string) string {
	if rel == "" || !filepath.IsLocal(rel) {
		return ""
	}
	p := filepath.Join(s.dir, rel)
	// Never follow a saved manifest out through a symlink.
	if real, err := filepath.EvalSymlinks(p); err == nil {
		r, err := filepath.Rel(s.dir, real)
		if err != nil || !filepath.IsLocal(r) {
			return ""
		}
	}
	return p
}
func (s *Store) exists(rel string) bool {
	p := s.path(rel)
	if p == "" {
		return false
	}
	i, e := os.Stat(p)
	return e == nil && i.Mode().IsRegular()
}
func (s *Store) valid(v Variant) bool {
	parent := filepath.Dir(v.Path)
	if parent != filepath.Join("media", "audio") && parent != "songs" {
		return false
	}
	if !knownType(strings.TrimPrefix(filepath.Ext(v.Path), ".")) {
		return false
	}
	p := s.path(v.Path)
	if p == "" {
		return false
	}
	i, e := os.Stat(p)
	return e == nil && i.Mode().IsRegular() && i.Size() == v.Stream.Size && i.Size() > 0
}

func cleanTrack(t music.Track) music.Track {
	t.LocalOnly = false
	t.URL = ""
	t.Album.ImageURL = ""
	t.Artists = slices.Clone(t.Artists)
	for i := range t.Artists {
		t.Artists[i].ImageURL = ""
	}
	t.Album.Artists = slices.Clone(t.Album.Artists)
	for i := range t.Album.Artists {
		t.Album.Artists[i].ImageURL = ""
	}
	return t
}
func cleanStream(st music.Stream) music.Stream { st.URL = ""; st.Headers = nil; return st }
func clone(r *Record) Record {
	b, _ := json.Marshal(r)
	var out Record
	_ = json.Unmarshal(b, &out)
	return out
}
func atomicJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".record-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func (s *Store) save(r *Record) error {
	if old := s.records[r.ID]; old != nil {
		var dropped []Variant
		for _, v := range old.Variants {
			if !slices.ContainsFunc(r.Variants, func(next Variant) bool { return next.Path == v.Path }) {
				dropped = append(dropped, v)
			}
		}
		if err := s.journalLegacy(dropped); err != nil {
			return err
		}
	}

	err := atomicJSON(filepath.Join(s.dir, "media", "records", r.ID+".json"), r)
	if err != nil {
		s.lastError = err
	}
	return err
}
func (s *Store) notify() {
	s.mu.Lock()
	fn := s.changed
	closed := s.closed
	s.mu.Unlock()
	if fn != nil && !closed {
		fn()
	}
}
func (s *Store) OnChange(fn func()) { s.mu.Lock(); s.changed = fn; s.mu.Unlock() }
func (s *Store) Error() error       { s.mu.Lock(); defer s.mu.Unlock(); return s.lastError }

func (s *Store) record(t music.Track, name string) *Record {
	id := t.StoredID
	if !safeID(id) {
		id = Key(t.Source, t.ID)
	}
	r := s.records[id]
	if r == nil {
		r = &Record{Version: 1, ID: id, Created: time.Now().UnixMilli(), CoverState: "pending", LyricsState: "pending"}
		s.records[id] = r
	}
	if t.Name != "" && (!t.LocalOnly || r.Track.Name == "") {
		r.Track = cleanTrack(t)
		r.Track.StoredID = id
		r.Legacy = false
	}
	if art := safeArtworkURL(t.Album.ImageURL); art != "" {
		r.CoverURL = art
	}
	if name != "" {
		r.SourceName = name
	}
	return r
}

// Remember also adopts an old opaque cache once its source returns the identity.
func (s *Store) Remember(t music.Track, name string) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return os.ErrClosed
	}
	r := s.record(t, name)
	err := s.save(r)
	s.mu.Unlock()
	s.notify()
	return err
}
func (s *Store) List() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Record
	for _, r := range s.records {
		if len(r.Variants) > 0 {
			snapshot := clone(r)
			snapshot.Variants = slices.DeleteFunc(snapshot.Variants, func(v Variant) bool { return !s.valid(v) })
			if len(snapshot.Variants) > 0 {
				out = append(out, snapshot)
			}
		}
	}
	slices.SortFunc(out, func(a, b Record) int {
		if a.Created > b.Created {
			return -1
		}
		if a.Created < b.Created {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}
func (s *Store) Get(t music.Track) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := t.StoredID
	if id == "" {
		id = Key(t.Source, t.ID)
	}
	r := s.records[id]
	if r == nil {
		return Record{}, false
	}
	return clone(r), true
}
func (s *Store) Track(r Record) music.Track {
	t := r.Track
	t.StoredID = r.ID
	t.LocalOnly = true
	t.Album.ImageURL = ""
	if s.exists(r.Cover) {
		t.Album.ImageURL = music.FileURL(s.path(r.Cover))
	}
	return t
}
func (s *Store) Has(t music.Track) bool {
	r, ok := s.Get(t)
	return ok && slices.ContainsFunc(r.Variants, s.valid)
}
func (s *Store) Touch(t music.Track) {
	s.mu.Lock()
	id := t.StoredID
	if id == "" {
		id = Key(t.Source, t.ID)
	}
	if r := s.records[id]; r != nil {
		r.Played = time.Now().UnixMilli()
		_ = s.save(r)
	}
	s.mu.Unlock()
}
func (s *Store) Cover(t music.Track) string {
	r, ok := s.Get(t)
	if ok && s.exists(r.Cover) {
		return music.FileURL(s.path(r.Cover))
	}
	return ""
}

func (s *Store) SetPinned(id string, pinned bool) error {
	s.mu.Lock()
	r := s.records[id]
	if r == nil {
		s.mu.Unlock()
		return ErrMissing
	}
	next := clone(r)
	for i := range next.Variants {
		next.Variants[i].Pinned = pinned
	}
	if err := s.save(&next); err != nil {
		s.mu.Unlock()
		return err
	}
	*r = next
	var cancel []func()
	if !pinned {
		for _, j := range s.jobs {
			if j.id == id {
				j.pin = false
				if j.refs == 0 {
					cancel = append(cancel, j.cancel)
				}
			}
		}
		for k, t := range s.tasks {
			if t.RecordID == id {
				delete(s.tasks, k)
			}
		}
		s.saveTasks()
	}
	s.mu.Unlock()
	for _, fn := range cancel {
		fn()
	}
	s.notify()
	return nil
}

func (s *Store) unlink(rel string) {
	_ = os.Remove(s.path(rel))
	if filepath.Dir(rel) == "songs" {
		_ = os.Remove(strings.TrimSuffix(s.path(rel), filepath.Ext(rel)) + ".json")
	}
}
func (s *Store) removePath(path string) {
	if s.leases[path] > 0 {
		s.garbage[path] = true
	} else {
		s.unlink(path)
	}
}
func (s *Store) deleteRecord(r *Record) error {
	if err := s.journalLegacy(r.Variants); err != nil {
		return err
	}
	// Hide the record durably before reclaiming its resources.
	if err := os.Remove(filepath.Join(s.dir, "media", "records", r.ID+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	delete(s.records, r.ID)
	for _, v := range r.Variants {
		s.removePath(v.Path)
	}
	s.gcArtwork()
	return nil
}
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	r := s.records[id]
	var err error
	if r != nil {
		err = s.deleteRecord(r)
	}
	jobs := s.cancelRecordLocked(id)
	s.mu.Unlock()
	for _, j := range jobs {
		j.cancel()
	}
	s.notify()
	return err
}
func (s *Store) ClearAutomatic() error {
	s.mu.Lock()
	s.generation++
	var first error
	for _, r := range s.records {
		next := clone(r)
		next.Variants = slices.DeleteFunc(next.Variants, func(v Variant) bool { return !v.Pinned })
		if len(next.Variants) == 0 {
			if err := s.deleteRecord(r); err != nil {
				first = err
			}
		} else if err := s.save(&next); err != nil {
			first = err
		} else {
			old := r.Variants
			*r = next
			for _, v := range old {
				if !v.Pinned {
					s.removePath(v.Path)
				}
			}
		}
	}
	s.gcArtwork()
	s.mu.Unlock()
	s.notify()
	return first
}
func (s *Store) Configure(limit int64, enabled bool) {
	s.mu.Lock()
	s.limit = limit
	s.enabled = enabled
	if !enabled {
		s.generation++
	}
	if enabled {
		s.prune("")
	}
	s.mu.Unlock()
	s.notify()
}
func (s *Store) prune(keep string) {
	var records []*Record
	var size int64
	for _, r := range s.records {
		records = append(records, r)
		for _, v := range r.Variants {
			if !v.Pinned {
				size += v.Stream.Size
			}
		}
	}
	slices.SortFunc(records, func(a, b *Record) int {
		if a.Played < b.Played {
			return -1
		}
		if a.Played > b.Played {
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	for _, r := range records {
		if s.usageLocked().Automatic <= s.limit {
			break
		}
		next := clone(r)
		var removed []Variant
		next.Variants = slices.DeleteFunc(next.Variants, func(v Variant) bool {
			if v.Pinned || v.Path == keep || s.leases[v.Path] > 0 {
				return false
			}
			size -= v.Stream.Size
			removed = append(removed, v)
			return true
		})
		if len(removed) == 0 {
			continue
		}
		if len(next.Variants) == 0 {
			_ = s.deleteRecord(r)
		} else if s.save(&next) == nil {
			*r = next
			for _, v := range removed {
				s.removePath(v.Path)
			}
		}
	}
	s.gcArtwork()
}
func (s *Store) gcArtwork() {
	used := map[string]bool{}
	for _, r := range s.records {
		if r.Cover != "" {
			used[r.Cover] = true
		}
	}
	files, _ := filepath.Glob(filepath.Join(s.dir, "media", "artwork", "*"))
	for _, p := range files {
		rel, _ := filepath.Rel(s.dir, p)
		if !used[rel] {
			s.removePath(rel)
		}
	}
}
func (s *Store) Usage() Usage { s.mu.Lock(); defer s.mu.Unlock(); return s.usageLocked() }
func (s *Store) usageLocked() Usage {
	var u Usage
	owners := map[string]bool{}
	for _, r := range s.records {
		if len(r.Variants) == 0 {
			continue
		}
		recordPath := filepath.Join("media", "records", r.ID+".json")
		owners[recordPath] = r.Pinned()
		if r.Cover != "" {
			owners[r.Cover] = owners[r.Cover] || r.Pinned()
		}
		for _, v := range r.Variants {
			owners[v.Path] = owners[v.Path] || v.Pinned
		}
	}
	for rel, pinned := range owners {
		if i, e := os.Stat(s.path(rel)); e == nil {
			if pinned {
				u.Pinned += i.Size()
			} else {
				u.Automatic += i.Size()
			}
			if filepath.Dir(rel) == "songs" {
				u.Total += i.Size()
			}
		}
	}
	_ = filepath.WalkDir(filepath.Join(s.dir, "media"), func(p string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() {
			if i, e := d.Info(); e == nil {
				u.Total += i.Size()
				if filepath.Base(filepath.Dir(p)) == "temp" {
					u.Temporary += i.Size()
				}
			}
		}
		return nil
	})
	return u
}

func (s *Store) migrate() error {
	files, _ := filepath.Glob(filepath.Join(s.dir, "songs", "*.json"))
	for _, p := range files {
		base := strings.TrimSuffix(filepath.Base(p), ".json")
		id, q, ok := strings.Cut(base, "-")
		if !ok || !safeID(id) {
			continue
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		var st music.Stream
		if json.Unmarshal(b, &st) != nil || !knownType(st.Type) {
			continue
		}
		rel := filepath.Join("songs", base+"."+st.Type)
		if s.deletedLegacy[rel] {
			continue
		}
		v := Variant{Path: rel, Stream: cleanStream(st), Created: time.Now().UnixMilli()}
		if !s.valid(v) {
			continue
		}
		if v.Stream.Quality == "" {
			v.Stream.Quality = q
		}
		r := s.records[id]
		if r != nil && slices.ContainsFunc(r.Variants, func(v Variant) bool { return v.Path == rel }) {
			continue
		}
		if r == nil {
			r = &Record{Version: 1, ID: id, Track: music.Track{ID: id, StoredID: id, Name: "Unidentified cached song"}, SourceName: "Unknown source", Created: v.Created, Legacy: true, CoverState: "pending", LyricsState: "pending"}
			s.records[id] = r
		}
		r.Variants = append(r.Variants, v)
		if e := s.save(r); e != nil {
			return e
		}
	}
	return nil
}
func knownType(t string) bool {
	return slices.Contains([]string{"mp3", "flac", "ogg", "wav", "m4a", "aac"}, t)
}

// AttachLegacy identifies matching records using an existing saved queue.
func (s *Store) AttachLegacy(tracks []music.Track) {
	for _, t := range tracks {
		if t.Source != "" && t.ID != "" {
			if r, ok := s.Get(t); ok && r.Legacy {
				_ = s.Remember(t, t.Source)
			}
		}
	}
}

func (s *Store) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	var jobs []*job
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	srv := s.server
	s.mu.Unlock()
	for _, j := range jobs {
		j.cancel()
	}
	if srv != nil {
		srv.close()
	}
	s.wg.Wait()
}

func (s *Store) copyFile(rel string, data []byte) error {
	p := s.path(rel)
	if p == "" {
		return fmt.Errorf("invalid saved path")
	}
	f, e := os.CreateTemp(filepath.Dir(p), ".asset-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	_, e = f.Write(data)
	if e == nil {
		e = f.Sync()
	}
	c := f.Close()
	if e == nil {
		e = c
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), p)
}

// DetectAudio rejects HTML/error bodies without decoding or producing sound.
func DetectAudio(r io.ReaderAt, size int64) (string, error) {
	b := make([]byte, min(size, 12))
	if len(b) < 4 {
		return "", errors.New("audio file is incomplete")
	}
	_, _ = r.ReadAt(b, 0)
	switch {
	case string(b[:4]) == "fLaC":
		return "flac", nil
	case string(b[:4]) == "OggS":
		return "ogg", nil
	case len(b) >= 12 && (string(b[:4]) == "RIFF" || string(b[:4]) == "RF64") && string(b[8:12]) == "WAVE":
		return "wav", nil
	case len(b) >= 8 && string(b[4:8]) == "ftyp":
		return "m4a", nil
	}
	off := int64(0)
	if len(b) >= 10 && string(b[:3]) == "ID3" {
		off = 10 + (int64(b[6]&127) << 21) + (int64(b[7]&127) << 14) + (int64(b[8]&127) << 7) + int64(b[9]&127)
		if off+4 > size {
			return "", errors.New("audio contains only tags")
		}
		_, _ = r.ReadAt(b[:4], off)
	}
	if b[0] == 255 && b[1]&0xe0 == 0xe0 {
		if b[1]&6 == 0 {
			return "aac", nil
		}
		return "mp3", nil
	}
	return "", errors.New("downloaded resource is not a supported audio file")
}

func (s *Store) journalLegacy(variants []Variant) error {
	changed := false
	var added []string
	for _, v := range variants {
		if filepath.Dir(v.Path) == "songs" && !s.deletedLegacy[v.Path] {
			s.deletedLegacy[v.Path] = true
			added = append(added, v.Path)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	err := atomicJSON(filepath.Join(s.dir, "media", "deleted-legacy.json"), s.deletedLegacy)
	if err != nil {
		for _, path := range added {
			delete(s.deletedLegacy, path)
		}
		s.lastError = err
	}
	return err
}
