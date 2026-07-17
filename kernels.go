package sharp

import (
	"image"
	"math"
)

// Additional high-quality, separable resampling kernels selectable through
// ResizeOptions.Interpolation. They continue the Interpolation enumeration
// started in resize.go (Nearest = 0, Bilinear = 1).
const (
	// Cubic is a Catmull-Rom bicubic filter (support 2): sharper than Bilinear
	// with mild overshoot, a good general-purpose photographic default.
	Cubic Interpolation = iota + 2
	// Mitchell is the Mitchell-Netravali filter (B = C = 1/3, support 2): a
	// balanced trade-off between blurring and ringing.
	Mitchell
	// Lanczos3 is a windowed-sinc filter (support 3): the highest quality of the
	// built-in kernels, with the most pronounced edge sharpening.
	Lanczos3
)

// kernelFor returns the weighting function and its support radius for a
// high-quality interpolation mode. The boolean reports whether interp names a
// separable kernel handled by resampleSeparable.
func kernelFor(interp Interpolation) (fn func(float64) float64, support float64, ok bool) {
	switch interp {
	case Cubic:
		return cubicKernel, 2, true
	case Mitchell:
		return mitchellKernel, 2, true
	case Lanczos3:
		return lanczos3Kernel, 3, true
	default:
		return nil, 0, false
	}
}

// cubicKernel is the Catmull-Rom cubic (a = -0.5).
func cubicKernel(x float64) float64 {
	const a = -0.5
	if x < 0 {
		x = -x
	}
	switch {
	case x < 1:
		return (a+2)*x*x*x - (a+3)*x*x + 1
	case x < 2:
		return a*x*x*x - 5*a*x*x + 8*a*x - 4*a
	default:
		return 0
	}
}

// mitchellKernel is the Mitchell-Netravali cubic with B = C = 1/3.
func mitchellKernel(x float64) float64 {
	const b, c = 1.0 / 3.0, 1.0 / 3.0
	if x < 0 {
		x = -x
	}
	x2 := x * x
	x3 := x2 * x
	switch {
	case x < 1:
		return ((12-9*b-6*c)*x3 + (-18+12*b+6*c)*x2 + (6 - 2*b)) / 6
	case x < 2:
		return ((-b-6*c)*x3 + (6*b+30*c)*x2 + (-12*b-48*c)*x + (8*b + 24*c)) / 6
	default:
		return 0
	}
}

// lanczos3Kernel is a Lanczos windowed-sinc with a = 3.
func lanczos3Kernel(x float64) float64 {
	const a = 3.0
	if x < 0 {
		x = -x
	}
	if x == 0 {
		return 1
	}
	if x >= a {
		return 0
	}
	px := math.Pi * x
	return a * math.Sin(px) * math.Sin(px/a) / (px * px)
}

// contribution is a single source sample and its weight for one output pixel.
type contribution struct {
	index  int
	weight float64
}

// computeContributions builds, for every output position, the list of source
// samples and normalised weights produced by filter. When downsampling
// (dstSize < srcSize) the filter is widened to average source pixels, giving
// proper anti-aliasing. Source indices are clamped to the edges.
func computeContributions(srcSize, dstSize int, filter func(float64) float64, support float64) [][]contribution {
	scale := float64(dstSize) / float64(srcSize)
	fscale := 1.0
	if scale < 1 {
		fscale = 1 / scale
	}
	fsupport := support * fscale
	res := make([][]contribution, dstSize)
	for x := 0; x < dstSize; x++ {
		center := (float64(x)+0.5)/scale - 0.5
		left := int(math.Ceil(center - fsupport))
		right := int(math.Floor(center + fsupport))
		var contribs []contribution
		var sum float64
		for i := left; i <= right; i++ {
			w := filter((float64(i) - center) / fscale)
			if w == 0 {
				continue
			}
			contribs = append(contribs, contribution{index: clampInt(i, 0, srcSize-1), weight: w})
			sum += w
		}
		if sum != 0 {
			for j := range contribs {
				contribs[j].weight /= sum
			}
		}
		res[x] = contribs
	}
	return res
}

// resampleSeparable resizes src into dst using the separable kernel named by
// interp (Cubic, Mitchell or Lanczos3). The horizontal pass is accumulated in
// floating point so the vertical pass sees unclamped intermediates.
func resampleSeparable(src, dst *image.RGBA, interp Interpolation) {
	filter, support, ok := kernelFor(interp)
	if !ok {
		resizeBilinear(src, dst)
		return
	}
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	tw, th := dst.Bounds().Dx(), dst.Bounds().Dy()

	xc := computeContributions(sw, tw, filter, support)
	yc := computeContributions(sh, th, filter, support)

	// Horizontal pass: src (sw x sh) -> tmp (tw x sh), kept as float64.
	tmp := make([]float64, tw*sh*4)
	for y := 0; y < sh; y++ {
		srow := src.PixOffset(sb.Min.X, sb.Min.Y+y)
		for x := 0; x < tw; x++ {
			var acc [4]float64
			for _, c := range xc[x] {
				si := srow + c.index*4
				acc[0] += float64(src.Pix[si]) * c.weight
				acc[1] += float64(src.Pix[si+1]) * c.weight
				acc[2] += float64(src.Pix[si+2]) * c.weight
				acc[3] += float64(src.Pix[si+3]) * c.weight
			}
			ti := (y*tw + x) * 4
			tmp[ti], tmp[ti+1], tmp[ti+2], tmp[ti+3] = acc[0], acc[1], acc[2], acc[3]
		}
	}

	// Vertical pass: tmp (tw x sh) -> dst (tw x th).
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			var acc [4]float64
			for _, c := range yc[y] {
				ti := (c.index*tw + x) * 4
				acc[0] += tmp[ti] * c.weight
				acc[1] += tmp[ti+1] * c.weight
				acc[2] += tmp[ti+2] * c.weight
				acc[3] += tmp[ti+3] * c.weight
			}
			di := dst.PixOffset(x, y)
			dst.Pix[di] = clampF(acc[0])
			dst.Pix[di+1] = clampF(acc[1])
			dst.Pix[di+2] = clampF(acc[2])
			dst.Pix[di+3] = clampF(acc[3])
		}
	}
}
