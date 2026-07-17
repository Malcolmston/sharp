package sharp

import "testing"

func benchResize(b *testing.B, interp Interpolation) {
	src := gradient(512, 512)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New(src).Resize(ResizeOptions{Width: 256, Height: 256, Interpolation: interp})
	}
}

// BenchmarkResizeBilinear measures the baseline separable bilinear resize.
func BenchmarkResizeBilinear(b *testing.B) { benchResize(b, Bilinear) }

// BenchmarkResizeCubic measures the Catmull-Rom cubic resize.
func BenchmarkResizeCubic(b *testing.B) { benchResize(b, Cubic) }

// BenchmarkResizeMitchell measures the Mitchell-Netravali resize.
func BenchmarkResizeMitchell(b *testing.B) { benchResize(b, Mitchell) }

// BenchmarkResizeLanczos3 measures the Lanczos3 resize.
func BenchmarkResizeLanczos3(b *testing.B) { benchResize(b, Lanczos3) }
