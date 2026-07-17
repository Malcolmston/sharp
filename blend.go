package sharp

import "math"

// BlendMode selects how overlay (source) colours are combined with base
// (backdrop) colours during Composite. The default, BlendOver, is ordinary
// source-over alpha compositing; the others are the separable blend modes from
// the W3C compositing specification, applied per channel in [0,1] before the
// alpha composite.
type BlendMode int

// Blend modes.
const (
	BlendOver BlendMode = iota
	BlendMultiply
	BlendScreen
	BlendOverlay
	BlendDarken
	BlendLighten
	BlendColorDodge
	BlendColorBurn
	BlendHardLight
	BlendSoftLight
	BlendDifference
	BlendExclusion
	BlendAdd
)

// blendChannel applies the separable blend function for mode to one channel,
// with backdrop cb and source cs both in [0,1].
func blendChannel(mode BlendMode, cb, cs float64) float64 {
	switch mode {
	case BlendMultiply:
		return cb * cs
	case BlendScreen:
		return cb + cs - cb*cs
	case BlendOverlay:
		return hardLight(cs, cb)
	case BlendDarken:
		return math.Min(cb, cs)
	case BlendLighten:
		return math.Max(cb, cs)
	case BlendColorDodge:
		if cb == 0 {
			return 0
		}
		if cs >= 1 {
			return 1
		}
		return math.Min(1, cb/(1-cs))
	case BlendColorBurn:
		if cb >= 1 {
			return 1
		}
		if cs <= 0 {
			return 0
		}
		return 1 - math.Min(1, (1-cb)/cs)
	case BlendHardLight:
		return hardLight(cb, cs)
	case BlendSoftLight:
		return softLight(cb, cs)
	case BlendDifference:
		return math.Abs(cb - cs)
	case BlendExclusion:
		return cb + cs - 2*cb*cs
	case BlendAdd:
		return math.Min(1, cb+cs)
	default: // BlendOver
		return cs
	}
}

// hardLight is the hard-light blend of backdrop cb and source cs.
func hardLight(cb, cs float64) float64 {
	if cs <= 0.5 {
		return cb * (2 * cs)
	}
	return cb + (2*cs - 1) - cb*(2*cs-1)
}

// softLight is the W3C soft-light blend of backdrop cb and source cs.
func softLight(cb, cs float64) float64 {
	if cs <= 0.5 {
		return cb - (1-2*cs)*cb*(1-cb)
	}
	var d float64
	if cb <= 0.25 {
		d = ((16*cb-12)*cb + 4) * cb
	} else {
		d = math.Sqrt(cb)
	}
	return cb + (2*cs-1)*(d-cb)
}
