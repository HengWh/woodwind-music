package music

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// StreamRelay lets a native player fetch one authenticated stream, with Range
// support. The random local URL never contains the upstream URL or credentials.
type StreamRelay struct {
	server *http.Server
	client *http.Client
	URL    string
}

func NewStreamRelay(address string, headers map[string]string) (*StreamRelay, error) {
	u, err := url.Parse(address)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid stream URL")
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return nil, err
	}
	path := "/" + hex.EncodeToString(secret)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	copied := make(map[string]string, len(headers))
	for k, v := range headers {
		copied[k] = v
	}
	client := HeadersStay(&http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 30 * time.Second}}, u.Hostname(), copied)
	relay := &StreamRelay{client: client, URL: "http://" + listener.Addr().String() + path}
	relay.server = &http.Server{ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || r.URL.RawQuery != "" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), r.Method, address, nil)
		if err != nil {
			http.Error(w, "stream unavailable", 502)
			return
		}
		req.Header.Set("User-Agent", Agent)
		for k, v := range copied {
			req.Header.Set(k, v)
		}
		for _, k := range []string{"Range", "If-Range"} {
			if v := r.Header.Get(k); v != "" {
				req.Header.Set(k, v)
			}
		}
		response, err := client.Do(req)
		if err != nil {
			http.Error(w, "stream unavailable", 502)
			return
		}
		defer response.Body.Close()
		for _, k := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
			if v := response.Header.Get(k); v != "" {
				w.Header().Set(k, v)
			}
		}
		w.WriteHeader(response.StatusCode)
		if r.Method != http.MethodHead {
			io.Copy(w, response.Body)
		}
	})}
	go relay.server.Serve(listener)
	return relay, nil
}
func (r *StreamRelay) Close() {
	if r != nil {
		r.server.Close()
		r.client.CloseIdleConnections()
	}
}
