// Command tray draws the wood wind music headphones and pine as a monochrome
// macOS menu bar template, supersampled at 2× of the menu bar's 18 points.
//
// Run it from the repository root:
//
//	go run ./icon/tray
//
// With -preview dir, it also writes the icon over light and dark menu bars,
// enlarged, to check by eye.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

type pt struct{ x, y float64 }

func sub(a, b pt) pt      { return pt{a.x - b.x, a.y - b.y} }
func length(a pt) float64 { return math.Hypot(a.x, a.y) }

// Keep only the strong silhouettes at menu bar sizes; the leaves and
// illustrated highlights belong to the full-color application icon.
const fit = 15.6

func roundedBox(p, center, half pt, radius float64) bool {
	q := pt{math.Abs(p.x-center.x) - half.x + radius, math.Abs(p.y-center.y) - half.y + radius}
	return length(pt{math.Max(q.x, 0), math.Max(q.y, 0)})+math.Min(math.Max(q.x, q.y), 0) <= radius
}

func inside(p pt) bool {
	band := p.y <= 8.5 && math.Abs(length(sub(p, pt{9, 8.5}))-6.5) <= 0.8
	cups := roundedBox(p, pt{2.5, 11}, pt{1.4, 2.4}, 0.9) ||
		roundedBox(p, pt{15.5, 11}, pt{1.4, 2.4}, 0.9)
	tree := p.y >= 14.3 && p.y <= 15.6 && math.Abs(p.x-9) <= 0.55
	for _, tier := range []struct{ top, bottom, width float64 }{
		{7.0, 10.0, 1.5}, {9.2, 12.1, 2.1}, {11.1, 14.4, 2.7},
	} {
		if p.y >= tier.top && p.y <= tier.bottom &&
			math.Abs(p.x-9) <= tier.width*(p.y-tier.top)/(tier.bottom-tier.top) {
			tree = true
		}
	}
	return band || cups || tree
}

// frame is the scale and offset from the square to the glyph, which fit
// the glyph's bounds to the square.
func frame() (scale float64, off pt) {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for y := -5.0; y < 25; y += 0.02 {
		for x := -5.0; x < 25; x += 0.02 {
			if inside(pt{x, y}) {
				x0, y0, x1, y1 = math.Min(x0, x), math.Min(y0, y), math.Max(x1, x), math.Max(y1, y)
			}
		}
	}
	scale = fit / math.Max(x1-x0, y1-y0)
	return scale, pt{(x0+x1)/2 - 9/scale, (y0+y1)/2 - 9/scale}
}

// render draws the icon px pixels square, each pixel's alpha the share of
// 16×16 samples inside the glyph.
func render(px int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, px, px))
	s := float64(px) / 18
	k, off := frame()
	const n = 16
	for y := 0; y < px; y++ {
		for x := 0; x < px; x++ {
			hit := 0
			for j := 0; j < n; j++ {
				for i := 0; i < n; i++ {
					p := pt{(float64(x)+(float64(i)+0.5)/n)/s/k + off.x, (float64(y)+(float64(j)+0.5)/n)/s/k + off.y}
					if inside(p) {
						hit++
					}
				}
			}
			img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, uint8(math.Round(float64(hit) * 255 / (n * n)))})
		}
	}
	return img
}

// preview puts the icon on a light menu bar and a dark one, tinted as
// macOS tints it, and enlarged with nearest pixels so each one shows.
func preview(icon *image.NRGBA, zoom int) *image.NRGBA {
	w := icon.Bounds().Dx()
	pad := w / 3
	side := w + 2*pad
	out := image.NewNRGBA(image.Rect(0, 0, side*2*zoom, side*zoom))
	grounds := []color.NRGBA{{236, 236, 238, 255}, {40, 40, 44, 255}}
	inks := []float64{0, 255}
	for g := range grounds {
		for y := 0; y < side; y++ {
			for x := 0; x < side; x++ {
				c := grounds[g]
				if ix, iy := x-pad, y-pad; ix >= 0 && iy >= 0 && ix < w && iy < w {
					a := float64(icon.NRGBAAt(ix, iy).A) / 255
					mix := func(v uint8) uint8 { return uint8(float64(v)*(1-a) + inks[g]*a) }
					c = color.NRGBA{mix(c.R), mix(c.G), mix(c.B), 255}
				}
				for dy := 0; dy < zoom; dy++ {
					for dx := 0; dx < zoom; dx++ {
						out.SetNRGBA((g*side+x)*zoom+dx, y*zoom+dy, c)
					}
				}
			}
		}
	}
	return out
}

func save(name string, img image.Image) {
	f, err := os.Create(name)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	out := flag.String("o", "internal/app/tray/tray-template.png", "where to write the icon")
	dir := flag.String("preview", "", "a directory for previews")
	flag.Parse()
	icon := render(36)
	save(*out, icon)
	if *dir != "" {
		save(filepath.Join(*dir, "preview-2x.png"), preview(icon, 8))
		save(filepath.Join(*dir, "preview-1x.png"), preview(render(18), 12))
		save(filepath.Join(*dir, "preview-large.png"), preview(render(360), 1))
	}
}
