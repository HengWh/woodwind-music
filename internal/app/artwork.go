package app

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"

	"github.com/nsgo-dev/nsgo/core"
	_ "golang.org/x/image/webp"

	"github.com/HengWh/woodwind-music/internal/music"
)

// coverTint returns the most vivid color of the cover at url, loading it if
// the app hasn't seen it: 0 until it has, so the daily card shows its red.
func (a *app) coverTint(url string) core.Color {
	if url == "" {
		return 0
	}
	if a.tintDone[url] {
		return a.tints[url]
	}
	if a.tintBusy[url] {
		return 0
	}
	if a.tints == nil {
		a.tints, a.tintDone, a.tintBusy = map[string]core.Color{}, map[string]bool{}, map[string]bool{}
	}
	a.tintBusy[url] = true
	go func() {
		data, err := music.Fetch(url)
		var tint core.Color
		if err == nil {
			tint = dominantColor(data)
		}
		a.post(func() {
			delete(a.tintBusy, url)
			a.tintDone[url] = true
			if tint != 0 {
				a.tints[url] = tint
			}
			a.artRev.Update(func(n int) int { return n + 1 })
		})
	}()
	return 0
}

// dominantColor is a cover's most vivid color, weighted by how saturated and
// bright it is, as MyGo finds it: a gray cover's average.
func dominantColor(data []byte) core.Color {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0
	}
	b := img.Bounds()
	const n = 24
	const buckets = 18
	var weight [buckets]float64
	var sum [buckets][3]float64
	var gray [3]float64
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return 0
	}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x*w/n+w/(2*n), b.Min.Y+y*h/n+h/(2*n))).(color.NRGBA)
			r, g, bl := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
			hue, sat, val := hsv(r, g, bl)
			gray[0], gray[1], gray[2] = gray[0]+r, gray[1]+g, gray[2]+bl
			wt := sat * sat * val
			if sat < 0.15 || val < 0.15 {
				continue
			}
			k := int(hue/360*buckets) % buckets
			weight[k] += wt
			sum[k][0], sum[k][1], sum[k][2] = sum[k][0]+r*wt, sum[k][1]+g*wt, sum[k][2]+bl*wt
		}
	}
	best := 0
	for k := range weight {
		if weight[k] > weight[best] {
			best = k
		}
	}
	if weight[best] < 2 {
		total := float64(n * n)
		return core.RGB(rgbHex(gray[0]/total, gray[1]/total, gray[2]/total))
	}
	wt := weight[best]
	return core.RGB(rgbHex(sum[best][0]/wt, sum[best][1]/wt, sum[best][2]/wt))
}

func rgbHex(r, g, b float64) uint32 {
	at := func(v float64) uint32 { return uint32(min(max(v, 0), 1)*255 + 0.5) }
	return at(r)<<16 | at(g)<<8 | at(b)
}

// hsv turns red, green and blue, from 0 to 1, into a hue in degrees, a
// saturation and a value.
func hsv(r, g, b float64) (h, s, v float64) {
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	v = mx
	if mx == 0 {
		return 0, 0, 0
	}
	s = (mx - mn) / mx
	d := mx - mn
	switch {
	case d == 0:
		h = 0
	case mx == r:
		h = math.Mod((g-b)/d, 6) * 60
	case mx == g:
		h = ((b-r)/d + 2) * 60
	default:
		h = ((r-g)/d + 4) * 60
	}
	if h < 0 {
		h += 360
	}
	return h, s, v
}

// hsvColor is the color of hue h in degrees, saturation s and value v.
func hsvColor(h, s, v float64) core.Color {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := v - c
	return core.RGB(rgbHex(r+m, g+m, b+m))
}
