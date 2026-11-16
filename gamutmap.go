package colorful

import (
	"fmt"
	"math"
)

// Tolerances used by GamutMapOkLch. They are exported as documentation of the
// algorithm's numerical behavior; changing them is not part of the stable API.
const (
	// GamutMapChannelTolerance is the per-channel slack of the sRGB
	// in-gamut acceptance test used during the chroma binary search. It
	// only absorbs OkLch<->sRGB round-trip round-off (observed below
	// 1e-12); anything larger would, after clamping, show up as a
	// lightness error near black where OkLch is most sensitive.
	GamutMapChannelTolerance = 1e-9

	// GamutMapChromaTolerance is the OkLch chroma resolution of the binary
	// search: the returned chroma is within this distance of the largest
	// displayable chroma at the color's lightness and hue.
	GamutMapChromaTolerance = 1e-6

	// GamutMapLightnessTolerance is how far the OkLch lightness may leave
	// [0, 1] before the input is rejected as unrepresentable instead of
	// being snapped to the black/white endpoint. 1e-3 is 0.1% of the
	// lightness range: visibly negligible, but comfortably above any
	// conversion round-off.
	GamutMapLightnessTolerance = 1e-3
)

// inSRGBGamutTolerant reports whether col's sRGB channels are all within
// [-GamutMapChannelTolerance, 1+GamutMapChannelTolerance].
func (col Color) inSRGBGamutTolerant() bool {
	r, g, b := col.values()
	for _, v := range []float64{r, g, b} {
		if v < -GamutMapChannelTolerance || v > 1.0+GamutMapChannelTolerance {
			return false
		}
	}
	return true
}

// The gamut mapper needs an OkLab conversion whose forward and inverse
// transforms are exact inverses of each other: the binary search evaluates
// the gamut boundary through the round trip, and any mismatch shows up as a
// non-monotonic boundary. The package's public XyzToOkLab/OkLabToXyz pair
// combines Ottosson's truncated forward matrix with a full-precision
// inverse, so its round trip drifts by up to ~5e-4 in XYZ. Fixing that pair
// would change the existing BlendOkLab/BlendOkLch results, so the mapper
// uses its own matched matrices from the CSS Color Level 4 conversion code
// (https://www.w3.org/TR/css-color-4/#color-conversion-code) instead. The
// public conversions are left untouched.

func xyzToOkLchPrecise(x, y, z float64) (l, c, h float64) {
	l_ := math.Cbrt(0.8190224379967030*x + 0.3619062600528904*y - 0.1288737815209879*z)
	m_ := math.Cbrt(0.0329836539323885*x + 0.9292868615863434*y + 0.0361446663506424*z)
	s_ := math.Cbrt(0.0481771893596242*x + 0.2642395317527308*y + 0.6335478284694309*z)

	l = 0.2104542553*l_ + 0.7936177850*m_ - 0.0040720468*s_
	a := 1.9779984951*l_ - 2.4285922050*m_ + 0.4505937099*s_
	b := 0.0259040371*l_ + 0.7827717662*m_ - 0.8086757660*s_

	c = math.Sqrt(a*a + b*b)
	h = math.Atan2(b, a)
	if h < 0 {
		h += 2 * math.Pi
	}
	return l, c, h * 180 / math.Pi
}

func okLchToXyzPrecise(l, c, h float64) (x, y, z float64) {
	hr := h * math.Pi / 180
	a := c * math.Cos(hr)
	b := c * math.Sin(hr)

	// Exact inverses of the forward matrices above (the LMS->OkLab inverse
	// matches OkLabToXyz in colors.go; the XYZ->LMS inverse is the exact
	// inverse of the CSS Color 4 matrix, whose printed inverse is rounded).
	l_ := 0.9999999984505196*l + 0.39633779217376774*a + 0.2158037580607588*b
	m_ := 1.0000000088817607*l - 0.10556134232365633*a - 0.0638541747717059*b
	s_ := 1.0000000546724108*l - 0.08948418209496574*a - 1.2914855378640917*b

	ll := l_ * l_ * l_
	mm := m_ * m_ * m_
	ss := s_ * s_ * s_

	x = 1.2268798758459241*ll - 0.55781499446021698*mm + 0.28139104566596462*ss
	y = -0.040575745214800875*ll + 1.1122868032803173*mm - 0.07171105806551642*ss
	z = -0.076372936674660011*ll - 0.42149333240224313*mm + 1.5869240198367816*ss
	return
}

func okLchToSRGBPrecise(l, c, h float64) Color {
	return Xyz(okLchToXyzPrecise(l, c, h))
}

// GamutMapOkLch maps col into the sRGB gamut using the CSS Color Level 4
// chroma-reduction strategy: the color is converted to OkLch, its lightness
// and hue are kept fixed, and its chroma is reduced by binary search until
// the color is displayable. This preserves the perceived lightness and hue
// of wide-gamut colors (e.g. Display-P3 poster artwork) far better than
// per-channel clamping, which shifts the hue of saturated colors.
//
// Behavior:
//   - Colors already inside the sRGB gamut are returned unchanged.
//   - Mapping is idempotent: mapping the result again returns it as is.
//   - Achromatic colors (grays) are in gamut by construction and pass
//     through untouched; colors whose lightness rounds to 0 or 1 snap to
//     black resp. white, so the endpoints are continuous.
//   - The result's chroma is within GamutMapChromaTolerance of the maximum
//     displayable chroma at the original lightness and hue.
//
// An error is returned instead of a silently wrong color when the input
// cannot be represented: non-finite (NaN/Inf) channels, or an OkLch
// lightness outside [0, 1] by more than GamutMapLightnessTolerance.
func (col Color) GamutMapOkLch() (Color, error) {
	r, g, b := col.values()
	for _, v := range []float64{r, g, b} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Color{}, fmt.Errorf("colorful: cannot gamut-map color with non-finite channel: %v", col)
		}
	}

	x, y, z := col.Xyz()
	l, c, h := xyzToOkLchPrecise(x, y, z)
	if math.IsNaN(l) || math.IsInf(l, 0) || math.IsNaN(c) || math.IsInf(c, 0) || math.IsNaN(h) {
		return Color{}, fmt.Errorf("colorful: cannot gamut-map color %v: unrepresentable in OkLch", col)
	}
	if l < -GamutMapLightnessTolerance || l > 1.0+GamutMapLightnessTolerance {
		return Color{}, fmt.Errorf("colorful: cannot gamut-map color %v: OkLch lightness %v outside [0, 1]", col, l)
	}

	// Snap the endpoints so near-black and near-white colors map smoothly
	// onto black and white instead of producing off-by-epsilon grays.
	if l <= 0.0 {
		return Color{0.0, 0.0, 0.0}, nil
	}
	if l >= 1.0 {
		return Color{1.0, 1.0, 1.0}, nil
	}

	// Already displayable: return the color untouched. Clamped() is the
	// identity for genuinely in-gamut colors and only trims excursions
	// below GamutMapChannelTolerance (e.g. a -1e-15 channel produced by
	// wide-gamut matrix round-off), which cannot shift the lightness
	// perceptibly even near black.
	if col.inSRGBGamutTolerant() {
		return col.Clamped(), nil
	}

	// Binary search for the largest displayable chroma at fixed L and h.
	// The chroma of a gray (c = 0) at 0 < L < 1 is always in gamut, so lo
	// is always a valid starting point and the search always converges.
	lo, hi := 0.0, c
	for hi-lo > GamutMapChromaTolerance {
		mid := 0.5 * (lo + hi)
		if okLchToSRGBPrecise(l, mid, h).inSRGBGamutTolerant() {
			lo = mid
		} else {
			hi = mid
		}
	}
	return okLchToSRGBPrecise(l, lo, h).Clamped(), nil
}
