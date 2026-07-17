// Package sharp is a high-level, fluent image-processing library for Go, built
// entirely on the standard library (image, image/color, image/png,
// image/jpeg). It is inspired by the ergonomics of the Node.js "sharp" library:
// you build a pipeline, chain operations, and export to a format.
//
// # The pipeline
//
// A Pipeline wraps an in-memory RGBA image. Construct one from a decoded image,
// a file, a byte slice or an io.Reader:
//
//	p := sharp.New(img)                 // from an image.Image
//	p := sharp.FromFile("in.png")       // decode a file
//	p := sharp.FromBytes(data)          // decode PNG/JPEG bytes
//	p := sharp.FromReader(r)            // decode from a reader
//
// Operations are methods that return the same *Pipeline, so they chain. They
// are applied eagerly and mutate the pipeline's internal buffer (the source
// image passed to New is copied first and never modified). If any step fails,
// the error is stored and every later operation is skipped; the error surfaces
// from the terminal export call or from Err. All operations are deterministic:
// identical input always yields identical output.
//
//	out, err := sharp.FromFile("in.jpg").
//		Resize(sharp.ResizeOptions{Width: 800, Fit: sharp.FitContain}).
//		Grayscale().
//		Sharpen(1).
//		ToJPEG(85)
//
// # Operations
//
// Geometry:
//   - Resize with nearest or bilinear sampling and FitExact/FitContain/FitCover.
//   - Crop / Extract a rectangular region, Extend (pad) with a fill colour.
//   - Rotate90/180/270 (exact) and Rotate (arbitrary angle, bilinear).
//   - FlipVertical/Flip and FlipHorizontal/Flop.
//
// Colour:
//   - Grayscale, Negate/Invert, Tint, Brightness, Contrast, Gamma,
//     Saturation, Threshold.
//
// Convolution:
//   - Blur (separable Gaussian), Sharpen, and a generic Convolve(Kernel).
//
// Composition:
//   - Composite another image with alpha blending and gravity/offset placement.
//   - Flatten onto a solid background colour, removing transparency.
//
// # Metadata and statistics
//
// Metadata reports the current width, height and detected source format. Stats
// computes per-channel means. Clone returns an independent copy of a pipeline
// so a partially-built pipeline can be branched.
//
// # Output
//
// Terminal methods materialise the result:
//
//	img, err := p.ToImage()      // the image.Image
//	buf, err := p.ToPNG()        // PNG bytes
//	buf, err := p.ToJPEG(90)     // JPEG bytes at quality 90
//	err := p.ToFile("out.png", sharp.FormatPNG, 0)
//
// # Determinism and colour handling
//
// Images are held as non-premultiplied 8-bit RGBA. Channel arithmetic is done
// in floating point and clamped to [0,255]. JPEG output has no alpha, so the
// encoder flattens transparency onto black; call Flatten first to control the
// background.
package sharp
