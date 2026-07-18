package sharp

import "math"

// RGBToHSV converts an 8-bit sRGB triple to Hue/Saturation/Value. Hue is
// returned in degrees in the range [0,360), while saturation and value are in
// [0,1]. It is the inverse of HSVToRGB. Grey inputs return a hue of 0.
func RGBToHSV(r, g, b uint8) (h, s, v float64) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	v = max
	d := max - min
	if max > 0 {
		s = d / max
	}
	if d == 0 {
		return 0, s, v
	}
	switch max {
	case rf:
		h = math.Mod((gf-bf)/d, 6)
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, v
}

// HSVToRGB converts a Hue/Saturation/Value colour to an 8-bit sRGB triple. Hue
// is taken in degrees (values outside [0,360) are wrapped) and saturation and
// value are clamped to [0,1]. It is the inverse of RGBToHSV.
func HSVToRGB(h, s, v float64) (r, g, b uint8) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	s = clamp01(s)
	v = clamp01(v)
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var rf, gf, bf float64
	switch {
	case h < 60:
		rf, gf, bf = c, x, 0
	case h < 120:
		rf, gf, bf = x, c, 0
	case h < 180:
		rf, gf, bf = 0, c, x
	case h < 240:
		rf, gf, bf = 0, x, c
	case h < 300:
		rf, gf, bf = x, 0, c
	default:
		rf, gf, bf = c, 0, x
	}
	return clampF((rf + m) * 255), clampF((gf + m) * 255), clampF((bf + m) * 255)
}

// srgbToLinear expands a gamma-encoded sRGB component in [0,1] to linear light.
func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// linearToSRGB gamma-encodes a linear-light component in [0,1] back to sRGB.
func linearToSRGB(c float64) float64 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

// RGBToXYZ converts an 8-bit sRGB triple to CIE 1931 XYZ under the D65 white
// point, using the standard sRGB gamma expansion. Y is normalised so that pure
// white (255,255,255) maps to approximately (0.9505, 1.0000, 1.0890). It is the
// inverse of XYZToRGB.
func RGBToXYZ(r, g, b uint8) (x, y, z float64) {
	rl := srgbToLinear(float64(r) / 255)
	gl := srgbToLinear(float64(g) / 255)
	bl := srgbToLinear(float64(b) / 255)
	x = 0.4124564*rl + 0.3575761*gl + 0.1804375*bl
	y = 0.2126729*rl + 0.7151522*gl + 0.0721750*bl
	z = 0.0193339*rl + 0.1191920*gl + 0.9503041*bl
	return x, y, z
}

// XYZToRGB converts a CIE 1931 XYZ colour (D65 white point) to an 8-bit sRGB
// triple, applying the sRGB gamma and clamping to the display gamut. It is the
// inverse of RGBToXYZ.
func XYZToRGB(x, y, z float64) (r, g, b uint8) {
	rl := 3.2404542*x - 1.5371385*y - 0.4985314*z
	gl := -0.9692660*x + 1.8760108*y + 0.0415560*z
	bl := 0.0556434*x - 0.2040259*y + 1.0572252*z
	rf := linearToSRGB(clamp01(rl))
	gf := linearToSRGB(clamp01(gl))
	bf := linearToSRGB(clamp01(bl))
	return clampF(rf * 255), clampF(gf * 255), clampF(bf * 255)
}

// D65 reference white in the same normalisation as RGBToXYZ.
const (
	labXn = 0.95047
	labYn = 1.00000
	labZn = 1.08883
)

// labF is the CIELAB nonlinearity applied to a normalised tristimulus ratio.
func labF(t float64) float64 {
	if t > 0.008856451679035631 { // (6/29)^3
		return math.Cbrt(t)
	}
	return 7.787037037037037*t + 16.0/116.0
}

// labFInv inverts labF.
func labFInv(t float64) float64 {
	c := t * t * t
	if c > 0.008856451679035631 {
		return c
	}
	return (t - 16.0/116.0) / 7.787037037037037
}

// RGBToLab converts an 8-bit sRGB triple to CIELAB (L*a*b*) under the D65 white
// point. L is in [0,100]; a and b are unbounded but typically within roughly
// [-128,127]. Pure white maps to L=100, a=0, b=0 and pure black to L=0. It is
// the inverse of LabToRGB.
func RGBToLab(r, g, b uint8) (l, a, bb float64) {
	x, y, z := RGBToXYZ(r, g, b)
	fx := labF(x / labXn)
	fy := labF(y / labYn)
	fz := labF(z / labZn)
	l = 116*fy - 16
	a = 500 * (fx - fy)
	bb = 200 * (fy - fz)
	return l, a, bb
}

// LabToRGB converts a CIELAB (L*a*b*) colour under the D65 white point to an
// 8-bit sRGB triple, clamping to the display gamut. It is the inverse of
// RGBToLab.
func LabToRGB(l, a, bb float64) (r, g, b uint8) {
	fy := (l + 16) / 116
	fx := fy + a/500
	fz := fy - bb/200
	x := labXn * labFInv(fx)
	y := labYn * labFInv(fy)
	z := labZn * labFInv(fz)
	return XYZToRGB(x, y, z)
}

// DeltaE76 returns the CIE76 colour difference (Euclidean distance in CIELAB)
// between two L*a*b* colours. A value of 0 means the colours are identical;
// values around 2.3 correspond to a just-noticeable difference.
func DeltaE76(l1, a1, b1, l2, a2, b2 float64) float64 {
	dl := l1 - l2
	da := a1 - a2
	db := b1 - b2
	return math.Sqrt(dl*dl + da*da + db*db)
}
