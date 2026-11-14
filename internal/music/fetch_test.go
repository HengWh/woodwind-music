package music

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFileURLs(t *testing.T) {
	for _, tc := range []struct {
		path, url string
		windows   bool
	}{
		{`C:\音乐\周杰伦 #1 100%.flac`, "file:///C:/%E9%9F%B3%E4%B9%90/%E5%91%A8%E6%9D%B0%E4%BC%A6%20%231%20100%25.flac", true},
		{`\\NAS\Music\专辑\a.flac`, "file://NAS/Music/%E4%B8%93%E8%BE%91/a.flac", true},
		{"/Music/a #1%.flac", "file:///Music/a%20%231%25.flac", false},
		{`/Music/a\b.flac`, "file:///Music/a%5Cb.flac", false},
	} {
		if got := fileURL(tc.path, tc.windows); got != tc.url {
			t.Errorf("FileURL(%q) = %q, want %q", tc.path, got, tc.url)
		}
		if got, ok := pathOf(tc.url, tc.windows); !ok || got != tc.path {
			t.Errorf("PathOf(%q) = %q, %v; want %q", tc.url, got, ok, tc.path)
		}
	}
	for _, tc := range []struct{ url, want string }{
		{"file://localhost/C:/Music/a.mp3", `C:\Music\a.mp3`},
		{"file:////NAS/Music/a.mp3", `\\NAS\Music\a.mp3`},
	} {
		if got, ok := pathOf(tc.url, true); !ok || got != tc.want {
			t.Errorf("PathOf(%q) = %q, %v", tc.url, got, ok)
		}
	}
	if _, ok := pathOf("file://NAS/Music/a.mp3", false); ok {
		t.Fatal("a remote file URL must not become a local path")
	}
	path := filepath.Join(t.TempDir(), "音乐 #100%.png")
	if err := os.WriteFile(path, []byte("cover"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := Fetch(FileURL(path)); err != nil || string(data) != "cover" {
		t.Fatalf("fetching a local cover: %q, %v", data, err)
	}
}

// useImageHeaders sets ImageHeaders for a test, putting back what was there.
func useImageHeaders(t *testing.T, f func(*url.URL) map[string]string) {
	was := ImageHeaders
	ImageHeaders = f
	t.Cleanup(func() { ImageHeaders = was })
}

// A picture is asked for with the headers its source gives for it, such as
// the cookie of the session it keeps its covers behind.
func TestFetchWithImageHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "music-token=abc" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("cover"))
	}))
	defer srv.Close()
	useImageHeaders(t, func(u *url.URL) map[string]string {
		return map[string]string{"Cookie": "music-token=abc"}
	})
	if b, err := Fetch(srv.URL + "/cover?id=1"); err != nil || string(b) != "cover" {
		t.Fatalf("got %q, %v", b, err)
	}
}

// Headers given for a host go no further than it: a redirect to another is
// asked for without them, so a source's token never reaches a stranger.
// Go itself only kept Cookie and Authorization back.
func TestHeadersStayOnTheirHost(t *testing.T) {
	var leaked string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Header.Get("X-Token")
		w.Write([]byte("cover"))
	}))
	defer other.Close()
	// 127.0.0.1 and localhost are other hosts, as far as headers go.
	port := must(url.Parse(other.URL)).Port()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://localhost:"+port+"/c.jpg", http.StatusFound)
	}))
	defer srv.Close()
	useImageHeaders(t, func(u *url.URL) map[string]string {
		if u.Hostname() != "127.0.0.1" {
			return nil
		}
		return map[string]string{"X-Token": "secret"}
	})
	if _, err := Fetch(srv.URL + "/c.jpg"); err != nil {
		t.Fatal(err)
	}
	if leaked != "" {
		t.Errorf("the redirect took the header along: %q", leaked)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// A picture whose URL says where its size goes, with {size}, as a source
// gives it or escaped, is asked for the size it shows; one shown at no size
// in particular is asked for at the most the app shows one at, and one
// without it as it is.
func TestSizedImageFillsItsPlaceholder(t *testing.T) {
	for _, c := range []struct {
		in   string
		px   int
		want string
	}{
		{"https://img.example.com/a.jpg?param={size}y{size}", 96, "https://img.example.com/a.jpg?param=96y96"},
		{"https://img.example.com/%7Bsize%7D/a.jpg", 240, "https://img.example.com/240/a.jpg"},
		{"https://img.example.com/a.jpg?w={size}", 0, "https://img.example.com/a.jpg?w=600"},
		{"https://img.example.com/a.jpg?size=big", 96, "https://img.example.com/a.jpg?size=big"},
	} {
		if got := SizedImage(c.in, c.px); got != c.want {
			t.Errorf("SizedImage(%q, %d) = %q, want %q", c.in, c.px, got, c.want)
		}
	}
}
