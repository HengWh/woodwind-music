package offline

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
)

type Task struct {
	ID           string      `json:"id"`
	RecordID     string      `json:"recordId"`
	Track        music.Track `json:"track"`
	Quality      string      `json:"quality"`
	State        string      `json:"state"`
	Error        string      `json:"error,omitempty"`
	Received     int64       `json:"received"`
	Total        int64       `json:"total"`
	Created      int64       `json:"created"`
	AllowMetered bool        `json:"allowMetered,omitempty"`
}

type job struct {
	mu           sync.Mutex
	store        *Store
	key, id, rel string
	track        music.Track
	quality      string
	ctx          context.Context
	cancel       context.CancelFunc
	f            *os.File
	n            int64
	info         music.Stream
	err          error
	done         bool
	ready        chan struct{}
	finished     chan struct{}
	change       chan struct{}
	priority     chan struct{}
	promote      sync.Once
	// Below: Store.mu, never job.mu. Network/read locks never surround UI callbacks.
	refs       int
	pin        bool
	generation uint64
	retained   bool
}

type Snapshot struct {
	Received, Size int64
	Done           bool
	Err            error
	Changed        <-chan struct{}
}
type Handle struct {
	j    *job
	once sync.Once
}

func (h *Handle) Snapshot() Snapshot {
	j := h.j
	j.mu.Lock()
	defer j.mu.Unlock()
	return Snapshot{j.n, j.info.Size, j.done, j.err, j.change}
}
func (h *Handle) Info(ctx context.Context) (*music.Stream, error) {
	j := h.j
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-j.ready:
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.err != nil {
		return nil, j.err
	}
	st := j.info
	st.Headers = nil
	return &st, nil
}
func (h *Handle) ReadAt(p []byte, off int64) (int, error) {
	j := h.j
	j.mu.Lock()
	defer j.mu.Unlock()
	if off < 0 {
		return 0, errors.New("negative audio offset")
	}
	if j.f == nil {
		return 0, os.ErrClosed
	}
	if off >= j.n {
		if j.err != nil {
			return 0, j.err
		}
		return 0, io.EOF
	}
	return j.f.ReadAt(p[:min(int64(len(p)), j.n-off)], off)
}
func (h *Handle) Wait(ctx context.Context, end int64) error {
	for {
		p := h.Snapshot()
		if p.Received >= end {
			return nil
		}
		if p.Done {
			if p.Err != nil {
				return p.Err
			}
			return io.EOF
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.Changed:
		}
	}
}
func (h *Handle) Close() error {
	h.once.Do(func() {
		j := h.j
		s := j.store
		s.mu.Lock()
		j.refs--
		last := j.refs == 0
		cancel := last && !j.pin
		finished := false
		select {
		case <-j.finished:
			finished = true
		default:
		}
		s.mu.Unlock()
		if cancel {
			j.cancel()
		}
		if last && finished {
			s.dispose(j)
		}
	})
	return nil
}
func (j *job) signal() { close(j.change); j.change = make(chan struct{}) }
func (s *Store) dispose(j *job) {
	// Close before processing deferred Windows deletions.
	j.mu.Lock()
	if j.f != nil {
		_ = j.f.Close()
		j.f = nil
	}
	j.mu.Unlock()
	s.mu.Lock()
	if j.rel != "" {
		s.leases[j.rel]--
		if s.leases[j.rel] <= 0 {
			delete(s.leases, j.rel)
			if s.garbage[j.rel] {
				s.unlink(j.rel)
				delete(s.garbage, j.rel)
			}
		}
	}
	retained := j.retained
	rel := j.rel
	if s.enabled && !s.closed {
		s.prune("")
	}
	if r := s.records[j.id]; r != nil && len(r.Variants) == 0 && s.jobs[j.key] == nil {
		_ = s.deleteRecord(r)
	}
	s.mu.Unlock()
	if !retained && rel != "" {
		_ = os.Remove(s.path(rel))
	}
}

func choose(r *Record, q string, strict bool, valid func(Variant) bool) int {
	best := -1
	want := slices.Index(music.Qualities, q)
	for i, v := range r.Variants {
		if !valid(v) {
			continue
		}
		rank := slices.Index(music.Qualities, v.Stream.Quality)
		if strict && rank < want {
			continue
		}
		if best < 0 || rank > slices.Index(music.Qualities, r.Variants[best].Stream.Quality) {
			best = i
		}
	}
	return best
}

// Acquire returns immediately; Info waits for address resolution. All consumers
// share a single transfer, while saved-library requests never fall back online.
func (s *Store) Acquire(t music.Track, q string) (*Handle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, os.ErrClosed
	}
	id := t.StoredID
	if id == "" {
		id = Key(t.Source, t.ID)
	}
	if r := s.records[id]; r != nil {
		if at := choose(r, q, false, s.valid); at >= 0 {
			v := r.Variants[at]
			f, err := os.Open(s.path(v.Path))
			if err == nil {
				ctx, cancel := context.WithCancel(context.Background())
				j := &job{store: s, id: id, ctx: ctx, cancel: cancel, f: f, n: v.Stream.Size, info: v.Stream, done: true, ready: make(chan struct{}), finished: make(chan struct{}), change: make(chan struct{}), refs: 1, rel: v.Path, retained: true}
				s.leases[v.Path]++
				close(j.ready)
				close(j.finished)
				return &Handle{j: j}, nil
			}
		}
	}
	if t.LocalOnly {
		return nil, ErrMissing
	}
	if s.blocked[t.Source] || s.source == nil || s.source(t.Source) == nil {
		return nil, ErrSource
	}
	j := s.startLocked(t, q, false, false)
	j.refs++
	j.promote.Do(func() { close(j.priority) })
	return &Handle{j: j}, nil
}

// SetSource installs the thread-safe registry before any tasks are resumed.
func (s *Store) SetSource(source func(string) music.Source) {
	s.mu.Lock()
	s.source = source
	s.mu.Unlock()
}

func (s *Store) Pin(t music.Track, q string, allowMetered bool) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return os.ErrClosed
	}
	id := t.StoredID
	if id == "" {
		id = Key(t.Source, t.ID)
	}
	if r := s.records[id]; r != nil {
		if at := choose(r, q, q != "", s.valid); at >= 0 {
			next := clone(r)
			next.Variants[at].Pinned = true
			err := s.save(&next)
			if err == nil {
				*r = next
			}
			s.mu.Unlock()
			s.notify()
			return err
		}
	}
	if t.Source == "" || t.Source == "local" {
		s.mu.Unlock()
		return ErrMissing
	}
	if q == "" {
		q = "high"
	}
	src := s.lookupSourceLocked(t.Source)
	if src != nil && src.Info().Playback == music.PlaybackRemote {
		s.mu.Unlock()
		return music.ErrUnsupported
	}
	s.startLocked(t, q, true, allowMetered)
	s.mu.Unlock()
	s.notify()
	return nil
}
func (s *Store) lookupSourceLocked(id string) music.Source {
	if s.blocked[id] || s.source == nil {
		return nil
	}
	return s.source(id)
}

func (s *Store) startLocked(t music.Track, q string, pin, allowMetered bool) *job {
	id := t.StoredID
	if id == "" {
		id = Key(t.Source, t.ID)
	}
	key := id + "/" + q
	if j := s.jobs[key]; j != nil {
		if pin {
			j.pin = true
			if task := s.tasks[j.key]; task != nil {
				task.AllowMetered = allowMetered
			} else {
				s.tasks[key] = &Task{ID: Key(id, q), RecordID: id, Track: cleanTrack(t), Quality: q, State: "queued", Created: time.Now().UnixMilli(), AllowMetered: allowMetered}
			}
			s.saveTasks()
		}
		return j
	}
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{store: s, key: key, id: id, track: t, quality: q, ctx: ctx, cancel: cancel, ready: make(chan struct{}), finished: make(chan struct{}), change: make(chan struct{}), priority: make(chan struct{}), pin: pin, generation: s.generation}
	if src := s.lookupSourceLocked(t.Source); src != nil {
		s.record(t, src.Info().Name)
	} else {
		s.record(t, t.Source)
	}
	s.jobs[key] = j
	if pin {
		taskID := Key(id, q)
		s.tasks[key] = &Task{ID: taskID, RecordID: id, Track: cleanTrack(t), Quality: q, State: "queued", Created: time.Now().UnixMilli(), AllowMetered: allowMetered}
		s.saveTasks()
	}
	s.wg.Add(1)
	go s.run(j)
	return j
}
func (s *Store) saveTasks() {
	var out []Task
	for _, t := range s.tasks {
		out = append(out, *t)
	}
	if err := atomicJSON(filepath.Join(s.dir, "media", "tasks.json"), out); err != nil {
		s.lastError = err
	}
}
func (s *Store) taskState(j *job, state string, err error) {
	s.mu.Lock()
	if t := s.tasks[j.key]; t != nil {
		t.State = state
		t.Error = ""
		if err != nil {
			t.Error = safeError(err)
		}
		s.saveTasks()
	}
	s.mu.Unlock()
	s.notify()
}
func (s *Store) run(j *job) {
	defer s.wg.Done()
	ready := false
	var final error
	defer func() {
		j.mu.Lock()
		j.done = true
		j.err = final
		if !ready {
			close(j.ready)
		}
		j.signal()
		j.mu.Unlock()
		s.mu.Lock()
		if s.jobs[j.key] == j {
			if final != nil {
				state := stateFor(final)
				if errors.Is(final, context.Canceled) {
					if s.closed {
						state = "interrupted"
					} else if s.blocked[j.track.Source] {
						state = "waiting-plugin"
					}
				}
				if task := s.tasks[j.key]; task != nil {
					task.State = state
					task.Error = safeError(final)
					s.saveTasks()
				}
			}
			delete(s.jobs, j.key)
		}
		close(j.finished)
		last := j.refs == 0
		s.mu.Unlock()
		if last {
			s.dispose(j)
		}
		s.notify()
	}()
	// A playback consumer can take priority over the bounded background queue.
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-j.priority:
	case <-j.ctx.Done():
		final = j.ctx.Err()
		return
	}
	s.mu.Lock()
	src := s.lookupSourceLocked(j.track.Source)
	pin := j.pin
	metered := s.wifiOnly && s.network != nil && !s.network()
	allow := false
	if t := s.tasks[j.key]; t != nil {
		allow = t.AllowMetered
	}
	s.mu.Unlock()
	if src == nil {
		final = ErrSource
		return
	}
	if pin && metered && !allow {
		final = errNetwork
		return
	}
	s.taskState(j, "resolving", nil)
	st, err := src.Stream(j.track.ID, j.quality)
	if err != nil {
		final = err
		return
	}
	if st == nil || st.URL == "" {
		final = music.ErrUnavailable
		return
	}
	if err = j.ctx.Err(); err != nil {
		final = err
		return
	}
	f, err := os.CreateTemp(filepath.Join(s.dir, "media", "temp"), "audio-*.part")
	if err != nil {
		final = err
		return
	}
	rel, _ := filepath.Rel(s.dir, f.Name())
	j.mu.Lock()
	j.f = f
	j.info = *st
	j.info.Headers = nil
	j.mu.Unlock()
	s.mu.Lock()
	j.rel = rel
	s.leases[rel]++
	s.mu.Unlock()
	var body io.ReadCloser
	expected := st.Size
	if p, ok := music.PathOf(st.URL); ok {
		body, err = os.Open(p)
	} else {
		req, e := http.NewRequestWithContext(j.ctx, "GET", st.URL, nil)
		if e != nil {
			final = e
			return
		}
		req.Header.Set("User-Agent", music.Agent)
		for k, v := range st.Headers {
			req.Header.Set(k, v)
		}
		res, e := music.HeadersStay(http.DefaultClient, req.URL.Hostname(), st.Headers).Do(req)
		if e != nil {
			final = e
			return
		}
		body = res.Body
		if res.StatusCode != http.StatusOK {
			body.Close()
			final = errors.New(res.Status)
			return
		}
		if expected <= 0 {
			expected = res.ContentLength
		}
	}
	if err != nil {
		final = err
		return
	}
	defer body.Close()
	j.mu.Lock()
	if expected > 0 {
		j.info.Size = expected
	}
	close(j.ready)
	ready = true
	j.signal()
	j.mu.Unlock()
	s.taskState(j, "downloading", nil)
	digest := sha256.New()
	buf := make([]byte, 64<<10)
	lastNotify := time.Now()
	for {
		if err = j.ctx.Err(); err != nil {
			final = err
			return
		}
		n, e := body.Read(buf)
		if n > 0 {
			digest.Write(buf[:n])
			j.mu.Lock()
			written, werr := f.Write(buf[:n])
			j.n += int64(written)
			received := j.n
			j.signal()
			j.mu.Unlock()
			if werr != nil {
				final = werr
				return
			}
			if written != n {
				final = io.ErrShortWrite
				return
			}
			if time.Since(lastNotify) > 200*time.Millisecond {
				s.mu.Lock()
				if t := s.tasks[j.key]; t != nil {
					t.Received = received
					t.Total = expected
				}
				s.mu.Unlock()
				s.notify()
				lastNotify = time.Now()
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			final = e
			return
		}
	}
	j.mu.Lock()
	size := j.n
	j.mu.Unlock()
	if expected > 0 && size != expected {
		final = io.ErrUnexpectedEOF
		return
	}
	if size == 0 {
		final = io.ErrUnexpectedEOF
		return
	}
	typ, err := DetectAudio(f, size)
	if err != nil {
		final = err
		return
	}
	if err = f.Sync(); err != nil {
		final = err
		return
	}
	s.mu.Lock()
	pin = j.pin
	keep := pin || (s.enabled && j.generation == s.generation && (!s.wifiOnly || s.network == nil || s.network()))
	blocked := s.blocked[j.track.Source] || s.closed
	s.mu.Unlock()
	if blocked || j.ctx.Err() != nil {
		final = context.Canceled
		return
	}
	if keep {
		// No active reader retains an OS file handle across this rename.
		j.mu.Lock()
		err = j.f.Close()
		if err != nil {
			j.mu.Unlock()
			final = err
			return
		}
		var nonce [8]byte
		_, _ = rand.Read(nonce[:])
		dest := filepath.Join("media", "audio", j.id+"-"+hex.EncodeToString(nonce[:])+"."+typ)
		err = os.Rename(s.path(rel), s.path(dest))
		if err != nil {
			j.f, _ = os.Open(s.path(rel))
			j.mu.Unlock()
			final = err
			return
		}
		j.f, err = os.Open(s.path(dest))
		if err != nil {
			j.mu.Unlock()
			final = err
			return
		}
		j.info.Type = typ
		j.info.Size = size
		s.mu.Lock()
		delete(s.leases, rel)
		s.leases[dest]++
		j.rel = dest
		// Clear/delete can invalidate a transfer while publication waits for the lock.
		keep = j.pin || (s.enabled && j.generation == s.generation)
		if keep && !s.closed && !s.blocked[j.track.Source] {
			r := s.record(j.track, src.Info().Name)
			next := clone(r)
			v := Variant{Hash: hex.EncodeToString(digest.Sum(nil)), Path: dest, Stream: cleanStream(j.info), Pinned: j.pin, Created: time.Now().UnixMilli()}
			duplicate := -1
			for i, old := range next.Variants {
				if old.Hash != "" && old.Hash == v.Hash && old.Stream.Quality == v.Stream.Quality {
					duplicate = i
					break
				}
			}
			if duplicate >= 0 {
				next.Variants[duplicate].Pinned = next.Variants[duplicate].Pinned || v.Pinned
			} else {
				next.Variants = append(next.Variants, v)
			}
			if err = s.save(&next); err == nil {
				*r = next
				j.retained = duplicate < 0
			}
			if err != nil {
				final = err
			}
		}
		s.mu.Unlock()
		j.mu.Unlock()
		if final != nil {
			return
		}
		s.mu.Lock()
		if j.retained {
			s.prune(dest)
		}
		s.mu.Unlock()
		if j.retained {
			metadata := j.track
			metadata.LocalOnly = false
			s.Fill(metadata, src, false)
		}
	}
	j.mu.Lock()
	j.info.Type = typ
	j.info.Size = size
	j.mu.Unlock()
	s.mu.Lock()
	if task := s.tasks[j.key]; task != nil && j.pin {
		task.State = "saved"
		task.Received = size
		task.Total = size
		task.Error = ""
		if j.quality == "lossless" && st.Quality != "lossless" {
			task.Error = "Requested lossless quality was not available"
		}
		s.saveTasks()
	}
	s.mu.Unlock()
}

var errNetwork = errors.New("waiting for an unmetered network")

func stateFor(err error) string {
	switch {
	case errors.Is(err, syscall.ENOSPC):
		return "waiting-space"
	case errors.Is(err, context.Canceled):
		return "paused"
	case errors.Is(err, errNetwork):
		return "waiting-network"
	case errors.Is(err, ErrSource):
		return "waiting-plugin"
	case errors.Is(err, music.ErrSignIn):
		return "waiting-login"
	default:
		return "failed"
	}
}

func (s *Store) Tasks() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, *t)
	}
	slices.SortFunc(out, func(a, b Task) int {
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
func (s *Store) Retry(id string, allow bool) error {
	s.mu.Lock()
	var task *Task
	for _, t := range s.tasks {
		if t.ID == id {
			c := *t
			task = &c
			break
		}
	}
	s.mu.Unlock()
	if task == nil {
		return ErrMissing
	}
	s.mu.Lock()
	active := s.jobs[task.RecordID+"/"+task.Quality]
	s.mu.Unlock()
	if active != nil && active.ctx.Err() != nil {
		<-active.finished
	}
	return s.Pin(task.Track, task.Quality, allow || task.AllowMetered)
}
func (s *Store) Pause(id string) {
	s.mu.Lock()
	var cancel context.CancelFunc
	for key, t := range s.tasks {
		if t.ID == id {
			t.State = "paused"
			if j := s.jobs[key]; j != nil {
				j.pin = false
				if j.refs == 0 {
					cancel = j.cancel
				}
			}
			s.saveTasks()
			break
		}
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.notify()
}
func (s *Store) Cancel(id string) {
	s.Pause(id)
	s.mu.Lock()
	for k, t := range s.tasks {
		if t.ID == id {
			delete(s.tasks, k)
			break
		}
	}
	s.saveTasks()
	s.mu.Unlock()
	s.notify()
}
func (s *Store) cancelRecordLocked(id string) []*job {
	var out []*job
	for _, j := range s.jobs {
		if j.id == id {
			j.pin = false
			j.generation = ^uint64(0)
			out = append(out, j)
		}
	}
	for k, t := range s.tasks {
		if t.RecordID == id {
			delete(s.tasks, k)
		}
	}
	s.saveTasks()
	return out
}
func (s *Store) RemoveSource(id string) {
	s.mu.Lock()
	s.blocked[id] = true
	var jobs []*job
	for _, j := range s.jobs {
		if j.track.Source == id {
			jobs = append(jobs, j)
		}
	}
	s.mu.Unlock()
	for _, j := range jobs {
		j.cancel()
	}
	s.notify()
}
func (s *Store) AllowSource(id string) { s.mu.Lock(); delete(s.blocked, id); s.mu.Unlock() }
func (s *Store) Network(wifiOnly bool, unmetered func() bool) {
	s.mu.Lock()
	s.wifiOnly = wifiOnly
	s.network = unmetered
	s.mu.Unlock()
}
func (s *Store) ResumeWaiting() {
	for _, t := range s.Tasks() {
		if t.State == "waiting-network" || t.State == "waiting-plugin" || t.State == "waiting-login" {
			_ = s.Retry(t.ID, false)
		}
	}
}
func (s *Store) ResumeAfterRestart() {
	for _, t := range s.Tasks() {
		if t.State == "interrupted" {
			_ = s.Retry(t.ID, false)
		}
	}
}

var errorURL = regexp.MustCompile(`https?://[^\s]+`)

func safeError(err error) string {
	var e *url.Error
	if errors.As(err, &e) {
		return e.Op + ": " + e.Err.Error()
	}
	return errorURL.ReplaceAllStringFunc(err.Error(), func(raw string) string {
		u, e := url.Parse(raw)
		if e != nil {
			return "[resource]"
		}
		u.User = nil
		u.RawQuery = ""
		u.Fragment = ""
		return u.String()
	})
}
