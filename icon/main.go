// Command icon packages the selected WoodWind artwork for release and dev
// builds. Run go run ./icon from the repository root to refresh the PNGs
// and the macOS ICNS. No Xcode icon compiler is needed.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"

	"golang.org/x/image/draw"
)

const source = "assets/woodwind-icon-anime/woodwind-icon-anime-1024.png"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 1024 {
		return fmt.Errorf("%s must be a 1024×1024 PNG", source)
	}
	for _, path := range []string{"resources/icon.png", "resources/icon-dev.png", "assets/Icon.png"} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	// PNG-backed ICNS entries cover standard and Retina displays.
	entries := []struct {
		kind string
		size int
	}{
		{"icp4", 16}, {"icp5", 32}, {"ic11", 32}, {"ic12", 64},
		{"ic07", 128}, {"ic08", 256}, {"ic13", 256},
		{"ic09", 512}, {"ic14", 512}, {"ic10", 1024},
	}
	var body bytes.Buffer
	encoded := map[int][]byte{1024: data}
	for _, entry := range entries {
		if encoded[entry.size] == nil {
			scaled := image.NewNRGBA(image.Rect(0, 0, entry.size, entry.size))
			draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Src, nil)
			var out bytes.Buffer
			if err := png.Encode(&out, scaled); err != nil {
				return err
			}
			encoded[entry.size] = out.Bytes()
		}
		body.WriteString(entry.kind)
		_ = binary.Write(&body, binary.BigEndian, uint32(8+len(encoded[entry.size])))
		body.Write(encoded[entry.size])
	}
	var icon bytes.Buffer
	icon.WriteString("icns")
	_ = binary.Write(&icon, binary.BigEndian, uint32(8+body.Len()))
	icon.Write(body.Bytes())
	return os.WriteFile("resources/icon.icns", icon.Bytes(), 0o644)
}
