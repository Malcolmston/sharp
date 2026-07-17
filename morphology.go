package sharp

import (
	"fmt"
	"image"
	"sort"
)

// MorphOp selects a morphological operation for Morphology.
type MorphOp int

// Morphological operations. Erode and Dilate use a square structuring element;
// Open is erode-then-dilate (removes small bright specks) and Close is
// dilate-then-erode (fills small dark holes).
const (
	MorphErode MorphOp = iota
	MorphDilate
	MorphOpen
	MorphClose
)

// Median replaces each pixel with the per-channel median of its
// (2*radius+1)x(2*radius+1) neighbourhood, an edge-preserving noise filter.
// Edges are handled by clamping sample coordinates. A radius < 1 is a no-op.
func (p *Pipeline) Median(radius int) *Pipeline {
	if p.err != nil {
		return p
	}
	if radius < 1 {
		return p
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	window := (2*radius + 1) * (2*radius + 1)
	var rs, gs, bs, as []uint8
	rs = make([]uint8, 0, window)
	gs = make([]uint8, 0, window)
	bs = make([]uint8, 0, window)
	as = make([]uint8, 0, window)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rs, gs, bs, as = rs[:0], gs[:0], bs[:0], as[:0]
			for ky := -radius; ky <= radius; ky++ {
				sy := clampInt(y+ky, 0, h-1)
				for kx := -radius; kx <= radius; kx++ {
					sx := clampInt(x+kx, 0, w-1)
					i := p.img.PixOffset(b.Min.X+sx, b.Min.Y+sy)
					rs = append(rs, p.img.Pix[i])
					gs = append(gs, p.img.Pix[i+1])
					bs = append(bs, p.img.Pix[i+2])
					as = append(as, p.img.Pix[i+3])
				}
			}
			di := dst.PixOffset(x, y)
			dst.Pix[di] = medianOf(rs)
			dst.Pix[di+1] = medianOf(gs)
			dst.Pix[di+2] = medianOf(bs)
			dst.Pix[di+3] = medianOf(as)
		}
	}
	p.img = dst
	return p
}

// medianOf returns the median of vals, mutating (sorting) the slice in place.
func medianOf(vals []uint8) uint8 {
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	return vals[len(vals)/2]
}

// Erode applies grayscale morphological erosion with a square structuring
// element of the given radius (each channel replaced by its neighbourhood
// minimum). A radius < 1 is a no-op.
func (p *Pipeline) Erode(radius int) *Pipeline { return p.Morphology(MorphErode, radius) }

// Dilate applies grayscale morphological dilation with a square structuring
// element of the given radius (each channel replaced by its neighbourhood
// maximum). A radius < 1 is a no-op.
func (p *Pipeline) Dilate(radius int) *Pipeline { return p.Morphology(MorphDilate, radius) }

// Morphology applies the morphological operation op with a square structuring
// element of the given radius. Open and Close are compositions of Erode and
// Dilate. A radius < 1 is a no-op. RGB and alpha are all processed.
func (p *Pipeline) Morphology(op MorphOp, radius int) *Pipeline {
	if p.err != nil {
		return p
	}
	if radius < 1 {
		return p
	}
	switch op {
	case MorphErode:
		p.img = rankFilter(p.img, radius, false)
	case MorphDilate:
		p.img = rankFilter(p.img, radius, true)
	case MorphOpen:
		p.img = rankFilter(rankFilter(p.img, radius, false), radius, true)
	case MorphClose:
		p.img = rankFilter(rankFilter(p.img, radius, true), radius, false)
	default:
		p.err = fmt.Errorf("sharp: unknown morphology op %d", op)
	}
	return p
}

// rankFilter returns a copy of src where each channel is replaced by the
// neighbourhood maximum (dilate=true) or minimum (dilate=false) over a square
// window of the given radius. Coordinates are clamped at the edges.
func rankFilter(src *image.RGBA, radius int, dilate bool) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var best [4]uint8
			if dilate {
				best = [4]uint8{0, 0, 0, 0}
			} else {
				best = [4]uint8{255, 255, 255, 255}
			}
			for ky := -radius; ky <= radius; ky++ {
				sy := clampInt(y+ky, 0, h-1)
				for kx := -radius; kx <= radius; kx++ {
					sx := clampInt(x+kx, 0, w-1)
					i := src.PixOffset(b.Min.X+sx, b.Min.Y+sy)
					for c := 0; c < 4; c++ {
						v := src.Pix[i+c]
						if dilate {
							if v > best[c] {
								best[c] = v
							}
						} else if v < best[c] {
							best[c] = v
						}
					}
				}
			}
			di := dst.PixOffset(x, y)
			copy(dst.Pix[di:di+4], best[:])
		}
	}
	return dst
}

// UnsharpOptions configures an Unsharp (unsharp-mask) sharpen.
type UnsharpOptions struct {
	// Sigma is the Gaussian blur radius used to build the mask; larger values
	// sharpen coarser detail. Must be > 0.
	Sigma float64
	// Amount scales the high-frequency detail added back (1 = standard). Zero
	// maps to 1.
	Amount float64
	// Threshold, in [0,255], suppresses sharpening where the local difference is
	// small, avoiding noise amplification in flat areas.
	Threshold float64
}

// Unsharp sharpens the image using an unsharp mask: it subtracts a Gaussian-
// blurred copy to isolate detail, then adds a scaled multiple of that detail
// back. This is the high-quality, sigma-based counterpart to the fixed 3x3
// Sharpen. A Sigma <= 0 is a no-op. Alpha is preserved.
func (p *Pipeline) Unsharp(opts UnsharpOptions) *Pipeline {
	if p.err != nil {
		return p
	}
	if opts.Sigma <= 0 {
		return p
	}
	amount := opts.Amount
	if amount == 0 {
		amount = 1
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Blurred copy for the low-frequency reference.
	blurred := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(blurred.Pix, p.img.Pix)
	bp := &Pipeline{img: blurred}
	bp.Blur(opts.Sigma)

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		orow := p.img.PixOffset(b.Min.X, b.Min.Y+y)
		brow := bp.img.PixOffset(0, y)
		drow := dst.PixOffset(0, y)
		for x := 0; x < w; x++ {
			oi := orow + x*4
			bi := brow + x*4
			di := drow + x*4
			for c := 0; c < 3; c++ {
				o := float64(p.img.Pix[oi+c])
				diff := o - float64(bp.img.Pix[bi+c])
				if diff < 0 {
					if -diff < opts.Threshold {
						diff = 0
					}
				} else if diff < opts.Threshold {
					diff = 0
				}
				dst.Pix[di+c] = clampF(o + amount*diff)
			}
			dst.Pix[di+3] = p.img.Pix[oi+3]
		}
	}
	p.img = dst
	return p
}
