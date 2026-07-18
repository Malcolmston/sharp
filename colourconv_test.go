package sharp

import (
	"math"
	"testing"
)

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestRGBToHSVKnown(t *testing.T) {
	cases := []struct {
		r, g, b uint8
		h, s, v float64
	}{
		{255, 0, 0, 0, 1, 1},
		{0, 255, 0, 120, 1, 1},
		{0, 0, 255, 240, 1, 1},
		{255, 255, 0, 60, 1, 1},
		{0, 255, 255, 180, 1, 1},
		{255, 0, 255, 300, 1, 1},
		{255, 255, 255, 0, 0, 1},
		{0, 0, 0, 0, 0, 0},
		{128, 128, 128, 0, 0, 128.0 / 255.0},
	}
	for _, c := range cases {
		h, s, v := RGBToHSV(c.r, c.g, c.b)
		if !approx(h, c.h, 0.01) || !approx(s, c.s, 0.001) || !approx(v, c.v, 0.001) {
			t.Errorf("RGBToHSV(%d,%d,%d) = (%g,%g,%g), want (%g,%g,%g)", c.r, c.g, c.b, h, s, v, c.h, c.s, c.v)
		}
	}
}

func TestHSVRoundTrip(t *testing.T) {
	for r := 0; r < 256; r += 17 {
		for g := 0; g < 256; g += 33 {
			for b := 0; b < 256; b += 51 {
				h, s, v := RGBToHSV(uint8(r), uint8(g), uint8(b))
				nr, ng, nb := HSVToRGB(h, s, v)
				if abs8(nr, uint8(r)) > 1 || abs8(ng, uint8(g)) > 1 || abs8(nb, uint8(b)) > 1 {
					t.Fatalf("HSV round trip (%d,%d,%d) -> (%d,%d,%d)", r, g, b, nr, ng, nb)
				}
			}
		}
	}
}

func TestHSVToRGBWrapsHue(t *testing.T) {
	r1, g1, b1 := HSVToRGB(0, 1, 1)
	r2, g2, b2 := HSVToRGB(360, 1, 1)
	r3, g3, b3 := HSVToRGB(-360, 1, 1)
	if r1 != r2 || g1 != g2 || b1 != b2 || r1 != r3 || g1 != g3 || b1 != b3 {
		t.Errorf("hue wrap mismatch: %v %v %v", []uint8{r1, g1, b1}, []uint8{r2, g2, b2}, []uint8{r3, g3, b3})
	}
}

func TestRGBToXYZWhite(t *testing.T) {
	x, y, z := RGBToXYZ(255, 255, 255)
	if !approx(x, 0.9505, 0.001) || !approx(y, 1.0, 0.001) || !approx(z, 1.089, 0.001) {
		t.Errorf("white XYZ = (%g,%g,%g)", x, y, z)
	}
	x, y, z = RGBToXYZ(0, 0, 0)
	if !approx(x, 0, 1e-9) || !approx(y, 0, 1e-9) || !approx(z, 0, 1e-9) {
		t.Errorf("black XYZ = (%g,%g,%g)", x, y, z)
	}
}

func TestXYZRoundTrip(t *testing.T) {
	for r := 0; r < 256; r += 25 {
		for g := 0; g < 256; g += 40 {
			for b := 0; b < 256; b += 60 {
				x, y, z := RGBToXYZ(uint8(r), uint8(g), uint8(b))
				nr, ng, nb := XYZToRGB(x, y, z)
				if abs8(nr, uint8(r)) > 1 || abs8(ng, uint8(g)) > 1 || abs8(nb, uint8(b)) > 1 {
					t.Fatalf("XYZ round trip (%d,%d,%d) -> (%d,%d,%d)", r, g, b, nr, ng, nb)
				}
			}
		}
	}
}

func TestRGBToLabKnown(t *testing.T) {
	l, a, b := RGBToLab(255, 255, 255)
	if !approx(l, 100, 0.01) || !approx(a, 0, 0.01) || !approx(b, 0, 0.01) {
		t.Errorf("white Lab = (%g,%g,%g)", l, a, b)
	}
	l, a, b = RGBToLab(0, 0, 0)
	if !approx(l, 0, 0.01) {
		t.Errorf("black L = %g", l)
	}
	// Reference sRGB red -> CIELAB (D65) ~ (53.24, 80.09, 67.20).
	l, a, b = RGBToLab(255, 0, 0)
	if !approx(l, 53.24, 0.05) || !approx(a, 80.09, 0.1) || !approx(b, 67.20, 0.1) {
		t.Errorf("red Lab = (%g,%g,%g)", l, a, b)
	}
}

func TestLabRoundTrip(t *testing.T) {
	for r := 0; r < 256; r += 25 {
		for g := 0; g < 256; g += 40 {
			for b := 0; b < 256; b += 60 {
				l, aa, bb := RGBToLab(uint8(r), uint8(g), uint8(b))
				nr, ng, nb := LabToRGB(l, aa, bb)
				if abs8(nr, uint8(r)) > 1 || abs8(ng, uint8(g)) > 1 || abs8(nb, uint8(b)) > 1 {
					t.Fatalf("Lab round trip (%d,%d,%d) -> (%d,%d,%d)", r, g, b, nr, ng, nb)
				}
			}
		}
	}
}

func TestDeltaE76(t *testing.T) {
	if d := DeltaE76(50, 10, 20, 50, 10, 20); d != 0 {
		t.Errorf("identical DeltaE76 = %g, want 0", d)
	}
	// 3-4-5 right triangle in a/b plane plus a dL of 0.
	if d := DeltaE76(50, 0, 0, 50, 3, 4); !approx(d, 5, 1e-9) {
		t.Errorf("DeltaE76 = %g, want 5", d)
	}
}

func abs8(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}
