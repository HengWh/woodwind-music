package music

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStreamRelayHeadersRangeAndLifetime(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private" {
			t.Error("missing authentication")
		}
		if r.Header.Get("Range") != "bytes=2-4" {
			t.Errorf("range: %q", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", "bytes 2-4/6")
		w.WriteHeader(206)
		io.WriteString(w, "cde")
	}))
	defer origin.Close()
	relay, err := NewStreamRelay(origin.URL, map[string]string{"Authorization": "Bearer private"})
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	request, _ := http.NewRequest("GET", relay.URL, nil)
	request.Header.Set("Range", "bytes=2-4")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 206 || string(data) != "cde" || response.Header.Get("Content-Range") != "bytes 2-4/6" {
		t.Fatalf("response: %d %s", response.StatusCode, data)
	}
	denied, err := http.Get(relay.URL + "/guess")
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != 404 {
		t.Fatal("unguessable route did not reject another path")
	}
	relay.Close()
	if response, err := http.Get(relay.URL); err == nil {
		response.Body.Close()
		t.Fatal("relay survived close")
	}
}
func TestStreamRelayStripsCredentialsAcrossHosts(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Token") != "" {
			t.Error("credentials followed redirect")
		}
		io.WriteString(w, "audio")
	}))
	defer destination.Close()
	// localhost and 127.0.0.1 are deliberately distinct hosts.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, strings.Replace(destination.URL, "127.0.0.1", "localhost", 1), 302)
	}))
	defer origin.Close()
	relay, err := NewStreamRelay(origin.URL, map[string]string{"Authorization": "secret", "X-Token": "private"})
	if err != nil {
		t.Fatal(err)
	}
	defer relay.Close()
	response, err := http.Get(relay.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
}
