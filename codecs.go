package sharp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/gif"
	"os"
)

// defaultDensity is the pixel density (dots per inch) assumed when a source
// image carries no density metadata.
const defaultDensity = 72

// decodeImage decodes an encoded image, dispatching to the in-package BMP and
// TIFF decoders by magic number and otherwise to the standard library
// (PNG/JPEG/GIF). It returns the decoded image and the detected Format.
func decodeImage(data []byte) (image.Image, Format, error) {
	switch {
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		img, err := decodeBMP(data)
		return img, FormatBMP, err
	case isTIFF(data):
		img, err := decodeTIFF(data)
		return img, FormatTIFF, err
	default:
		img, format, err := image.Decode(bytes.NewReader(data))
		return img, normaliseFormat(format), err
	}
}

// ToGIF encodes the current image as a single-frame GIF. Colours are quantised
// to a 256-entry palette by the standard library encoder.
func (p *Pipeline) ToGIF() ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, p.img, &gif.Options{NumColors: 256}); err != nil {
		return nil, fmt.Errorf("sharp: encode gif: %w", err)
	}
	return buf.Bytes(), nil
}

// ToBMP encodes the current image as an uncompressed BMP. Images with any
// transparency are written as 32-bit BGRA; otherwise 24-bit BGR is used.
func (p *Pipeline) ToBMP() ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	return encodeBMP(p.img), nil
}

// ToTIFF encodes the current image as a baseline, uncompressed 8-bit RGBA TIFF
// (little-endian).
func (p *Pipeline) ToTIFF() ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	return encodeTIFF(p.img), nil
}

// ToRaw returns the raw, non-premultiplied 8-bit RGBA pixel bytes of the current
// image in row-major order (4 bytes per pixel). The returned slice is a copy.
func (p *Pipeline) ToRaw() ([]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	out := make([]byte, len(p.img.Pix))
	copy(out, p.img.Pix)
	return out, nil
}

// FromRaw builds a pipeline from raw pixel bytes. channels must be 1
// (grayscale), 3 (RGB) or 4 (RGBA); the data length must equal
// width*height*channels. Grayscale expands to equal RGB with opaque alpha and
// RGB gets opaque alpha.
func FromRaw(pix []byte, width, height, channels int) *Pipeline {
	if width <= 0 || height <= 0 {
		return &Pipeline{err: fmt.Errorf("sharp: FromRaw needs positive dimensions")}
	}
	if channels != 1 && channels != 3 && channels != 4 {
		return &Pipeline{err: fmt.Errorf("sharp: FromRaw channels must be 1, 3 or 4, got %d", channels)}
	}
	if len(pix) != width*height*channels {
		return &Pipeline{err: fmt.Errorf("sharp: FromRaw expected %d bytes, got %d", width*height*channels, len(pix))}
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < width*height; i++ {
		di := i * 4
		si := i * channels
		switch channels {
		case 1:
			v := pix[si]
			dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3] = v, v, v, 255
		case 3:
			dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3] = pix[si], pix[si+1], pix[si+2], 255
		case 4:
			dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3] = pix[si], pix[si+1], pix[si+2], pix[si+3]
		}
	}
	return &Pipeline{img: dst, format: FormatRaw, density: defaultDensity}
}

// imageHasAlpha reports whether img contains any non-opaque pixel.
func imageHasAlpha(img *image.RGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.PixOffset(b.Min.X, y)
		for x := 0; x < b.Dx(); x++ {
			if img.Pix[row+x*4+3] != 255 {
				return true
			}
		}
	}
	return false
}

// --- BMP ---------------------------------------------------------------------

// encodeBMP writes a bottom-up, uncompressed Windows BMP.
func encodeBMP(img *image.RGBA) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	bpp := 24
	if imageHasAlpha(img) {
		bpp = 32
	}
	rowSize := ((bpp*w + 31) / 32) * 4
	pixArray := rowSize * h
	const headerSize = 14 + 40
	out := make([]byte, headerSize+pixArray)

	// BITMAPFILEHEADER.
	out[0], out[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(out[2:], uint32(headerSize+pixArray))
	binary.LittleEndian.PutUint32(out[10:], headerSize)
	// BITMAPINFOHEADER.
	binary.LittleEndian.PutUint32(out[14:], 40)
	binary.LittleEndian.PutUint32(out[18:], uint32(w))
	binary.LittleEndian.PutUint32(out[22:], uint32(h))
	binary.LittleEndian.PutUint16(out[26:], 1)
	binary.LittleEndian.PutUint16(out[28:], uint16(bpp))
	binary.LittleEndian.PutUint32(out[30:], 0) // BI_RGB
	binary.LittleEndian.PutUint32(out[34:], uint32(pixArray))
	binary.LittleEndian.PutUint32(out[38:], 2835)
	binary.LittleEndian.PutUint32(out[42:], 2835)

	bytesPP := bpp / 8
	for y := 0; y < h; y++ {
		// Bottom-up: source row h-1-y goes to strip row y.
		srcY := h - 1 - y
		dstRow := headerSize + y*rowSize
		for x := 0; x < w; x++ {
			si := img.PixOffset(b.Min.X+x, b.Min.Y+srcY)
			di := dstRow + x*bytesPP
			out[di] = img.Pix[si+2]   // B
			out[di+1] = img.Pix[si+1] // G
			out[di+2] = img.Pix[si]   // R
			if bytesPP == 4 {
				out[di+3] = img.Pix[si+3] // A
			}
		}
	}
	return out
}

// decodeBMP decodes an uncompressed 24- or 32-bit BMP.
func decodeBMP(data []byte) (*image.RGBA, error) {
	if len(data) < 54 || data[0] != 'B' || data[1] != 'M' {
		return nil, fmt.Errorf("sharp: not a BMP")
	}
	dataOffset := binary.LittleEndian.Uint32(data[10:])
	headerSize := binary.LittleEndian.Uint32(data[14:])
	if headerSize < 40 {
		return nil, fmt.Errorf("sharp: unsupported BMP header size %d", headerSize)
	}
	width := int(int32(binary.LittleEndian.Uint32(data[18:])))
	rawHeight := int(int32(binary.LittleEndian.Uint32(data[22:])))
	bpp := binary.LittleEndian.Uint16(data[28:])
	compression := binary.LittleEndian.Uint32(data[30:])
	if compression != 0 {
		return nil, fmt.Errorf("sharp: only uncompressed BMP is supported (compression=%d)", compression)
	}
	if bpp != 24 && bpp != 32 {
		return nil, fmt.Errorf("sharp: only 24/32-bit BMP is supported (bpp=%d)", bpp)
	}
	topDown := rawHeight < 0
	height := rawHeight
	if topDown {
		height = -height
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("sharp: invalid BMP dimensions %dx%d", width, height)
	}
	bytesPP := int(bpp) / 8
	rowSize := ((int(bpp)*width + 31) / 32) * 4
	if int(dataOffset)+rowSize*height > len(data) {
		return nil, fmt.Errorf("sharp: BMP pixel data truncated")
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		var srcRowIndex int
		if topDown {
			srcRowIndex = y
		} else {
			srcRowIndex = height - 1 - y
		}
		srcRow := int(dataOffset) + srcRowIndex*rowSize
		for x := 0; x < width; x++ {
			si := srcRow + x*bytesPP
			di := dst.PixOffset(x, y)
			dst.Pix[di] = data[si+2]   // R
			dst.Pix[di+1] = data[si+1] // G
			dst.Pix[di+2] = data[si]   // B
			if bytesPP == 4 {
				dst.Pix[di+3] = data[si+3]
			} else {
				dst.Pix[di+3] = 255
			}
		}
	}
	return dst, nil
}

// --- TIFF --------------------------------------------------------------------

// isTIFF reports whether data begins with a little- or big-endian TIFF header.
func isTIFF(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return (data[0] == 'I' && data[1] == 'I' && data[2] == 0x2A && data[3] == 0x00) ||
		(data[0] == 'M' && data[1] == 'M' && data[2] == 0x00 && data[3] == 0x2A)
}

// TIFF tag identifiers used by the encoder/decoder.
const (
	tImageWidth      = 256
	tImageLength     = 257
	tBitsPerSample   = 258
	tCompression     = 259
	tPhotometric     = 262
	tStripOffsets    = 273
	tSamplesPerPixel = 277
	tRowsPerStrip    = 278
	tStripByteCounts = 279
	tExtraSamples    = 338
)

// encodeTIFF writes a baseline, uncompressed 8-bit RGBA little-endian TIFF.
func encodeTIFF(img *image.RGBA) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	le := binary.LittleEndian

	dataOffset := 8
	dataLen := w * h * 4
	ifdOffset := dataOffset + dataLen
	const numEntries = 10
	ifdSize := 2 + numEntries*12 + 4
	bpsOffset := ifdOffset + ifdSize

	total := bpsOffset + 8 // 4 SHORTs for BitsPerSample
	out := make([]byte, total)

	// Header.
	out[0], out[1] = 'I', 'I'
	le.PutUint16(out[2:], 42)
	le.PutUint32(out[4:], uint32(ifdOffset))

	// Pixel data (row-major RGBA).
	di := dataOffset
	for y := 0; y < h; y++ {
		si := img.PixOffset(b.Min.X, b.Min.Y+y)
		copy(out[di:di+w*4], img.Pix[si:si+w*4])
		di += w * 4
	}

	// IFD.
	le.PutUint16(out[ifdOffset:], numEntries)
	entry := ifdOffset + 2
	putEntry := func(tag, typ uint16, count uint32, value uint32) {
		le.PutUint16(out[entry:], tag)
		le.PutUint16(out[entry+2:], typ)
		le.PutUint32(out[entry+4:], count)
		le.PutUint32(out[entry+8:], value)
		entry += 12
	}
	const (
		typeShort = 3
		typeLong  = 4
	)
	// Tags must be written in ascending order.
	putEntry(tImageWidth, typeLong, 1, uint32(w))
	putEntry(tImageLength, typeLong, 1, uint32(h))
	putEntry(tBitsPerSample, typeShort, 4, uint32(bpsOffset))
	putEntry(tCompression, typeShort, 1, 1)
	putEntry(tPhotometric, typeShort, 1, 2) // RGB
	putEntry(tStripOffsets, typeLong, 1, uint32(dataOffset))
	putEntry(tSamplesPerPixel, typeShort, 1, 4)
	putEntry(tRowsPerStrip, typeLong, 1, uint32(h))
	putEntry(tStripByteCounts, typeLong, 1, uint32(dataLen))
	putEntry(tExtraSamples, typeShort, 1, 2) // unassociated alpha
	le.PutUint32(out[entry:], 0)             // next IFD

	// BitsPerSample array [8,8,8,8].
	for i := 0; i < 4; i++ {
		le.PutUint16(out[bpsOffset+i*2:], 8)
	}
	return out
}

// decodeTIFF decodes an uncompressed 8-bit baseline TIFF (grayscale, RGB or
// RGBA), single- or multi-strip.
func decodeTIFF(data []byte) (*image.RGBA, error) {
	if !isTIFF(data) {
		return nil, fmt.Errorf("sharp: not a TIFF")
	}
	var bo binary.ByteOrder
	if data[0] == 'I' {
		bo = binary.LittleEndian
	} else {
		bo = binary.BigEndian
	}
	ifdOffset := bo.Uint32(data[4:])
	if int(ifdOffset)+2 > len(data) {
		return nil, fmt.Errorf("sharp: TIFF IFD out of range")
	}
	n := int(bo.Uint16(data[ifdOffset:]))
	tags := map[uint16][]uint32{}
	for i := 0; i < n; i++ {
		off := int(ifdOffset) + 2 + i*12
		if off+12 > len(data) {
			return nil, fmt.Errorf("sharp: TIFF IFD entry out of range")
		}
		tag := bo.Uint16(data[off:])
		typ := bo.Uint16(data[off+2:])
		count := bo.Uint32(data[off+4:])
		vals, err := tiffValues(data, bo, typ, count, off+8)
		if err != nil {
			return nil, err
		}
		tags[tag] = vals
	}

	first := func(tag uint16, def uint32) uint32 {
		if v, ok := tags[tag]; ok && len(v) > 0 {
			return v[0]
		}
		return def
	}
	if c := first(tCompression, 1); c != 1 {
		return nil, fmt.Errorf("sharp: only uncompressed TIFF is supported (compression=%d)", c)
	}
	width := int(first(tImageWidth, 0))
	height := int(first(tImageLength, 0))
	samples := int(first(tSamplesPerPixel, 1))
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("sharp: invalid TIFF dimensions %dx%d", width, height)
	}
	if bps, ok := tags[tBitsPerSample]; ok {
		for _, v := range bps {
			if v != 8 {
				return nil, fmt.Errorf("sharp: only 8-bit TIFF is supported")
			}
		}
	}
	offsets := tags[tStripOffsets]
	counts := tags[tStripByteCounts]
	if len(offsets) == 0 {
		return nil, fmt.Errorf("sharp: TIFF missing strip offsets")
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	rowBytes := width * samples
	px := 0 // running pixel index across strips
	for s, so := range offsets {
		var cb int
		if s < len(counts) {
			cb = int(counts[s])
		} else {
			cb = rowBytes * height
		}
		if int(so)+cb > len(data) {
			cb = len(data) - int(so)
		}
		strip := data[so : int(so)+cb]
		for off := 0; off+samples <= len(strip); off += samples {
			if px >= width*height {
				break
			}
			di := px * 4
			switch samples {
			case 1:
				v := strip[off]
				dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3] = v, v, v, 255
			case 3:
				dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3] = strip[off], strip[off+1], strip[off+2], 255
			default: // 4 or more; take first four
				dst.Pix[di], dst.Pix[di+1], dst.Pix[di+2], dst.Pix[di+3] = strip[off], strip[off+1], strip[off+2], strip[off+3]
			}
			px++
		}
	}
	return dst, nil
}

// tiffValues reads count values of the given TIFF field type. Only SHORT and
// LONG (the types this package emits and needs) are supported; values that do
// not fit inline are read from the offset they point to.
func tiffValues(data []byte, bo binary.ByteOrder, typ uint16, count uint32, fieldOff int) ([]uint32, error) {
	var size int
	switch typ {
	case 3: // SHORT
		size = 2
	case 4: // LONG
		size = 4
	default:
		// Unsupported types are returned empty; callers use defaults.
		return nil, nil
	}
	total := int(count) * size
	base := fieldOff
	if total > 4 {
		base = int(bo.Uint32(data[fieldOff:]))
	}
	if base+total > len(data) {
		return nil, fmt.Errorf("sharp: TIFF value out of range")
	}
	out := make([]uint32, count)
	for i := 0; i < int(count); i++ {
		if size == 2 {
			out[i] = uint32(bo.Uint16(data[base+i*2:]))
		} else {
			out[i] = bo.Uint32(data[base+i*4:])
		}
	}
	return out, nil
}

// writeCodecFile is a shared helper that writes encoded bytes to path.
func writeCodecFile(path string, data []byte, err error) error {
	if err != nil {
		return err
	}
	if werr := os.WriteFile(path, data, 0o644); werr != nil {
		return fmt.Errorf("sharp: write %q: %w", path, werr)
	}
	return nil
}
