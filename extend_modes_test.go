package sharp

import (
	"image"
	"image/color"
	"testing"
)

// distinct2x2 builds a 2x2 image with unique corner colours.
func distinct2x2() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{10, 0, 0, 255}) // TL
	img.Set(1, 0, color.RGBA{20, 0, 0, 255}) // TR
	img.Set(0, 1, color.RGBA{30, 0, 0, 255}) // BL
	img.Set(1, 1, color.RGBA{40, 0, 0, 255}) // BR
	return img
}

func TestExtendWithBackground(t *testing.T) {
	p := New(distinct2x2()).ExtendWith(ExtendOptions{
		Top: 1, Right: 1, Bottom: 1, Left: 1,
		Background: RGBA{99, 99, 99, 255},
		Mode:       ExtendBackground,
	})
	img := mustImage(t, p)
	if img.Bounds().Dx() != 4 || img.Bounds().Dy() != 4 {
		t.Fatalf("size = %v", img.Bounds())
	}
	if c := at(t, img, 0, 0); c.R != 99 {
		t.Errorf("corner not background: %+v", c)
	}
	if c := at(t, img, 1, 1); c.R != 10 {
		t.Errorf("interior TL = %+v, want R=10", c)
	}
}

func TestExtendWithCopy(t *testing.T) {
	p := New(distinct2x2()).ExtendWith(ExtendOptions{
		Top: 1, Right: 1, Bottom: 1, Left: 1, Mode: ExtendCopy,
	})
	img := mustImage(t, p)
	// Top-left border pixel replicates the nearest edge pixel (TL = 10).
	if c := at(t, img, 0, 0); c.R != 10 {
		t.Errorf("copy TL border = %+v, want R=10", c)
	}
	// Bottom-right border replicates BR = 40.
	if c := at(t, img, 3, 3); c.R != 40 {
		t.Errorf("copy BR border = %+v, want R=40", c)
	}
}

func TestExtendWithRepeat(t *testing.T) {
	p := New(distinct2x2()).ExtendWith(ExtendOptions{
		Top: 2, Right: 2, Bottom: 2, Left: 2, Mode: ExtendRepeat,
	})
	img := mustImage(t, p)
	// left=2, so source x=0 maps to output x=2. Output x=0 -> src (0-2) mod 2 = 0.
	if c := at(t, img, 0, 2); c.R != 10 {
		t.Errorf("repeat (0,2) = %+v, want R=10", c)
	}
	// Output x=1 -> src (1-2) mod 2 = 1 (wrapped) -> TR=20 at row of TL.
	if c := at(t, img, 1, 2); c.R != 20 {
		t.Errorf("repeat (1,2) = %+v, want R=20", c)
	}
}

func TestExtendWithMirror(t *testing.T) {
	p := New(distinct2x2()).ExtendWith(ExtendOptions{
		Top: 1, Right: 1, Bottom: 1, Left: 1, Mode: ExtendMirror,
	})
	img := mustImage(t, p)
	// left=1: output x=0 -> src index -1 mirrors to 0. Row y=1 -> src y=0.
	// So (0,1) mirrors TL column -> R=10.
	if c := at(t, img, 0, 1); c.R != 10 {
		t.Errorf("mirror (0,1) = %+v, want R=10", c)
	}
	// Right border output x=3 -> src index 2 mirrors to 1 (TR=20) on top row.
	if c := at(t, img, 3, 1); c.R != 20 {
		t.Errorf("mirror (3,1) = %+v, want R=20", c)
	}
}

func TestExtendIndex(t *testing.T) {
	// Mirror over n=3: sequence for i=-3..5.
	wantMirror := []int{2, 1, 0, 0, 1, 2, 2, 1, 0}
	for k, i := range []int{-3, -2, -1, 0, 1, 2, 3, 4, 5} {
		if got := extendIndex(i, 3, ExtendMirror); got != wantMirror[k] {
			t.Errorf("mirror index i=%d -> %d, want %d", i, got, wantMirror[k])
		}
	}
	// Repeat over n=3.
	wantRepeat := []int{0, 1, 2, 0, 1, 2, 0, 1, 2}
	for k, i := range []int{-3, -2, -1, 0, 1, 2, 3, 4, 5} {
		if got := extendIndex(i, 3, ExtendRepeat); got != wantRepeat[k] {
			t.Errorf("repeat index i=%d -> %d, want %d", i, got, wantRepeat[k])
		}
	}
}
