package sharp

import (
	"fmt"
	"strings"
)

// Luma returns the rounded Rec. 601 luminance of an 8-bit sRGB triple, using
// the weights 0.299R + 0.587G + 0.114B. It is the same luma the library uses
// internally for Grayscale and Threshold, exposed for callers that need it.
func Luma(r, g, b uint8) uint8 {
	return uint8(luma(r, g, b) + 0.5)
}

// Sepia applies the classic sepia-tone colour matrix to the image, warming it
// into shades of brown. It is a convenience wrapper around the same transform
// available through Recomb. Alpha is preserved.
func (p *Pipeline) Sepia() *Pipeline {
	if p.err != nil {
		return p
	}
	p.eachPixel(func(r, g, b uint8) (uint8, uint8, uint8) {
		fr, fg, fb := float64(r), float64(g), float64(b)
		nr := clampF(0.393*fr + 0.769*fg + 0.189*fb)
		ng := clampF(0.349*fr + 0.686*fg + 0.168*fb)
		nb := clampF(0.272*fr + 0.534*fg + 0.131*fb)
		return nr, ng, nb
	})
	return p
}

// Unflatten makes fully white pixels transparent by setting the alpha of every
// pixel whose red, green and blue channels are all 255 to zero. It is the
// inverse-in-spirit of Flatten and mirrors the unflatten operation of the Node
// sharp library, useful for turning a white background into transparency.
func (p *Pipeline) Unflatten() *Pipeline {
	if p.err != nil {
		return p
	}
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := p.img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			i := row + x*4
			if p.img.Pix[i] == 255 && p.img.Pix[i+1] == 255 && p.img.Pix[i+2] == 255 {
				p.img.Pix[i+3] = 0
			}
		}
	}
	return p
}

// ToColourspace converts the working image into the requested colour space.
// Recognised values are "b-w", "bw", "grey", "gray", "greyscale" and
// "grayscale" (all of which convert to greyscale) and "srgb", "rgb" and "rgb8"
// (which leave the sRGB image unchanged). Any other value sets an error on the
// pipeline. It corresponds to the toColourspace method of the Node sharp
// library.
func (p *Pipeline) ToColourspace(space string) *Pipeline {
	if p.err != nil {
		return p
	}
	switch strings.ToLower(strings.TrimSpace(space)) {
	case "b-w", "bw", "grey", "gray", "greyscale", "grayscale":
		return p.Grayscale()
	case "srgb", "rgb", "rgb8":
		return p
	default:
		p.err = fmt.Errorf("sharp: unsupported colourspace %q", space)
		return p
	}
}

// ToColorspace is a US-spelling alias for ToColourspace.
func (p *Pipeline) ToColorspace(space string) *Pipeline { return p.ToColourspace(space) }

// WithDensity records a pixel density in dots per inch on the pipeline, which is
// reported back through Metadata. Non-positive values are ignored. It mirrors
// setting density via the Node sharp library's withMetadata call and does not
// alter any pixels.
func (p *Pipeline) WithDensity(dpi int) *Pipeline {
	if p.err != nil {
		return p
	}
	if dpi > 0 {
		p.density = dpi
	}
	return p
}

// ResizeWidth scales the image to the given width in pixels, deriving the height
// from the source aspect ratio with bilinear sampling. It is a shorthand for a
// Resize with only a width supplied.
func (p *Pipeline) ResizeWidth(width int) *Pipeline {
	return p.Resize(ResizeOptions{Width: width, Fit: FitExact, Interpolation: Bilinear})
}

// ResizeHeight scales the image to the given height in pixels, deriving the
// width from the source aspect ratio with bilinear sampling. It is a shorthand
// for a Resize with only a height supplied.
func (p *Pipeline) ResizeHeight(height int) *Pipeline {
	return p.Resize(ResizeOptions{Height: height, Fit: FitExact, Interpolation: Bilinear})
}

// ToFormat encodes the current image to the requested format and returns the
// bytes, dispatching to the matching ToPNG/ToJPEG/ToGIF/ToBMP/ToTIFF/ToRaw
// method. For JPEG, quality selects the compression level (0 uses the default);
// it is ignored for the other formats. FormatUnknown falls back to the
// pipeline's source format, then to PNG.
func (p *Pipeline) ToFormat(format Format, quality int) ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	if format == FormatUnknown {
		format = p.format
	}
	switch format {
	case FormatJPEG:
		return p.ToJPEG(quality)
	case FormatGIF:
		return p.ToGIF()
	case FormatBMP:
		return p.ToBMP()
	case FormatTIFF:
		return p.ToTIFF()
	case FormatRaw:
		return p.ToRaw()
	case FormatPNG, FormatUnknown:
		return p.ToPNG()
	default:
		return nil, fmt.Errorf("sharp: unsupported format %q", format)
	}
}
