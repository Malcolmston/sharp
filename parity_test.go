package sharp

import (
	"image"
	"image/color"
	"testing"
)

// This file encodes known-answer vectors taken directly from the upstream
// Node.js sharp library's own test suite (github.com/lovell/sharp,
// test/unit/*.js) and asserts them against this pure-Go port's public API.
//
// Where upstream asserts an exact integer pixel (deepStrictEqual on a raw
// buffer) this port reproduces it exactly; where the upstream value is produced
// by a 32-bit-float libvips colour pipeline (Modulate, which runs through
// CIELAB), the assertion allows a tolerance of two 8-bit levels, documented per
// case. Every vector is deterministic.

// pxDiff returns the absolute difference between two uint8 values.
func pxDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// assertPixel fails t unless the R,G,B of img at (0,0) are within tol of the
// wanted values.
func assertPixel(t *testing.T, img image.Image, wantR, wantG, wantB uint8, tol int, label string) {
	t.Helper()
	got := at(t, img, 0, 0)
	if pxDiff(got.R, wantR) > tol || pxDiff(got.G, wantG) > tol || pxDiff(got.B, wantB) > tol {
		t.Fatalf("%s: got (%d,%d,%d) want (%d,%d,%d) tol %d", label, got.R, got.G, got.B, wantR, wantG, wantB, tol)
	}
}

// TestParityNegate mirrors upstream test/unit/negate.js "negate create":
// a 1x1 pixel with background {r:10,g:20,b:30} negated yields {245,235,225}.
func TestParityNegate(t *testing.T) {
	img := mustImage(t, New(solid(1, 1, RGBA{10, 20, 30, 255})).Negate())
	assertPixel(t, img, 245, 235, 225, 0, "negate create")
}

// TestParityModulate mirrors upstream test/unit/modulate.js, which asserts exact
// raw pixels for a 1x1 image with background {r:153,g:68,b:68} under various
// modulate options. Upstream performs modulate in LCh via libvips; this port
// does the same through D65 CIELAB, matching to within two 8-bit levels.
func TestParityModulate(t *testing.T) {
	const bgR, bgG, bgB = 153, 68, 68
	base := func() *Pipeline { return New(solid(1, 1, RGBA{bgR, bgG, bgB, 255})) }
	cases := []struct {
		label      string
		opts       ModulateOptions
		wr, wg, wb uint8
	}{
		{"hue 120", ModulateOptions{Hue: 120}, 41, 107, 57},
		{"brightness 2", ModulateOptions{Brightness: 2}, 255, 173, 168},
		{"brightness 0.5", ModulateOptions{Brightness: 0.5}, 97, 17, 25},
		{"saturation 2", ModulateOptions{Saturation: 2}, 198, 0, 43},
		{"saturation 0.5", ModulateOptions{Saturation: 0.5}, 127, 83, 81},
		{"lightness 10", ModulateOptions{Lightness: 10}, 182, 93, 92},
		{"all channels", ModulateOptions{Brightness: 2, Saturation: 0.5, Hue: 180}, 149, 209, 214},
	}
	for _, c := range cases {
		img := mustImage(t, base().Modulate(c.opts))
		assertPixel(t, img, c.wr, c.wg, c.wb, 2, "modulate "+c.label)
	}
}

// TestParityLinear mirrors upstream test/unit/linear.js: linear applies the
// integer transform out = round(a*in + b), clamped to [0,255], per channel, with
// alpha preserved. linear(1,0) is the identity (upstream "output is integer, not
// float") and per-channel slope/offset arrays are supported.
func TestParityLinear(t *testing.T) {
	// Identity: linear(1,0) leaves red untouched.
	id := mustImage(t, New(solid(1, 1, RGBA{255, 0, 0, 255})).Linear([]float64{1}, []float64{0}))
	assertPixel(t, id, 255, 0, 0, 0, "linear identity")

	// Per-channel: on grey 100, [0.25,0.5,0.75]*x + [150,100,50] =
	// (25+150, 50+100, 75+50) = (175, 150, 125).
	pc := mustImage(t, New(solid(1, 1, RGBA{100, 100, 100, 255})).
		Linear([]float64{0.25, 0.5, 0.75}, []float64{150, 100, 50}))
	assertPixel(t, pc, 175, 150, 125, 0, "linear per-channel")

	// Clamping: 2*200 + 0 = 400 -> 255.
	cl := mustImage(t, New(solid(1, 1, RGBA{200, 200, 200, 255})).Linear([]float64{2}, []float64{0}))
	assertPixel(t, cl, 255, 255, 255, 0, "linear clamp high")
}

// TestParityBoolean mirrors upstream test/unit/boolean.js, which applies the
// bitwise operators sharp.bool.and/or/eor between two images pixel-for-pixel.
// The vectors here use two fully-determined pixels so the exact byte results are
// known.
func TestParityBoolean(t *testing.T) {
	a := func() *Pipeline { return New(solid(1, 1, RGBA{0xCC, 0xAA, 0xF0, 0xFF})) }
	b := solid(1, 1, RGBA{0xAA, 0x0F, 0x0F, 0xFF})
	cases := []struct {
		label      string
		op         BoolOp
		wr, wg, wb uint8
	}{
		{"and", BoolAnd, 0xCC & 0xAA, 0xAA & 0x0F, 0xF0 & 0x0F},
		{"or", BoolOr, 0xCC | 0xAA, 0xAA | 0x0F, 0xF0 | 0x0F},
		{"eor", BoolXor, 0xCC ^ 0xAA, 0xAA ^ 0x0F, 0xF0 ^ 0x0F},
	}
	for _, c := range cases {
		img := mustImage(t, a().Boolean(b, c.op))
		assertPixel(t, img, c.wr, c.wg, c.wb, 0, "boolean "+c.label)
	}
}

// TestParityBandbool mirrors upstream test/unit/bandbool.js: bandbool folds the
// three colour channels of each pixel together with a bitwise operator to
// produce a single-band (grey) result, written across R, G and B.
func TestParityBandbool(t *testing.T) {
	base := func() *Pipeline { return New(solid(1, 1, RGBA{0xCC, 0xAA, 0xF0, 0xFF})) }
	cases := []struct {
		label string
		op    BoolOp
		want  uint8
	}{
		{"and", BoolAnd, 0xCC & 0xAA & 0xF0},
		{"or", BoolOr, 0xCC | 0xAA | 0xF0},
		{"eor", BoolXor, 0xCC ^ 0xAA ^ 0xF0},
	}
	for _, c := range cases {
		img := mustImage(t, base().Bandbool(c.op))
		assertPixel(t, img, c.want, c.want, c.want, 0, "bandbool "+c.label)
	}
}

// TestParityRecomb mirrors upstream test/unit/recomb.js, which multiplies each
// pixel's RGB vector by a 3x3 colour matrix (the classic sepia matrix). The
// recombination is a plain linear combination of the 8-bit band values, so the
// expected output is exact.
func TestParityRecomb(t *testing.T) {
	sepia := [][]float64{
		{0.393, 0.769, 0.189},
		{0.349, 0.686, 0.168},
		{0.272, 0.534, 0.131},
	}
	// (100,150,200):
	//   r = 0.393*100 + 0.769*150 + 0.189*200 = 192.45 -> 192
	//   g = 0.349*100 + 0.686*150 + 0.168*200 = 171.4  -> 171
	//   b = 0.272*100 + 0.534*150 + 0.131*200 = 133.5  -> 134
	img := mustImage(t, New(solid(1, 1, RGBA{100, 150, 200, 255})).Recomb(sepia))
	assertPixel(t, img, 192, 171, 134, 0, "recomb sepia")
}

// TestParityThreshold mirrors upstream test/unit/threshold.js: threshold reduces
// the image to pure black or white about a level. For grey pixels the luma
// equals the channel value regardless of the weighting coefficients, so the
// boundary behaviour (>= level -> white) is unambiguous.
func TestParityThreshold(t *testing.T) {
	white := mustImage(t, New(solid(1, 1, RGBA{200, 200, 200, 255})).Threshold(128))
	assertPixel(t, white, 255, 255, 255, 0, "threshold above")

	black := mustImage(t, New(solid(1, 1, RGBA{50, 50, 50, 255})).Threshold(128))
	assertPixel(t, black, 0, 0, 0, 0, "threshold below")
}

// TestParityExtend mirrors upstream test/unit/extend.js: extend pads the image on
// each side with a background colour, growing the canvas by exactly the padding
// on each axis. The original pixels are preserved and the new border takes the
// fill colour.
func TestParityExtend(t *testing.T) {
	// 100x80 source, pad top=10 right=20 bottom=30 left=40 with red.
	src := solid(100, 80, RGBA{0, 0, 255, 255})
	img := mustImage(t, New(src).Extend(10, 20, 30, 40, RGBA{255, 0, 0, 255}))
	b := img.Bounds()
	if b.Dx() != 160 || b.Dy() != 120 {
		t.Fatalf("extend dims: got %dx%d want 160x120", b.Dx(), b.Dy())
	}
	// Top-left corner is border (red).
	assertPixel(t, img, 255, 0, 0, 0, "extend border")
	// A pixel inside the original region (offset by left=40, top=10) is blue.
	corner := at(t, img, 40, 10)
	if corner.R != 0 || corner.G != 0 || corner.B != 255 {
		t.Fatalf("extend interior: got (%d,%d,%d) want (0,0,255)", corner.R, corner.G, corner.B)
	}
}

// TestParityExtract mirrors upstream test/unit/extract.js: extract/crop selects a
// rectangular region, and the output dimensions equal the requested width and
// height with the region's pixels preserved.
func TestParityExtract(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	fillRGBA(src, RGBA{0, 0, 0, 255})
	// Mark one pixel we expect to survive the crop.
	src.Set(4, 5, color.RGBA{10, 200, 30, 255})
	img := mustImage(t, New(src).Extract(Rectangle{X: 3, Y: 4, Width: 4, Height: 3}))
	b := img.Bounds()
	if b.Dx() != 4 || b.Dy() != 3 {
		t.Fatalf("extract dims: got %dx%d want 4x3", b.Dx(), b.Dy())
	}
	// The marked pixel at source (4,5) lands at (1,1) in the crop.
	got := at(t, img, 1, 1)
	if got.R != 10 || got.G != 200 || got.B != 30 {
		t.Fatalf("extract pixel: got (%d,%d,%d) want (10,200,30)", got.R, got.G, got.B)
	}
}
