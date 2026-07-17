# sharp

High-performance, fluent image processing for Go — inspired by the Node.js
[`sharp`](https://github.com/lovell/sharp) library, but written entirely with
the Go **standard library** (`image`, `image/color`, `image/png`,
`image/jpeg`, `image/gif`). No cgo, no third-party dependencies. BMP and
uncompressed TIFF codecs are implemented in-package.

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
sharp.FromBytes(data)     // decode PNG/JPEG/GIF/BMP/TIFF bytes
sharp.FromReader(r)       // decode from an io.Reader
sharp.FromRaw(pix, w, h, channels) // wrap raw pixel bytes (1/3/4 channels)
sharp.JoinChannels(r, g, b)        // combine single-band images
```

## Operations

**Geometry**

```go
p.Resize(sharp.ResizeOptions{Width: 300, Height: 200, Fit: sharp.FitCover, Interpolation: sharp.Lanczos3})
p.ResizeTo(300, 200)              // shorthand: bilinear, exact
p.Affine([6]float64{1, 0.2, 0, 0, 1, 0}, sharp.AffineOptions{Background: sharp.White})
p.Trim(10)                        // auto-crop uniform borders
p.Crop(sharp.Rectangle{X: 10, Y: 10, Width: 100, Height: 100})
p.Extract(rect)                   // alias of Crop
p.Extend(top, right, bottom, left, sharp.Black)  // pad
p.Rotate90(); p.Rotate180(); p.Rotate270()
p.Rotate(37.5, sharp.White)       // arbitrary angle, bilinear
p.FlipVertical(); p.FlipHorizontal()   // aka Flip / Flop
```

Fit modes: `FitExact` (stretch), `FitContain` (fit inside, keep ratio),
`FitCover` (cover box, keep ratio, centre-crop). Interpolation kernels:
`Nearest`, `Bilinear`, `Cubic` (Catmull-Rom), `Mitchell`, `Lanczos3`.

**Colour and tone**

```go
p.Grayscale()
p.Negate()          // aka Invert
p.Tint(sharp.RGBA{R: 128, B: 255, A: 255})
p.Brightness(1.2)
p.Contrast(1.5)
p.Gamma(2.2)
p.Saturation(0)     // 0 = grayscale, 1 = unchanged, >1 = boost
p.Threshold(128)
p.Normalise(sharp.NormaliseOptions{})              // contrast stretch
p.Linear([]float64{1.1}, []float64{-5})            // a*x + b per channel
p.Modulate(sharp.ModulateOptions{Brightness: 1.1, Saturation: 1.2, Hue: 30})
p.CLAHE(sharp.CLAHEOptions{Width: 32, Height: 32, MaxSlope: 3})
```

**Convolution and morphology**

```go
p.Blur(2.0)         // separable Gaussian, sigma in pixels
p.Sharpen(1.0)      // fixed 3x3 sharpen
p.Unsharp(sharp.UnsharpOptions{Sigma: 1.5, Amount: 1.0, Threshold: 0}) // unsharp mask
p.Median(1); p.Erode(1); p.Dilate(1)
p.Morphology(sharp.MorphOpen, 1)
k, _ := sharp.NewKernel([]float64{0, -1, 0, -1, 5, -1, 0, -1, 0}, 1, 0)
p.Convolve(k)
```

**Channels and bands**

```go
p.ExtractChannel(sharp.ChannelG)
p.RemoveAlpha(); p.EnsureAlpha(1.0)
p.Recomb([][]float64{{0.393, 0.769, 0.189}, {0.349, 0.686, 0.168}, {0.272, 0.534, 0.131}}) // sepia
p.Boolean(other, sharp.BoolAnd)
p.Bandbool(sharp.BoolOr)
```

**Composition**

```go
p.Composite(overlay, sharp.CompositeOptions{UseGravity: true, Gravity: sharp.GravityCenter, Opacity: 0.8})
p.Composite(overlay, sharp.CompositeOptions{Blend: sharp.BlendMultiply})
p.Flatten(sharp.White)   // remove transparency onto a background
```

Blend modes: `BlendOver` (default), `BlendMultiply`, `BlendScreen`,
`BlendOverlay`, `BlendDarken`, `BlendLighten`, `BlendColorDodge`,
`BlendColorBurn`, `BlendHardLight`, `BlendSoftLight`, `BlendDifference`,
`BlendExclusion`, `BlendAdd`.

## Metadata, stats, output

```go
md, _ := p.Metadata()   // Width, Height, Format, Channels, HasAlpha, Density, Space
st, _ := p.Stats()      // per-channel means

img, _ := p.ToImage()         // image.Image
buf, _ := p.ToPNG()           // PNG bytes
buf, _ := p.ToJPEG(90)        // JPEG bytes at quality 90
buf, _ := p.ToGIF()           // GIF bytes
buf, _ := p.ToBMP()           // BMP bytes
buf, _ := p.ToTIFF()          // uncompressed TIFF bytes
buf, _ := p.ToRaw()           // raw RGBA bytes
err   := p.ToFile("out.png", sharp.FormatPNG, 0)
```

Use `Clone()` to branch a partially-built pipeline into an independent copy.

## Format support

Read: PNG, JPEG, GIF, BMP, uncompressed TIFF, and raw pixel buffers. Write:
PNG, JPEG, GIF, BMP, uncompressed TIFF, and raw. WebP and AVIF are intentionally
out of scope — the Go standard library ships no codec for them and this project
takes no third-party dependencies.

## License

See repository.
