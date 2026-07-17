package sharp

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
)

// Format identifies an encoded image format.
type Format string

// Supported image formats.
const (
	FormatUnknown Format = ""
	FormatPNG     Format = "png"
	FormatJPEG    Format = "jpeg"
)

// Pipeline is a fluent, chainable image-processing pipeline. Construct one with
// New, FromFile or FromBytes, chain any number of operations, then materialise
// the result with ToImage, ToPNG, ToJPEG or ToFile.
//
// Operations are applied eagerly and mutate the pipeline's internal RGBA
// buffer. If any step fails, the error is retained and every subsequent
// operation becomes a no-op; the error surfaces from the terminal call or from
// Err. All operations are deterministic.
type Pipeline struct {
	img    *image.RGBA
	format Format
	err    error
}

// New creates a pipeline from an already-decoded image. The source pixels are
// copied into an internal RGBA buffer, so the caller's image is never mutated.
func New(img image.Image) *Pipeline {
	if img == nil {
		return &Pipeline{err: fmt.Errorf("sharp: New called with nil image")}
	}
	return &Pipeline{img: toRGBA(img), format: FormatUnknown}
}

// FromBytes decodes an encoded image (PNG or JPEG) from a byte slice and
// returns a pipeline seeded with it. The detected format is recorded and
// reported by Metadata.
func FromBytes(data []byte) *Pipeline {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return &Pipeline{err: fmt.Errorf("sharp: decode: %w", err)}
	}
	return &Pipeline{img: toRGBA(img), format: normaliseFormat(format)}
}

// FromReader decodes an encoded image from r.
func FromReader(r io.Reader) *Pipeline {
	img, format, err := image.Decode(r)
	if err != nil {
		return &Pipeline{err: fmt.Errorf("sharp: decode: %w", err)}
	}
	return &Pipeline{img: toRGBA(img), format: normaliseFormat(format)}
}

// FromFile reads and decodes an image file from disk.
func FromFile(path string) *Pipeline {
	f, err := os.Open(path)
	if err != nil {
		return &Pipeline{err: fmt.Errorf("sharp: open %q: %w", path, err)}
	}
	defer func() { _ = f.Close() }()
	return FromReader(f)
}

// Err returns the first error that occurred in the pipeline, if any.
func (p *Pipeline) Err() error { return p.err }

// Clone returns an independent deep copy of the pipeline, including any pending
// error state. Mutating the clone never affects the original.
func (p *Pipeline) Clone() *Pipeline {
	if p.err != nil {
		return &Pipeline{err: p.err, format: p.format}
	}
	dst := image.NewRGBA(p.img.Bounds())
	copy(dst.Pix, p.img.Pix)
	return &Pipeline{img: dst, format: p.format}
}

// Metadata describes the current image held by a pipeline.
type Metadata struct {
	Width  int
	Height int
	Format Format
}

// Metadata returns the dimensions and source format of the current image.
func (p *Pipeline) Metadata() (Metadata, error) {
	if p.err != nil {
		return Metadata{}, p.err
	}
	b := p.img.Bounds()
	return Metadata{Width: b.Dx(), Height: b.Dy(), Format: p.format}, nil
}

// Stats holds simple per-channel statistics for an image, with each mean in the
// range [0,255].
type Stats struct {
	MeanR float64
	MeanG float64
	MeanB float64
	MeanA float64
}

// Stats computes the mean of each channel across all pixels.
func (p *Pipeline) Stats() (Stats, error) {
	if p.err != nil {
		return Stats{}, p.err
	}
	b := p.img.Bounds()
	n := float64(b.Dx() * b.Dy())
	if n == 0 {
		return Stats{}, nil
	}
	var sr, sg, sb, sa float64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			sr += float64(p.img.Pix[i])
			sg += float64(p.img.Pix[i+1])
			sb += float64(p.img.Pix[i+2])
			sa += float64(p.img.Pix[i+3])
		}
	}
	return Stats{MeanR: sr / n, MeanG: sg / n, MeanB: sb / n, MeanA: sa / n}, nil
}

// ToImage returns the resulting image, or the pipeline's error.
func (p *Pipeline) ToImage() (image.Image, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.img, nil
}

// PNGOptions configures PNG encoding.
type PNGOptions struct {
	// Compression selects the PNG compression level. The zero value uses the
	// encoder default.
	Compression png.CompressionLevel
}

// ToPNG encodes the current image as PNG and returns the bytes.
func (p *Pipeline) ToPNG() ([]byte, error) {
	return p.ToPNGWithOptions(PNGOptions{})
}

// ToPNGWithOptions encodes the current image as PNG using the supplied options.
func (p *Pipeline) ToPNGWithOptions(opts PNGOptions) ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	enc := png.Encoder{CompressionLevel: opts.Compression}
	var buf bytes.Buffer
	if err := enc.Encode(&buf, p.img); err != nil {
		return nil, fmt.Errorf("sharp: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// JPEGOptions configures JPEG encoding.
type JPEGOptions struct {
	// Quality is the JPEG quality in the range [1,100]. Values outside the
	// range are clamped; the zero value maps to the encoder default of 75.
	Quality int
}

// ToJPEG encodes the current image as JPEG at the given quality (1-100). A
// quality of 0 selects the default (75).
func (p *Pipeline) ToJPEG(quality int) ([]byte, error) {
	return p.ToJPEGWithOptions(JPEGOptions{Quality: quality})
}

// ToJPEGWithOptions encodes the current image as JPEG using the supplied
// options. JPEG has no alpha channel, so the image is flattened onto black.
func (p *Pipeline) ToJPEGWithOptions(opts JPEGOptions) ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	q := opts.Quality
	switch {
	case q <= 0:
		q = jpeg.DefaultQuality
	case q > 100:
		q = 100
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, p.img, &jpeg.Options{Quality: q}); err != nil {
		return nil, fmt.Errorf("sharp: encode jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// ToFile encodes the image and writes it to path. The format is chosen from the
// requested Format; FormatUnknown falls back to the pipeline's source format,
// then to PNG. For JPEG, quality 0 selects the default.
func (p *Pipeline) ToFile(path string, format Format, quality int) error {
	if p.err != nil {
		return p.err
	}
	if format == FormatUnknown {
		format = p.format
	}
	var (
		data []byte
		err  error
	)
	switch format {
	case FormatJPEG:
		data, err = p.ToJPEG(quality)
	case FormatPNG, FormatUnknown:
		data, err = p.ToPNG()
	default:
		return fmt.Errorf("sharp: unsupported format %q", format)
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("sharp: write %q: %w", path, err)
	}
	return nil
}

// toRGBA returns img as an *image.RGBA, copying pixels. If img is already an
// *image.RGBA a fresh copy is still returned so callers never share buffers.
func toRGBA(img image.Image) *image.RGBA {
	b := img.Bounds()
	// Normalise the origin to (0,0) for predictable coordinates downstream.
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	if src, ok := img.(*image.RGBA); ok && b.Min.X == 0 && b.Min.Y == 0 {
		copy(dst.Pix, src.Pix)
		return dst
	}
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			i := dst.PixOffset(x, y)
			dst.Pix[i] = uint8(r >> 8)
			dst.Pix[i+1] = uint8(g >> 8)
			dst.Pix[i+2] = uint8(bl >> 8)
			dst.Pix[i+3] = uint8(a >> 8)
		}
	}
	return dst
}

func normaliseFormat(f string) Format {
	switch f {
	case "png":
		return FormatPNG
	case "jpeg":
		return FormatJPEG
	default:
		return FormatUnknown
	}
}

// clamp8 clamps an integer to a uint8.
func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// clampF clamps a float to [0,255] and rounds to a uint8.
func clampF(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}
