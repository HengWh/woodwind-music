package plugin

import (
	"strings"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
)

// kugou-lite already has a seed-song recommendation helper, but doesn't export
// it. Bridge it in memory, leaving the installed plugin and its data untouched.
// See bitbyte404/mygo-music-kugou-lite, index.js: likeThese and parseId.
func kugouLibrarySource(id, source string) string {
	if id != "kugou-lite" || strings.Contains(source, "__woodwindLibraryNext") {
		return source
	}
	for _, name := range []string{"parseId", "search"} {
		if !strings.Contains(source, "function "+name+"(") {
			return source
		}
	}
	if !strings.Contains(source, "function likeThese(") {
		// 1.3.2 predates likeThese. Its existing signed gateway can make the
		// same seed request; use its constants rather than new credentials.
		for _, marker := range []string{"function kg(", "const APP_ID =", "const CLIENT_VER =", "const SALT =", "const tracksOf =", "const list =", "const auth =", "function mid(", "const md5 ="} {
			if !strings.Contains(source, marker) {
				return source
			}
		}
		source += kugouLegacySeeds
	}
	return source + kugouLibraryBridge
}

func (p *Plugin) UsesLibrarySeeds(station string) bool {
	return p.manifest.ID == "kugou-lite" && station == "normal" && p.Can("__woodwindLibraryNext")
}

func (p *Plugin) StationNextFrom(station string, seeds []music.Track) ([]music.Track, error) {
	if !p.UsesLibrarySeeds(station) {
		return nil, music.ErrUnsupported
	}
	// Only song identity and matching metadata cross the plugin boundary, never
	// saved file paths, artwork, lyrics, or the rest of the user's library.
	var songs []music.Track
	for _, t := range seeds {
		song := music.Track{Name: t.Name, Duration: t.Duration, Album: music.Album{Name: t.Album.Name}}
		if t.Source == p.manifest.ID {
			song.ID = t.ID
		}
		for _, artist := range t.Artists {
			song.Artists = append(song.Artists, music.Artist{Name: artist.Name})
		}
		songs = append(songs, song)
	}
	var result struct {
		Matched bool          `json:"matched"`
		Tracks  []music.Track `json:"tracks"`
	}
	if err := p.call("__woodwindLibraryNext", map[string]any{"seeds": songs}, &result); err != nil {
		return nil, err
	}
	if !result.Matched {
		return nil, music.Fail(music.ErrUnavailable, i18n.T("No matching Kugou songs found for these downloads and likes."))
	}
	p.stamp(result.Tracks)
	return result.Tracks, nil
}

const kugouLibraryBridge = `
export async function __woodwindLibraryNext({ seeds }) {
  const normalize = (s) => String(s || "").trim().toLowerCase().replace(/\s+/g, " ");
  const singers = (t) => (t.artists || []).map((a) => normalize(a.name)).filter(Boolean).sort().join("\u0000");
  const mixes = [];
  for (const seed of seeds) {
    let mix = seed.id ? parseId(seed.id).mix : 0;
    if (!mix && normalize(seed.name) && singers(seed)) {
      const result = await search({ query: seed.name + " " + seed.artists.map((a) => a.name).join(" "), kind: "tracks", offset: 0 });
      let matches = (result.tracks || []).filter((t) => normalize(t.name) === normalize(seed.name) && singers(t) === singers(seed));
      if (normalize(seed.album?.name)) {
        const sameAlbum = matches.filter((t) => normalize(t.album?.name) === normalize(seed.album.name));
        if (sameAlbum.length) matches = sameAlbum;
      }
      matches = matches.filter((t) => !seed.duration || !t.duration || Math.abs(t.duration - seed.duration) <= 5000);
      const ids = [...new Set(matches.map((t) => parseId(t.id).mix).filter((id) => id > 0))];
      if (ids.length === 1) mix = ids[0];
    }
    if (mix > 0 && !mixes.includes(mix)) mixes.push(mix);
  }
  if (!mixes.length) return { matched: false, tracks: [] };
  const tracks = await likeThese(mixes);
  return { matched: true, tracks: typeof markUnavailableTracks === "function" ? await markUnavailableTracks(tracks) : tracks };
}
`

const kugouLegacySeeds = `
async function likeThese(mixes) {
  const at = Date.now();
  const request = {
    appid: APP_ID, clientver: CLIENT_VER, clienttime: at,
    key: md5(String(APP_ID) + SALT + CLIENT_VER + at),
    userid: Number(auth().userid || 0), mid: mid(), platform: "ios", area_code: 1,
    source_type: 2, playlist_ver: 2, client_playlist: [],
    recommend_source: mixes.map((id) => ({ ID: Number(id) })),
  };
  const response = await kg("/recommend", { bare: true, router: "songlistairec.kugou.com", body: request });
  return tracksOf(list(response.song_list, response.list));
}
`
