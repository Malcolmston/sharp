package sharp

import (
	"fmt"
	"math"
)

// NormaliseOptions configures a Normalise (contrast-stretch) operation.
type NormaliseOptions struct {
	// Lower and Upper are the luminance percentiles (0-100) mapped to 0 and 255
	// respectively. The zero value maps to Lower = 0, Upper = 100 (full range).
	Lower float64
	Upper float64
}

// Normalise stretches image contrast so that the chosen lower/upper luminance
// percentiles span the full [0,255] range. The same linear mapping is applied
// to all three colour channels, preserving hue. Alpha is untouched. If the
// image has no luminance spread the image is left unchanged.
func (p *Pipeline) Normalise(opts NormaliseOptions) *Pipeline {
	if p.err != nil {
		return p
	}
	lower, upper := opts.Lower, opts.Upper
	if lower == 0 && upper == 0 {
		upper = 100
	}
	if lower < 0 {
		lower = 0
	}
	if upper > 100 {
		upper = 100
	}
	if lower >= upper {
		return p
	}

	var hist [256]int
	b := p.img.Bounds()
	total := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			l := int(luma(p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2]) + 0.5)
			hist[clampInt(l, 0, 255)]++
			total++
		}
	}
	if total == 0 {
		return p
	}
	lo := percentile(hist[:], total, lower)
	hi := percentile(hist[:], total, upper)
	if hi <= lo {
		return p
	}
	scale := 255.0 / float64(hi-lo)
	var lut [256]uint8
	for i := 0; i < 256; i++ {
		lut[i] = clampF((float64(i) - float64(lo)) * scale)
	}
	p.eachPixel(func(r, g, bl uint8) (uint8, uint8, uint8) {
		return lut[r], lut[g], lut[bl]
	})
	return p
}

// Normalize is a US-spelling alias for Normalise.
func (p *Pipeline) Normalize(opts NormaliseOptions) *Pipeline { return p.Normalise(opts) }

// percentile returns the smallest intensity whose cumulative population reaches
// the given percentile pct (0-100) of total.
func percentile(hist []int, total int, pct float64) int {
	target := pct / 100 * float64(total)
	cum := 0
	for i := 0; i < len(hist); i++ {
		cum += hist[i]
		// Require at least one sample so a 0th percentile lands on the first
		// populated bin rather than on leading empty bins.
		if cum > 0 && float64(cum) >= target {
			return i
		}
	}
	return len(hist) - 1
}

// Linear applies an independent per-channel linear transform out = a*in + b to
// the RGB channels, with values in [0,255]. Each of a and b may have length 1
// (applied to all three channels) or length 3 (R, G, B). Alpha is preserved.
func (p *Pipeline) Linear(a, b []float64) *Pipeline {
	if p.err != nil {
		return p
	}
	ar, ag, ab, err := expand3(a, 1)
	if err != nil {
		p.err = fmt.Errorf("sharp: Linear slope: %w", err)
		return p
	}
	br, bg, bb, err := expand3(b, 0)
	if err != nil {
		p.err = fmt.Errorf("sharp: Linear offset: %w", err)
		return p
	}
	p.eachPixel(func(r, g, bl uint8) (uint8, uint8, uint8) {
		return clampF(ar*float64(r) + br), clampF(ag*float64(g) + bg), clampF(ab*float64(bl) + bb)
	})
	return p
}

// expand3 turns a scalar/3-vector option into three values, defaulting an empty
// slice to def.
func expand3(v []float64, def float64) (x, y, z float64, err error) {
	switch len(v) {
	case 0:
		return def, def, def, nil
	case 1:
		return v[0], v[0], v[0], nil
	case 3:
		return v[0], v[1], v[2], nil
	default:
		return 0, 0, 0, fmt.Errorf("length %d is not 0, 1 or 3", len(v))
	}
}

// ModulateOptions configures a Modulate operation. Multipliers default to 1 and
// additive/rotational terms default to 0 when left as the zero value.
type ModulateOptions struct {
	// Brightness multiplies lightness (1 = unchanged). Zero maps to 1.
	Brightness float64
	// Saturation multiplies saturation (1 = unchanged). Zero maps to 1.
	Saturation float64
	// Hue rotates the hue by this many degrees.
	Hue float64
	// Lightness adds this amount (in the 0-100 HSL lightness scale).
	Lightness float64
}

// Modulate adjusts brightness, saturation, hue and lightness together by
// converting each pixel to HSL, transforming, and converting back. It is a
// convenient single call for common tone tweaks. Alpha is preserved.
func (p *Pipeline) Modulate(opts ModulateOptions) *Pipeline {
	if p.err != nil {
		return p
	}
	brightness := opts.Brightness
	if brightness == 0 {
		brightness = 1
	}
	saturation := opts.Saturation
	if saturation == 0 {
		saturation = 1
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		h, s, l := rgbToHSL(r, g, b)
		h += opts.Hue / 360
		h -= math.Floor(h)
		s = clamp01(s * saturation)
		l = clamp01(l*brightness + opts.Lightness/100)
		return hslToRGB(h, s, l)
	})
	return p
}

// clamp01 clamps a float to [0,1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// rgbToHSL converts 8-bit RGB to HSL with each component in [0,1].
func rgbToHSL(r8, g8, b8 uint8) (h, s, l float64) {
	r := float64(r8) / 255
	g := float64(g8) / 255
	b := float64(b8) / 255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2
	if max == min {
		return 0, 0, l
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h /= 6
	return h, s, l
}

// hslToRGB converts HSL (components in [0,1]) back to 8-bit RGB.
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	if s == 0 {
		v := clampF(l * 255)
		return v, v, v
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r := hueToRGB(p, q, h+1.0/3)
	g := hueToRGB(p, q, h)
	b := hueToRGB(p, q, h-1.0/3)
	return clampF(r * 255), clampF(g * 255), clampF(b * 255)
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 1.0/2:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	default:
		return p
	}
}

// CLAHEOptions configures Contrast Limited Adaptive Histogram Equalisation.
type CLAHEOptions struct {
	// Width and Height are the tile dimensions in pixels. Zero maps to a
	// sensible default of one eighth of the image (minimum 1).
	Width  int
	Height int
	// MaxSlope is the contrast-limiting clip factor (typical 2-4). Zero maps to
	// 3. A value of 0 after mapping disables clipping.
	MaxSlope int
}

// CLAHE applies Contrast Limited Adaptive Histogram Equalisation to the image
// luminance and rescales the RGB channels to match, enhancing local contrast
// without amplifying noise as strongly as global equalisation. Tile mappings
// are bilinearly interpolated to avoid block artefacts. Alpha is preserved.
func (p *Pipeline) CLAHE(opts CLAHEOptions) *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return p
	}
	tileW, tileH := opts.Width, opts.Height
	if tileW <= 0 {
		tileW = maxInt(1, w/8)
	}
	if tileH <= 0 {
		tileH = maxInt(1, h/8)
	}
	maxSlope := opts.MaxSlope
	if opts.MaxSlope == 0 {
		maxSlope = 3
	}
	nx := (w + tileW - 1) / tileW
	ny := (h + tileH - 1) / tileH
	if nx < 1 {
		nx = 1
	}
	if ny < 1 {
		ny = 1
	}

	// Per-pixel luminance.
	lumaBuf := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < w; x++ {
			i := row + x*4
			lumaBuf[y*w+x] = uint8(luma(p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2]) + 0.5)
		}
	}

	// Build a mapping LUT for each tile.
	maps := make([][256]uint8, nx*ny)
	for ty := 0; ty < ny; ty++ {
		for tx := 0; tx < nx; tx++ {
			x0 := tx * tileW
			y0 := ty * tileH
			x1 := minInt(x0+tileW, w)
			y1 := minInt(y0+tileH, h)
			var hist [256]int
			count := 0
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					hist[lumaBuf[y*w+x]]++
					count++
				}
			}
			maps[ty*nx+tx] = claheMapping(hist, count, maxSlope)
		}
	}

	// Bilinearly interpolate the four surrounding tile mappings per pixel.
	for y := 0; y < h; y++ {
		gy := (float64(y)+0.5)/float64(tileH) - 0.5
		ty0 := int(math.Floor(gy))
		fy := gy - float64(ty0)
		ty1 := ty0 + 1
		ty0 = clampInt(ty0, 0, ny-1)
		ty1 = clampInt(ty1, 0, ny-1)
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < w; x++ {
			gx := (float64(x)+0.5)/float64(tileW) - 0.5
			tx0 := int(math.Floor(gx))
			fx := gx - float64(tx0)
			tx1 := tx0 + 1
			tx0 = clampInt(tx0, 0, nx-1)
			tx1 = clampInt(tx1, 0, nx-1)

			v := lumaBuf[y*w+x]
			m00 := float64(maps[ty0*nx+tx0][v])
			m10 := float64(maps[ty0*nx+tx1][v])
			m01 := float64(maps[ty1*nx+tx0][v])
			m11 := float64(maps[ty1*nx+tx1][v])
			top := m00*(1-fx) + m10*fx
			bot := m01*(1-fx) + m11*fx
			newL := top*(1-fy) + bot*fy

			i := row + x*4
			old := float64(v)
			if old <= 0 {
				continue
			}
			ratio := newL / old
			p.img.Pix[i] = clampF(float64(p.img.Pix[i]) * ratio)
			p.img.Pix[i+1] = clampF(float64(p.img.Pix[i+1]) * ratio)
			p.img.Pix[i+2] = clampF(float64(p.img.Pix[i+2]) * ratio)
		}
	}
	return p
}

// claheMapping builds a contrast-limited, redistributed CDF mapping for a tile
// histogram of count pixels.
func claheMapping(hist [256]int, count, maxSlope int) [256]uint8 {
	var out [256]uint8
	if count == 0 {
		for i := range out {
			out[i] = uint8(i)
		}
		return out
	}
	if maxSlope > 0 {
		clip := maxSlope * count / 256
		if clip < 1 {
			clip = 1
		}
		excess := 0
		for i := range hist {
			if hist[i] > clip {
				excess += hist[i] - clip
				hist[i] = clip
			}
		}
		// Redistribute clipped mass uniformly.
		bonus := excess / 256
		for i := range hist {
			hist[i] += bonus
		}
	}
	cum := 0
	for i := 0; i < 256; i++ {
		cum += hist[i]
		out[i] = clampF(float64(cum) / float64(count) * 255)
	}
	return out
}

// minInt returns the smaller of two ints.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
