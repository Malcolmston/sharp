# sharp

High-performance, fluent image processing for Go — inspired by the Node.js
[`sharp`](https://github.com/lovell/sharp) library, but written entirely with
the Go **standard library** (`image`, `image/color`, `image/png`,
`image/jpeg`). No cgo, no third-party dependencies.

## Install

```sh
go get github.com/malcolmston/sharp
```

Requires Go 1.24 or newer.

## Quick start

Build a pipeline, chain operations, export to a format:

```go
package main

import (
	"log"

	"github.com/malcolmston/sharp"
)

func main() {
	err := sharp.FromFile("input.jpg").
		Resize(sharp.ResizeOptions{Width: 800, Fit: sharp.FitContain}).
		Grayscale().
		Sharpen(1).
		ToFile("output.png", sharp.FormatPNG, 0)
	if err != nil {
		log.Fatal(err)
	}
}
```

Operations are applied eagerly and mutate the pipeline's internal buffer (the
image you pass to `New` is copied first, never modified). If any step fails, the
error is retained and later steps become no-ops; it surfaces from the terminal
export call or from `Err()`. Everything is deterministic.

## Constructors

```go
sharp.New(img)            // from a decoded image.Image
sharp.FromFile("in.png")  // decode a file
sharp.FromBytes(data)     // decode PNG/JPEG bytes
sharp.FromReader(r)       // decode from an io.Reader
```

## Operations

**Geometry**

```go
p.Resize(sharp.ResizeOptions{Width: 300, Height: 200, Fit: sharp.FitCover, Interpolation: sharp.Bilinear})
p.ResizeTo(300, 200)              // shorthand: bilinear, exact
p.Crop(sharp.Rectangle{X: 10, Y: 10, Width: 100, Height: 100})
p.Extract(rect)                   // alias of Crop
p.Extend(top, right, bottom, left, sharp.Black)  // pad
p.Rotate90(); p.Rotate180(); p.Rotate270()
p.Rotate(37.5, sharp.White)       // arbitrary angle, bilinear
p.FlipVertical(); p.FlipHorizontal()   // aka Flip / Flop
```

Fit modes: `FitExact` (stretch), `FitContain` (fit inside, keep ratio),
`FitCover` (cover box, keep ratio, centre-crop). Interpolation: `Nearest` or
`Bilinear`.

**Colour**

```go
p.Grayscale()
p.Negate()          // aka Invert
p.Tint(sharp.RGBA{R: 128, B: 255, A: 255})
p.Brightness(1.2)
p.Contrast(1.5)
p.Gamma(2.2)
p.Saturation(0)     // 0 = grayscale, 1 = unchanged, >1 = boost
p.Threshold(128)
```

**Convolution**

```go
p.Blur(2.0)         // separable Gaussian, sigma in pixels
p.Sharpen(1.0)
k, _ := sharp.NewKernel([]float64{0, -1, 0, -1, 5, -1, 0, -1, 0}, 1, 0)
p.Convolve(k)
```

**Composition**

```go
p.Composite(overlay, sharp.CompositeOptions{UseGravity: true, Gravity: sharp.GravityCenter, Opacity: 0.8})
p.Flatten(sharp.White)   // remove transparency onto a background
```

## Metadata, stats, output

```go
md, _ := p.Metadata()   // Width, Height, Format
st, _ := p.Stats()      // per-channel means

img, _ := p.ToImage()         // image.Image
buf, _ := p.ToPNG()           // PNG bytes
buf, _ := p.ToJPEG(90)        // JPEG bytes at quality 90
err   := p.ToFile("out.png", sharp.FormatPNG, 0)
```

Use `Clone()` to branch a partially-built pipeline into an independent copy.

## License

See repository.
