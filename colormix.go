package colorful

import (
	"errors"
	"image/color"
	"math"
)

// CSS color-mix() entry point.
//
// This implements the two-color interpolation defined by CSS Color Module
// Level 5 (color-mix()) on top of the Color Level 4 interpolation rules:
// https://www.w3.org/TR/css-color-5/#color-mix
// https://www.w3.org/TR/css-color-4/#interpolation
//
// Unlike the Blend* methods, colors carry an alpha channel and interpolation
// uses premultiplied alpha, so mixing with a transparent endpoint never fades
// through an unexpected black, and partial-alpha colors combine correctly.

// ColorMixSpace selects the color space in which the two colors are
// interpolated. Only the spaces required by CSS color-mix() that this library
// supports are listed.
type ColorMixSpace string

const (
	// ColorMixSRGB interpolates in the gamma-encoded sRGB space. This is the
	// "in srgb" color-mix() space.
	ColorMixSRGB ColorMixSpace = "srgb"

	// ColorMixOkLch interpolates in the cylindrical OkLCh space, interpolating
	// lightness and chroma in premultiplied form and the hue along an explicit
	// arc. This is the "in oklch" color-mix() space.
	ColorMixOkLch ColorMixSpace = "oklch"
)

// HueDirection selects which arc the OkLCh hue follows between the two
// endpoint hues (CSS Color 4 §13.6).
type HueDirection string

const (
	// HueShorter takes the shorter arc between the two hues (the CSS default).
	HueShorter HueDirection = "shorter"

	// HueLonger takes the longer arc between the two hues.
	HueLonger HueDirection = "longer"
)

// ColorRGBA is an sRGB color with a straight (non-premultiplied) alpha channel,
// every component being in [0..1]. It is the input and output type of ColorMix
// because the plain Color type has no alpha channel.
type ColorRGBA struct {
	R, G, B, A float64
}

// ColorMixOptions configures a ColorMix call.
type ColorMixOptions struct {
	// Space is the interpolation color space. It is required and must be one of
	// ColorMixSRGB or ColorMixOkLch.
	Space ColorMixSpace

	// P is the proportion of the second color, in [0..1]. The first color gets
	// weight (1-P) and the second color weight P. P == 0 returns the first color
	// and P == 1 returns the second one (alpha is always interpolated).
	P float64

	// Hue selects the OkLCh hue arc. The zero value and HueShorter both mean the
	// CSS default shorter arc. It is ignored for the sRGB space.
	Hue HueDirection
}

// Errors returned by ColorMix. They are returned as the sentinel values below,
// so both a direct comparison and errors.Is work.
var (
	// ErrNonFiniteColor is returned when any color component or the proportion
	// is NaN or ±Inf.
	ErrNonFiniteColor = errors.New("colorful: color component or proportion is not finite")

	// ErrInvalidComponent is returned when an R, G, B or A component is outside
	// the [0..1] range.
	ErrInvalidComponent = errors.New("colorful: color component outside the [0..1] range")

	// ErrInvalidRatio is returned when the mixing proportion P is not in [0..1].
	ErrInvalidRatio = errors.New("colorful: mixing proportion must be in [0..1]")

	// ErrUnsupportedSpace is returned when the requested interpolation space is
	// neither ColorMixSRGB nor ColorMixOkLch.
	ErrUnsupportedSpace = errors.New("colorful: unsupported color-mix interpolation space")

	// ErrUnsupportedHueDirection is returned when the requested hue direction is
	// not empty, HueShorter or HueLonger.
	ErrUnsupportedHueDirection = errors.New("colorful: unsupported hue interpolation direction")
)

// oklchPowerlessEpsilon mirrors the threshold used by the existing BlendHcl /
// BlendOkLch helpers: an OkLCh chroma at or below this value carries no
// meaningful hue, which is then inherited from the other endpoint.
const oklchPowerlessEpsilon = 0.00015

// gamut mapping constants from CSS Color 4 §14.2 (binary search with local
// MINDE). deltaEOK is the plain Euclidean distance in OkLab.
const (
	gamutJND     = 0.02
	gamutEpsilon = 0.0001
)

// ColorMix interpolates two sRGB colors with alpha according to CSS color-mix().
//
// The result always carries a usable alpha and, for OkLCh interpolation, is
// gamut mapped back into the sRGB gamut with the CSS Color 4 binary-search
// chroma reduction so the returned R, G and B are always in [0..1]. The function
// never returns a NaN: non-finite inputs, out-of-range components, an invalid
// proportion, an unsupported space or an unsupported hue direction all produce
// an explicit error.
func ColorMix(c1, c2 ColorRGBA, opts ColorMixOptions) (ColorRGBA, error) {
	if err := c1.validate(); err != nil {
		return ColorRGBA{}, err
	}
	if err := c2.validate(); err != nil {
		return ColorRGBA{}, err
	}
	if math.IsNaN(opts.P) || math.IsInf(opts.P, 0) {
		return ColorRGBA{}, ErrNonFiniteColor
	}
	if opts.P < 0.0 || opts.P > 1.0 {
		return ColorRGBA{}, ErrInvalidRatio
	}

	hue := opts.Hue
	if hue == "" {
		hue = HueShorter
	}
	if hue != HueShorter && hue != HueLonger {
		return ColorRGBA{}, ErrUnsupportedHueDirection
	}

	switch opts.Space {
	case ColorMixSRGB:
		return colorMixSRGB(c1, c2, opts.P), nil
	case ColorMixOkLch:
		return colorMixOkLch(c1, c2, opts.P, hue), nil
	default:
		return ColorRGBA{}, ErrUnsupportedSpace
	}
}

func (c ColorRGBA) validate() error {
	comps := [4]float64{c.R, c.G, c.B, c.A}
	for _, v := range comps {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ErrNonFiniteColor
		}
	}
	for _, v := range comps {
		if v < 0.0 || v > 1.0 {
			return ErrInvalidComponent
		}
	}
	return nil
}

// colorMixSRGB implements "in srgb": the gamma-encoded sRGB coordinates are
// premultiplied by alpha before being linearly interpolated (CSS Color 4 §13.5).
func colorMixSRGB(c1, c2 ColorRGBA, p float64) ColorRGBA {
	w1, w2 := 1.0-p, p
	alpha := w1*c1.A + w2*c2.A

	// Premultiplied interpolation.
	pr := w1*c1.R*c1.A + w2*c2.R*c2.A
	pg := w1*c1.G*c1.A + w2*c2.G*c2.A
	pb := w1*c1.B*c1.A + w2*c2.B*c2.A

	out := ColorRGBA{A: alpha}
	if alpha == 0.0 {
		// Fully transparent: the un-premultiplied color is undefined, so emit
		// transparent black rather than dividing by zero.
		return out
	}
	out.R = clamp01(pr / alpha)
	out.G = clamp01(pg / alpha)
	out.B = clamp01(pb / alpha)
	return out
}

// colorMixOkLch implements "in oklch". Lightness and chroma are premultiplied
// by alpha; the hue is not premultiplied and follows the requested arc.
func colorMixOkLch(c1, c2 ColorRGBA, p float64, hue HueDirection) ColorRGBA {
	w1, w2 := 1.0-p, p
	alpha := w1*c1.A + w2*c2.A

	l1, chroma1, h1 := Color{c1.R, c1.G, c1.B}.OkLch()
	l2, chroma2, h2 := Color{c2.R, c2.G, c2.B}.OkLch()

	// A fully transparent endpoint has no carried color, and an achromatic
	// endpoint has a powerless hue. Both inherit the other endpoint's hue.
	missing1 := c1.A == 0.0 || chroma1 <= oklchPowerlessEpsilon
	missing2 := c2.A == 0.0 || chroma2 <= oklchPowerlessEpsilon
	switch {
	case missing1 && !missing2:
		h1 = h2
	case missing2 && !missing1:
		h2 = h1
	case missing1 && missing2:
		h1, h2 = 0.0, 0.0
	}

	if hue == HueShorter {
		h1, h2 = hueShorter(h1, h2)
	} else {
		h1, h2 = hueLonger(h1, h2)
	}

	// Premultiply L and C (never the hue), then interpolate and un-premultiply
	// by the interpolated alpha.
	pl := w1*l1*c1.A + w2*l2*c2.A
	pc := w1*chroma1*c1.A + w2*chroma2*c2.A
	hh := w1*h1 + w2*h2

	out := ColorRGBA{A: alpha}
	if alpha == 0.0 {
		return out
	}
	l := pl / alpha
	chroma := pc / alpha

	mixed := gamutMapOkLchToSRGB(l, chroma, hh)
	out.R, out.G, out.B = mixed.R, mixed.G, mixed.B
	return out
}

// hueShorter adjusts two hue angles so that h2-h1 lies in [-180, 180],
// implementing the CSS "shorter" fix-up (Color 4 §13.6.1).
func hueShorter(h1, h2 float64) (float64, float64) {
	if h2-h1 > 180.0 {
		h1 += 360.0
	} else if h2-h1 < -180.0 {
		h2 += 360.0
	}
	return h1, h2
}

// hueLonger adjusts two hue angles so that h2-h1 takes the long way around,
// implementing the CSS "longer" fix-up (Color 4 §13.6.2).
func hueLonger(h1, h2 float64) (float64, float64) {
	d := h2 - h1
	if 0.0 < d && d < 180.0 {
		h1 += 360.0
	} else if -180.0 < d && d <= 0.0 {
		h2 += 360.0
	}
	return h1, h2
}

// inSrgbGamut reports whether the color is inside the sRGB gamut, using the
// small epsilon tolerance from the CSS gamut-mapping pseudocode.
func inSrgbGamut(c Color) bool {
	return -gamutEpsilon <= c.R && c.R <= 1.0+gamutEpsilon &&
		-gamutEpsilon <= c.G && c.G <= 1.0+gamutEpsilon &&
		-gamutEpsilon <= c.B && c.B <= 1.0+gamutEpsilon
}

// deltaEOK is the Euclidean distance between two colors in OkLab (Color 4 §20.3).
func deltaEOK(reference, sample Color) float64 {
	l1, a1, b1 := reference.OkLab()
	l2, a2, b2 := sample.OkLab()
	dl, da, db := l1-l2, a1-a2, b1-b2
	return math.Sqrt(dl*dl + da*da + db*db)
}

// gamutMapOkLchToSRGB maps an OkLCh color into the sRGB gamut using the CSS
// Color 4 §14.2.1 binary-search gamut mapping with local MINDE. It holds
// lightness and hue constant while reducing chroma, which avoids the hue shift
// of a naive component clip except where the clipped color is already within a
// just-noticeable difference of the target.
func gamutMapOkLchToSRGB(l, chroma, h float64) Color {
	// Lightness out of range resolves to the white or black point.
	if l >= 1.0 {
		return Color{1, 1, 1}
	}
	if l <= 0.0 {
		return Color{0, 0, 0}
	}

	current := OkLch(l, chroma, h)
	if inSrgbGamut(current) {
		return current.Clamped()
	}

	clip := func(c Color) Color { return c.Clamped() }

	clipped := clip(current)
	if e := deltaEOK(clipped, current); e < gamutJND {
		return clipped
	}

	minChroma, maxChroma := 0.0, chroma
	minInGamut := true
	for maxChroma-minChroma > gamutEpsilon {
		c := (minChroma + maxChroma) / 2.0
		current = OkLch(l, c, h)
		if minInGamut && inSrgbGamut(current) {
			minChroma = c
			continue
		}

		clipped = clip(current)
		e := deltaEOK(clipped, current)
		if e < gamutJND {
			if gamutJND-e < gamutEpsilon {
				return clipped
			}
			minInGamut = false
			minChroma = c
		} else {
			maxChroma = c
		}
	}
	return clip(OkLch(l, minChroma, h))
}

// Color drops the alpha channel and returns the straight sRGB color.
func (c ColorRGBA) Color() Color {
	return Color{c.R, c.G, c.B}
}

// WithAlpha attaches an alpha channel to an opaque color.
func (c Color) WithAlpha(alpha float64) ColorRGBA {
	return ColorRGBA{c.R, c.G, c.B, alpha}
}

// RGBA implements color.Color. As required by that interface the returned
// components are alpha-premultiplied and scaled to [0..65535].
func (c ColorRGBA) RGBA() (r, g, b, a uint32) {
	a = uint32(c.A*65535.0 + 0.5)
	r = uint32(c.R*c.A*65535.0 + 0.5)
	g = uint32(c.G*c.A*65535.0 + 0.5)
	b = uint32(c.B*c.A*65535.0 + 0.5)
	return
}

// NRGBA quantizes the straight-alpha color to 8-bit per component.
func (c ColorRGBA) NRGBA() color.NRGBA {
	return color.NRGBA{
		R: uint8(clamp01(c.R)*255.0 + 0.5),
		G: uint8(clamp01(c.G)*255.0 + 0.5),
		B: uint8(clamp01(c.B)*255.0 + 0.5),
		A: uint8(clamp01(c.A)*255.0 + 0.5),
	}
}
