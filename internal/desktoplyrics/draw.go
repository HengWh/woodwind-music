package desktoplyrics

import "github.com/nsgo-dev/nsgo/core"

func drawLyrics(d *core.Drawing, size core.Size, f Frame) {
	d.Clear(0, 0, float32(size.Width), float32(size.Height))
	drawLine(d, f.Words, float32(size.Width), 3, max(f.FontSize, 1), f.Color, true)
	drawLine(d, []Word{{Text: f.Secondary}}, float32(size.Width), 70, 18, 0xffffff, false)
}

func drawLine(d *core.Drawing, words []Word, width, top float32, fontSize int, color uint32, highlight bool) {
	font := core.Font{Size: core.Points(fontSize), Weight: core.FontWeightSemibold}
	widths := make([]float32, len(words))
	total := float32(0)
	for i, word := range words {
		widths[i] = d.MeasureText(word.Text, font)
		total += widths[i]
	}
	available := max(width-32, 1)
	scale := float32(1)
	if total > available {
		scale = available / total
	}
	d.Save().Translate((width-total*scale)/2, top).Scale(scale, scale)
	d.SetTextBaseline(core.BaselineTop).SetStrokeColor(core.RGB(0)).SetLineWidth(float32(fontSize) * .07)
	// All outlines go behind all fills, so adjacent CJK glyphs and the progress
	// boundary cannot have their thin strokes painted over by another outline.
	x := float32(0)
	for i, word := range words {
		d.StrokeText(word.Text, x, 0, font)
		x += widths[i]
	}
	d.SetFillColor(core.RGB(0xffffff))
	x = 0
	for i, word := range words {
		d.FillText(word.Text, x, 0, font)
		x += widths[i]
	}
	if highlight {
		d.SetFillColor(core.RGB(color))
		x = 0
		for i, word := range words {
			progress := float32(min(max(word.Progress, 0), 1))
			if progress > 0 {
				d.Save().BeginPath().Rect(x, -4, widths[i]*progress, float32(fontSize)*1.7).Clip()
				d.FillText(word.Text, x, 0, font).Restore()
			}
			x += widths[i]
		}
	}
	d.Restore()
}
