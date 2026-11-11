package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"

	"nsgo-music/internal/app"
)

//go:embed testdata/tone.mp3
var toneMP3 []byte

func TestMain(m *testing.M) { nsgotest.Main(m) }

// toneWAV writes a one-second sine tone as a WAV.
func toneWAV(t *testing.T, path string) {
	t.Helper()
	const rate = 44100
	data := make([]byte, rate*2)
	for i := 0; i < rate; i++ {
		binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(6000*math.Sin(2*math.Pi*440*float64(i)/rate))))
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(data)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*2))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// syncsafe is an ID3 size.
func syncsafe(n int) []byte {
	return []byte{byte(n>>21) & 0x7f, byte(n>>14) & 0x7f, byte(n>>7) & 0x7f, byte(n) & 0x7f}
}

// id3Frame is an ID3v2.3 frame.
func id3Frame(id string, data []byte) []byte {
	var b bytes.Buffer
	b.WriteString(id)
	binary.Write(&b, binary.BigEndian, uint32(len(data)))
	b.Write([]byte{0, 0})
	b.Write(data)
	return b.Bytes()
}

// id3Text is an ID3 text frame in UTF-8.
func id3Text(id, s string) []byte { return id3Frame(id, append([]byte{3}, []byte(s)...)) }

// albumArt draws a cover of the app's colors.
func albumArt(seed int) []byte {
	im := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			f := float32(x+y) / 800
			r := 226 - 70*f + float32(seed*12)
			g := 51 + 60*f
			b := 60 + 120*f
			im.Set(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
		}
	}
	for y := 90; y < 310; y++ {
		for x := 90; x < 310; x++ {
			dx, dy := float64(x-200), float64(y-200)
			if dx*dx+dy*dy < 70*70 {
				im.Set(x, y, color.RGBA{255, 250, 245, 255})
			}
		}
	}
	var b bytes.Buffer
	png.Encode(&b, im)
	return b.Bytes()
}

// taggedMP3 writes the tone with its tags and cover.
func taggedMP3(t *testing.T, path, title, artist, album string, art []byte) {
	t.Helper()
	raw := toneMP3
	var frames bytes.Buffer
	frames.Write(id3Text("TIT2", title))
	frames.Write(id3Text("TPE1", artist))
	frames.Write(id3Text("TALB", album))
	apic := append([]byte{0}, []byte("image/png\x00")...)
	apic = append(apic, 3, 0)
	apic = append(apic, art...)
	frames.Write(id3Frame("APIC", apic))
	var out bytes.Buffer
	out.WriteString("ID3\x03\x00\x00")
	out.Write(syncsafe(frames.Len()))
	out.Write(frames.Bytes())
	// The tone is a second long: repeat its frames for a song to look at.
	for range 20 {
		out.Write(raw)
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// lrc writes timed lyrics beside a track.
func lrc(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestShots saves pictures of the app's pages, for a person to look at:
// NSGO_SHOTS=1 go tool nsgo test -target macos -run TestShots.
func TestShots(t *testing.T) {
	if os.Getenv("NSGO_SHOTS") == "" {
		t.Skip("set NSGO_SHOTS=1 to save screenshots")
	}
	data := t.TempDir()
	music := t.TempDir()
	art := albumArt(0)
	taggedMP3(t, filepath.Join(music, "01 Haruhikage.mp3"), "Haruhikage", "MyGO!!!!!", "迷跡波", art)
	taggedMP3(t, filepath.Join(music, "02 Utakotoba.mp3"), "詩超絆", "MyGO!!!!!", "迷跡波", art)
	taggedMP3(t, filepath.Join(music, "03 Mayoiuta.mp3"), "迷星叫", "MyGO!!!!!", "迷跡波", art)
	lrc(t, filepath.Join(music, "01 Haruhikage.lrc"), "[00:00.00]迷子でもいい、迷子でも進め\n[00:00.40]答えを急がないで\n[00:00.80]そのままでいい\n")
	taggedMP3(t, filepath.Join(music, "04 Haruhikage (Live).mp3"), "春日影 (Live)", "MyGO!!!!!", "Live", albumArt(3))
	lib := map[string]any{"folders": []string{music}}
	b, _ := json.Marshal(lib)
	if err := os.WriteFile(filepath.Join(data, "library.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{}
	if os.Getenv("NSGO_LIGHT") != "" {
		settings["appearance"] = "light"
	}
	if os.Getenv("NSGO_EN") != "" {
		settings["language"] = "en"
	}
	if len(settings) > 0 {
		b, _ := json.Marshal(settings)
		if err := os.WriteFile(filepath.Join(data, "settings.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("NSGO_MUSIC_DATA_DIR", data)
	app := nsgotest.Start(t, app.Root, nsgotest.Size(1180, 780))
	app.WaitUntil(8*time.Second, func() bool { return app.Exists("text:迷跡波") })
	shot := func(name string) {
		time.Sleep(1500 * time.Millisecond)
		app.Settle()
		app.Screenshot(name)
	}
	shot("home")
	app.Press("label:迷跡波")
	app.WaitUntil(5*time.Second, func() bool { return app.Exists("text:詩超絆") })
	shot("album")
	app.Press("id:nav-artists")
	app.WaitUntil(5*time.Second, func() bool { return app.Exists("text:MyGO!!!!!") })
	app.Press("text:MyGO!!!!! >> nth=0")
	app.WaitUntil(5*time.Second, func() bool { return app.Exists("text:春日影 (Live)") })
	shot("artist")
	app.Press("text:Haruhikage >> nth=0")
	shot("player")
	app.Press("id:play-pause")
	app.Settle()
	app.Press("id:bar-open")
	app.WaitUntil(3*time.Second, func() bool { return app.Exists("text:迷子でもいい、迷子でも進め") })
	shot("now-playing")
	app.Press("id:np-tab-3")
	shot("stage")
	app.Press("id:np-tab-2")
	shot("wall")
	app.Press("id:np-tab-1")
	shot("queue")
	app.Press("id:np-close")
	app.Press("id:nav-liked")
	shot("liked")
	app.Type("id:search-input", "Haru")
	app.Submit("id:search-input")
	app.WaitUntil(5*time.Second, func() bool { return app.Exists("text:Haruhikage") })
	shot("results")
	app.Press("id:new-playlist")
	shot("dialog")
	app.Type("id:playlist-name", "Roadtrip")
	app.Press("id:name-save")
	app.WaitUntil(3*time.Second, func() bool { return app.Exists("text:Roadtrip") })
	shot("playlist")
	_ = core.Color(0)
}
