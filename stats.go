package sharp

import "math"

// ChannelStat holds summary statistics for a single colour band across all
// pixels of an image. Min and Max are the extreme sample values (0-255); Mean
// and StdDev are the arithmetic mean and population standard deviation of the
// samples; Sum is the total of all samples in the band.
type ChannelStat struct {
	Min    uint8
	Max    uint8
	Mean   float64
	StdDev float64
	Sum    float64
}

// ChannelStats computes per-band statistics for the current image and returns a
// slice of four ChannelStat values in red, green, blue, alpha order. It mirrors
// the richer per-channel information reported by the Node sharp library's
// stats() call. An empty image yields zero-valued statistics.
func (p *Pipeline) ChannelStats() ([]ChannelStat, error) {
	if p.err != nil {
		return nil, p.err
	}
	b := p.img.Bounds()
	n := b.Dx() * b.Dy()
	out := make([]ChannelStat, 4)
	if n == 0 {
		return out, nil
	}
	var sum, sumSq [4]float64
	min := [4]uint8{255, 255, 255, 255}
	max := [4]uint8{0, 0, 0, 0}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			for c := 0; c < 4; c++ {
				v := p.img.Pix[i+c]
				fv := float64(v)
				sum[c] += fv
				sumSq[c] += fv * fv
				if v < min[c] {
					min[c] = v
				}
				if v > max[c] {
					max[c] = v
				}
			}
		}
	}
	fn := float64(n)
	for c := 0; c < 4; c++ {
		mean := sum[c] / fn
		variance := sumSq[c]/fn - mean*mean
		if variance < 0 {
			variance = 0
		}
		out[c] = ChannelStat{
			Min:    min[c],
			Max:    max[c],
			Mean:   mean,
			StdDev: math.Sqrt(variance),
			Sum:    sum[c],
		}
	}
	return out, nil
}

// Histogram holds 256-bin intensity histograms for an image. R, G and B count
// the occurrences of each 8-bit value in the respective colour band, and
// Luminance counts the rounded Rec. 601 luma of every pixel.
type Histogram struct {
	R         [256]uint64
	G         [256]uint64
	B         [256]uint64
	Luminance [256]uint64
}

// Histogram computes per-channel and luminance histograms for the current
// image. The counts across each band sum to the pixel count.
func (p *Pipeline) Histogram() (Histogram, error) {
	var h Histogram
	if p.err != nil {
		return h, p.err
	}
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			r, g, bl := p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2]
			h.R[r]++
			h.G[g]++
			h.B[bl]++
			h.Luminance[uint8(luma(r, g, bl)+0.5)]++
		}
	}
	return h, nil
}

// Entropy returns the Shannon entropy (in bits) of the image's luminance
// histogram. It ranges from 0 for a single-valued image up to 8 for a perfectly
// uniform distribution of all 256 luma levels, and is a useful measure of how
// much detail an image carries.
func (p *Pipeline) Entropy() (float64, error) {
	if p.err != nil {
		return 0, p.err
	}
	h, err := p.Histogram()
	if err != nil {
		return 0, err
	}
	var total uint64
	for _, c := range h.Luminance {
		total += c
	}
	if total == 0 {
		return 0, nil
	}
	ft := float64(total)
	var e float64
	for _, c := range h.Luminance {
		if c == 0 {
			continue
		}
		pr := float64(c) / ft
		e -= pr * math.Log2(pr)
	}
	return e, nil
}

// Sharpness returns a scalar estimate of image sharpness computed as the mean
// absolute discrete Laplacian of luminance over the interior pixels. Flat
// images score 0; higher values indicate stronger local contrast (edges and
// fine detail). Images smaller than 3x3 return 0.
func (p *Pipeline) Sharpness() (float64, error) {
	if p.err != nil {
		return 0, p.err
	}
	b := p.img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 3 || h < 3 {
		return 0, nil
	}
	lum := make([]float64, w*h)
	for y := 0; y < h; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < w; x++ {
			i := row + x*4
			lum[y*w+x] = luma(p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2])
		}
	}
	var sum float64
	count := 0
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := lum[y*w+x]
			lap := 4*c - lum[y*w+x-1] - lum[y*w+x+1] - lum[(y-1)*w+x] - lum[(y+1)*w+x]
			sum += math.Abs(lap)
			count++
		}
	}
	if count == 0 {
		return 0, nil
	}
	return sum / float64(count), nil
}

// IsOpaque reports whether every pixel in the image is fully opaque (alpha 255).
// It corresponds to the isOpaque flag exposed by the Node sharp library.
func (p *Pipeline) IsOpaque() (bool, error) {
	if p.err != nil {
		return false, p.err
	}
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			if p.img.Pix[row+x*4+3] != 255 {
				return false, nil
			}
		}
	}
	return true, nil
}

// DominantColor returns the most prominent opaque colour in the image. Pixels
// are grouped into a coarse 16x16x16 RGB histogram, the most populated bin is
// selected, and the mean colour of the pixels falling in that bin is returned
// with full opacity. Fully transparent pixels are ignored. It mirrors the
// dominant colour reported by the Node sharp library's stats() call.
func (p *Pipeline) DominantColor() (RGBA, error) {
	if p.err != nil {
		return RGBA{}, p.err
	}
	const bins = 16
	const shift = 4 // 256 / 16
	var count [bins * bins * bins]uint64
	var sumR, sumG, sumB [bins * bins * bins]uint64
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			if p.img.Pix[i+3] == 0 {
				continue
			}
			r, g, bl := p.img.Pix[i], p.img.Pix[i+1], p.img.Pix[i+2]
			idx := (int(r)>>shift)*bins*bins + (int(g)>>shift)*bins + (int(bl) >> shift)
			count[idx]++
			sumR[idx] += uint64(r)
			sumG[idx] += uint64(g)
			sumB[idx] += uint64(bl)
		}
	}
	best := -1
	var bestCount uint64
	for i := range count {
		if count[i] > bestCount {
			bestCount = count[i]
			best = i
		}
	}
	if best < 0 {
		return RGBA{}, nil
	}
	c := count[best]
	return RGBA{
		R: uint8((sumR[best] + c/2) / c),
		G: uint8((sumG[best] + c/2) / c),
		B: uint8((sumB[best] + c/2) / c),
		A: 255,
	}, nil
}

// DominantColour is a British-spelling alias for DominantColor.
func (p *Pipeline) DominantColour() (RGBA, error) { return p.DominantColor() }

// MeanColor returns the arithmetic mean colour of the image across every pixel,
// including the alpha channel, rounded to 8-bit components.
func (p *Pipeline) MeanColor() (RGBA, error) {
	if p.err != nil {
		return RGBA{}, p.err
	}
	st, err := p.Stats()
	if err != nil {
		return RGBA{}, err
	}
	return RGBA{
		R: clampF(st.MeanR),
		G: clampF(st.MeanG),
		B: clampF(st.MeanB),
		A: clampF(st.MeanA),
	}, nil
}

// MeanColour is a British-spelling alias for MeanColor.
func (p *Pipeline) MeanColour() (RGBA, error) { return p.MeanColor() }
