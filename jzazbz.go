package colorful

// Jzazbz is a perceptually uniform color space designed by Safdar, Cui, Kim,
// and Luo for high dynamic range and wide gamut image signals. It combines an
// absolute, PQ-based luminance encoding with cone-response opponent channels,
// which makes it a much better fit for HDR grading than CIE L*a*b* or Oklab:
// a given RGB difference near black and the same difference near a bright
// highlight no longer map to the same perceived distance.
//
// Unlike the other color spaces in this package, Jzazbz is defined in terms of
// *absolute* CIE XYZ, i.e. tristimulus values whose Y is measured in cd/m²
// (nits). The implementation follows the original paper:
//
//	Safdar, M., Cui, G., Kim, Y. J., & Luo, M. R. (2017).
//	Perceptually uniform color space for image signals including high dynamic
//	range and wide gamut. Optics Express, 25(13), 15131.
//	doi:10.1364/OE.25.015131
//
// The reference white is D65 (the same white point as sRGB); no chromatic
// adaptation is performed. The ST 2084 (PQ) transfer function used internally
// has a peak luminance of JzazbzPeakLuminance = 10,000 cd/m² and treats 0 as
// black. The practical encoding range is therefore Y in [0, 10000] cd/m².
//
// Two conventions are provided for colors stored as sRGB in Color:
//
//   - SDR scale (Color.Jzazbz, Jzazbz, Color.DistanceJzazbz): the linearized
//     RGB value 1.0 represents BT.2408 diffuse/reference white, i.e.
//     JzazbzSDRWhiteLuminance = 203 cd/m².
//   - HDR scale (Color.JzazbzHDR, JzazbzHDR, Color.DistanceJzazbzHDR): the
//     linearized RGB value 1.0 represents the PQ peak, 10,000 cd/m².
//
// For already absolute (display-linear, cd/m²) input, use XyzAbsToJzazbz,
// JzazbzToXyzAbs, and DistanceJzazbzAbs directly.

import (
	"errors"
	"fmt"
	"math"
)

const (
	// JzazbzPeakLuminance is the peak luminance of the ST 2084 (PQ) transfer
	// function used by Jzazbz, in cd/m² (nits). It is also the meaning of a
	// linearized RGB value of 1.0 for the HDR input scale.
	JzazbzPeakLuminance = 10000.0

	// JzazbzSDRWhiteLuminance is the luminance assigned to SDR diffuse white
	// (BT.2408 reference white), in cd/m². It is the meaning of a linearized
	// RGB value of 1.0 for the SDR input scale and the default used by the
	// unprefixed Color.Jzazbz and Jzazbz functions.
	JzazbzSDRWhiteLuminance = 203.0
)

// Errors returned by the Jzazbz functions. They always return finite outputs
// (typically zeros) together with a non-nil error, so that callers can use
// errors.Is and never receive NaN.
var (
	// ErrJzazbzNonFinite is returned when an input channel is NaN or +/-Inf,
	// or when a reference white luminance is not finite.
	ErrJzazbzNonFinite = errors.New("colorful: Jzazbz input contains a non-finite value")

	// ErrJzazbzNegative is returned for a negative absolute luminance, or a
	// negative absolute X or Z tristimulus value. Jzazbz is only defined for
	// physical, non-negative absolute XYZ.
	ErrJzazbzNegative = errors.New("colorful: Jzazbz input has a negative luminance or tristimulus value")

	// ErrJzazbzOutOfRange is returned when absolute Y or any cone response is
	// above the 10,000 cd/m² PQ peak, or when a Jzazbz lightness is outside
	// [0, 1]. Such values are outside the space's defined range and cannot be
	// encoded by ST 2084.
	ErrJzazbzOutOfRange = errors.New("colorful: Jzazbz input is outside the supported range of 0..10000 cd/m² or Jz 0..1")

	// ErrJzazbzDegenerate is returned by the inverse transform when a Jzazbz
	// triplet does not map back to a representable absolute XYZ color, for
	// example when a cone response falls below the black point or when the
	// result would need a negative tristimulus value.
	ErrJzazbzDegenerate = errors.New("colorful: Jzazbz value does not map to a valid absolute XYZ color")

	// ErrJzazbzInvalidWhite is returned when a reference white luminance is
	// not in (0, JzazbzPeakLuminance].
	ErrJzazbzInvalidWhite = errors.New("colorful: Jzazbz reference white luminance must be in (0, 10000] cd/m²")
)

// Constants of the Jzazbz transform, from Safdar et al. (2017), equation (1)
// and the published Matlab reference implementation.
const (
	jzazbzB  = 1.15
	jzazbzG  = 0.66
	jzazbzN  = 2610.0 / 16384.0 // 2610 / 2^14
	jzazbzP  = 1.7 * 2523.0 / 32.0
	jzazbzC1 = 3424.0 / 4096.0 // 3424 / 2^12
	jzazbzC2 = 2413.0 / 128.0  // 2413 / 2^7
	jzazbzC3 = 2392.0 / 128.0  // 2392 / 2^7
	jzazbzD  = -0.56
	jzazbzD0 = 1.6295499532821566e-11

	// jzazbzRelTol is the relative tolerance used to accept round-off around
	// the black point, the PQ peak, and the [0, 1] lightness range. Values
	// further outside are reported as errors instead of being clamped.
	jzazbzRelTol = 1e-9
)

// jzazbzPQBlack is the PQ encoding of 0 cd/m², i.e. C1^P. The EOTF maps
// anything at or below this value back to black.
var jzazbzPQBlack = math.Pow(jzazbzC1, jzazbzP)

// Matrices from the published Jzazbz reference implementation. The inverse
// matrices are listed explicitly (rather than computed at run time) to keep
// results reproducible across platforms.
var (
	jzazbzXyzToLms = [3][3]float64{
		{0.41478972, 0.579999, 0.0146480},
		{-0.2015100, 1.120649, 0.0531008},
		{-0.0166008, 0.264800, 0.6684799},
	}
	jzazbzLmsToXyz = [3][3]float64{
		{1.9242264357876067, -1.0047923125953657, 0.037651404030618},
		{0.35031676209499907, 0.7264811939316552, -0.06538442294808501},
		{-0.09098281098284752, -0.3127282905230739, 1.5227665613052603},
	}
	jzazbzLmsToIzazbz = [3][3]float64{
		{0.5, 0.5, 0.0},
		{3.524000, -4.066708, 0.542708},
		{0.199076, 1.096799, -1.295875},
	}
	jzazbzIzazbzToLms = [3][3]float64{
		{1.0, 0.1386050432715393, 0.05804731615611899},
		{0.9999999999999999, -0.1386050432715393, -0.05804731615611874},
		{0.9999999999999998, -0.09601924202631892, -0.8118918960560387},
	}
)

func jzazbzFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func jzazbzMul(m [3][3]float64, v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func jzazbzErr(base error, format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", base, fmt.Sprintf(format, args...))
}

// jzazbzPQ applies the ST 2084 inverse EOTF to a luminance C in cd/m²,
// returning the PQ-encoded value. C is expected to be in [0, 10000].
func jzazbzPQ(c float64) float64 {
	t := math.Pow(c/JzazbzPeakLuminance, jzazbzN)
	return math.Pow((jzazbzC1+jzazbzC2*t)/(1.0+jzazbzC3*t), jzazbzP)
}

// jzazbzPQInv applies the ST 2084 EOTF to a PQ-encoded value N, returning a
// luminance in cd/m². N is expected to be in [jzazbzPQBlack, 1].
func jzazbzPQInv(n float64) float64 {
	vp := math.Pow(n, 1.0/jzazbzP)
	// Below the black point both numerator and denominator of the EOTF are
	// non-positive; clamp there instead of taking a fractional power of a
	// negative ratio.
	num := jzazbzC1 - vp
	den := jzazbzC3*vp - jzazbzC2
	if num >= 0 || den >= 0 {
		return 0.0
	}
	return JzazbzPeakLuminance * math.Pow(num/den, 1.0/jzazbzN)
}

// XyzAbsToJzazbz converts absolute CIE XYZ tristimulus values (D65 reference
// white, Y in cd/m²) to Jzazbz. The x, y, and z arguments are absolute, not
// relative to a white of Y=1: a perfectly white 203 cd/m² display therefore
// has Y=203.
//
// The defined input domain is finite XYZ with X, Y, Z >= 0, Y <= 10000, whose
// LMS cone responses also all lie in [0, 10000] cd/m², the range the ST 2084
// transfer function can encode. The extra cone condition only rejects very
// bright, strongly chromatic values whose luminance is already close to the
// peak.
//
// Inputs outside this domain, including negative luminance and non-finite
// values, return an error together with zero coordinates; the function never
// returns NaN.
func XyzAbsToJzazbz(x, y, z float64) (jz, az, bz float64, err error) {
	if !jzazbzFinite(x) || !jzazbzFinite(y) || !jzazbzFinite(z) {
		return 0, 0, 0, jzazbzErr(ErrJzazbzNonFinite, "XYZ=(%g, %g, %g)", x, y, z)
	}
	if y < 0 {
		return 0, 0, 0, jzazbzErr(ErrJzazbzNegative, "Y=%g", y)
	}
	if x < 0 || z < 0 {
		return 0, 0, 0, jzazbzErr(ErrJzazbzNegative, "XYZ=(%g, %g, %g)", x, y, z)
	}

	peak := JzazbzPeakLuminance
	tol := jzazbzRelTol * peak
	if y > peak+tol {
		return 0, 0, 0, jzazbzErr(ErrJzazbzOutOfRange, "Y=%g", y)
	}

	// Modify X and Y to minimize blue curvature.
	xm := jzazbzB*x - (jzazbzB-1)*z
	ym := jzazbzG*y - (jzazbzG-1)*x

	lms := jzazbzMul(jzazbzXyzToLms, [3]float64{xm, ym, z})
	for i, v := range lms {
		if !jzazbzFinite(v) {
			return 0, 0, 0, jzazbzErr(ErrJzazbzOutOfRange, "cone response %d is not finite", i)
		}
		if v < -tol || v > peak+tol {
			return 0, 0, 0, jzazbzErr(ErrJzazbzOutOfRange, "cone response %d = %g cd/m²", i, v)
		}
		// Clamp only round-off at the boundaries; anything larger errored above.
		lms[i] = math.Max(0.0, math.Min(v, peak))
	}

	lp := jzazbzPQ(lms[0])
	mp := jzazbzPQ(lms[1])
	sp := jzazbzPQ(lms[2])

	// Black: all cone responses are zero, so the opponent channels cancel
	// exactly in theory. Return the exact origin instead of the round-off
	// residue of the matrix products.
	if lms[0] == 0 && lms[1] == 0 && lms[2] == 0 {
		return 0, 0, 0, nil
	}

	iz := 0.5*lp + 0.5*mp
	az = 3.524000*lp - 4.066708*mp + 0.542708*sp
	bz = 0.199076*lp + 1.096799*mp - 1.295875*sp
	jz = (1.0+jzazbzD)*iz/(1.0+jzazbzD*iz) - jzazbzD0

	if !jzazbzFinite(jz) || !jzazbzFinite(az) || !jzazbzFinite(bz) {
		return 0, 0, 0, jzazbzErr(ErrJzazbzOutOfRange, "result is not finite")
	}
	// Black maps to exactly zero up to round-off in the matrix products.
	if jz < 0 && jz > -jzazbzRelTol {
		jz = 0
	}
	if jz > 1.0+jzazbzRelTol {
		return 0, 0, 0, jzazbzErr(ErrJzazbzOutOfRange, "Jz=%g", jz)
	}
	jz = math.Max(0.0, math.Min(jz, 1.0))
	return jz, az, bz, nil
}

// JzazbzToXyzAbs is the inverse of XyzAbsToJzazbz. It returns absolute CIE XYZ
// (D65 reference white) with Y in cd/m².
//
// Jz must be in [0, 1]. The a and b channels are finite but otherwise only
// constrained by the requirement that the resulting cone responses be
// encodable by PQ and the resulting XYZ be physical (non-negative, with
// Y <= 10000). Triplets that violate this, for example a chroma outside the
// representable gamut or a value below the black point, return
// ErrJzazbzDegenerate. The function never returns NaN.
func JzazbzToXyzAbs(jz, az, bz float64) (x, y, z float64, err error) {
	if !jzazbzFinite(jz) || !jzazbzFinite(az) || !jzazbzFinite(bz) {
		return 0, 0, 0, jzazbzErr(ErrJzazbzNonFinite, "Jzazbz=(%g, %g, %g)", jz, az, bz)
	}
	tol := jzazbzRelTol
	if jz < -tol || jz > 1.0+tol {
		return 0, 0, 0, jzazbzErr(ErrJzazbzOutOfRange, "Jz=%g", jz)
	}
	jz = math.Max(0.0, math.Min(jz, 1.0))

	jzp := jz + jzazbzD0
	den := 1.0 + jzazbzD - jzazbzD*jzp
	if den == 0 || !jzazbzFinite(den) {
		return 0, 0, 0, jzazbzErr(ErrJzazbzDegenerate, "Iz denominator is degenerate")
	}
	iz := jzp / den

	lmsP := jzazbzMul(jzazbzIzazbzToLms, [3]float64{iz, az, bz})
	black := jzazbzPQBlack * (1.0 - tol)
	for i, n := range lmsP {
		if !jzazbzFinite(n) {
			return 0, 0, 0, jzazbzErr(ErrJzazbzDegenerate, "cone encoding %d is not finite", i)
		}
		if n < black || n > 1.0+tol {
			return 0, 0, 0, jzazbzErr(ErrJzazbzDegenerate, "cone encoding %d = %g is below black or above the PQ peak", i, n)
		}
		lmsP[i] = math.Max(jzazbzPQBlack, math.Min(n, 1.0))
	}

	lms := [3]float64{
		jzazbzPQInv(lmsP[0]),
		jzazbzPQInv(lmsP[1]),
		jzazbzPQInv(lmsP[2]),
	}

	xyz := jzazbzMul(jzazbzLmsToXyz, lms)
	xm, ym, za := xyz[0], xyz[1], xyz[2]
	xa := (xm + (jzazbzB-1)*za) / jzazbzB
	ya := (ym + (jzazbzG-1)*xa) / jzazbzG

	if !jzazbzFinite(xa) || !jzazbzFinite(ya) || !jzazbzFinite(za) {
		return 0, 0, 0, jzazbzErr(ErrJzazbzDegenerate, "XYZ result is not finite")
	}

	// Round-off may push a component marginally below zero or the luminance
	// marginally above the peak; clamp that, but reject genuine out-of-gamut
	// values.
	scale := math.Max(1.0, math.Max(math.Abs(xa), math.Max(math.Abs(ya), math.Abs(za))))
	absTol := jzazbzRelTol * scale
	if xa < -absTol || ya < -absTol || za < -absTol {
		return 0, 0, 0, jzazbzErr(ErrJzazbzDegenerate, "XYZ=(%g, %g, %g) has a negative component", xa, ya, za)
	}
	if ya > JzazbzPeakLuminance+absTol {
		return 0, 0, 0, jzazbzErr(ErrJzazbzDegenerate, "Y=%g exceeds the PQ peak", ya)
	}

	x = math.Max(0.0, xa)
	y = math.Max(0.0, math.Min(ya, JzazbzPeakLuminance))
	z = math.Max(0.0, za)
	return x, y, z, nil
}

// jzazbzWhite validates a reference white luminance.
func jzazbzWhite(whiteLuminance float64) error {
	if !jzazbzFinite(whiteLuminance) {
		return jzazbzErr(ErrJzazbzNonFinite, "white luminance=%g", whiteLuminance)
	}
	if whiteLuminance <= 0 || whiteLuminance > JzazbzPeakLuminance {
		return jzazbzErr(ErrJzazbzInvalidWhite, "white luminance=%g", whiteLuminance)
	}
	return nil
}

// JzazbzWithWhite converts the color to Jzazbz using an explicit reference
// white luminance in cd/m². The linearized RGB value 1.0 is mapped to
// whiteLuminance, which must be finite and in (0, 10000]; the color itself may
// extend above 1.0 up to the PQ peak.
//
// The D65 white point is used, as required by Jzazbz; no chromatic adaptation
// is performed.
func (col Color) JzazbzWithWhite(whiteLuminance float64) (jz, az, bz float64, err error) {
	if err := jzazbzWhite(whiteLuminance); err != nil {
		return 0, 0, 0, err
	}
	x, y, z := col.Xyz()
	return XyzAbsToJzazbz(x*whiteLuminance, y*whiteLuminance, z*whiteLuminance)
}

// Jzazbz converts an SDR color to Jzazbz, treating the linearized RGB value
// 1.0 as BT.2408 diffuse white at 203 cd/m² (JzazbzSDRWhiteLuminance).
// sRGB is a D65 space, so no chromatic adaptation is needed.
//
// Because Color channels are normally in [0, 1], use JzazbzHDR for colors
// that are scaled to the 10,000 cd/m² PQ peak, or XyzAbsToJzazbz for
// absolute XYZ.
func (col Color) Jzazbz() (jz, az, bz float64, err error) {
	return col.JzazbzWithWhite(JzazbzSDRWhiteLuminance)
}

// JzazbzHDR converts an HDR color to Jzazbz, treating the linearized RGB value
// 1.0 as the 10,000 cd/m² PQ peak (JzazbzPeakLuminance). For example, BT.2100
// RGB code values normalized to [0, 1] should be passed through this scale,
// not through the SDR Color.Jzazbz.
func (col Color) JzazbzHDR() (jz, az, bz float64, err error) {
	return col.JzazbzWithWhite(JzazbzPeakLuminance)
}

// JzazbzWithWhite is the inverse of Color.JzazbzWithWhite. The returned color
// may have channels outside [0, 1]; it is not clamped, matching Lab and the
// other constructors in this package.
func JzazbzWithWhite(jz, az, bz, whiteLuminance float64) (Color, error) {
	if err := jzazbzWhite(whiteLuminance); err != nil {
		return Color{}, err
	}
	x, y, z, err := JzazbzToXyzAbs(jz, az, bz)
	if err != nil {
		return Color{}, err
	}
	return Xyz(x/whiteLuminance, y/whiteLuminance, z/whiteLuminance), nil
}

// Jzazbz creates an SDR color from Jzazbz coordinates, using 203 cd/m²
// (JzazbzSDRWhiteLuminance) for diffuse white. It is the inverse of
// Color.Jzazbz.
func Jzazbz(jz, az, bz float64) (Color, error) {
	return JzazbzWithWhite(jz, az, bz, JzazbzSDRWhiteLuminance)
}

// JzazbzHDR creates an HDR color from Jzazbz coordinates, using 10,000 cd/m²
// (JzazbzPeakLuminance) for the linearized RGB value 1.0. It is the inverse of
// Color.JzazbzHDR.
func JzazbzHDR(jz, az, bz float64) (Color, error) {
	return JzazbzWithWhite(jz, az, bz, JzazbzPeakLuminance)
}

// DistanceJzazbz computes the Euclidean distance between two SDR colors in
// Jzazbz, using 203 cd/m² for diffuse white. It behaves like DistanceLab, but
// its lightness/chroma weighting is designed for HDR and wide-gamut content
// and remains meaningful at highlight luminance levels.
func (c1 Color) DistanceJzazbz(c2 Color) (float64, error) {
	return c1.DistanceJzazbzWithWhite(c2, JzazbzSDRWhiteLuminance)
}

// DistanceJzazbzHDR computes the Euclidean distance between two HDR colors in
// Jzazbz, using 10,000 cd/m² for the linearized RGB value 1.0. Use this
// instead of DistanceJzazbz whenever the inputs are PQ/HDR-scaled; the two
// scales produce different, not interchangeable, distances.
func (c1 Color) DistanceJzazbzHDR(c2 Color) (float64, error) {
	return c1.DistanceJzazbzWithWhite(c2, JzazbzPeakLuminance)
}

// DistanceJzazbzWithWhite computes the Euclidean distance between two colors
// in Jzazbz using an explicit reference white luminance in cd/m².
func (c1 Color) DistanceJzazbzWithWhite(c2 Color, whiteLuminance float64) (float64, error) {
	j1, a1, b1, err := c1.JzazbzWithWhite(whiteLuminance)
	if err != nil {
		return 0, err
	}
	j2, a2, b2, err := c2.JzazbzWithWhite(whiteLuminance)
	if err != nil {
		return 0, err
	}
	return math.Sqrt(sq(j1-j2) + sq(a1-a2) + sq(b1-b2)), nil
}

// DistanceJzazbzAbs computes the Euclidean distance in Jzazbz between two
// absolute CIE XYZ colors (D65 white, cd/m²). It is the absolute-input
// counterpart of DistanceJzazbz and DistanceJzazbzHDR.
func DistanceJzazbzAbs(x1, y1, z1, x2, y2, z2 float64) (float64, error) {
	j1, a1, b1, err := XyzAbsToJzazbz(x1, y1, z1)
	if err != nil {
		return 0, err
	}
	j2, a2, b2, err := XyzAbsToJzazbz(x2, y2, z2)
	if err != nil {
		return 0, err
	}
	return math.Sqrt(sq(j1-j2) + sq(a1-a2) + sq(b1-b2)), nil
}
