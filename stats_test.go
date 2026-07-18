package sharp

import (
	"image"
	"image/color"
	"testing"
)

func TestChannelStatsSolid(t *testing.T) {
	p := New(solid(4, 5, RGBA{10, 20, 30, 255}))
	stats, err := p.ChannelStats()
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 4 {
		t.Fatalf("got %d channels, want 4", len(stats))
	}
	want := []uint8{10, 20, 30, 255}
	for i, s := range stats {
		if s.Min != want[i] || s.Max != want[i] {
			t.Errorf("channel %d min/max = %d/%d, want %d", i, s.Min, s.Max, want[i])
		}
		if !approx(s.Mean, float64(want[i]), 1e-9) {
			t.Errorf("channel %d mean = %g, want %d", i, s.Mean, want[i])
		}
		if !approx(s.StdDev, 0, 1e-9) {
			t.Errorf("channel %d stddev = %g, want 0", i, s.StdDev)
		}
		if !approx(s.Sum, float64(want[i])*20, 1e-9) {
			t.Errorf("channel %d sum = %g", i, s.Sum)
		}
	}
}

func TestChannelStatsStdDev(t *testing.T) {
	// Two-pixel red channel: 0 and 200 -> mean 100, population stddev 100.
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{0, 0, 0, 255})
	img.Set(1, 0, color.RGBA{200, 0, 0, 255})
	stats, err := New(img).ChannelStats()
	if err != nil {
		t.Fatal(err)
	}
	if !approx(stats[0].Mean, 100, 1e-9) || !approx(stats[0].StdDev, 100, 1e-9) {
		t.Errorf("R mean/stddev = %g/%g, want 100/100", stats[0].Mean, stats[0].StdDev)
	}
}

func TestHistogramCounts(t *testing.T) {
	h, err := New(solid(3, 3, RGBA{7, 7, 7, 255})).Histogram()
	if err != nil {
		t.Fatal(err)
	}
	if h.R[7] != 9 || h.G[7] != 9 || h.B[7] != 9 || h.Luminance[7] != 9 {
		t.Errorf("histogram bins wrong: %d %d %d %d", h.R[7], h.G[7], h.B[7], h.Luminance[7])
	}
	var total uint64
	for _, c := range h.R {
		total += c
	}
	if total != 9 {
		t.Errorf("total = %d, want 9", total)
	}
}

func TestEntropy(t *testing.T) {
	// Solid image -> zero entropy.
	e, err := New(solid(4, 4, RGBA{100, 100, 100, 255})).Entropy()
	if err != nil {
		t.Fatal(err)
	}
	if !approx(e, 0, 1e-9) {
		t.Errorf("solid entropy = %g, want 0", e)
	}
	// Half black, half white (distinct luma) -> exactly 1 bit.
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{0, 0, 0, 255})
	img.Set(1, 0, color.RGBA{255, 255, 255, 255})
	e, err = New(img).Entropy()
	if err != nil {
		t.Fatal(err)
	}
	if !approx(e, 1, 1e-9) {
		t.Errorf("two-level entropy = %g, want 1", e)
	}
}

func TestSharpness(t *testing.T) {
	// Flat image has zero sharpness.
	s, err := New(solid(5, 5, RGBA{50, 50, 50, 255})).Sharpness()
	if err != nil {
		t.Fatal(err)
	}
	if !approx(s, 0, 1e-9) {
		t.Errorf("flat sharpness = %g, want 0", s)
	}
	// 3x3 with a bright centre: only interior pixel is the centre.
	// Laplacian = 4*255 - 0*4 = 1020.
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	img.Set(1, 1, color.RGBA{255, 255, 255, 255})
	s, err = New(img).Sharpness()
	if err != nil {
		t.Fatal(err)
	}
	if !approx(s, 1020, 0.5) {
		t.Errorf("centre sharpness = %g, want 1020", s)
	}
}

func TestIsOpaque(t *testing.T) {
	if ok, _ := New(solid(2, 2, RGBA{0, 0, 0, 255})).IsOpaque(); !ok {
		t.Error("opaque image reported non-opaque")
	}
	if ok, _ := New(solid(2, 2, RGBA{0, 0, 0, 128})).IsOpaque(); ok {
		t.Error("translucent image reported opaque")
	}
}

func TestDominantColor(t *testing.T) {
	// Mostly one colour with a couple of outliers.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{200, 30, 40, 255})
		}
	}
	img.Set(0, 0, color.RGBA{0, 200, 0, 255})
	c, err := New(img).DominantColor()
	if err != nil {
		t.Fatal(err)
	}
	if abs8(c.R, 200) > 8 || abs8(c.G, 30) > 8 || abs8(c.B, 40) > 8 || c.A != 255 {
		t.Errorf("dominant = %+v, want ~{200,30,40,255}", c)
	}
}

func TestDominantColorIgnoresTransparent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	// Three transparent green pixels, one opaque red.
	img.Set(0, 0, color.RGBA{0, 255, 0, 0})
	img.Set(1, 0, color.RGBA{0, 255, 0, 0})
	img.Set(0, 1, color.RGBA{0, 255, 0, 0})
	img.Set(1, 1, color.RGBA{240, 10, 10, 255})
	c, _ := New(img).DominantColor()
	if abs8(c.R, 240) > 8 {
		t.Errorf("transparent pixels not ignored: %+v", c)
	}
}

func TestMeanColor(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{0, 100, 200, 255})
	img.Set(1, 0, color.RGBA{100, 200, 0, 255})
	c, err := New(img).MeanColour()
	if err != nil {
		t.Fatal(err)
	}
	if c.R != 50 || c.G != 150 || c.B != 100 || c.A != 255 {
		t.Errorf("mean = %+v, want {50,150,100,255}", c)
	}
}

func BenchmarkChannelStats(b *testing.B) {
	p := New(gradient(256, 256))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.ChannelStats(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEntropy(b *testing.B) {
	p := New(gradient(256, 256))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Entropy(); err != nil {
			b.Fatal(err)
		}
	}
}
