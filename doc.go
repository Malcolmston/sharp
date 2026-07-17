// Package sharp is a high-level, fluent image-processing library for Go, built
// entirely on the standard library (image, image/color, image/png, image/jpeg,
// image/gif; BMP and uncompressed TIFF codecs are implemented in-package). It
// is inspired by the ergonomics of the Node.js "sharp" library: you build a
// pipeline, chain operations, and export to a format.
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
//   - Resize with Nearest, Bilinear, Cubic (Catmull-Rom), Mitchell or Lanczos3
//     sampling and FitExact/FitContain/FitCover.
//   - Affine (arbitrary 2x3 matrix, bilinear) and Trim (auto-crop borders).
//   - Crop / Extract a rectangular region, Extend (pad) with a fill colour.
//   - Rotate90/180/270 (exact) and Rotate (arbitrary angle, bilinear).
//   - FlipVertical/Flip and FlipHorizontal/Flop.
//
// Colour and tone:
//   - Grayscale, Negate/Invert, Tint, Brightness, Contrast, Gamma,
//     Saturation, Threshold.
//   - Normalise (contrast stretch), Linear (a*x+b per channel),
//     Modulate (brightness/saturation/hue/lightness) and CLAHE.
//
// Convolution and morphology:
//   - Blur (separable Gaussian), Sharpen, Unsharp (sigma-based unsharp mask)
//     and a generic Convolve(Kernel).
//   - Median, Erode, Dilate and a generic Morphology (open/close).
//
// Channels and bands:
//   - ExtractChannel, JoinChannels, RemoveAlpha, EnsureAlpha, Recomb (colour
//     matrix) and bitwise Boolean/Bandbool operations.
//
// Composition:
//   - Composite another image with alpha blending, gravity/offset placement and
//     a choice of blend modes (multiply, screen, overlay, darken, lighten, ...).
//   - Flatten onto a solid background colour, removing transparency.
//
// # Metadata and statistics
//
// Metadata reports the current width, height, detected source format, channel
// count, alpha presence, density and colour space. Stats computes per-channel
// means. Clone returns an independent copy of a pipeline so a partially-built
// pipeline can be branched.
//
// # Codecs
//
// Input decodes PNG, JPEG, GIF, BMP and uncompressed TIFF (the last two
// implemented in-package), plus raw pixel buffers via FromRaw. Output supports
// PNG, JPEG, GIF, BMP, TIFF and raw bytes. There is no standard-library codec
// for WebP or AVIF, so those formats are out of scope.
//
// # Output
//
// Terminal methods materialise the result:
//
//	img, err := p.ToImage()      // the image.Image
//	buf, err := p.ToPNG()        // PNG bytes
//	buf, err := p.ToJPEG(90)     // JPEG bytes at quality 90
//	buf, err := p.ToGIF()        // GIF bytes
//	buf, err := p.ToBMP()        // BMP bytes
//	buf, err := p.ToTIFF()       // TIFF bytes
//	buf, err := p.ToRaw()        // raw RGBA bytes
//	err := p.ToFile("out.png", sharp.FormatPNG, 0)
//
// # Determinism and colour handling
//
// Images are held as non-premultiplied 8-bit RGBA. Channel arithmetic is done
// in floating point and clamped to [0,255]. JPEG output has no alpha, so the
// encoder flattens transparency onto black; call Flatten first to control the
// background.
package sharp
