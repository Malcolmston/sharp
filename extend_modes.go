package sharp

import (
	"fmt"
	"image"
)

// ExtendMode selects how the border pixels added by ExtendWith are filled.
type ExtendMode int

// Extend fill strategies. They mirror the extendWith options of the Node sharp
// library.
const (
	// ExtendBackground fills the new border with a solid colour (the default).
	ExtendBackground ExtendMode = iota
	// ExtendCopy replicates the nearest edge pixel outward (edge clamp).
	ExtendCopy
	// ExtendRepeat tiles the source image to fill the border (wrap-around).
	ExtendRepeat
	// ExtendMirror reflects the source across each edge.
	ExtendMirror
)

// ExtendOptions configures an ExtendWith operation. Top, Right, Bottom and Left
// give the padding in pixels on each side (negative values are treated as
// zero). Background is used only when Mode is ExtendBackground.
type ExtendOptions struct {
	Top        int
	Right      int
	Bottom     int
	Left       int
	Background RGBA
	Mode       ExtendMode
}

// ExtendWith pads the image on each side using the chosen ExtendMode. Unlike
// Extend, which always fills with a solid colour, ExtendWith can replicate,
// tile or mirror the source pixels into the new border, matching the extendWith
// behaviour of the Node sharp library.
func (p *Pipeline) ExtendWith(opts ExtendOptions) *Pipeline {
	if p.err != nil {
		return p
	}
	top := maxInt(opts.Top, 0)
	right := maxInt(opts.Right, 0)
	bottom := maxInt(opts.Bottom, 0)
	left := maxInt(opts.Left, 0)
	if opts.Mode == ExtendBackground {
		return p.Extend(top, right, bottom, left, opts.Background)
	}
	if opts.Mode < ExtendBackground || opts.Mode > ExtendMirror {
		p.err = fmt.Errorf("sharp: ExtendWith unknown mode %d", opts.Mode)
		return p
	}

	b := p.img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		p.err = fmt.Errorf("sharp: ExtendWith on empty image")
		return p
	}
	w := sw + left + right
	h := sh + top + bottom
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := extendIndex(y-top, sh, opts.Mode)
		for x := 0; x < w; x++ {
			sx := extendIndex(x-left, sw, opts.Mode)
			si := p.img.PixOffset(b.Min.X+sx, b.Min.Y+sy)
			di := dst.PixOffset(x, y)
			copy(dst.Pix[di:di+4], p.img.Pix[si:si+4])
		}
	}
	p.img = dst
	return p
}

// extendIndex maps a possibly out-of-range coordinate i into [0,n) according to
// the border mode. n must be positive.
func extendIndex(i, n int, mode ExtendMode) int {
	if i >= 0 && i < n {
		return i
	}
	switch mode {
	case ExtendCopy:
		return clampInt(i, 0, n-1)
	case ExtendRepeat:
		i %= n
		if i < 0 {
			i += n
		}
		return i
	case ExtendMirror:
		if n == 1 {
			return 0
		}
		period := 2 * n
		i %= period
		if i < 0 {
			i += period
		}
		if i >= n {
			i = period - 1 - i
		}
		return i
	default:
		return clampInt(i, 0, n-1)
	}
}
