package plugin

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HengWh/woodwind-music/internal/music"
)

// The same private-helper signatures and composite IDs as kugou-lite. Requests
// stay in memory; these tests need neither a Kugou account nor network access.
const kugouRadioFixture = `
const requests = [];
function parseId(id) { return { mix: Number(String(id).split("_")[1]) || 0 }; }
async function likeThese(ids) {
  requests.push(ids);
  return ids.map((id) => ({ id: "next_" + id, name: "Next " + id, source: "wrong" }));
}
async function markUnavailableTracks(tracks) { return tracks; }
export function stations() { return [{ id: "normal", description: "old" }, { id: "small", description: "niche" }]; }
export function stationNext() { return [{ id: "legacy" }]; }
export function inspect() { return requests; }
`

func TestKugouLibraryRadioKnownIDsAndCompatibility(t *testing.T) {
	source := kugouRadioFixture + `export function search() { throw new Error("known IDs must not search"); }`
	dir, data := write(t, `{"id":"kugou-lite"}`, source)
	p, err := Load(dir, data)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if !p.UsesLibrarySeeds("normal") || p.UsesLibrarySeeds("small") {
		t.Fatal("only Private FM should use library seeds")
	}
	tracks, err := p.StationNextFrom("normal", []music.Track{{ID: "A_42_1", Source: "kugou-lite"}, {ID: "B_42_2", Source: "kugou-lite"}, {ID: "C_99_3", Source: "kugou-lite"}})
	if err != nil || len(tracks) != 2 || tracks[0].ID != "next_42" || tracks[1].ID != "next_99" || tracks[0].Source != "kugou-lite" {
		t.Fatalf("seeded results: %+v, %v", tracks, err)
	}
	stations, err := p.Stations()
	if err != nil || stations[0].Description == "old" || stations[1].Description != "niche" {
		t.Fatalf("station descriptions: %+v, %v", stations, err)
	}
	if tracks, err = p.StationNext("small"); err != nil || tracks[0].ID != "legacy" {
		t.Fatalf("niche FM changed: %+v, %v", tracks, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "index.js"))
	if err != nil || string(b) != source {
		t.Fatal("loading rewrote the installed plugin")
	}
	for _, manifest := range []string{plain, `{"id":"kugou-lite"}`} {
		p := load(t, manifest, `export function stationNext() { return [{ id: "legacy" }]; }`)
		if p.UsesLibrarySeeds("normal") {
			t.Fatal("enabled the bridge without its supported helpers")
		}
		if _, err := p.StationNextFrom("normal", nil); !errors.Is(err, music.ErrUnsupported) {
			t.Fatal("unsupported bridge:", err)
		}
	}
}

func TestKugouLibraryRadioLegacySignedRequest(t *testing.T) {
	requests := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/recommend" || r.Header.Get("X-Router") != "songlistairec.kugou.com" {
			t.Errorf("wrong route: %s, %s", r.URL.Path, r.Header.Get("X-Router"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests <- body
		fmt.Fprint(w, `{"song_list":[{"id":"recommended","name":"Next"}]}`)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	manifest, _ := json.Marshal(map[string]any{"id": "kugou-lite", "permissions": map[string]any{"network": []string{u.Hostname()}}})
	endpoint, _ := json.Marshal(srv.URL)
	p := load(t, string(manifest), `
const APP_ID = 3116;
const CLIENT_VER = 11440;
const SALT = "test-salt";
const tracksOf = (xs) => xs;
const list = (...xs) => xs.find(Array.isArray) || [];
const auth = () => ({ userid: "123" });
const md5 = (s) => host.crypto.md5(s);
function mid() { return "test-device"; }
function parseId(id) { return { mix: Number(id.split("_")[1]) }; }
export function search() { throw new Error("unexpected search"); }
async function kg(path, options) {
  if (!options.bare) throw new Error("expected a bare signed request");
  const r = await fetch(`+string(endpoint)+` + path, { method: "POST", headers: { "X-Router": options.router }, body: JSON.stringify(options.body) });
  return r.json();
}`)
	if !p.UsesLibrarySeeds("normal") {
		t.Fatal("1.3.2 helper structure was not adapted")
	}
	var selected []music.Track
	for id := 301; id <= 310; id++ {
		selected = append(selected, music.Track{ID: fmt.Sprintf("HASH_%d_0", id), Source: "kugou-lite"})
	}
	tracks, err := p.StationNextFrom("normal", selected)
	if err != nil || len(tracks) != 1 || tracks[0].ID != "recommended" || tracks[0].Source != "kugou-lite" {
		t.Fatalf("legacy result: %+v, %v", tracks, err)
	}
	request := <-requests
	stamp := fmt.Sprintf("%.0f", request["clienttime"])
	wantKey := fmt.Sprintf("%x", md5.Sum([]byte("3116test-salt11440"+stamp)))
	if request["key"] != wantKey || request["userid"] != float64(123) || request["appid"] != float64(3116) {
		t.Fatal("invalid signed request", request)
	}
	seeds, _ := json.Marshal(request["recommend_source"])
	if string(seeds) != `[{"ID":301},{"ID":302},{"ID":303},{"ID":304},{"ID":305},{"ID":306},{"ID":307},{"ID":308},{"ID":309},{"ID":310}]` {
		t.Fatal("request did not use all ten selected seeds", string(seeds))
	}
}

func TestKugouLibraryRadioMatchesLocalSongsConservatively(t *testing.T) {
	seed := music.Track{ID: "local_999_0", Source: "local", Name: "Song", Artists: []music.Artist{{Name: "Singer"}}, Duration: 200000}
	for _, tt := range []struct {
		name    string
		results []music.Track
		album   string
		want    string
	}{
		{"exact", []music.Track{{ID: "A_7_1", Name: " song ", Artists: seed.Artists, Duration: 201000}}, "", "next_7"},
		{"other singer", []music.Track{{ID: "A_7_1", Name: "Song", Artists: []music.Artist{{Name: "Cover Singer"}}}}, "", ""},
		{"other version", []music.Track{{ID: "A_7_1", Name: "Song (Live)", Artists: seed.Artists}}, "", ""},
		{"different duration", []music.Track{{ID: "A_7_1", Name: "Song", Artists: seed.Artists, Duration: 230000}}, "", ""},
		{"ambiguous", []music.Track{{ID: "A_7_1", Name: "Song", Artists: seed.Artists}, {ID: "B_8_2", Name: "Song", Artists: seed.Artists}}, "", ""},
		{"same song duplicate", []music.Track{{ID: "A_7_1", Name: "Song", Artists: seed.Artists}, {ID: "B_7_2", Name: "Song", Artists: seed.Artists}}, "", "next_7"},
		{"album disambiguates", []music.Track{{ID: "A_7_1", Name: "Song", Artists: seed.Artists, Album: music.Album{Name: "One"}}, {ID: "B_8_2", Name: "Song", Artists: seed.Artists, Album: music.Album{Name: "Two"}}}, "Two", "next_8"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.results)
			p := load(t, `{"id":"kugou-lite"}`, kugouRadioFixture+`export function search({ query, kind }) {
if (!query.includes("Singer") || kind !== "tracks") throw new Error("bad search");
return { tracks: `+string(b)+` }; }`)
			local := seed
			local.Album.Name = tt.album
			tracks, err := p.StationNextFrom("normal", []music.Track{local})
			if tt.want == "" {
				if !errors.Is(err, music.ErrUnavailable) {
					t.Fatalf("uncertain match should be skipped: %+v, %v", tracks, err)
				}
				var requests [][]int
				if err := p.call("inspect", nil, &requests); err != nil || len(requests) != 0 {
					t.Fatalf("recommended from an uncertain match: %v, %v", requests, err)
				}
			} else if err != nil || len(tracks) != 1 || tracks[0].ID != tt.want {
				t.Fatalf("got %+v, %v; want %s", tracks, err, tt.want)
			}
		})
	}
}

func TestKugouLibraryRadioOnlySharesSeedIdentity(t *testing.T) {
	p := load(t, `{"id":"kugou-lite"}`, `
export function __woodwindLibraryNext({ seeds }) {
  return { matched: true, tracks: [{ id: JSON.stringify(seeds) }] };
}`)
	tracks, err := p.StationNextFrom("normal", []music.Track{{ID: "A_7_1", Source: "kugou-lite", StoredID: "private-storage", LocalOnly: true, URL: "private-url", Name: "Song", Artists: []music.Artist{{ID: "private-artist", Name: "Singer", ImageURL: "file:///private-artist"}}, Album: music.Album{ID: "private-album", Name: "Album", ImageURL: "file:///private-cover"}}})
	if err != nil || len(tracks) != 1 {
		t.Fatalf("result: %+v, %v", tracks, err)
	}
	if strings.Contains(tracks[0].ID, "private") || strings.Contains(tracks[0].ID, "localOnly") {
		t.Fatal("shared saved-library fields", tracks[0].ID)
	}
	var seeds []music.Track
	if err := json.Unmarshal([]byte(tracks[0].ID), &seeds); err != nil || len(seeds) != 1 || seeds[0].ID != "A_7_1" || seeds[0].Name != "Song" {
		t.Fatal("lost seed identity", seeds, err)
	}
}

func TestKugouLibraryRadioRequestFailuresStayVisible(t *testing.T) {
	p := load(t, `{"id":"kugou-lite"}`, strings.Replace(kugouRadioFixture, "requests.push(ids);", `throw new Error("recommendation service failed");`, 1)+`export function search() { return {}; }`)
	_, err := p.StationNextFrom("normal", []music.Track{{ID: "A_7_1", Source: "kugou-lite"}})
	if err == nil || !strings.Contains(err.Error(), "recommendation service failed") {
		t.Fatal("lost recommendation error", err)
	}
	// An unrelated plugin with the same helper names is never adapted.
	p = load(t, plain, kugouRadioFixture+`export function search() { return {}; }`)
	if p.Can("__woodwindLibraryNext") {
		t.Fatal("adapted an unrelated plugin")
	}
}

func TestKugouLibraryRadioPassesTenKnownIDs(t *testing.T) {
	p := load(t, `{"id":"kugou-lite"}`, kugouRadioFixture+`export function search() { throw new Error("known IDs must not search"); }`)
	var selected []music.Track
	for id := 101; id <= 110; id++ {
		selected = append(selected, music.Track{ID: fmt.Sprintf("HASH_%d_0", id), Source: "kugou-lite"})
	}
	tracks, err := p.StationNextFrom("normal", selected)
	if err != nil || len(tracks) != 10 {
		t.Fatalf("ten-seed result: %d tracks, %v", len(tracks), err)
	}
	var requests [][]int
	if err := p.call("inspect", nil, &requests); err != nil || len(requests) != 1 || len(requests[0]) != 10 {
		t.Fatalf("helper input: %v, %v", requests, err)
	}
	for i, id := range requests[0] {
		if id != 101+i {
			t.Fatal("wrong Kugou song ID", id)
		}
	}
}
