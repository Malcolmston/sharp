package sharp

import (
	"fmt"
	"image"
	"math"
)

// AffineOptions configures an Affine transform.
type AffineOptions struct {
	// Background fills output pixels whose inverse-mapped source coordinate
	// falls outside the source image.
	Background RGBA
}

// Affine applies an arbitrary 2x3 affine transform to the image, sampling the
// source bilinearly. The matrix m = [a, b, c, d, e, f] maps a source point
// (x, y) to the destination point
//
//	X = a*x + b*y + c
//	Y = d*x + e*y + f
//
// The output canvas is sized to the axis-aligned bounding box of the
// transformed source and translated so its top-left corner is the origin.
// Exposed areas are filled with opts.Background. The matrix must be invertible
// (non-zero determinant of its 2x2 linear part).
func (p *Pipeline) Affine(m [6]float64, opts AffineOptions) *Pipeline {
	if p.err != nil {
		return p
	}
	a, b, c, d, e, f := m[0], m[1], m[2], m[3], m[4], m[5]
	det := a*e - b*d
	if det == 0 {
		p.err = fmt.Errorf("sharp: Affine matrix is singular (det=0)")
		return p
	}
	bnds := p.img.Bounds()
	sw, sh := bnds.Dx(), bnds.Dy()

	// Forward-map the four corners to find the output bounding box.
	fwd := func(x, y float64) (float64, float64) { return a*x + b*y + c, d*x + e*y + f }
	xs := make([]float64, 0, 4)
	ys := make([]float64, 0, 4)
	for _, pt := range [][2]float64{{0, 0}, {float64(sw), 0}, {0, float64(sh)}, {float64(sw), float64(sh)}} {
		x, y := fwd(pt[0], pt[1])
		xs = append(xs, x)
		ys = append(ys, y)
	}
	minX, maxX := xs[0], xs[0]
	minY, maxY := ys[0], ys[0]
	for i := 1; i < 4; i++ {
		minX, maxX = math.Min(minX, xs[i]), math.Max(maxX, xs[i])
		minY, maxY = math.Min(minY, ys[i]), math.Max(maxY, ys[i])
	}
	ow := int(math.Ceil(maxX - minX))
	oh := int(math.Ceil(maxY - minY))
	if ow < 1 {
		ow = 1
	}
	if oh < 1 {
		oh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, ow, oh))
	fillRGBA(dst, opts.Background)

	for oy := 0; oy < oh; oy++ {
		for ox := 0; ox < ow; ox++ {
			// Destination pixel centre in transformed space.
			bigX := float64(ox) + 0.5 + minX - c
			bigY := float64(oy) + 0.5 + minY - f
			sx := (e*bigX - b*bigY) / det
			sy := (-d*bigX + a*bigY) / det
			// sx, sy address pixel centres; shift by -0.5 for sampling.
			fx := sx - 0.5
			fy := sy - 0.5
			if fx < -0.5 || fx > float64(sw)-0.5 || fy < -0.5 || fy > float64(sh)-0.5 {
				continue
			}
			di := dst.PixOffset(ox, oy)
			bilinearSample(p.img, fx, fy, dst.Pix[di:di+4])
		}
	}
	p.img = dst
	return p
}

// Trim auto-crops uniform borders. The reference border colour is taken from
// the top-left pixel; any pixel whose per-channel difference from it exceeds
// threshold (0-255, compared on the maximum absolute channel delta including
// alpha) is treated as content. The image is cropped to the bounding box of the
// content. A threshold < 0 is treated as 0. If the whole image matches the
// border colour the image is left unchanged.
func (p *Pipeline) Trim(threshold float64) *Pipeline {
	if p.err != nil {
		return p
	}
	if threshold < 0 {
		threshold = 0
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return p
	}
	ref := p.img.Pix[p.img.PixOffset(b.Min.X, b.Min.Y) : p.img.PixOffset(b.Min.X, b.Min.Y)+4]
	r0, g0, b0, a0 := ref[0], ref[1], ref[2], ref[3]

	diff := func(x, y int) bool {
		i := p.img.PixOffset(b.Min.X+x, b.Min.Y+y)
		dr := absDiff(p.img.Pix[i], r0)
		dg := absDiff(p.img.Pix[i+1], g0)
		db := absDiff(p.img.Pix[i+2], b0)
		da := absDiff(p.img.Pix[i+3], a0)
		m := dr
		if dg > m {
			m = dg
		}
		if db > m {
			m = db
		}
		if da > m {
			m = da
		}
		return float64(m) > threshold
	}

	minX, minY := w, h
	maxX, maxY := -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if diff(x, y) {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < 0 {
		// Entirely uniform: nothing to trim.
		return p
	}
	return p.Extract(Rectangle{X: minX, Y: minY, Width: maxX - minX + 1, Height: maxY - minY + 1})
}

// absDiff returns the absolute difference of two bytes.
func absDiff(a, b uint8) int {
	if a > b {
		return int(a) - int(b)
	}
	return int(b) - int(a)
}
