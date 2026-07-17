package sharp_test

import (
	"fmt"
	"image"
	"image/color"

	"github.com/malcolmston/sharp"
)

// Example builds a small gradient image, resizes it, and prints the resulting
// bounds and format.
func Example() {
	src := image.NewRGBA(image.Rect(0, 0, 100, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 100; x++ {
			src.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}

	out, err := sharp.New(src).
		ResizeTo(50, 30).
		Grayscale().
		ToImage()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("bounds=%v\n", out.Bounds())
	// Output:
	// bounds=(0,0)-(50,30)
}
