package sharp

import (
	"fmt"
	"image"
)

// Channel identifies one band of an RGBA image.
type Channel int

// Channel indices.
const (
	ChannelR Channel = iota
	ChannelG
	ChannelB
	ChannelA
)

// ExtractChannel reduces the image to a single band, producing a grayscale
// image whose R, G and B are all set to the selected channel's value and whose
// alpha is fully opaque.
func (p *Pipeline) ExtractChannel(ch Channel) *Pipeline {
	if p.err != nil {
		return p
	}
	if ch < ChannelR || ch > ChannelA {
		p.err = fmt.Errorf("sharp: ExtractChannel invalid channel %d", ch)
		return p
	}
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			v := p.img.Pix[i+int(ch)]
			p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2], p.img.Pix[i+3] = v, v, v, 255
		}
	}
	return p
}

// RemoveAlpha discards transparency by setting every pixel's alpha to fully
// opaque. RGB values are left untouched (they are not pre-multiplied).
func (p *Pipeline) RemoveAlpha() *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			p.img.Pix[row+x*4+3] = 255
		}
	}
	return p
}

// EnsureAlpha guarantees the image carries an alpha channel. Because the
// internal buffer is always RGBA, this is a no-op when any pixel is already
// non-opaque; when the image is fully opaque (effectively no alpha information)
// every pixel's alpha is set to round(alpha*255), where alpha is in [0,1].
func (p *Pipeline) EnsureAlpha(alpha float64) *Pipeline {
	if p.err != nil {
		return p
	}
	if p.hasAlpha() {
		return p
	}
	a := clampF(clamp01(alpha) * 255)
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			p.img.Pix[row+x*4+3] = a
		}
	}
	return p
}

// hasAlpha reports whether any pixel is non-opaque.
func (p *Pipeline) hasAlpha() bool {
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			if p.img.Pix[row+x*4+3] != 255 {
				return true
			}
		}
	}
	return false
}

// JoinChannels builds a new pipeline by combining single-band images into the
// R, G, B and (optionally) A channels of one image. Each source contributes its
// red channel as the band value. Exactly 3 or 4 images must be supplied and all
// must share the same dimensions; with 3 the result is fully opaque.
func JoinChannels(channels ...image.Image) *Pipeline {
	if len(channels) != 3 && len(channels) != 4 {
		return &Pipeline{err: fmt.Errorf("sharp: JoinChannels needs 3 or 4 images, got %d", len(channels))}
	}
	bands := make([]*image.RGBA, len(channels))
	for i, c := range channels {
		if c == nil {
			return &Pipeline{err: fmt.Errorf("sharp: JoinChannels image %d is nil", i)}
		}
		bands[i] = toRGBA(c)
	}
	w, h := bands[0].Bounds().Dx(), bands[0].Bounds().Dy()
	for i, band := range bands {
		if band.Bounds().Dx() != w || band.Bounds().Dy() != h {
			return &Pipeline{err: fmt.Errorf("sharp: JoinChannels image %d size mismatch", i)}
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			di := dst.PixOffset(x, y)
			for c := 0; c < len(bands); c++ {
				dst.Pix[di+c] = bands[c].Pix[bands[c].PixOffset(x, y)]
			}
			if len(bands) == 3 {
				dst.Pix[di+3] = 255
			}
		}
	}
	return &Pipeline{img: dst, density: defaultDensity}
}

// Recomb multiplies each pixel's channel vector by a colour matrix. A 3x3
// matrix recombines RGB (alpha preserved); a 4x4 matrix recombines RGBA. The
// matrix is given row-major as a slice of rows. Results are clamped to [0,255].
func (p *Pipeline) Recomb(matrix [][]float64) *Pipeline {
	if p.err != nil {
		return p
	}
	n := len(matrix)
	if n != 3 && n != 4 {
		p.err = fmt.Errorf("sharp: Recomb matrix must be 3x3 or 4x4, got %d rows", n)
		return p
	}
	for i, row := range matrix {
		if len(row) != n {
			p.err = fmt.Errorf("sharp: Recomb row %d has length %d, want %d", i, len(row), n)
			return p
		}
	}
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			in := [4]float64{float64(p.img.Pix[i]), float64(p.img.Pix[i+1]), float64(p.img.Pix[i+2]), float64(p.img.Pix[i+3])}
			var out [4]float64
			for r := 0; r < n; r++ {
				for cc := 0; cc < n; cc++ {
					out[r] += matrix[r][cc] * in[cc]
				}
			}
			p.img.Pix[i] = clampF(out[0])
			p.img.Pix[i+1] = clampF(out[1])
			p.img.Pix[i+2] = clampF(out[2])
			if n == 4 {
				p.img.Pix[i+3] = clampF(out[3])
			}
		}
	}
	return p
}

// BoolOp selects a bitwise operation for Boolean and Bandbool.
type BoolOp int

// Bitwise operations.
const (
	BoolAnd BoolOp = iota
	BoolOr
	BoolXor
)

// applyBool applies a bitwise operator to two bytes.
func applyBool(op BoolOp, a, b uint8) uint8 {
	switch op {
	case BoolAnd:
		return a & b
	case BoolOr:
		return a | b
	default:
		return a ^ b
	}
}

// Boolean applies a per-channel bitwise operation between the current image and
// other, pixel-for-pixel over the overlapping region. The two images must share
// dimensions. Alpha is combined with the same operator.
func (p *Pipeline) Boolean(other image.Image, op BoolOp) *Pipeline {
	if p.err != nil {
		return p
	}
	if op < BoolAnd || op > BoolXor {
		p.err = fmt.Errorf("sharp: Boolean invalid op %d", op)
		return p
	}
	if other == nil {
		p.err = fmt.Errorf("sharp: Boolean called with nil image")
		return p
	}
	ov := toRGBA(other)
	b := p.img.Bounds()
	if ov.Bounds().Dx() != b.Dx() || ov.Bounds().Dy() != b.Dy() {
		p.err = fmt.Errorf("sharp: Boolean size mismatch %dx%d vs %dx%d", b.Dx(), b.Dy(), ov.Bounds().Dx(), ov.Bounds().Dy())
		return p
	}
	for y := 0; y < b.Dy(); y++ {
		prow := p.img.PixOffset(b.Min.X, b.Min.Y+y)
		orow := ov.PixOffset(0, y)
		for x := 0; x < b.Dx(); x++ {
			pi := prow + x*4
			oi := orow + x*4
			for c := 0; c < 4; c++ {
				p.img.Pix[pi+c] = applyBool(op, p.img.Pix[pi+c], ov.Pix[oi+c])
			}
		}
	}
	return p
}

// Bandbool reduces the three colour channels of each pixel into a single value
// by folding them together with a bitwise operator, producing a grayscale
// (single-band) image with the result written to R, G and B. Alpha is set fully
// opaque.
func (p *Pipeline) Bandbool(op BoolOp) *Pipeline {
	if p.err != nil {
		return p
	}
	if op < BoolAnd || op > BoolXor {
		p.err = fmt.Errorf("sharp: Bandbool invalid op %d", op)
		return p
	}
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			v := applyBool(op, applyBool(op, p.img.Pix[i], p.img.Pix[i+1]), p.img.Pix[i+2])
			p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2], p.img.Pix[i+3] = v, v, v, 255
		}
	}
	return p
}
