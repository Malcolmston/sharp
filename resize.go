package sharp

import (
	"fmt"
	"image"
)

// Interpolation selects the sampling algorithm used when resizing or rotating.
type Interpolation int

// Available interpolation kernels.
const (
	// Nearest performs nearest-neighbour sampling: fast and exact for integer
	// scale factors, blocky otherwise.
	Nearest Interpolation = iota
	// Bilinear performs bilinear sampling: smoother, the sensible default.
	Bilinear
)

// Fit controls how the target dimensions are interpreted when both a width and
// a height are supplied to Resize.
type Fit int

// Fit modes.
const (
	// FitExact stretches the image to exactly width x height, ignoring aspect
	// ratio.
	FitExact Fit = iota
	// FitContain scales the image to fit entirely within width x height,
	// preserving aspect ratio. The output is the scaled image (no padding).
	FitContain
	// FitCover scales the image to completely cover width x height, preserving
	// aspect ratio, then centre-crops the overflow to exactly width x height.
	FitCover
)

// ResizeOptions configures a Resize operation. A zero Width or Height means
// "derive from the other dimension while preserving aspect ratio". At least one
// of Width or Height must be non-zero.
type ResizeOptions struct {
	Width         int
	Height        int
	Fit           Fit
	Interpolation Interpolation
}

// Resize scales the image according to opts.
func (p *Pipeline) Resize(opts ResizeOptions) *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		p.err = fmt.Errorf("sharp: Resize on empty image")
		return p
	}
	if opts.Width < 0 || opts.Height < 0 {
		p.err = fmt.Errorf("sharp: Resize with negative dimension")
		return p
	}
	if opts.Width == 0 && opts.Height == 0 {
		p.err = fmt.Errorf("sharp: Resize requires a width or height")
		return p
	}

	tw, th := opts.Width, opts.Height
	// Derive missing dimension from aspect ratio.
	if tw == 0 {
		tw = int(float64(th)*float64(sw)/float64(sh) + 0.5)
	}
	if th == 0 {
		th = int(float64(tw)*float64(sh)/float64(sw) + 0.5)
	}
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}

	switch opts.Fit {
	case FitExact:
		p.img = sample(p.img, tw, th, opts.Interpolation)
	case FitContain:
		cw, ch := scaleToFit(sw, sh, tw, th, false)
		p.img = sample(p.img, cw, ch, opts.Interpolation)
	case FitCover:
		cw, ch := scaleToFit(sw, sh, tw, th, true)
		scaled := sample(p.img, cw, ch, opts.Interpolation)
		p.img = centerCrop(scaled, tw, th)
	default:
		p.err = fmt.Errorf("sharp: unknown fit mode %d", opts.Fit)
	}
	return p
}

// ResizeTo is a convenience wrapper for a bilinear FitExact resize.
func (p *Pipeline) ResizeTo(width, height int) *Pipeline {
	return p.Resize(ResizeOptions{Width: width, Height: height, Fit: FitExact, Interpolation: Bilinear})
}

// scaleToFit returns the dimensions of the source scaled to either fit inside
// (cover=false) or cover (cover=true) the target box, preserving aspect ratio.
func scaleToFit(sw, sh, tw, th int, cover bool) (int, int) {
	rw := float64(tw) / float64(sw)
	rh := float64(th) / float64(sh)
	var r float64
	if cover {
		r = maxF(rw, rh)
	} else {
		r = minF(rw, rh)
	}
	w := int(float64(sw)*r + 0.5)
	h := int(float64(sh)*r + 0.5)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

// centerCrop crops src to w x h taken from its centre. If src is smaller than
// the requested crop in a dimension, the full extent is used.
func centerCrop(src *image.RGBA, w, h int) *image.RGBA {
	b := src.Bounds()
	if w > b.Dx() {
		w = b.Dx()
	}
	if h > b.Dy() {
		h = b.Dy()
	}
	ox := b.Min.X + (b.Dx()-w)/2
	oy := b.Min.Y + (b.Dy()-h)/2
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		si := src.PixOffset(ox, oy+y)
		di := dst.PixOffset(0, y)
		copy(dst.Pix[di:di+w*4], src.Pix[si:si+w*4])
	}
	return dst
}

// sample resizes src to tw x th using the chosen interpolation.
func sample(src *image.RGBA, tw, th int, interp Interpolation) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	if sw == tw && sh == th {
		copy(dst.Pix, src.Pix)
		return dst
	}
	switch interp {
	case Nearest:
		resizeNearest(src, dst)
	default:
		resizeBilinear(src, dst)
	}
	return dst
}

func resizeNearest(src, dst *image.RGBA) {
	sb, db := src.Bounds(), dst.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	tw, th := db.Dx(), db.Dy()
	for y := 0; y < th; y++ {
		sy := (y*sh + sh/2) / th
		if sy >= sh {
			sy = sh - 1
		}
		for x := 0; x < tw; x++ {
			sx := (x*sw + sw/2) / tw
			if sx >= sw {
				sx = sw - 1
			}
			si := src.PixOffset(sb.Min.X+sx, sb.Min.Y+sy)
			di := dst.PixOffset(x, y)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
}

func resizeBilinear(src, dst *image.RGBA) {
	sb, db := src.Bounds(), dst.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	tw, th := db.Dx(), db.Dy()

	// Map destination pixel centres back into source space.
	scaleX := float64(sw) / float64(tw)
	scaleY := float64(sh) / float64(th)

	for y := 0; y < th; y++ {
		fy := (float64(y)+0.5)*scaleY - 0.5
		y0 := int(fy)
		if fy < 0 {
			y0 = 0
			fy = 0
		}
		dy := fy - float64(y0)
		y1 := y0 + 1
		if y0 >= sh {
			y0 = sh - 1
		}
		if y1 >= sh {
			y1 = sh - 1
		}
		for x := 0; x < tw; x++ {
			fx := (float64(x)+0.5)*scaleX - 0.5
			x0 := int(fx)
			if fx < 0 {
				x0 = 0
				fx = 0
			}
			dx := fx - float64(x0)
			x1 := x0 + 1
			if x0 >= sw {
				x0 = sw - 1
			}
			if x1 >= sw {
				x1 = sw - 1
			}
			di := dst.PixOffset(x, y)
			i00 := src.PixOffset(sb.Min.X+x0, sb.Min.Y+y0)
			i10 := src.PixOffset(sb.Min.X+x1, sb.Min.Y+y0)
			i01 := src.PixOffset(sb.Min.X+x0, sb.Min.Y+y1)
			i11 := src.PixOffset(sb.Min.X+x1, sb.Min.Y+y1)
			for c := 0; c < 4; c++ {
				top := float64(src.Pix[i00+c])*(1-dx) + float64(src.Pix[i10+c])*dx
				bot := float64(src.Pix[i01+c])*(1-dx) + float64(src.Pix[i11+c])*dx
				dst.Pix[di+c] = clampF(top*(1-dy) + bot*dy)
			}
		}
	}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
