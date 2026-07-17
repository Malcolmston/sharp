package sharp

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// solid builds a w x h image filled with c.
func solid(w, h int, c RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRGBA(img, c)
	return img
}

// checker builds a 2x2 image with distinct corner colours for orientation
// tests: TL red, TR green, BL blue, BR white.
func checker() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(1, 0, color.RGBA{0, 255, 0, 255})
	img.Set(0, 1, color.RGBA{0, 0, 255, 255})
	img.Set(1, 1, color.RGBA{255, 255, 255, 255})
	return img
}

func at(t *testing.T, img image.Image, x, y int) color.RGBA {
	t.Helper()
	r, g, b, a := img.At(x, y).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func mustImage(t *testing.T, p *Pipeline) *image.RGBA {
	t.Helper()
	img, err := p.ToImage()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return img.(*image.RGBA)
}

func TestNewNilAndError(t *testing.T) {
	p := New(nil)
	if p.Err() == nil {
		t.Fatal("expected error for nil image")
	}
	if _, err := p.ToImage(); err == nil {
		t.Fatal("ToImage should propagate error")
	}
	if _, err := p.ToPNG(); err == nil {
		t.Fatal("ToPNG should propagate error")
	}
	if _, err := p.ToJPEG(90); err == nil {
		t.Fatal("ToJPEG should propagate error")
	}
	if _, err := p.Metadata(); err == nil {
		t.Fatal("Metadata should propagate error")
	}
	if _, err := p.Stats(); err == nil {
		t.Fatal("Stats should propagate error")
	}
	// Ops on errored pipeline stay no-ops and keep the error.
	if p.Grayscale().Resize(ResizeOptions{Width: 2}).Err() == nil {
		t.Fatal("ops should not clear error")
	}
}

func TestNewCopiesSource(t *testing.T) {
	src := solid(3, 3, RGBA{10, 20, 30, 255})
	p := New(src)
	p.Negate()
	// Original must be untouched.
	if got := at(t, src, 0, 0); got != (color.RGBA{10, 20, 30, 255}) {
		t.Fatalf("source mutated: %+v", got)
	}
}

func TestMetadataAndStats(t *testing.T) {
	p := New(solid(4, 2, RGBA{100, 150, 200, 255}))
	md, err := p.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if md.Width != 4 || md.Height != 2 {
		t.Fatalf("bad metadata: %+v", md)
	}
	st, err := p.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.MeanR != 100 || st.MeanG != 150 || st.MeanB != 200 || st.MeanA != 255 {
		t.Fatalf("bad stats: %+v", st)
	}
}

func TestFromBytesRoundTrip(t *testing.T) {
	src := solid(5, 5, RGBA{40, 80, 120, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	p := FromBytes(buf.Bytes())
	md, err := p.Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if md.Format != FormatPNG {
		t.Fatalf("expected png, got %q", md.Format)
	}
	if md.Width != 5 || md.Height != 5 {
		t.Fatalf("bad dims: %+v", md)
	}

	// JPEG detection.
	var jb bytes.Buffer
	if err := jpeg.Encode(&jb, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if f, _ := FromBytes(jb.Bytes()).Metadata(); f.Format != FormatJPEG {
		t.Fatalf("expected jpeg, got %q", f.Format)
	}

	// Bad data.
	if FromBytes([]byte("not an image")).Err() == nil {
		t.Fatal("expected decode error")
	}
}

func TestFromFileAndToFile(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "in.png")
	src := solid(6, 4, RGBA{200, 100, 50, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pngPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	p := FromFile(pngPath)
	if p.Err() != nil {
		t.Fatal(p.Err())
	}

	outPNG := filepath.Join(dir, "out.png")
	if err := p.Clone().ToFile(outPNG, FormatUnknown, 0); err != nil {
		t.Fatal(err)
	}
	outJPG := filepath.Join(dir, "out.jpg")
	if err := p.Clone().ToFile(outJPG, FormatJPEG, 80); err != nil {
		t.Fatal(err)
	}
	// Re-decode the PNG output.
	rp := FromFile(outPNG)
	if md, _ := rp.Metadata(); md.Width != 6 || md.Height != 4 {
		t.Fatalf("roundtrip dims wrong: %+v", md)
	}

	if FromFile(filepath.Join(dir, "nope.png")).Err() == nil {
		t.Fatal("expected open error")
	}
	if err := New(src).ToFile(outPNG, Format("gif"), 0); err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestToJPEGQualityClamp(t *testing.T) {
	p := New(solid(8, 8, RGBA{123, 45, 67, 255}))
	for _, q := range []int{0, -5, 50, 200} {
		b, err := p.Clone().ToJPEG(q)
		if err != nil {
			t.Fatalf("q=%d: %v", q, err)
		}
		if len(b) == 0 {
			t.Fatalf("q=%d: empty jpeg", q)
		}
	}
}

func TestToPNGWithOptions(t *testing.T) {
	p := New(solid(8, 8, RGBA{1, 2, 3, 255}))
	b, err := p.ToPNGWithOptions(PNGOptions{Compression: png.BestCompression})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(b)); err != nil {
		t.Fatalf("output not valid png: %v", err)
	}
}

func TestResizeExactNearestAndBilinear(t *testing.T) {
	src := checker()
	// Nearest upscaling doubles each pixel block.
	n := mustImage(t, New(src).Resize(ResizeOptions{Width: 4, Height: 4, Fit: FitExact, Interpolation: Nearest}))
	if at(t, n, 0, 0) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("nearest TL wrong: %+v", at(t, n, 0, 0))
	}
	if at(t, n, 3, 3) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("nearest BR wrong: %+v", at(t, n, 3, 3))
	}
	// Bilinear same-size is identity.
	b := mustImage(t, New(src).Resize(ResizeOptions{Width: 2, Height: 2, Fit: FitExact, Interpolation: Bilinear}))
	if at(t, b, 0, 0) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("bilinear identity failed: %+v", at(t, b, 0, 0))
	}
}

func TestResizeAspectDerivation(t *testing.T) {
	src := solid(100, 50, White)
	// Only width given: height should be derived to keep ratio (2:1) -> 20.
	got := mustImage(t, New(src).ResizeTo(40, 0))
	if got.Bounds().Dx() != 40 || got.Bounds().Dy() != 20 {
		t.Fatalf("derived height wrong: %v", got.Bounds())
	}
	got2 := mustImage(t, New(src).Resize(ResizeOptions{Height: 10}))
	if got2.Bounds().Dx() != 20 || got2.Bounds().Dy() != 10 {
		t.Fatalf("derived width wrong: %v", got2.Bounds())
	}
}

func TestResizeFitContainCover(t *testing.T) {
	src := solid(100, 50, White)
	contain := mustImage(t, New(src).Resize(ResizeOptions{Width: 40, Height: 40, Fit: FitContain}))
	// Fits inside 40x40 preserving 2:1 -> 40x20.
	if contain.Bounds().Dx() != 40 || contain.Bounds().Dy() != 20 {
		t.Fatalf("contain wrong: %v", contain.Bounds())
	}
	cover := mustImage(t, New(src).Resize(ResizeOptions{Width: 40, Height: 40, Fit: FitCover}))
	// Cover crops to exactly 40x40.
	if cover.Bounds().Dx() != 40 || cover.Bounds().Dy() != 40 {
		t.Fatalf("cover wrong: %v", cover.Bounds())
	}
}

func TestResizeErrors(t *testing.T) {
	src := solid(4, 4, White)
	if New(src).Resize(ResizeOptions{}).Err() == nil {
		t.Fatal("expected error for no dims")
	}
	if New(src).Resize(ResizeOptions{Width: -1, Height: 2}).Err() == nil {
		t.Fatal("expected error for negative dim")
	}
	if New(src).Resize(ResizeOptions{Width: 2, Height: 2, Fit: Fit(99)}).Err() == nil {
		t.Fatal("expected error for bad fit")
	}
	empty := &Pipeline{img: image.NewRGBA(image.Rect(0, 0, 0, 0))}
	if empty.Resize(ResizeOptions{Width: 2}).Err() == nil {
		t.Fatal("expected error for empty image")
	}
}

func TestExtractAndCrop(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, color.RGBA{uint8(x * 10), uint8(y * 10), 0, 255})
		}
	}
	got := mustImage(t, New(src).Crop(Rectangle{X: 1, Y: 1, Width: 2, Height: 2}))
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 2 {
		t.Fatalf("crop dims wrong: %v", got.Bounds())
	}
	if at(t, got, 0, 0) != (color.RGBA{10, 10, 0, 255}) {
		t.Fatalf("crop origin wrong: %+v", at(t, got, 0, 0))
	}
	// Error cases.
	if New(src).Extract(Rectangle{Width: 0, Height: 2}).Err() == nil {
		t.Fatal("expected error for zero width")
	}
	if New(src).Extract(Rectangle{X: 3, Y: 0, Width: 3, Height: 1}).Err() == nil {
		t.Fatal("expected out-of-bounds error")
	}
}

func TestExtend(t *testing.T) {
	src := solid(2, 2, RGBA{255, 0, 0, 255})
	got := mustImage(t, New(src).Extend(1, 2, 3, 4, RGBA{0, 0, 0, 255}))
	if got.Bounds().Dx() != 2+2+4 || got.Bounds().Dy() != 2+1+3 {
		t.Fatalf("extend dims wrong: %v", got.Bounds())
	}
	// Corner is fill.
	if at(t, got, 0, 0) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("fill wrong: %+v", at(t, got, 0, 0))
	}
	// Interior original pixel at (left=4, top=1).
	if at(t, got, 4, 1) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("interior wrong: %+v", at(t, got, 4, 1))
	}
	// Negative treated as zero.
	got2 := mustImage(t, New(src).Extend(-5, 0, 0, 0, Black))
	if got2.Bounds().Dy() != 2 {
		t.Fatalf("negative extend not clamped: %v", got2.Bounds())
	}
}

func TestFlips(t *testing.T) {
	src := checker()
	fv := mustImage(t, New(src).FlipVertical())
	if at(t, fv, 0, 0) != (color.RGBA{0, 0, 255, 255}) { // was BL blue
		t.Fatalf("flip vertical wrong: %+v", at(t, fv, 0, 0))
	}
	fh := mustImage(t, New(src).Flop())
	if at(t, fh, 0, 0) != (color.RGBA{0, 255, 0, 255}) { // was TR green
		t.Fatalf("flip horizontal wrong: %+v", at(t, fh, 0, 0))
	}
	// Flip alias matches FlipVertical.
	fa := mustImage(t, New(src).Flip())
	if at(t, fa, 0, 0) != at(t, fv, 0, 0) {
		t.Fatalf("Flip alias mismatch: %+v", at(t, fa, 0, 0))
	}
}

func TestRotate90180270(t *testing.T) {
	src := checker() // TL red, TR green, BL blue, BR white
	r90 := mustImage(t, New(src).Rotate90())
	// 90 CW: TL(red) -> top-right.
	if at(t, r90, 1, 0) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("rotate90 wrong: %+v", at(t, r90, 1, 0))
	}
	r180 := mustImage(t, New(src).Rotate180())
	if at(t, r180, 1, 1) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("rotate180 wrong: %+v", at(t, r180, 1, 1))
	}
	r270 := mustImage(t, New(src).Rotate270())
	if at(t, r270, 0, 1) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("rotate270 wrong: %+v", at(t, r270, 0, 1))
	}
	// Non-square dimensions swap.
	rect := solid(4, 2, White)
	got := mustImage(t, New(rect).Rotate90())
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 4 {
		t.Fatalf("rotate90 dims wrong: %v", got.Bounds())
	}
}

func TestRotateArbitrary(t *testing.T) {
	src := solid(10, 10, RGBA{255, 255, 255, 255})
	// 45 degrees enlarges canvas; corners filled with black.
	got := mustImage(t, New(src).Rotate(45, Black))
	if got.Bounds().Dx() <= 10 {
		t.Fatalf("expected enlarged canvas, got %v", got.Bounds())
	}
	// Top-left corner should be background fill.
	if at(t, got, 0, 0) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("corner not filled: %+v", at(t, got, 0, 0))
	}
	// Centre should remain white.
	c := got.Bounds().Dx() / 2
	if at(t, got, c, c) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("centre not white: %+v", at(t, got, c, c))
	}
	// Multiples of 90 dispatch to exact and 0/360 are no-ops.
	if mustImage(t, New(src).Rotate(360, Black)).Bounds().Dx() != 10 {
		t.Fatal("360 should be no-op")
	}
	if mustImage(t, New(checker()).Rotate(-90, Black)).Bounds().Dx() != 2 {
		t.Fatal("negative multiple should dispatch")
	}
	if mustImage(t, New(checker()).Rotate(180, Black)).Bounds().Dy() != 2 {
		t.Fatal("180 dispatch failed")
	}
}

func TestGrayscaleAndThreshold(t *testing.T) {
	p := New(solid(2, 2, RGBA{255, 0, 0, 255})).Grayscale()
	g := mustImage(t, p)
	want := clampF(0.299 * 255)
	if got := at(t, g, 0, 0); got.R != want || got.R != got.G || got.G != got.B {
		t.Fatalf("grayscale wrong: %+v want %d", got, want)
	}
	// Threshold: bright -> white, dark -> black.
	bright := mustImage(t, New(solid(1, 1, White)).Threshold(128))
	if at(t, bright, 0, 0) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatal("threshold bright wrong")
	}
	dark := mustImage(t, New(solid(1, 1, RGBA{10, 10, 10, 255})).Threshold(128))
	if at(t, dark, 0, 0) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatal("threshold dark wrong")
	}
}

func TestNegateTintBrightness(t *testing.T) {
	neg := mustImage(t, New(solid(1, 1, RGBA{10, 20, 30, 255})).Invert())
	if at(t, neg, 0, 0) != (color.RGBA{245, 235, 225, 255}) {
		t.Fatalf("negate wrong: %+v", at(t, neg, 0, 0))
	}
	tint := mustImage(t, New(solid(1, 1, White)).Tint(RGBA{128, 0, 255, 255}))
	got := at(t, tint, 0, 0)
	if got.R != 128 || got.G != 0 || got.B != 255 {
		t.Fatalf("tint wrong: %+v", got)
	}
	br := mustImage(t, New(solid(1, 1, RGBA{100, 100, 100, 255})).Brightness(2))
	if at(t, br, 0, 0) != (color.RGBA{200, 200, 200, 255}) {
		t.Fatalf("brightness wrong: %+v", at(t, br, 0, 0))
	}
	// Negative brightness clamps to black.
	dark := mustImage(t, New(solid(1, 1, White)).Brightness(-1))
	if at(t, dark, 0, 0) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatal("negative brightness should be black")
	}
}

func TestContrastGammaSaturation(t *testing.T) {
	// Contrast > 1 pushes 200 away from midpoint upward.
	c := mustImage(t, New(solid(1, 1, RGBA{200, 200, 200, 255})).Contrast(2))
	if at(t, c, 0, 0).R != clampF((200-128)*2+128) {
		t.Fatalf("contrast wrong: %+v", at(t, c, 0, 0))
	}
	// Negative contrast clamps factor to 0 -> midpoint grey.
	cn := mustImage(t, New(solid(1, 1, White)).Contrast(-1))
	if at(t, cn, 0, 0).R != 128 {
		t.Fatalf("contrast clamp wrong: %+v", at(t, cn, 0, 0))
	}
	// Gamma 1 is identity; gamma <=0 is no-op.
	id := mustImage(t, New(solid(1, 1, RGBA{123, 50, 200, 255})).Gamma(1))
	if at(t, id, 0, 0) != (color.RGBA{123, 50, 200, 255}) {
		t.Fatalf("gamma 1 not identity: %+v", at(t, id, 0, 0))
	}
	noop := mustImage(t, New(solid(1, 1, RGBA{123, 50, 200, 255})).Gamma(0))
	if at(t, noop, 0, 0) != (color.RGBA{123, 50, 200, 255}) {
		t.Fatal("gamma<=0 should be no-op")
	}
	// Gamma 2 brightens mid-tones.
	brighter := mustImage(t, New(solid(1, 1, RGBA{128, 128, 128, 255})).Gamma(2))
	if at(t, brighter, 0, 0).R <= 128 {
		t.Fatalf("gamma 2 should brighten: %+v", at(t, brighter, 0, 0))
	}
	// Saturation 0 => grayscale (all channels equal to luma).
	s := mustImage(t, New(solid(1, 1, RGBA{255, 0, 0, 255})).Saturation(0))
	g := at(t, s, 0, 0)
	if g.R != g.G || g.G != g.B {
		t.Fatalf("saturation 0 not grayscale: %+v", g)
	}
	// Negative saturation clamps to 0 -> grayscale too.
	sn := mustImage(t, New(solid(1, 1, RGBA{255, 0, 0, 255})).Saturation(-3))
	if gn := at(t, sn, 0, 0); gn.R != gn.G {
		t.Fatalf("negative saturation not grayscale: %+v", gn)
	}
}

func TestConvolveAndKernel(t *testing.T) {
	// Identity kernel leaves image unchanged.
	id, err := NewKernel([]float64{0, 0, 0, 0, 1, 0, 0, 0, 0}, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			src.Set(x, y, color.RGBA{uint8(x * 40), uint8(y * 40), 100, 255})
		}
	}
	got := mustImage(t, New(src).Convolve(id))
	if at(t, got, 1, 1) != at(t, src, 1, 1) {
		t.Fatalf("identity convolve changed pixel: %+v vs %+v", at(t, got, 1, 1), at(t, src, 1, 1))
	}
	// Alpha preserved.
	if at(t, got, 1, 1).A != 255 {
		t.Fatal("convolve dropped alpha")
	}

	// NewKernel error paths.
	if _, err := NewKernel([]float64{1, 2, 3, 4}, 1, 0); err == nil {
		t.Fatal("expected error for even-sized kernel")
	}
	// Invalid kernel via Convolve directly.
	if New(src).Convolve(Kernel{Size: 2, Weights: []float64{1, 2, 3, 4}}).Err() == nil {
		t.Fatal("expected invalid kernel error")
	}
	// Divisor zero treated as 1 (box sum then offset).
	box := Kernel{Size: 3, Weights: []float64{1, 1, 1, 1, 1, 1, 1, 1, 1}, Divisor: 0, Offset: 0}
	if New(src).Convolve(box).Err() != nil {
		t.Fatal("box kernel should succeed")
	}
}

func TestBlurAndSharpen(t *testing.T) {
	// Sharp edge image: left black, right white.
	src := image.NewRGBA(image.Rect(0, 0, 9, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 9; x++ {
			c := uint8(0)
			if x >= 5 {
				c = 255
			}
			src.Set(x, y, color.RGBA{c, c, c, 255})
		}
	}
	blur := mustImage(t, New(src).Blur(1.5))
	// A pixel near the boundary should now be an intermediate value.
	v := at(t, blur, 5, 1).R
	if v == 0 || v == 255 {
		t.Fatalf("blur did not soften edge: %d", v)
	}
	// sigma <= 0 is a no-op.
	same := mustImage(t, New(src).Blur(0))
	if at(t, same, 0, 0) != at(t, src, 0, 0) {
		t.Fatal("blur 0 should be no-op")
	}
	// Sharpen increases local contrast; amount<=0 no-op.
	sh := mustImage(t, New(src).Sharpen(1))
	if sh.Bounds() != src.Bounds() {
		t.Fatal("sharpen changed dims")
	}
	if noop := mustImage(t, New(src).Sharpen(0)); at(t, noop, 0, 0) != at(t, src, 0, 0) {
		t.Fatal("sharpen 0 should be no-op")
	}
}

func TestCompositeOffsetAndGravity(t *testing.T) {
	base := solid(4, 4, RGBA{0, 0, 0, 255})
	overlay := solid(2, 2, RGBA{255, 0, 0, 255})
	// Offset placement.
	got := mustImage(t, New(base).Composite(overlay, CompositeOptions{Left: 1, Top: 1}))
	if at(t, got, 1, 1) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("composite offset wrong: %+v", at(t, got, 1, 1))
	}
	if at(t, got, 0, 0) != (color.RGBA{0, 0, 0, 255}) {
		t.Fatalf("composite bled outside: %+v", at(t, got, 0, 0))
	}
	// Gravity center.
	gc := mustImage(t, New(base).Composite(overlay, CompositeOptions{UseGravity: true, Gravity: GravityCenter}))
	if at(t, gc, 1, 1) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("gravity center wrong: %+v", at(t, gc, 1, 1))
	}
	// Bottom-right gravity places overlay at (2,2).
	br := mustImage(t, New(base).Composite(overlay, CompositeOptions{UseGravity: true, Gravity: GravityBottomRight}))
	if at(t, br, 3, 3) != (color.RGBA{255, 0, 0, 255}) {
		t.Fatalf("gravity BR wrong: %+v", at(t, br, 3, 3))
	}
	// 50% opacity blends half-way.
	semi := mustImage(t, New(solid(2, 2, RGBA{0, 0, 0, 255})).
		Composite(solid(2, 2, RGBA{255, 255, 255, 255}), CompositeOptions{Opacity: 0.5}))
	if r := at(t, semi, 0, 0).R; r < 120 || r > 135 {
		t.Fatalf("opacity blend wrong: %d", r)
	}
}

func TestCompositeTransparentOverlay(t *testing.T) {
	base := solid(2, 2, RGBA{10, 20, 30, 255})
	overlay := solid(2, 2, Transparent) // fully transparent
	got := mustImage(t, New(base).Composite(overlay, CompositeOptions{}))
	if at(t, got, 0, 0) != (color.RGBA{10, 20, 30, 255}) {
		t.Fatalf("transparent overlay changed base: %+v", at(t, got, 0, 0))
	}
}

func TestFlatten(t *testing.T) {
	// Half-transparent red over white background => pink, opaque.
	src := solid(2, 2, RGBA{255, 0, 0, 128})
	got := mustImage(t, New(src).Flatten(White))
	c := at(t, got, 0, 0)
	if c.A != 255 {
		t.Fatalf("flatten not opaque: %+v", c)
	}
	if c.G == 0 || c.B == 0 {
		t.Fatalf("flatten did not blend background: %+v", c)
	}
}

func TestCloneIndependence(t *testing.T) {
	p := New(solid(2, 2, RGBA{50, 60, 70, 255}))
	c := p.Clone()
	c.Negate()
	orig := mustImage(t, p)
	if at(t, orig, 0, 0) != (color.RGBA{50, 60, 70, 255}) {
		t.Fatalf("clone mutated original: %+v", at(t, orig, 0, 0))
	}
	// Cloning an errored pipeline preserves the error.
	if New(nil).Clone().Err() == nil {
		t.Fatal("clone should preserve error")
	}
}

func TestChaining(t *testing.T) {
	src := solid(20, 10, RGBA{200, 100, 50, 255})
	out, err := New(src).
		Resize(ResizeOptions{Width: 10, Height: 5, Fit: FitExact, Interpolation: Bilinear}).
		Grayscale().
		Sharpen(0.5).
		Extend(1, 1, 1, 1, Black).
		ToPNG()
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 12 || img.Bounds().Dy() != 7 {
		t.Fatalf("chained dims wrong: %v", img.Bounds())
	}
}

func TestHelpers(t *testing.T) {
	if clamp8(-1) != 0 || clamp8(300) != 255 || clamp8(128) != 128 {
		t.Fatal("clamp8 wrong")
	}
	if clampInt(5, 0, 3) != 3 || clampInt(-1, 0, 3) != 0 {
		t.Fatal("clampInt wrong")
	}
	if maxInt(2, 5) != 5 || minF(1, 2) != 1 || maxF(1, 2) != 2 {
		t.Fatal("min/max wrong")
	}
	if RGBA(White).NRGBA().R != 255 {
		t.Fatal("NRGBA conversion wrong")
	}
}

func TestNonRGBASource(t *testing.T) {
	// Feed a non-RGBA image type to exercise the generic conversion path.
	gray := image.NewGray(image.Rect(0, 0, 3, 3))
	gray.Set(0, 0, color.Gray{Y: 200})
	p := New(gray)
	if md, _ := p.Metadata(); md.Width != 3 {
		t.Fatalf("gray conversion wrong: %+v", md)
	}
}
