package colorful

import (
	"errors"
	"math"
)

// APCA (Accessible Perceptual Contrast Algorithm) is a perceptual model for
// the contrast of text against a background. Unlike the Distance* functions,
// which measure color similarity, APCA predicts readability and is polarity
// sensitive: swapping foreground and background can change the result.
//
// This file implements the APCA-W3 0.1.9 (SAPC-8, G-4g constants, revision
// July 3, 2022) contrast function, matching the reference implementation
// published as the npm package apca-w3@0.1.9:
// https://github.com/Myndex/apca-w3
//
// A positive value means dark text on a light background, a negative value
// means light text on a dark background.

// APCA version constants. Only APCA W3 0.1.9 is currently implemented.
const (
	APCAVersionW3019 = "0.1.9"
)

// Errors returned by the APCA functions. Callers can use errors.Is to check
// for them; the functions always also return a (finite) zero contrast.
var (
	// ErrAPCANonFinite is returned when any color channel or alpha value is
	// NaN or +/-Inf.
	ErrAPCANonFinite = errors.New("colorful: APCA input contains a non-finite channel or alpha value")
	// ErrAPCATransparent is returned when the foreground or background has an
	// alpha below one and no opaque base color was supplied to composite
	// against.
	ErrAPCATransparent = errors.New("colorful: APCA foreground or background is translucent but no opaque base was given")
)

// ColorAlpha carries an sRGB color together with its alpha, both in [0..1].
// It is the alpha-aware counterpart of Color, which is always treated as
// fully opaque.
type ColorAlpha struct {
	Color
	A float64
}

// AlphaColor builds a ColorAlpha from an sRGB color and an alpha value in
// [0..1].
func AlphaColor(c Color, alpha float64) ColorAlpha {
	return ColorAlpha{Color: c, A: alpha}
}

// IsValid reports whether the color channels and the alpha are finite and in
// [0..1].
func (c ColorAlpha) IsValid() bool {
	return c.Color.IsValid() && 0.0 <= c.A && c.A <= 1.0
}

// SA98G G-4g constants from APCA-W3 0.1.9, kept unexported and grouped to
// track the reference implementation's SA98G constant table one to one.
var sa98g = struct {
	mainTRC                        float64
	sRco, sGco, sBco               float64
	normBG, normTXT, revTXT, revBG float64
	blkThrs, blkClmp               float64
	scaleBoW, scaleWoB             float64
	loBoWoffset, loWoBoffset       float64
	deltaYmin, loClip              float64
}{
	mainTRC:     2.4, // 2.4 exponent emulating actual monitor perception
	sRco:        0.2126729,
	sGco:        0.7151522,
	sBco:        0.0721750,
	normBG:      0.56,
	normTXT:     0.57,
	revTXT:      0.62,
	revBG:       0.65,
	blkThrs:     0.022,
	blkClmp:     1.414,
	scaleBoW:    1.14,
	scaleWoB:    1.14,
	loBoWoffset: 0.027,
	loWoBoffset: 0.027,
	deltaYmin:   0.0005,
	loClip:      0.1,
}

// hasNonFinite reports whether any channel is NaN or +/-Inf.
func hasNonFinite(c Color) bool {
	return math.IsNaN(c.R) || math.IsNaN(c.G) || math.IsNaN(c.B) ||
		math.IsInf(c.R, 0) || math.IsInf(c.G, 0) || math.IsInf(c.B, 0)
}

// srgbToYAPCA converts an sRGB color in [0..1] channels to relative
// luminance Y using the simple 2.4 monitor exponent, as the APCA reference
// does. Note that this deliberately differs from LinearRgb, which uses the
// piecewise sRGB transfer function. Channels are clamped to [0..1] first.
func srgbToYAPCA(c Color) float64 {
	linear := func(channel float64) float64 {
		return math.Pow(clamp01(channel), sa98g.mainTRC)
	}
	return sa98g.sRco*linear(c.R) +
		sa98g.sGco*linear(c.G) +
		sa98g.sBco*linear(c.B)
}

// apcaContrast implements the signed APCAcontrast() function of APCA-W3
// 0.1.9 for already-computed linear luminances. txtY is the text (foreground)
// luminance and bgY the background luminance; they must not be swapped, since
// the result is polarity sensitive.
func apcaContrast(txtY, bgY float64) float64 {
	// Soft clamp black for either color near black.
	if txtY <= sa98g.blkThrs {
		txtY += math.Pow(sa98g.blkThrs-txtY, sa98g.blkClmp)
	}
	if bgY <= sa98g.blkThrs {
		bgY += math.Pow(sa98g.blkThrs-bgY, sa98g.blkClmp)
	}

	// Return zero early for an extremely small luminance difference.
	if math.Abs(bgY-txtY) < sa98g.deltaYmin {
		return 0.0
	}

	var outputContrast float64
	if bgY > txtY {
		// Normal polarity: dark text on a light background (BoW).
		sapc := (math.Pow(bgY, sa98g.normBG) -
			math.Pow(txtY, sa98g.normTXT)) * sa98g.scaleBoW
		// Low-contrast smooth rollout and low clip, preventing polarity
		// reversal for very low contrasts.
		if sapc < sa98g.loClip {
			return 0.0
		}
		outputContrast = sapc - sa98g.loBoWoffset
	} else {
		// Reverse polarity: light text on a dark background (WoB), always
		// returned as a negative number.
		sapc := (math.Pow(bgY, sa98g.revBG) -
			math.Pow(txtY, sa98g.revTXT)) * sa98g.scaleWoB
		if sapc > -sa98g.loClip {
			return 0.0
		}
		outputContrast = sapc + sa98g.loWoBoffset
	}

	return outputContrast * 100.0
}

// compositeOver blends src onto the opaque color dst in gamma-encoded sRGB
// space, the standard simple alpha compositing used by the APCA reference.
func compositeOver(src ColorAlpha, dst Color) Color {
	a := clamp01(src.A)
	return Color{
		R: dst.R*(1.0-a) + src.R*a,
		G: dst.G*(1.0-a) + src.G*a,
		B: dst.B*(1.0-a) + src.B*a,
	}
}

// ContrastAPCA returns the signed APCA-W3 0.1.9 contrast of foreground text
// c against an opaque background. A positive result means dark text on a
// light background, a negative result means light text on a dark background.
// Swapping the two colors can change the result.
//
// Channels outside the sRGB gamut ([0..1]) are clamped into range, matching
// Color.Clamped. A non-finite channel makes the function return 0.0 together
// with ErrAPCANonFinite; it never returns NaN.
func (c Color) ContrastAPCA(background Color) (float64, error) {
	if hasNonFinite(c) || hasNonFinite(background) {
		return 0.0, ErrAPCANonFinite
	}
	return apcaContrast(srgbToYAPCA(c), srgbToYAPCA(background)), nil
}

// ContrastAPCA is the alpha-aware APCA-W3 0.1.9 contrast of foreground text
// c against background. Alpha is composited before the contrast is computed:
// if either color has an alpha below one, base must be the opaque color seen
// behind both of them; the background is first composited over base and the
// foreground is then composited over the resulting background. When both
// colors are fully opaque base is ignored and may be the zero value.
//
// Fully transparent foreground or background without an opaque base returns
// 0.0 and ErrAPCATransparent, since the contrast is undefined. Non-finite
// channels or alpha return 0.0 and ErrAPCANonFinite. Channels outside [0..1]
// are clamped into range, alpha outside [0..1] is clamped as well. The
// returned contrast is always a finite number.
func (c ColorAlpha) ContrastAPCA(background ColorAlpha, base ...Color) (float64, error) {
	if hasNonFinite(c.Color) || hasNonFinite(background.Color) ||
		math.IsNaN(c.A) || math.IsInf(c.A, 0) ||
		math.IsNaN(background.A) || math.IsInf(background.A, 0) {
		return 0.0, ErrAPCANonFinite
	}

	alphaFG := clamp01(c.A)
	alphaBG := clamp01(background.A)

	if alphaFG < 1.0 || alphaBG < 1.0 {
		if len(base) == 0 {
			return 0.0, ErrAPCATransparent
		}
		if hasNonFinite(base[0]) {
			return 0.0, ErrAPCANonFinite
		}
	}

	var bg Color
	switch {
	case alphaBG >= 1.0:
		bg = background.Color
	case len(base) > 0:
		bg = compositeOver(background, base[0])
	default:
		return 0.0, ErrAPCATransparent
	}

	var fg Color
	switch {
	case alphaFG >= 1.0:
		fg = c.Color
	case len(base) > 0:
		fg = compositeOver(c, bg)
	default:
		return 0.0, ErrAPCATransparent
	}

	return apcaContrast(srgbToYAPCA(fg), srgbToYAPCA(bg)), nil
}
