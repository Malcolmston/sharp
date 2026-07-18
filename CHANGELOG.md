# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project adheres to
semantic versioning.

## [0.3.0] - 2026-07-18

Further parity push toward the Node.js `sharp` feature set, still standard-library
only (no cgo, no third-party dependencies).

### Added

- **Statistics** (parity with `sharp.stats()`): `ChannelStats` (per-band min,
  max, mean, standard deviation and sum via the new `ChannelStat` type),
  `Histogram` (256-bin R/G/B/luminance counts via the new `Histogram` type),
  `Entropy` (Shannon entropy of luminance), `Sharpness` (mean absolute
  Laplacian), `IsOpaque`, `DominantColor`/`DominantColour` and
  `MeanColor`/`MeanColour`.
- **Colour-space conversions**: `RGBToHSV`/`HSVToRGB`, `RGBToXYZ`/`XYZToRGB`,
  `RGBToLab`/`LabToRGB` (CIELAB under D65) and `DeltaE76` (CIE76 colour
  difference), plus the exported `Luma` helper.
- **Extend modes** (parity with `extendWith`): `ExtendWith` with the new
  `ExtendOptions`/`ExtendMode` types, supporting background, edge-copy, repeat
  (tile) and mirror border fills.
- **Operations**: `Unflatten` (white → transparent), `Sepia`,
  `ToColourspace`/`ToColorspace` (`b-w`/`srgb`), `WithDensity`, `ResizeWidth`,
  `ResizeHeight` and `ToFormat` (encode to any supported format by tag).
- Benchmarks for `ChannelStats` and `Entropy`.

## [0.2.0] - 2026-07-17

Large parity push toward the Node.js `sharp` feature set, still standard-library
only (no cgo, no third-party dependencies).

### Added

- **High-quality resize kernels**: `Cubic` (Catmull-Rom), `Mitchell` and
  `Lanczos3`, selectable via `ResizeOptions.Interpolation`, implemented as
  separable resamplers with proper anti-aliasing on downscale.
- **Geometry**: `Affine` (arbitrary 2x3 matrix, bilinear sampled) and `Trim`
  (auto-crop uniform borders with a threshold).
- **Tone / histogram**: `Normalise`/`Normalize` (contrast stretch by luminance
  percentile), `Linear` (per-channel `a*x + b`), `Modulate`
  (brightness/saturation/hue/lightness in HSL) and `CLAHE` (contrast-limited
  adaptive histogram equalisation with tile interpolation).
- **Spatial / morphology**: `Median`, `Erode`, `Dilate`, generic `Morphology`
  (erode/dilate/open/close) and `Unsharp` (sigma-based unsharp mask).
- **Channels / bands**: `ExtractChannel`, `JoinChannels`, `RemoveAlpha`,
  `EnsureAlpha`, `Recomb` (3x3/4x4 colour matrix) and bitwise `Boolean` /
  `Bandbool`.
- **Codecs**: GIF read/write (via `image/gif`), in-package BMP read/write and
  uncompressed TIFF read/write, plus raw pixel `FromRaw` / `ToRaw`.
- **Compositing**: blend modes on `Composite` via `CompositeOptions.Blend`
  (over, multiply, screen, overlay, darken, lighten, colour-dodge, colour-burn,
  hard-light, soft-light, difference, exclusion, add).
- **Metadata**: expanded to report `Channels`, `HasAlpha`, `Density` and
  `Space` alongside width/height/format.
- Benchmarks for the resize kernels.

### Notes

- WebP and AVIF remain unsupported: the Go standard library provides no codec
  for them and this project takes no third-party dependencies.

## [0.1.0]

- Initial pipeline: constructors, resize (nearest/bilinear), crop/extract,
  extend, flips, rotations, colour adjustments, Gaussian blur, generic
  convolution, composite and flatten, with PNG/JPEG I/O.
