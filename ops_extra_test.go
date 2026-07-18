package sharp

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestLuma(t *testing.T) {
	cases := []struct {
		r, g, b uint8
		want    uint8
	}{
		{255, 255, 255, 255},
		{0, 0, 0, 0},
		{255, 0, 0, 76},  // 0.299*255 = 76.245
		{0, 255, 0, 150}, // 0.587*255 = 149.685 -> 150
		{0, 0, 255, 29},  // 0.114*255 = 29.07
	}
	for _, c := range cases {
		if got := Luma(c.r, c.g, c.b); got != c.want {
			t.Errorf("Luma(%d,%d,%d) = %d, want %d", c.r, c.g, c.b, got, c.want)
		}
	}
}

func TestSepia(t *testing.T) {
	p := New(solid(2, 2, RGBA{100, 100, 100, 255})).Sepia()
	img := mustImage(t, p)
	c := at(t, img, 0, 0)
	// 0.393+0.769+0.189 = 1.351 *100 = 135.1
	// 0.349+0.686+0.168 = 1.203 *100 = 120.3
	// 0.272+0.534+0.131 = 0.937 *100 = 93.7
	if c.R != 135 || c.G != 120 || c.B != 94 {
		t.Errorf("sepia = %+v, want {135,120,94}", c)
	}
	if c.A != 255 {
		t.Errorf("sepia alpha = %d", c.A)
	}
}

func TestUnflatten(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255}) // white -> transparent
	img.Set(1, 0, color.RGBA{255, 255, 254, 255}) // near-white -> kept
	out := mustImage(t, New(img).Unflatten())
	if a := at(t, out, 0, 0).A; a != 0 {
		t.Errorf("white alpha = %d, want 0", a)
	}
	if a := at(t, out, 1, 0).A; a != 255 {
		t.Errorf("non-white alpha = %d, want 255", a)
	}
}

func TestToColourspace(t *testing.T) {
	// Greyscale conversion.
	out := mustImage(t, New(solid(2, 2, RGBA{255, 0, 0, 255})).ToColourspace("b-w"))
	c := at(t, out, 0, 0)
	if c.R != c.G || c.G != c.B {
		t.Errorf("b-w did not equalise channels: %+v", c)
	}
	if c.R != 76 {
		t.Errorf("b-w red luma = %d, want 76", c.R)
	}
	// srgb is a no-op.
	out = mustImage(t, New(solid(2, 2, RGBA{10, 20, 30, 255})).ToColorspace("srgb"))
	if c := at(t, out, 0, 0); c.R != 10 || c.G != 20 || c.B != 30 {
		t.Errorf("srgb altered pixels: %+v", c)
	}
	// Unknown space is an error.
	if err := New(solid(1, 1, White)).ToColourspace("cmyk").Err(); err == nil {
		t.Error("expected error for unsupported colourspace")
	}
}

func TestWithDensity(t *testing.T) {
	md, err := New(solid(2, 2, White)).WithDensity(300).Metadata()
	if err != nil {
		t.Fatal(err)
	}
	if md.Density != 300 {
		t.Errorf("density = %d, want 300", md.Density)
	}
	// Non-positive ignored (keeps default).
	md, _ = New(solid(2, 2, White)).WithDensity(0).Metadata()
	if md.Density != defaultDensity {
		t.Errorf("density = %d, want default %d", md.Density, defaultDensity)
	}
}

func TestResizeWidthHeight(t *testing.T) {
	img := mustImage(t, New(solid(100, 50, White)).ResizeWidth(20))
	if img.Bounds().Dx() != 20 || img.Bounds().Dy() != 10 {
		t.Errorf("ResizeWidth -> %v, want 20x10", img.Bounds())
	}
	img = mustImage(t, New(solid(100, 50, White)).ResizeHeight(25))
	if img.Bounds().Dx() != 50 || img.Bounds().Dy() != 25 {
		t.Errorf("ResizeHeight -> %v, want 50x25", img.Bounds())
	}
}

func TestToFormat(t *testing.T) {
	p := New(solid(3, 3, RGBA{1, 2, 3, 255}))
	png, err := p.ToFormat(FormatPNG, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, format, err := image.Decode(bytes.NewReader(png)); err != nil || format != "png" {
		t.Errorf("ToFormat PNG decode: format=%q err=%v", format, err)
	}
	jpg, err := p.ToFormat(FormatJPEG, 80)
	if err != nil {
		t.Fatal(err)
	}
	if _, format, err := image.Decode(bytes.NewReader(jpg)); err != nil || format != "jpeg" {
		t.Errorf("ToFormat JPEG decode: format=%q err=%v", format, err)
	}
	raw, err := p.ToFormat(FormatRaw, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 3*3*4 {
		t.Errorf("raw length = %d, want 36", len(raw))
	}
}
