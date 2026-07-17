package sharp

import (
	"image"
	"image/color"
	"math"
)

// RGBA is a convenience colour type with 8-bit non-premultiplied channels. It
// is used for fills, tints and background colours.
type RGBA struct {
	R uint8
	G uint8
	B uint8
	A uint8
}

// NRGBA converts to the standard library's color.NRGBA.
func (c RGBA) NRGBA() color.NRGBA { return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A} }

// Black, White and Transparent are common fill colours.
var (
	Black       = RGBA{0, 0, 0, 255}
	White       = RGBA{255, 255, 255, 255}
	Transparent = RGBA{0, 0, 0, 0}
)

// eachPixel applies fn to every pixel's RGB channels in place. The alpha
// channel is left untouched. fn receives and returns 0-255 channel values.
func (p *Pipeline) eachPixel(fn func(r, g, b uint8) (uint8, uint8, uint8)) {
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			nr, ng, nb := fn(p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2])
			p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2] = nr, ng, nb
		}
	}
}

// Grayscale converts the image to grayscale using Rec. 601 luma weights.
func (p *Pipeline) Grayscale() *Pipeline {
	if p.err != nil {
		return p
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		y := clampF(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b))
		return y, y, y
	})
	return p
}

// Grayscale weights are also available via luma for callers that need them.
func luma(r, g, b uint8) float64 {
	return 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
}

// Negate inverts the RGB channels (photographic negative). Alpha is preserved.
func (p *Pipeline) Negate() *Pipeline {
	if p.err != nil {
		return p
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		return 255 - r, 255 - g, 255 - b
	})
	return p
}

// Invert is an alias for Negate.
func (p *Pipeline) Invert() *Pipeline { return p.Negate() }

// Tint multiplies each channel by the tint colour (normalised to [0,1]),
// producing a colour cast. A tint of White leaves the image unchanged.
func (p *Pipeline) Tint(t RGBA) *Pipeline {
	if p.err != nil {
		return p
	}
	fr := float64(t.R) / 255
	fg := float64(t.G) / 255
	fb := float64(t.B) / 255
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		return clampF(float64(r) * fr), clampF(float64(g) * fg), clampF(float64(b) * fb)
	})
	return p
}

// Brightness scales pixel intensity. A factor of 1 is a no-op, 0 produces
// black, and values above 1 brighten. Negative factors are clamped to 0.
func (p *Pipeline) Brightness(factor float64) *Pipeline {
	if p.err != nil {
		return p
	}
	if factor < 0 {
		factor = 0
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		return clampF(float64(r) * factor), clampF(float64(g) * factor), clampF(float64(b) * factor)
	})
	return p
}

// Contrast adjusts contrast about the midpoint (128). A factor of 1 is a no-op,
// values above 1 increase contrast and values in [0,1) reduce it.
func (p *Pipeline) Contrast(factor float64) *Pipeline {
	if p.err != nil {
		return p
	}
	if factor < 0 {
		factor = 0
	}
	adjust := func(v uint8) uint8 {
		return clampF((float64(v)-128)*factor + 128)
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		return adjust(r), adjust(g), adjust(b)
	})
	return p
}

// Gamma applies gamma correction. Output = 255 * (in/255)^(1/gamma). A gamma of
// 1 is a no-op; values above 1 brighten mid-tones. Gamma must be positive;
// non-positive values are ignored.
func (p *Pipeline) Gamma(gamma float64) *Pipeline {
	if p.err != nil {
		return p
	}
	if gamma <= 0 {
		return p
	}
	inv := 1.0 / gamma
	var lut [256]uint8
	for i := 0; i < 256; i++ {
		lut[i] = clampF(255 * math.Pow(float64(i)/255, inv))
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		return lut[r], lut[g], lut[b]
	})
	return p
}

// Saturation scales colour saturation about the pixel luma. A factor of 0
// yields grayscale, 1 is a no-op and values above 1 boost saturation.
func (p *Pipeline) Saturation(factor float64) *Pipeline {
	if p.err != nil {
		return p
	}
	if factor < 0 {
		factor = 0
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		l := luma(r, g, b)
		nr := l + (float64(r)-l)*factor
		ng := l + (float64(g)-l)*factor
		nb := l + (float64(b)-l)*factor
		return clampF(nr), clampF(ng), clampF(nb)
	})
	return p
}

// Threshold converts the image to black and white: pixels whose luma is greater
// than or equal to level (0-255) become white, the rest black. Alpha is
// preserved.
func (p *Pipeline) Threshold(level uint8) *Pipeline {
	if p.err != nil {
		return p
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		if luma(r, g, b) >= float64(level) {
			return 255, 255, 255
		}
		return 0, 0, 0
	})
	return p
}

// fillRGBA fills the whole image with c (non-premultiplied), writing straight
// into the RGBA buffer.
func fillRGBA(img *image.RGBA, c RGBA) {
	b := img.Bounds()
	if b.Empty() {
		return
	}
	// Fill first row, then copy it down.
	first := img.PixOffset(b.Min.X, b.Min.Y)
	for x := 0; x < b.Dx(); x++ {
		i := first + x*4
		img.Pix[i] = c.R
		img.Pix[i+1] = c.G
		img.Pix[i+2] = c.B
		img.Pix[i+3] = c.A
	}
	rowLen := b.Dx() * 4
	for y := b.Min.Y + 1; y < b.Max.Y; y++ {
		di := img.PixOffset(b.Min.X, y)
		copy(img.Pix[di:di+rowLen], img.Pix[first:first+rowLen])
	}
}
