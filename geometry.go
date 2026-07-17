package sharp

import (
	"fmt"
	"image"
	"math"
)

// Rectangle describes an axis-aligned region for crop/extract operations.
type Rectangle struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Crop extracts the rectangular region r from the current image. It is an alias
// for Extract. The region must lie fully within the image bounds.
func (p *Pipeline) Crop(r Rectangle) *Pipeline { return p.Extract(r) }

// Extract extracts the rectangular region r from the current image.
func (p *Pipeline) Extract(r Rectangle) *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	if r.Width <= 0 || r.Height <= 0 {
		p.err = fmt.Errorf("sharp: Extract requires positive width and height")
		return p
	}
	if r.X < 0 || r.Y < 0 || r.X+r.Width > b.Dx() || r.Y+r.Height > b.Dy() {
		p.err = fmt.Errorf("sharp: Extract region %+v out of bounds %dx%d", r, b.Dx(), b.Dy())
		return p
	}
	dst := image.NewRGBA(image.Rect(0, 0, r.Width, r.Height))
	for y := 0; y < r.Height; y++ {
		si := p.img.PixOffset(r.X, r.Y+y)
		di := dst.PixOffset(0, y)
		copy(dst.Pix[di:di+r.Width*4], p.img.Pix[si:si+r.Width*4])
	}
	p.img = dst
	return p
}

// Extend pads the image by the given number of pixels on each side, filling the
// new border with fill. Negative values are treated as zero.
func (p *Pipeline) Extend(top, right, bottom, left int, fill RGBA) *Pipeline {
	if p.err != nil {
		return p
	}
	top = maxInt(top, 0)
	right = maxInt(right, 0)
	bottom = maxInt(bottom, 0)
	left = maxInt(left, 0)
	b := p.img.Bounds()
	w := b.Dx() + left + right
	h := b.Dy() + top + bottom
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRGBA(dst, fill)
	for y := 0; y < b.Dy(); y++ {
		si := p.img.PixOffset(b.Min.X, b.Min.Y+y)
		di := dst.PixOffset(left, top+y)
		copy(dst.Pix[di:di+b.Dx()*4], p.img.Pix[si:si+b.Dx()*4])
	}
	p.img = dst
	return p
}

// FlipVertical mirrors the image top-to-bottom.
func (p *Pipeline) FlipVertical() *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		si := p.img.PixOffset(0, h-1-y)
		di := dst.PixOffset(0, y)
		copy(dst.Pix[di:di+w*4], p.img.Pix[si:si+w*4])
	}
	p.img = dst
	return p
}

// Flip is an alias for FlipVertical.
func (p *Pipeline) Flip() *Pipeline { return p.FlipVertical() }

// FlipHorizontal mirrors the image left-to-right.
func (p *Pipeline) FlipHorizontal() *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := p.img.PixOffset(w-1-x, y)
			di := dst.PixOffset(x, y)
			copy(dst.Pix[di:di+4], p.img.Pix[si:si+4])
		}
	}
	p.img = dst
	return p
}

// Flop is an alias for FlipHorizontal.
func (p *Pipeline) Flop() *Pipeline { return p.FlipHorizontal() }

// Rotate90 rotates the image 90 degrees clockwise.
func (p *Pipeline) Rotate90() *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := p.img.PixOffset(x, y)
			di := dst.PixOffset(h-1-y, x)
			copy(dst.Pix[di:di+4], p.img.Pix[si:si+4])
		}
	}
	p.img = dst
	return p
}

// Rotate180 rotates the image 180 degrees.
func (p *Pipeline) Rotate180() *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := p.img.PixOffset(x, y)
			di := dst.PixOffset(w-1-x, h-1-y)
			copy(dst.Pix[di:di+4], p.img.Pix[si:si+4])
		}
	}
	p.img = dst
	return p
}

// Rotate270 rotates the image 270 degrees clockwise (90 counter-clockwise).
func (p *Pipeline) Rotate270() *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := p.img.PixOffset(x, y)
			di := dst.PixOffset(y, w-1-x)
			copy(dst.Pix[di:di+4], p.img.Pix[si:si+4])
		}
	}
	p.img = dst
	return p
}

// Rotate rotates the image by an arbitrary angle in degrees (clockwise) about
// its centre using bilinear sampling. The output canvas is enlarged to hold the
// whole rotated image and the exposed corners are filled with fill. Multiples
// of 90 degrees are dispatched to the exact rotations.
func (p *Pipeline) Rotate(degrees float64, fill RGBA) *Pipeline {
	if p.err != nil {
		return p
	}
	// Normalise to [0,360).
	norm := math.Mod(degrees, 360)
	if norm < 0 {
		norm += 360
	}
	switch norm {
	case 0:
		return p
	case 90:
		return p.Rotate90()
	case 180:
		return p.Rotate180()
	case 270:
		return p.Rotate270()
	}

	b := p.img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	theta := norm * math.Pi / 180
	sin, cos := math.Sin(theta), math.Cos(theta)

	// New bounding box that contains the rotated source.
	nw := int(math.Ceil(math.Abs(float64(sw)*cos) + math.Abs(float64(sh)*sin)))
	nh := int(math.Ceil(math.Abs(float64(sw)*sin) + math.Abs(float64(sh)*cos)))
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	fillRGBA(dst, fill)

	scx := float64(sw) / 2
	scy := float64(sh) / 2
	dcx := float64(nw) / 2
	dcy := float64(nh) / 2

	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			// Rotate destination coords backwards into source space.
			rx := float64(x) + 0.5 - dcx
			ry := float64(y) + 0.5 - dcy
			sx := cos*rx + sin*ry + scx - 0.5
			sy := -sin*rx + cos*ry + scy - 0.5
			if sx < -0.5 || sx > float64(sw)-0.5 || sy < -0.5 || sy > float64(sh)-0.5 {
				continue
			}
			di := dst.PixOffset(x, y)
			bilinearSample(p.img, sx, sy, dst.Pix[di:di+4])
		}
	}
	p.img = dst
	return p
}

// bilinearSample writes the bilinearly interpolated RGBA sample at (sx,sy) into
// out (length 4). Coordinates are clamped to the image edges.
func bilinearSample(src *image.RGBA, sx, sy float64, out []uint8) {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	x0 := int(math.Floor(sx))
	y0 := int(math.Floor(sy))
	dx := sx - float64(x0)
	dy := sy - float64(y0)
	x1, y1 := x0+1, y0+1
	x0 = clampInt(x0, 0, sw-1)
	x1 = clampInt(x1, 0, sw-1)
	y0 = clampInt(y0, 0, sh-1)
	y1 = clampInt(y1, 0, sh-1)
	i00 := src.PixOffset(b.Min.X+x0, b.Min.Y+y0)
	i10 := src.PixOffset(b.Min.X+x1, b.Min.Y+y0)
	i01 := src.PixOffset(b.Min.X+x0, b.Min.Y+y1)
	i11 := src.PixOffset(b.Min.X+x1, b.Min.Y+y1)
	for c := 0; c < 4; c++ {
		top := float64(src.Pix[i00+c])*(1-dx) + float64(src.Pix[i10+c])*dx
		bot := float64(src.Pix[i01+c])*(1-dx) + float64(src.Pix[i11+c])*dx
		out[c] = clampF(top*(1-dy) + bot*dy)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
