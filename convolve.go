package sharp

import (
	"fmt"
	"image"
	"math"
)

// Kernel is a square convolution kernel. Weights are stored row-major and Size
// must be odd (1, 3, 5, ...). Divisor and Offset are applied after the weighted
// sum: out = sum/Divisor + Offset. A zero Divisor is treated as 1.
type Kernel struct {
	Size    int
	Weights []float64
	Divisor float64
	Offset  float64
}

// NewKernel builds a Kernel from a square slice of weights, inferring the size.
// It returns an error if len(weights) is not a positive odd perfect square.
func NewKernel(weights []float64, divisor, offset float64) (Kernel, error) {
	n := len(weights)
	size := int(math.Sqrt(float64(n)))
	if size*size != n || size%2 == 0 || size < 1 {
		return Kernel{}, fmt.Errorf("sharp: kernel length %d is not an odd square", n)
	}
	return Kernel{Size: size, Weights: weights, Divisor: divisor, Offset: offset}, nil
}

// Convolve applies an arbitrary convolution kernel to the RGB channels. Edges
// are handled by clamping sample coordinates to the image bounds. The alpha
// channel is preserved unchanged.
func (p *Pipeline) Convolve(k Kernel) *Pipeline {
	if p.err != nil {
		return p
	}
	if k.Size < 1 || k.Size%2 == 0 || len(k.Weights) != k.Size*k.Size {
		p.err = fmt.Errorf("sharp: invalid kernel (size=%d, weights=%d)", k.Size, len(k.Weights))
		return p
	}
	div := k.Divisor
	if div == 0 {
		div = 1
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	radius := k.Size / 2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sr, sg, sb float64
			ki := 0
			for ky := -radius; ky <= radius; ky++ {
				sy := clampInt(y+ky, 0, h-1)
				for kx := -radius; kx <= radius; kx++ {
					sx := clampInt(x+kx, 0, w-1)
					wt := k.Weights[ki]
					ki++
					if wt == 0 {
						continue
					}
					si := p.img.PixOffset(b.Min.X+sx, b.Min.Y+sy)
					sr += float64(p.img.Pix[si]) * wt
					sg += float64(p.img.Pix[si+1]) * wt
					sb += float64(p.img.Pix[si+2]) * wt
				}
			}
			di := dst.PixOffset(x, y)
			ai := p.img.PixOffset(b.Min.X+x, b.Min.Y+y)
			dst.Pix[di] = clampF(sr/div + k.Offset)
			dst.Pix[di+1] = clampF(sg/div + k.Offset)
			dst.Pix[di+2] = clampF(sb/div + k.Offset)
			dst.Pix[di+3] = p.img.Pix[ai+3]
		}
	}
	p.img = dst
	return p
}

// Sharpen applies a standard 3x3 sharpening kernel. Amount scales the
// sharpening strength; a value <= 0 leaves the image unchanged and 1 is the
// canonical strength.
func (p *Pipeline) Sharpen(amount float64) *Pipeline {
	if p.err != nil {
		return p
	}
	if amount <= 0 {
		return p
	}
	c := 1 + 4*amount
	e := -amount
	k := Kernel{
		Size: 3,
		Weights: []float64{
			0, e, 0,
			e, c, e,
			0, e, 0,
		},
		Divisor: 1,
	}
	return p.Convolve(k)
}

// Blur applies a Gaussian blur with the given standard deviation (sigma) in
// pixels. A sigma <= 0 leaves the image unchanged. The blur is separable and
// implemented as two 1-D passes for efficiency; alpha is blurred alongside RGB.
func (p *Pipeline) Blur(sigma float64) *Pipeline {
	if p.err != nil {
		return p
	}
	if sigma <= 0 {
		return p
	}
	kernel := gaussianKernel1D(sigma)
	radius := len(kernel) / 2
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Horizontal pass into tmp.
	tmp := image.NewRGBA(image.Rect(0, 0, w, h))
	blurPass(p.img, tmp, kernel, radius, true)
	// Vertical pass into dst.
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	blurPass(tmp, dst, kernel, radius, false)
	p.img = dst
	return p
}

// blurPass runs a single separable 1-D convolution. When horizontal is true the
// kernel is applied along x, otherwise along y. All four channels are blurred.
func blurPass(src, dst *image.RGBA, kernel []float64, radius int, horizontal bool) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var acc [4]float64
			for t := -radius; t <= radius; t++ {
				var sx, sy int
				if horizontal {
					sx = clampInt(x+t, 0, w-1)
					sy = y
				} else {
					sx = x
					sy = clampInt(y+t, 0, h-1)
				}
				wt := kernel[t+radius]
				si := src.PixOffset(b.Min.X+sx, b.Min.Y+sy)
				acc[0] += float64(src.Pix[si]) * wt
				acc[1] += float64(src.Pix[si+1]) * wt
				acc[2] += float64(src.Pix[si+2]) * wt
				acc[3] += float64(src.Pix[si+3]) * wt
			}
			di := dst.PixOffset(x, y)
			dst.Pix[di] = clampF(acc[0])
			dst.Pix[di+1] = clampF(acc[1])
			dst.Pix[di+2] = clampF(acc[2])
			dst.Pix[di+3] = clampF(acc[3])
		}
	}
}

// gaussianKernel1D returns a normalised 1-D Gaussian kernel for the given
// sigma. The radius is ceil(3*sigma).
func gaussianKernel1D(sigma float64) []float64 {
	radius := int(math.Ceil(3 * sigma))
	if radius < 1 {
		radius = 1
	}
	size := 2*radius + 1
	kernel := make([]float64, size)
	var sum float64
	twoSigma2 := 2 * sigma * sigma
	for i := -radius; i <= radius; i++ {
		v := math.Exp(-float64(i*i) / twoSigma2)
		kernel[i+radius] = v
		sum += v
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	return kernel
}
