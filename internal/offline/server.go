package offline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// localServer serves one shared transfer to Android's native player. Ranges
// read the same growing file; seeking never starts a second upstream download.
type localServer struct {
	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	items    map[string]*job
}

func (h *Handle) URL() (string, func(), error) {
	s := h.j.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", nil, errors.New("offline store is closed")
	}
	if s.server == nil {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", nil, err
		}
		ls := &localServer{listener: ln, items: map[string]*job{}}
		ls.server = &http.Server{Handler: ls, ReadHeaderTimeout: 10 * time.Second}
		s.server = ls
		go func() { _ = ls.server.Serve(ln) }()
	}
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", nil, err
	}
	token := "/" + hex.EncodeToString(nonce[:])
	ls := s.server
	ls.mu.Lock()
	ls.items[token] = h.j
	ls.mu.Unlock()
	return "http://" + ls.listener.Addr().String() + token, func() { ls.mu.Lock(); delete(ls.items, token); ls.mu.Unlock() }, nil
}
func (ls *localServer) close() { _ = ls.server.Close() }
func (ls *localServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ls.mu.Lock()
	j := ls.items[r.URL.Path]
	ls.mu.Unlock()
	if j != nil {
		s := j.store
		s.mu.Lock()
		if j.refs > 0 {
			j.refs++
		} else {
			j = nil
		}
		s.mu.Unlock()
	}
	if j == nil {
		http.NotFound(w, r)
		return
	}
	h := &Handle{j: j}
	defer h.Close()
	st, err := h.Info(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	size := st.Size
	if size <= 0 {
		for {
			p := h.Snapshot()
			if p.Done {
				if p.Err != nil {
					http.Error(w, p.Err.Error(), 502)
					return
				}
				size = p.Received
				break
			}
			select {
			case <-r.Context().Done():
				return
			case <-p.Changed:
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", audioMIME(st.Type))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "audio."+st.Type, time.Time{}, &seekReader{h: h, ctx: r.Context(), size: size})
}
func audioMIME(typ string) string {
	switch typ {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	case "m4a":
		return "audio/mp4"
	case "aac":
		return "audio/aac"
	case "wav":
		return "audio/wav"
	}
	return "application/octet-stream"
}

type seekReader struct {
	h         *Handle
	ctx       context.Context
	pos, size int64
}

func (r *seekReader) Read(p []byte) (int, error) {
	if r.pos >= r.size {
		return 0, io.EOF
	}
	if err := r.h.Wait(r.ctx, r.pos+1); err != nil {
		return 0, err
	}
	n, err := r.h.ReadAt(p, r.pos)
	r.pos += int64(n)
	return n, err
}
func (r *seekReader) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		off += r.pos
	case io.SeekEnd:
		off += r.size
	case io.SeekStart:
	default:
		return 0, errors.New("invalid seek")
	}
	if off < 0 {
		return 0, errors.New("invalid audio offset " + strconv.FormatInt(off, 10))
	}
	r.pos = off
	return off, nil
}
