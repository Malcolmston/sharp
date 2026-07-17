package sharp

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// gradient builds a w x h horizontal gradient (R and G ramp with x, B fixed).
func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(x * 255 / maxInt(1, w-1))
			img.Set(x, y, color.RGBA{R: v, G: v, B: 100, A: 255})
		}
	}
	return img
}

func TestHighQualityResizeKernels(t *testing.T) {
	src := gradient(16, 16)
	for _, interp := range []Interpolation{Cubic, Mitchell, Lanczos3} {
		up := mustImage(t, New(src).Resize(ResizeOptions{Width: 32, Height: 32, Interpolation: interp}))
		if up.Bounds().Dx() != 32 || up.Bounds().Dy() != 32 {
			t.Fatalf("interp %d upscale dims wrong: %v", interp, up.Bounds())
		}
		down := mustImage(t, New(src).Resize(ResizeOptions{Width: 8, Height: 8, Interpolation: interp}))
		if down.Bounds().Dx() != 8 || down.Bounds().Dy() != 8 {
			t.Fatalf("interp %d downscale dims wrong: %v", interp, down.Bounds())
		}
		// A midpoint of a smooth ramp should stay within the source's value range.
		mid := at(t, up, 16, 16)
		if mid.B < 90 || mid.B > 110 {
			t.Fatalf("interp %d unexpected blue: %d", interp, mid.B)
		}
	}
	// Same-size resize with a kernel is an exact copy.
	same := mustImage(t, New(src).Resize(ResizeOptions{Width: 16, Height: 16, Interpolation: Lanczos3}))
	if at(t, same, 5, 5) != at(t, src, 5, 5) {
		t.Fatalf("same-size lanczos changed pixel")
	}
}

func TestKernelWeights(t *testing.T) {
	if cubicKernel(0) != 1 {
		t.Fatalf("cubic(0)=%v", cubicKernel(0))
	}
	if cubicKernel(3) != 0 || mitchellKernel(3) != 0 || lanczos3Kernel(3) != 0 {
		t.Fatal("kernels should be zero outside support")
	}
	if lanczos3Kernel(0) != 1 {
		t.Fatal("lanczos(0) should be 1")
	}
	if v := mitchellKernel(0); v < 0.7 || v > 0.9 {
		t.Fatalf("mitchell(0)=%v out of expected range", v)
	}
	if _, _, ok := kernelFor(Bilinear); ok {
		t.Fatal("Bilinear should not be a separable kernel")
	}
}

func TestAffineIdentityAndTranslate(t *testing.T) {
	src := checker()
	// Identity matrix reproduces the image.
	id := mustImage(t, New(src).Affine([6]float64{1, 0, 0, 0, 1, 0}, AffineOptions{}))
	if id.Bounds().Dx() != 2 || id.Bounds().Dy() != 2 {
		t.Fatalf("affine identity dims wrong: %v", id.Bounds())
	}
	if at(t, id, 0, 0) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("affine identity TL wrong: %+v", at(t, id, 0, 0))
	}
	// 2x scale enlarges the canvas.
	scaled := mustImage(t, New(src).Affine([6]float64{2, 0, 0, 0, 2, 0}, AffineOptions{}))
	if scaled.Bounds().Dx() != 4 || scaled.Bounds().Dy() != 4 {
		t.Fatalf("affine scale dims wrong: %v", scaled.Bounds())
	}
	// Singular matrix errors.
	if New(src).Affine([6]float64{0, 0, 0, 0, 0, 0}, AffineOptions{}).Err() == nil {
		t.Fatal("expected singular matrix error")
	}
}

func TestTrim(t *testing.T) {
	// 8x8 white border with a 2x2 red block at (3,3).
	img := solid(8, 8, White)
	for y := 3; y < 5; y++ {
		for x := 3; x < 5; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	got := mustImage(t, New(img).Trim(10))
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 2 {
		t.Fatalf("trim dims wrong: %v", got.Bounds())
	}
	if at(t, got, 0, 0) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("trim content wrong: %+v", at(t, got, 0, 0))
	}
	// Uniform image is unchanged.
	uni := mustImage(t, New(solid(4, 4, White)).Trim(10))
	if uni.Bounds().Dx() != 4 {
		t.Fatal("uniform trim should be no-op")
	}
	// Negative threshold treated as zero, still trims.
	if New(img).Trim(-5).Err() != nil {
		t.Fatal("negative threshold should not error")
	}
}

func TestNormaliseLinear(t *testing.T) {
	// Low-contrast image: values 100..140.
	img := image.NewRGBA(image.Rect(0, 0, 41, 1))
	for x := 0; x <= 40; x++ {
		v := uint8(100 + x)
		img.Set(x, 0, color.RGBA{v, v, v, 255})
	}
	n := mustImage(t, New(img).Normalise(NormaliseOptions{}))
	// After stretching, extremes should approach 0 and 255.
	if at(t, n, 0, 0).R > 5 {
		t.Fatalf("normalise low end not stretched: %d", at(t, n, 0, 0).R)
	}
	if at(t, n, 40, 0).R < 250 {
		t.Fatalf("normalise high end not stretched: %d", at(t, n, 40, 0).R)
	}
	// Lower>=Upper is a no-op.
	same := mustImage(t, New(img).Normalise(NormaliseOptions{Lower: 50, Upper: 50}))
	if at(t, same, 0, 0).R != 100 {
		t.Fatal("degenerate normalise should be no-op")
	}
	// Linear: out = 2*x + 10.
	lin := mustImage(t, New(solid(1, 1, RGBA{10, 20, 30, 255})).Linear([]float64{2}, []float64{10}))
	if got := at(t, lin, 0, 0); got.R != 30 || got.G != 50 || got.B != 70 {
		t.Fatalf("linear scalar wrong: %+v", got)
	}
	// Per-channel slopes.
	lin3 := mustImage(t, New(solid(1, 1, RGBA{10, 10, 10, 255})).Linear([]float64{1, 2, 3}, nil))
	if got := at(t, lin3, 0, 0); got.R != 10 || got.G != 20 || got.B != 30 {
		t.Fatalf("linear per-channel wrong: %+v", got)
	}
	// Bad length errors.
	if New(solid(1, 1, White)).Linear([]float64{1, 2}, nil).Err() == nil {
		t.Fatal("expected linear length error")
	}
}

func TestModulate(t *testing.T) {
	// Saturation 0 desaturates a red pixel toward gray.
	desat := mustImage(t, New(solid(1, 1, RGBA{200, 50, 50, 255})).Modulate(ModulateOptions{Saturation: 0.001}))
	g := at(t, desat, 0, 0)
	if absDiff(g.R, g.G) > 10 || absDiff(g.G, g.B) > 10 {
		t.Fatalf("modulate desaturate failed: %+v", g)
	}
	// Brightness > 1 lightens.
	br := mustImage(t, New(solid(1, 1, RGBA{100, 100, 100, 255})).Modulate(ModulateOptions{Brightness: 1.5}))
	if at(t, br, 0, 0).R <= 100 {
		t.Fatalf("modulate brightness failed: %+v", at(t, br, 0, 0))
	}
	// Hue rotation of 120 degrees maps red toward green.
	hue := mustImage(t, New(solid(1, 1, RGBA{255, 0, 0, 255})).Modulate(ModulateOptions{Hue: 120}))
	h := at(t, hue, 0, 0)
	if h.G < h.R || h.G < h.B {
		t.Fatalf("hue rotation failed: %+v", h)
	}
	// Zero-value options are a near no-op.
	same := mustImage(t, New(solid(1, 1, RGBA{123, 80, 40, 255})).Modulate(ModulateOptions{}))
	if absDiff(at(t, same, 0, 0).R, 123) > 2 {
		t.Fatalf("zero modulate changed image: %+v", at(t, same, 0, 0))
	}
}

func TestCLAHE(t *testing.T) {
	src := gradient(32, 32)
	got := mustImage(t, New(src).CLAHE(CLAHEOptions{Width: 8, Height: 8, MaxSlope: 3}))
	if got.Bounds() != src.Bounds() {
		t.Fatalf("clahe changed dims: %v", got.Bounds())
	}
	// Defaults path.
	def := mustImage(t, New(src).CLAHE(CLAHEOptions{}))
	if def.Bounds() != src.Bounds() {
		t.Fatal("clahe default dims wrong")
	}
	// A flat image stays flat (ratio 1 where luma > 0).
	flat := mustImage(t, New(solid(16, 16, RGBA{120, 120, 120, 255})).CLAHE(CLAHEOptions{Width: 4, Height: 4}))
	if v := at(t, flat, 8, 8); absDiff(v.R, v.G) > 2 {
		t.Fatalf("clahe distorted flat image: %+v", v)
	}
}

func TestMedianMorphology(t *testing.T) {
	// Salt noise on gray; median removes an isolated white pixel.
	img := solid(5, 5, RGBA{100, 100, 100, 255})
	img.Set(2, 2, color.RGBA{255, 255, 255, 255})
	med := mustImage(t, New(img).Median(1))
	if at(t, med, 2, 2).R != 100 {
		t.Fatalf("median did not remove speck: %+v", at(t, med, 2, 2))
	}
	if New(img).Median(0).Err() != nil {
		t.Fatal("median radius 0 should be no-op")
	}

	// Erode shrinks a bright block; dilate grows it.
	block := solid(7, 7, Black)
	for y := 2; y < 5; y++ {
		for x := 2; x < 5; x++ {
			block.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	er := mustImage(t, New(block).Erode(1))
	if at(t, er, 2, 2).R != 0 {
		t.Fatalf("erode did not shrink edge: %+v", at(t, er, 2, 2))
	}
	if at(t, er, 3, 3).R != 255 {
		t.Fatalf("erode removed interior: %+v", at(t, er, 3, 3))
	}
	di := mustImage(t, New(block).Dilate(1))
	if at(t, di, 1, 1).R != 255 {
		t.Fatalf("dilate did not grow edge: %+v", at(t, di, 1, 1))
	}
	// Open and Close run without error and keep dims.
	op := mustImage(t, New(block).Morphology(MorphOpen, 1))
	cl := mustImage(t, New(block).Morphology(MorphClose, 1))
	if op.Bounds() != block.Bounds() || cl.Bounds() != block.Bounds() {
		t.Fatal("open/close changed dims")
	}
	if New(block).Morphology(MorphOp(99), 1).Err() == nil {
		t.Fatal("expected bad morph op error")
	}
}

func TestUnsharp(t *testing.T) {
	// Edge image; unsharp should overshoot near the boundary.
	src := image.NewRGBA(image.Rect(0, 0, 9, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 9; x++ {
			c := uint8(80)
			if x >= 5 {
				c = 160
			}
			src.Set(x, y, color.RGBA{c, c, c, 255})
		}
	}
	sh := mustImage(t, New(src).Unsharp(UnsharpOptions{Sigma: 1.0, Amount: 1.5}))
	if sh.Bounds() != src.Bounds() {
		t.Fatal("unsharp changed dims")
	}
	// The dark side of the edge should be pushed darker than the flat region.
	if at(t, sh, 4, 1).R >= 80 {
		t.Fatalf("unsharp did not create undershoot: %d", at(t, sh, 4, 1).R)
	}
	// Sigma <= 0 no-op.
	if noop := mustImage(t, New(src).Unsharp(UnsharpOptions{Sigma: 0})); at(t, noop, 0, 0) != at(t, src, 0, 0) {
		t.Fatal("unsharp sigma 0 should be no-op")
	}
	// High threshold suppresses sharpening (flat interior unchanged).
	hi := mustImage(t, New(src).Unsharp(UnsharpOptions{Sigma: 1, Amount: 2, Threshold: 300}))
	if at(t, hi, 0, 1).R != 80 {
		t.Fatalf("threshold did not suppress: %+v", at(t, hi, 0, 1))
	}
}

func TestChannelOps(t *testing.T) {
	src := solid(2, 2, RGBA{10, 20, 30, 40})
	// ExtractChannel green -> gray 20.
	ex := mustImage(t, New(src).ExtractChannel(ChannelG))
	if got := at(t, ex, 0, 0); got.R != 20 || got.G != 20 || got.B != 20 || got.A != 255 {
		t.Fatalf("extract channel wrong: %+v", got)
	}
	if New(src).ExtractChannel(Channel(9)).Err() == nil {
		t.Fatal("expected extract channel error")
	}
	// RemoveAlpha forces opacity.
	ra := mustImage(t, New(src).RemoveAlpha())
	if at(t, ra, 0, 0).A != 255 {
		t.Fatal("remove alpha failed")
	}
	// EnsureAlpha on an opaque image sets the requested alpha.
	ea := mustImage(t, New(solid(2, 2, RGBA{1, 2, 3, 255})).EnsureAlpha(0.5))
	if a := at(t, ea, 0, 0).A; a < 126 || a > 130 {
		t.Fatalf("ensure alpha wrong: %d", a)
	}
	// EnsureAlpha on an already-transparent image is a no-op.
	ea2 := mustImage(t, New(src).EnsureAlpha(1))
	if at(t, ea2, 0, 0).A != 40 {
		t.Fatalf("ensure alpha should not touch existing alpha: %+v", at(t, ea2, 0, 0))
	}
}

func TestJoinChannels(t *testing.T) {
	r := solid(2, 2, RGBA{200, 0, 0, 255})
	g := solid(2, 2, RGBA{50, 0, 0, 255})
	b := solid(2, 2, RGBA{25, 0, 0, 255})
	joined := mustImage(t, JoinChannels(r, g, b))
	if got := at(t, joined, 0, 0); got.R != 200 || got.G != 50 || got.B != 25 || got.A != 255 {
		t.Fatalf("join channels wrong: %+v", got)
	}
	// Four-channel join.
	a := solid(2, 2, RGBA{128, 0, 0, 255})
	j4 := mustImage(t, JoinChannels(r, g, b, a))
	if at(t, j4, 0, 0).A != 128 {
		t.Fatalf("join 4 channel alpha wrong: %+v", at(t, j4, 0, 0))
	}
	// Errors.
	if JoinChannels(r, g).Err() == nil {
		t.Fatal("expected count error")
	}
	if JoinChannels(r, g, nil).Err() == nil {
		t.Fatal("expected nil image error")
	}
	if JoinChannels(r, g, solid(3, 3, White)).Err() == nil {
		t.Fatal("expected size mismatch error")
	}
}

func TestRecomb(t *testing.T) {
	// Swap R and B via a 3x3 permutation.
	m := [][]float64{
		{0, 0, 1},
		{0, 1, 0},
		{1, 0, 0},
	}
	got := mustImage(t, New(solid(1, 1, RGBA{10, 20, 30, 255})).Recomb(m))
	if c := at(t, got, 0, 0); c.R != 30 || c.G != 20 || c.B != 10 {
		t.Fatalf("recomb 3x3 wrong: %+v", c)
	}
	// 4x4 identity keeps alpha.
	id4 := [][]float64{
		{1, 0, 0, 0},
		{0, 1, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
	}
	got4 := mustImage(t, New(solid(1, 1, RGBA{10, 20, 30, 128})).Recomb(id4))
	if at(t, got4, 0, 0).A != 128 {
		t.Fatalf("recomb 4x4 alpha wrong: %+v", at(t, got4, 0, 0))
	}
	if New(solid(1, 1, White)).Recomb([][]float64{{1, 0}, {0, 1}}).Err() == nil {
		t.Fatal("expected recomb size error")
	}
	if New(solid(1, 1, White)).Recomb([][]float64{{1, 0, 0}, {0, 1}, {0, 0, 1}}).Err() == nil {
		t.Fatal("expected recomb row length error")
	}
}

func TestBooleanBandbool(t *testing.T) {
	a := solid(2, 2, RGBA{0xF0, 0x0F, 0xFF, 0xFF})
	b := solid(2, 2, RGBA{0x0F, 0xFF, 0x0F, 0xFF})
	and := mustImage(t, New(a).Boolean(b, BoolAnd))
	if c := at(t, and, 0, 0); c.R != 0x00 || c.G != 0x0F || c.B != 0x0F {
		t.Fatalf("boolean AND wrong: %+v", c)
	}
	or := mustImage(t, New(a).Boolean(b, BoolOr))
	if c := at(t, or, 0, 0); c.R != 0xFF || c.G != 0xFF || c.B != 0xFF {
		t.Fatalf("boolean OR wrong: %+v", c)
	}
	xor := mustImage(t, New(a).Boolean(b, BoolXor))
	if c := at(t, xor, 0, 0); c.R != 0xFF {
		t.Fatalf("boolean XOR wrong: %+v", c)
	}
	// Errors.
	if New(a).Boolean(nil, BoolAnd).Err() == nil {
		t.Fatal("expected nil boolean error")
	}
	if New(a).Boolean(solid(3, 3, White), BoolAnd).Err() == nil {
		t.Fatal("expected boolean size error")
	}
	if New(a).Boolean(b, BoolOp(9)).Err() == nil {
		t.Fatal("expected boolean op error")
	}
	// Bandbool OR folds channels.
	bb := mustImage(t, New(solid(1, 1, RGBA{0x01, 0x02, 0x04, 0xFF})).Bandbool(BoolOr))
	if at(t, bb, 0, 0).R != 0x07 {
		t.Fatalf("bandbool OR wrong: %+v", at(t, bb, 0, 0))
	}
	if New(a).Bandbool(BoolOp(9)).Err() == nil {
		t.Fatal("expected bandbool op error")
	}
}

func TestBlendModes(t *testing.T) {
	base := solid(1, 1, RGBA{100, 100, 100, 255})
	over := solid(1, 1, RGBA{200, 50, 50, 255})
	cases := []struct {
		mode BlendMode
		name string
	}{
		{BlendMultiply, "multiply"},
		{BlendScreen, "screen"},
		{BlendOverlay, "overlay"},
		{BlendDarken, "darken"},
		{BlendLighten, "lighten"},
		{BlendColorDodge, "dodge"},
		{BlendColorBurn, "burn"},
		{BlendHardLight, "hardlight"},
		{BlendSoftLight, "softlight"},
		{BlendDifference, "difference"},
		{BlendExclusion, "exclusion"},
		{BlendAdd, "add"},
	}
	for _, tc := range cases {
		got := mustImage(t, New(base).Composite(over, CompositeOptions{Blend: tc.mode}))
		if got.Bounds().Dx() != 1 {
			t.Fatalf("%s changed dims", tc.name)
		}
	}
	// Multiply of 100 and 200 => ~78 (100/255 * 200/255 * 255).
	mul := at(t, mustImage(t, New(base).Composite(over, CompositeOptions{Blend: BlendMultiply})), 0, 0)
	if mul.R < 74 || mul.R > 82 {
		t.Fatalf("multiply red wrong: %d", mul.R)
	}
	// Darken picks the min per channel.
	dk := at(t, mustImage(t, New(base).Composite(over, CompositeOptions{Blend: BlendDarken})), 0, 0)
	if dk.G != 50 {
		t.Fatalf("darken green wrong: %d", dk.G)
	}
	// BlendOver matches the default source-over path exactly.
	def := at(t, mustImage(t, New(base).Composite(over, CompositeOptions{})), 0, 0)
	ovr := at(t, mustImage(t, New(base).Composite(over, CompositeOptions{Blend: BlendOver})), 0, 0)
	if def != ovr {
		t.Fatalf("BlendOver != default: %+v vs %+v", ovr, def)
	}
}

func TestGIFRoundTrip(t *testing.T) {
	src := solid(8, 8, RGBA{200, 100, 50, 255})
	data, err := New(src).ToGIF()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gif.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("gif not decodable: %v", err)
	}
	// Decoded through the pipeline reports GIF format.
	if md, _ := FromBytes(data).Metadata(); md.Format != FormatGIF {
		t.Fatalf("gif format not detected: %+v", md.Format)
	}
}

func TestBMPRoundTrip(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 5, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 5; x++ {
			src.Set(x, y, color.RGBA{uint8(x * 40), uint8(y * 80), 60, 255})
		}
	}
	data, err := New(src).ToBMP()
	if err != nil {
		t.Fatal(err)
	}
	p := FromBytes(data)
	md, err := p.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if md.Format != FormatBMP || md.Width != 5 || md.Height != 3 {
		t.Fatalf("bmp metadata wrong: %+v", md)
	}
	out := mustImage(t, p)
	if at(t, out, 4, 2) != at(t, src, 4, 2) {
		t.Fatalf("bmp pixel mismatch: %+v vs %+v", at(t, out, 4, 2), at(t, src, 4, 2))
	}
	// 32-bit path (with alpha).
	al := solid(3, 3, RGBA{10, 20, 30, 128})
	d2, _ := New(al).ToBMP()
	o2 := mustImage(t, FromBytes(d2))
	if at(t, o2, 1, 1).A != 128 {
		t.Fatalf("bmp alpha lost: %+v", at(t, o2, 1, 1))
	}
}

func TestTIFFRoundTrip(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, color.RGBA{uint8(x * 50), uint8(y * 50), 90, uint8(200 - x*10)})
		}
	}
	data, err := New(src).ToTIFF()
	if err != nil {
		t.Fatal(err)
	}
	p := FromBytes(data)
	md, err := p.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if md.Format != FormatTIFF || md.Width != 4 || md.Height != 4 {
		t.Fatalf("tiff metadata wrong: %+v", md)
	}
	out := mustImage(t, p)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if at(t, out, x, y) != at(t, src, x, y) {
				t.Fatalf("tiff mismatch at %d,%d: %+v vs %+v", x, y, at(t, out, x, y), at(t, src, x, y))
			}
		}
	}
}

func TestRawRoundTrip(t *testing.T) {
	src := solid(3, 2, RGBA{11, 22, 33, 255})
	raw, err := New(src).ToRaw()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 3*2*4 {
		t.Fatalf("raw length wrong: %d", len(raw))
	}
	rebuilt := mustImage(t, FromRaw(raw, 3, 2, 4))
	if at(t, rebuilt, 0, 0) != (color.RGBA{11, 22, 33, 255}) {
		t.Fatalf("raw rgba round trip wrong: %+v", at(t, rebuilt, 0, 0))
	}
	// Grayscale raw.
	g := mustImage(t, FromRaw([]byte{0, 128, 255}, 3, 1, 1))
	if at(t, g, 1, 0).R != 128 || at(t, g, 2, 0).A != 255 {
		t.Fatalf("raw gray wrong: %+v", at(t, g, 1, 0))
	}
	// RGB raw.
	rgb := mustImage(t, FromRaw([]byte{1, 2, 3}, 1, 1, 3))
	if got := at(t, rgb, 0, 0); got.R != 1 || got.G != 2 || got.B != 3 || got.A != 255 {
		t.Fatalf("raw rgb wrong: %+v", got)
	}
	// Errors.
	if FromRaw(nil, 0, 1, 4).Err() == nil {
		t.Fatal("expected dim error")
	}
	if FromRaw([]byte{1}, 1, 1, 2).Err() == nil {
		t.Fatal("expected channel error")
	}
	if FromRaw([]byte{1, 2}, 1, 1, 4).Err() == nil {
		t.Fatal("expected length error")
	}
	if p := New(nil); func() bool { _, e := p.ToRaw(); return e == nil }() {
		t.Fatal("ToRaw should propagate error")
	}
}

func TestMetadataChannels(t *testing.T) {
	opaque := New(solid(2, 2, RGBA{1, 2, 3, 255}))
	md, _ := opaque.Metadata()
	if md.Channels != 3 || md.HasAlpha || md.Density != 72 || md.Space != "srgb" {
		t.Fatalf("opaque metadata wrong: %+v", md)
	}
	trans := New(solid(2, 2, RGBA{1, 2, 3, 100}))
	md2, _ := trans.Metadata()
	if md2.Channels != 4 || !md2.HasAlpha {
		t.Fatalf("transparent metadata wrong: %+v", md2)
	}
}

func TestCodecToFile(t *testing.T) {
	dir := t.TempDir()
	src := solid(4, 4, RGBA{100, 120, 140, 255})
	for _, tc := range []struct {
		name   string
		format Format
	}{
		{"out.gif", FormatGIF},
		{"out.bmp", FormatBMP},
		{"out.tiff", FormatTIFF},
	} {
		path := filepath.Join(dir, tc.name)
		if err := New(src).ToFile(path, tc.format, 0); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			t.Fatalf("%s not written: %v", tc.name, err)
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	// Truncated BMP.
	if _, err := decodeBMP([]byte("BM")); err == nil {
		t.Fatal("expected truncated bmp error")
	}
	// Non-TIFF to decodeTIFF.
	if _, err := decodeTIFF([]byte("xxxx")); err == nil {
		t.Fatal("expected tiff magic error")
	}
	// PNG still decodes through decodeImage.
	var buf bytes.Buffer
	_ = png.Encode(&buf, solid(2, 2, White))
	if _, f, err := decodeImage(buf.Bytes()); err != nil || f != FormatPNG {
		t.Fatalf("png through decodeImage wrong: f=%v err=%v", f, err)
	}
}

func TestNewOpsErrorPropagation(t *testing.T) {
	p := New(nil)
	// Every new operation must be a no-op that preserves the error.
	ops := []func(*Pipeline) *Pipeline{
		func(p *Pipeline) *Pipeline { return p.Affine([6]float64{1, 0, 0, 0, 1, 0}, AffineOptions{}) },
		func(p *Pipeline) *Pipeline { return p.Trim(10) },
		func(p *Pipeline) *Pipeline { return p.Normalise(NormaliseOptions{}) },
		func(p *Pipeline) *Pipeline { return p.Normalize(NormaliseOptions{}) },
		func(p *Pipeline) *Pipeline { return p.Linear([]float64{1}, []float64{0}) },
		func(p *Pipeline) *Pipeline { return p.Modulate(ModulateOptions{}) },
		func(p *Pipeline) *Pipeline { return p.CLAHE(CLAHEOptions{}) },
		func(p *Pipeline) *Pipeline { return p.Median(1) },
		func(p *Pipeline) *Pipeline { return p.Erode(1) },
		func(p *Pipeline) *Pipeline { return p.Dilate(1) },
		func(p *Pipeline) *Pipeline { return p.Morphology(MorphOpen, 1) },
		func(p *Pipeline) *Pipeline { return p.Unsharp(UnsharpOptions{Sigma: 1}) },
		func(p *Pipeline) *Pipeline { return p.ExtractChannel(ChannelR) },
		func(p *Pipeline) *Pipeline { return p.RemoveAlpha() },
		func(p *Pipeline) *Pipeline { return p.EnsureAlpha(1) },
		func(p *Pipeline) *Pipeline { return p.Recomb([][]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}) },
		func(p *Pipeline) *Pipeline { return p.Boolean(solid(1, 1, White), BoolAnd) },
		func(p *Pipeline) *Pipeline { return p.Bandbool(BoolOr) },
	}
	for i, op := range ops {
		if op(p).Err() == nil {
			t.Fatalf("op %d cleared error", i)
		}
	}
	// Terminal codec methods propagate the error.
	if _, err := p.ToGIF(); err == nil {
		t.Fatal("ToGIF should propagate error")
	}
	if _, err := p.ToBMP(); err == nil {
		t.Fatal("ToBMP should propagate error")
	}
	if _, err := p.ToTIFF(); err == nil {
		t.Fatal("ToTIFF should propagate error")
	}
}
