package sharp

import (
	"image"
)

// Gravity anchors an overlay or region relative to the base image.
type Gravity int

// Gravity anchors.
const (
	GravityTopLeft Gravity = iota
	GravityTop
	GravityTopRight
	GravityLeft
	GravityCenter
	GravityRight
	GravityBottomLeft
	GravityBottom
	GravityBottomRight
)

// CompositeOptions configures how an overlay image is combined with the base.
type CompositeOptions struct {
	// Left and Top position the overlay's top-left corner in base pixels. They
	// are used only when Gravity is nil (see UseGravity).
	Left int
	Top  int
	// UseGravity, when true, positions the overlay using Gravity instead of
	// Left/Top.
	UseGravity bool
	Gravity    Gravity
	// Opacity scales the overlay's alpha in [0,1]. The zero value means fully
	// opaque (1.0); use a small positive number for near-transparent overlays.
	Opacity float64
	// Blend selects the blend mode. The zero value, BlendOver, is ordinary
	// source-over compositing.
	Blend BlendMode
}

// Composite alpha-blends overlay onto the current image ("source-over"). The
// base dimensions are preserved; parts of the overlay outside the base are
// clipped.
func (p *Pipeline) Composite(overlay image.Image, opts CompositeOptions) *Pipeline {
	if p.err != nil {
		return p
	}
	ov := toRGBA(overlay)
	ob := ov.Bounds()
	ow, oh := ob.Dx(), ob.Dy()
	bb := p.img.Bounds()
	bw, bh := bb.Dx(), bb.Dy()

	opacity := opts.Opacity
	if opacity == 0 {
		opacity = 1
	}
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}

	var left, top int
	if opts.UseGravity {
		left, top = anchor(opts.Gravity, bw, bh, ow, oh)
	} else {
		left, top = opts.Left, opts.Top
	}

	for oy := 0; oy < oh; oy++ {
		dy := top + oy
		if dy < 0 || dy >= bh {
			continue
		}
		for ox := 0; ox < ow; ox++ {
			dx := left + ox
			if dx < 0 || dx >= bw {
				continue
			}
			si := ov.PixOffset(ob.Min.X+ox, ob.Min.Y+oy)
			sa := float64(ov.Pix[si+3]) / 255 * opacity
			if sa <= 0 {
				continue
			}
			di := p.img.PixOffset(bb.Min.X+dx, bb.Min.Y+dy)
			da := float64(p.img.Pix[di+3]) / 255
			outA := sa + da*(1-sa)
			for c := 0; c < 3; c++ {
				sc := float64(ov.Pix[si+c]) / 255
				dc := float64(p.img.Pix[di+c]) / 255
				// Blend the source colour against the backdrop, then perform the
				// full alpha composite (reduces to source-over for BlendOver).
				bc := blendChannel(opts.Blend, dc, sc)
				var v float64
				if outA > 0 {
					v = (sa*(1-da)*sc + sa*da*bc + (1-sa)*da*dc) / outA
				}
				p.img.Pix[di+c] = clampF(v * 255)
			}
			p.img.Pix[di+3] = clampF(outA * 255)
		}
	}
	return p
}

// Flatten composites the image over a solid background colour, removing any
// transparency. The output is fully opaque.
func (p *Pipeline) Flatten(bg RGBA) *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	br := float64(bg.R)
	bgc := float64(bg.G)
	bbl := float64(bg.B)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			a := float64(p.img.Pix[i+3]) / 255
			ia := 1 - a
			p.img.Pix[i] = clampF(float64(p.img.Pix[i])*a + br*ia)
			p.img.Pix[i+1] = clampF(float64(p.img.Pix[i+1])*a + bgc*ia)
			p.img.Pix[i+2] = clampF(float64(p.img.Pix[i+2])*a + bbl*ia)
			p.img.Pix[i+3] = 255
		}
	}
	return p
}

// anchor computes the top-left overlay position for a gravity within a base of
// size bw x bh for an overlay of size ow x oh.
func anchor(g Gravity, bw, bh, ow, oh int) (int, int) {
	var x, y int
	switch g {
	case GravityTopLeft, GravityLeft, GravityBottomLeft:
		x = 0
	case GravityTop, GravityCenter, GravityBottom:
		x = (bw - ow) / 2
	default: // right column
		x = bw - ow
	}
	switch g {
	case GravityTopLeft, GravityTop, GravityTopRight:
		y = 0
	case GravityLeft, GravityCenter, GravityRight:
		y = (bh - oh) / 2
	default: // bottom row
		y = bh - oh
	}
	return x, y
}
