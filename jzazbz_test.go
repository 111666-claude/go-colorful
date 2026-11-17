package colorful

import (
	"errors"
	"math"
	"testing"
)

// jzazbzFinite3 reports whether all three values are finite (not NaN/Inf).
func jzazbzFinite3(v [3]float64) bool {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}

// jzazbzAbsRefs are reference conversions from absolute CIE XYZ (D65) to
// Jzazbz, quoted from the test-suite of the colour-science reference
// implementation, which validates against the Matlab code published with
// Safdar et al. (2017), doi:10.1364/OE.25.015131:
//
//	https://github.com/colour-science/colour/blob/develop/colour/models/tests/test_jzazbz.py
//
// Note that the XYZ values there are absolute, so their small magnitude means
// a very dark color rather than a relative white.
var jzazbzAbsRefs = []struct {
	xyz [3]float64
	jz  [3]float64
}{
	{[3]float64{0.20654008, 0.12197225, 0.05136952}, [3]float64{0.00535048, 0.00924302, 0.00526007}},
	{[3]float64{0.14222010, 0.23042768, 0.10495772}, [3]float64{0.00619681, -0.00608426, 0.00534077}},
	{[3]float64{0.96907232, 1.00000000, 1.12179215}, [3]float64{0.01766826, 0.00064174, -0.00052906}},
	{[3]float64{0, 0, 0}, [3]float64{0, 0, 0}},
}

func TestXyzAbsToJzazbzReference(t *testing.T) {
	for _, tt := range jzazbzAbsRefs {
		jz, az, bz, err := XyzAbsToJzazbz(tt.xyz[0], tt.xyz[1], tt.xyz[2])
		if err != nil {
			t.Errorf("XyzAbsToJzazbz(%v): unexpected error: %v", tt.xyz, err)
			continue
		}
		got := [3]float64{jz, az, bz}
		for i := range got {
			if math.Abs(got[i]-tt.jz[i]) > 1e-7 {
				t.Errorf("XyzAbsToJzazbz(%v) = %v, want %v", tt.xyz, got, tt.jz)
				break
			}
		}
	}
}

func TestJzazbzToXyzAbsReference(t *testing.T) {
	for _, tt := range jzazbzAbsRefs {
		x, y, z, err := JzazbzToXyzAbs(tt.jz[0], tt.jz[1], tt.jz[2])
		if err != nil {
			t.Errorf("JzazbzToXyzAbs(%v): unexpected error: %v", tt.jz, err)
			continue
		}
		got := [3]float64{x, y, z}
		for i := range got {
			// The published reference triplets are rounded to 8 decimals, so
			// allow the corresponding absolute error (matching colour-science,
			// which uses 1e-6 for this direction).
			if math.Abs(got[i]-tt.xyz[i]) > 1e-6 {
				t.Errorf("JzazbzToXyzAbs(%v) = %v, want %v", tt.jz, got, tt.xyz)
				break
			}
		}
	}
}

// Reference values for D65 white and Rec. 2020 primaries at several absolute
// luminance levels, cross-checked against the CSS Color HDR sample code and
// the color.js implementation:
//
//	https://drafts.csswg.org/css-color-hdr/#Jzazbz
//	https://github.com/color-js/color.js/blob/main/src/spaces/jzazbz.js
//
// The white entry at 203 cd/m² is the "diffuse white" value quoted in the CSS
// Color HDR specification: jzazbz(0.22207 -0.00016 -0.00012).
var jzazbzHdrRefs = []struct {
	name string
	xyz  [3]float64
	jz   [3]float64
}{
	{
		"D65 white @ 203 cd/m²",
		[3]float64{0.9504559270516717 * 203, 1 * 203, 1.0890577507598784 * 203},
		[3]float64{0.222065249535739, -0.000160624926182767, -0.000117033997700022},
	},
	{
		"D65 white @ 1000 cd/m²",
		[3]float64{0.9504559270516717 * 1000, 1 * 1000, 1.0890577507598784 * 1000},
		[3]float64{0.409124110444834, -0.000201094436155957, -0.000146514265889297},
	},
	{
		"D65 white @ 10000 cd/m²",
		[3]float64{0.9504559270516717 * 10000, 1 * 10000, 1.0890577507598784 * 10000},
		[3]float64{0.988606961045266, -0.000236258074281182, -0.000172125186090000},
	},
	{
		"Rec. 2020 red @ 203 cd/m²",
		[3]float64{0.6369580483012914 * 203, 0.2627002120112671 * 203, 0},
		[3]float64{0.154543004144470, 0.164299304898897, 0.135062817633853},
	},
	{
		"Rec. 2020 green @ 1000 cd/m²",
		[3]float64{0.14461690358620832 * 1000, 0.6779980715188708 * 1000, 0.028072693049087428 * 1000},
		[3]float64{0.301296027920955, -0.276047662456918, 0.199764060546476},
	},
	{
		"Rec. 2020 blue @ 10000 cd/m²",
		[3]float64{0.1688809751641721 * 10000, 0.05930171646986196 * 10000, 1.0609850577107909 * 10000},
		[3]float64{0.438635808343465, -0.187513503895987, -0.354215136551206},
	},
}

func TestXyzAbsToJzazbzHDRReference(t *testing.T) {
	for _, tt := range jzazbzHdrRefs {
		jz, az, bz, err := XyzAbsToJzazbz(tt.xyz[0], tt.xyz[1], tt.xyz[2])
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.name, err)
			continue
		}
		got := [3]float64{jz, az, bz}
		for i := range got {
			if math.Abs(got[i]-tt.jz[i]) > 1e-9 {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.jz)
				break
			}
		}
	}
}

// TestDistanceJzazbzAbsReference verifies distances over absolute XYZ against
// values computed from the color.js reference conversions of the same inputs:
// a D65 white vs. Rec. 2020 red pair at 1000 cd/m², and a white @1000 cd/m²
// vs. white @10000 cd/m² highlight pair.
func TestDistanceJzazbzAbsReference(t *testing.T) {
	white := func(y float64) [3]float64 {
		return [3]float64{0.9504559270516717 * y, y, 1.0890577507598784 * y}
	}
	red1000 := [3]float64{0.6369580483012914 * 1000, 0.2627002120112671 * 1000, 0}

	w := white(1000)
	d, err := DistanceJzazbzAbs(w[0], w[1], w[2], red1000[0], red1000[1], red1000[2])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const wantWhiteRed = 0.310751842638462 // sqrt((0.409124-0.290550)^2 + (-0.000201-0.214260)^2 + (-0.000147-0.190938)^2)
	if math.Abs(d-wantWhiteRed) > 1e-9 {
		t.Errorf("white/Rec.2020-red distance @1000 cd/m² = %v, want %v", d, wantWhiteRed)
	}

	w10k := white(10000)
	d, err = DistanceJzazbzAbs(w[0], w[1], w[2], w10k[0], w10k[1], w10k[2])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const wantWhiteLevels = 0.579482852233268 // sqrt((0.409124-0.988607)^2 + (-0.000201+0.000236)^2 + (-0.000147+0.000172)^2)
	if math.Abs(d-wantWhiteLevels) > 1e-9 {
		t.Errorf("white 1000/10000 cd/m² distance = %v, want %v", d, wantWhiteLevels)
	}
}

// CSS Color HDR and color.js agree that sRGB white is
// jzazbz(0.222065 -0.000161 -0.000117) under the 203 cd/m² SDR convention.
func TestJzazbzSDRWhite(t *testing.T) {
	white := Color{1, 1, 1}
	jz, az, bz, err := white.Jzazbz()
	if err != nil {
		t.Fatalf("white.Jzazbz(): unexpected error: %v", err)
	}
	got := [3]float64{jz, az, bz}
	want := [3]float64{0.222065249535739, -0.000160624926182767, -0.000117033997700022}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("white.Jzazbz() = %v, want %v", got, want)
		}
	}
}

// TestJzazbzHDRWhite verifies the 10,000 cd/m² HDR input scale.
func TestJzazbzHDRWhite(t *testing.T) {
	white := Color{1, 1, 1}
	jz, az, bz, err := white.JzazbzHDR()
	if err != nil {
		t.Fatalf("white.JzazbzHDR(): unexpected error: %v", err)
	}
	got := [3]float64{jz, az, bz}
	want := [3]float64{0.988606961045266, -0.000236258074281182, -0.000172125186090000}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Fatalf("white.JzazbzHDR() = %v, want %v", got, want)
		}
	}
}

// TestJzazbzBlack verifies that black is exactly the origin and survives a
// roundtrip in both directions.
func TestJzazbzBlack(t *testing.T) {
	jz, az, bz, err := XyzAbsToJzazbz(0, 0, 0)
	if err != nil {
		t.Fatalf("XyzAbsToJzazbz(0,0,0): unexpected error: %v", err)
	}
	if jz != 0 || az != 0 || bz != 0 {
		t.Errorf("black -> Jzazbz = (%v, %v, %v), want exactly (0, 0, 0)", jz, az, bz)
	}
	x, y, z, err := JzazbzToXyzAbs(0, 0, 0)
	if err != nil {
		t.Fatalf("JzazbzToXyzAbs(0,0,0): unexpected error: %v", err)
	}
	if x != 0 || y != 0 || z != 0 {
		t.Errorf("Jzazbz black -> XYZ = (%v, %v, %v), want exactly (0, 0, 0)", x, y, z)
	}

	c, err := Jzazbz(0, 0, 0)
	if err != nil {
		t.Fatalf("Jzazbz(0,0,0): unexpected error: %v", err)
	}
	if !c.AlmostEqualRgb(Color{0, 0, 0}) {
		t.Errorf("Jzazbz(0,0,0) = %v, want black", c)
	}
}

// TestJzazbzRoundtripAbs checks forward/inverse stability across the whole
// legal luminance range, using D65 white and the Rec. 2020 primaries.
func TestJzazbzRoundtripAbs(t *testing.T) {
	chromas := []struct {
		name string
		xyz  [3]float64
	}{
		{"D65 white", [3]float64{0.9504559270516717, 1, 1.0890577507598784}},
		{"Rec. 2020 red", [3]float64{0.6369580483012914, 0.2627002120112671, 0}},
		{"Rec. 2020 green", [3]float64{0.14461690358620832, 0.6779980715188708, 0.028072693049087428}},
		{"Rec. 2020 blue", [3]float64{0.1688809751641721, 0.05930171646986196, 1.0609850577107909}},
	}
	luminances := []float64{0, 1e-6, 1e-3, 0.01, 0.1, 1, 10, 100, 203, 1000, 4000, 8000, 10000}

	for _, ch := range chromas {
		for _, y := range luminances {
			x, z := ch.xyz[0]*y, ch.xyz[2]*y
			jz, az, bz, err := XyzAbsToJzazbz(x, y, z)
			if err != nil {
				t.Fatalf("%s @ %g cd/m²: unexpected forward error: %v", ch.name, y, err)
			}
			x2, y2, z2, err := JzazbzToXyzAbs(jz, az, bz)
			if err != nil {
				t.Fatalf("%s @ %g cd/m²: unexpected inverse error: %v", ch.name, y, err)
			}
			scale := math.Max(1, y)
			for _, d := range []float64{x2 - x, y2 - y, z2 - z} {
				if math.Abs(d)/scale > 1e-9 {
					t.Errorf("%s @ %g cd/m²: roundtrip XYZ=(%g, %g, %g), want (%g, %g, %g)",
						ch.name, y, x2, y2, z2, x, y, z)
					break
				}
			}
		}
	}
}

// TestJzazbzRoundtripColor checks the sRGB-facing SDR and HDR conventions.
func TestJzazbzRoundtripColor(t *testing.T) {
	colors := []Color{
		{0, 0, 0},
		{1, 1, 1},
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
		{0.2, 0.4, 0.6},
		{1.0, 1.0, 0.5},
	}
	for _, c := range colors {
		for _, tc := range []struct {
			name string
			fwd  func(Color) (float64, float64, float64, error)
			inv  func(float64, float64, float64) (Color, error)
		}{
			{"SDR", Color.Jzazbz, Jzazbz},
			{"HDR", Color.JzazbzHDR, JzazbzHDR},
		} {
			jz, az, bz, err := tc.fwd(c)
			if err != nil {
				t.Errorf("%s %v: unexpected forward error: %v", tc.name, c, err)
				continue
			}
			back, err := tc.inv(jz, az, bz)
			if err != nil {
				t.Errorf("%s %v: unexpected inverse error: %v", tc.name, c, err)
				continue
			}
			if !back.AlmostEqualRgb(c) {
				t.Errorf("%s roundtrip: %v -> %v, want %v", tc.name, c, back, c)
			}
		}
	}
}

// TestJzazbzRoundtripWhiteScale verifies Color.JzazbzWithWhite against the
// equivalent absolute conversion for a non-default white level.
func TestJzazbzRoundtripWhiteScale(t *testing.T) {
	c := Color{0.3, 0.6, 0.9}
	const white = 1000.0

	jz, az, bz, err := c.JzazbzWithWhite(white)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	x, y, z := c.Xyz()
	wjz, waz, wbz, err := XyzAbsToJzazbz(x*white, y*white, z*white)
	if err != nil {
		t.Fatalf("unexpected absolute error: %v", err)
	}
	if jz != wjz || az != waz || bz != wbz {
		t.Errorf("Color.JzazbzWithWhite(%g) = (%v, %v, %v), want (%v, %v, %v)",
			white, jz, az, bz, wjz, waz, wbz)
	}

	back, err := JzazbzWithWhite(jz, az, bz, white)
	if err != nil {
		t.Fatalf("unexpected inverse error: %v", err)
	}
	if !back.AlmostEqualRgb(c) {
		t.Errorf("roundtrip with white %g: %v -> %v, want %v", white, c, back, c)
	}
}

func TestJzazbzForwardErrors(t *testing.T) {
	tests := []struct {
		name    string
		x, y, z float64
		want    error
	}{
		{"negative luminance", 0.5, -1, 0.5, ErrJzazbzNegative},
		{"negative X", -1, 1, 0.5, ErrJzazbzNegative},
		{"negative Z", 0.5, 1, -1, ErrJzazbzNegative},
		{"NaN X", math.NaN(), 1, 0.5, ErrJzazbzNonFinite},
		{"NaN Y", 0.5, math.NaN(), 0.5, ErrJzazbzNonFinite},
		{"+Inf Z", 0.5, 1, math.Inf(1), ErrJzazbzNonFinite},
		{"-Inf Y", 0.5, math.Inf(-1), 0.5, ErrJzazbzNonFinite},
		{"luminance above peak", 0, 10001, 0, ErrJzazbzOutOfRange},
		{"huge luminance", 0, 1e9, 0, ErrJzazbzOutOfRange},
	}
	for _, tt := range tests {
		jz, az, bz, err := XyzAbsToJzazbz(tt.x, tt.y, tt.z)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
		if !jzazbzFinite3([3]float64{jz, az, bz}) {
			t.Errorf("%s: returned non-finite Jzazbz (%v, %v, %v)", tt.name, jz, az, bz)
		}
	}
}

func TestJzazbzInverseErrors(t *testing.T) {
	tests := []struct {
		name       string
		jz, az, bz float64
		want       error
	}{
		{"negative lightness", -0.1, 0, 0, ErrJzazbzOutOfRange},
		{"lightness above one", 1.5, 0, 0, ErrJzazbzOutOfRange},
		{"NaN lightness", math.NaN(), 0, 0, ErrJzazbzNonFinite},
		{"Inf chroma", 0.5, math.Inf(1), 0, ErrJzazbzNonFinite},
		{"chroma below black", 0, 0.5, 0, ErrJzazbzDegenerate},
		{"chroma below black (negative)", 0, -0.5, 0, ErrJzazbzDegenerate},
		{"chroma beyond gamut", 0.5, 10, 10, ErrJzazbzDegenerate},
		{"lightness one exceeds peak", 1, 0, 0, ErrJzazbzDegenerate},
	}
	for _, tt := range tests {
		x, y, z, err := JzazbzToXyzAbs(tt.jz, tt.az, tt.bz)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, tt.want)
		}
		if !jzazbzFinite3([3]float64{x, y, z}) {
			t.Errorf("%s: returned non-finite XYZ (%v, %v, %v)", tt.name, x, y, z)
		}
		if err != nil && (x != 0 || y != 0 || z != 0) {
			t.Errorf("%s: error path returned XYZ (%v, %v, %v), want zeros", tt.name, x, y, z)
		}
	}
}

// Round-off just outside the nominal lightness range is clamped rather than
// rejected, so that values that came from the forward transform round-trip.
// The upper end is not clamped in practice: the forward transform never
// produces Jz above the lightness of the brightest representable color
// (about 0.996 for in-range XYZ), so Jz = 1 is already degenerate.
func TestJzazbzToleranceAtBoundaries(t *testing.T) {
	if _, _, _, err := JzazbzToXyzAbs(-1e-12, 0, 0); err != nil {
		t.Errorf("Jz slightly below 0 should be clamped, got error: %v", err)
	}
}

func TestJzazbzInvalidWhite(t *testing.T) {
	c := Color{0.5, 0.5, 0.5}
	whites := []float64{0, -203, 10000.0001, math.NaN(), math.Inf(1)}
	for _, w := range whites {
		if _, _, _, err := c.JzazbzWithWhite(w); !errors.Is(err, ErrJzazbzInvalidWhite) && !errors.Is(err, ErrJzazbzNonFinite) {
			t.Errorf("white %v: error = %v, want ErrJzazbzInvalidWhite or ErrJzazbzNonFinite", w, err)
		}
		if _, err := JzazbzWithWhite(0.5, 0, 0, w); !errors.Is(err, ErrJzazbzInvalidWhite) && !errors.Is(err, ErrJzazbzNonFinite) {
			t.Errorf("white %v: constructor error = %v, want ErrJzazbzInvalidWhite or ErrJzazbzNonFinite", w, err)
		}
	}
}

// TestJzazbzNoNaN feeds pathological values through every entry point and
// verifies that no NaN or Inf ever escapes, with or without an error.
func TestJzazbzNoNaN(t *testing.T) {
	vals := []float64{
		math.NaN(), math.Inf(1), math.Inf(-1),
		math.MaxFloat64, -math.MaxFloat64,
		math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64,
		0, -0, 1, -1, 1e-300, 1e300,
	}
	for _, a := range vals {
		for _, b := range vals {
			jz, az, bz, _ := XyzAbsToJzazbz(a, b, a)
			if !jzazbzFinite3([3]float64{jz, az, bz}) {
				t.Fatalf("XyzAbsToJzazbz(%v, %v, %v) returned non-finite values (%v, %v, %v)", a, b, a, jz, az, bz)
			}
			x, y, z, _ := JzazbzToXyzAbs(a, b, a)
			if !jzazbzFinite3([3]float64{x, y, z}) {
				t.Fatalf("JzazbzToXyzAbs(%v, %v, %v) returned non-finite values (%v, %v, %v)", a, b, a, x, y, z)
			}
		}
	}

	// Distances too, including the error paths.
	c1 := Color{math.NaN(), 0, 0}
	c2 := Color{1, 1, 1}
	if d, err := c1.DistanceJzazbz(c2); err == nil || math.IsNaN(d) || math.IsInf(d, 0) {
		t.Errorf("DistanceJzazbz with NaN color = (%v, %v), want finite value and error", d, err)
	}
	if d, err := c2.DistanceJzazbzHDR(Color{-1, 0, 0}); err == nil || math.IsNaN(d) || math.IsInf(d, 0) {
		t.Errorf("DistanceJzazbzHDR with negative color = (%v, %v), want finite value and error", d, err)
	}
	if d, err := DistanceJzazbzAbs(0, -1, 0, 0, 1, 0); err == nil || math.IsNaN(d) || math.IsInf(d, 0) {
		t.Errorf("DistanceJzazbzAbs with negative luminance = (%v, %v), want finite value and error", d, err)
	}
}

func TestDistanceJzazbz(t *testing.T) {
	c1 := Color{0.2, 0.4, 0.6}
	c2 := Color{0.7, 0.1, 0.3}

	d, err := c1.DistanceJzazbz(c2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j1, a1, b1, _ := c1.Jzazbz()
	j2, a2, b2, _ := c2.Jzazbz()
	want := math.Sqrt(sq(j1-j2) + sq(a1-a2) + sq(b1-b2))
	if math.Abs(d-want) > 1e-12 {
		t.Errorf("DistanceJzazbz = %v, want %v", d, want)
	}
	if d == 0 {
		t.Errorf("DistanceJzazbz of different colors is zero")
	}

	if d, err = c1.DistanceJzazbz(c1); err != nil || d != 0 {
		t.Errorf("DistanceJzazbz(c, c) = (%v, %v), want (0, nil)", d, err)
	}

	// The explicit-white variant must agree with the default.
	dw, err := c1.DistanceJzazbzWithWhite(c2, JzazbzSDRWhiteLuminance)
	if err != nil || dw != want {
		t.Errorf("DistanceJzazbzWithWhite(SDR white) = (%v, %v), want (%v, nil)", dw, err, want)
	}
}

// TestDistanceJzazbzSDRvsHDR verifies that the SDR (203 cd/m² white) and HDR
// (10,000 cd/m² peak) input scales are distinct and not interchangeable.
func TestDistanceJzazbzSDRvsHDR(t *testing.T) {
	white := Color{1, 1, 1}
	red := Color{1, 0, 0}

	sdr, err := white.DistanceJzazbz(red)
	if err != nil {
		t.Fatalf("unexpected SDR error: %v", err)
	}
	hdr, err := white.DistanceJzazbzHDR(red)
	if err != nil {
		t.Fatalf("unexpected HDR error: %v", err)
	}
	if math.Abs(sdr-hdr) < 0.1 {
		t.Errorf("SDR and HDR distances are too similar: SDR=%v, HDR=%v", sdr, hdr)
	}
	if sdr <= 0 || hdr <= 0 {
		t.Errorf("distances must be positive: SDR=%v, HDR=%v", sdr, hdr)
	}
}

// TestDistanceJzazbzLuminanceDependence verifies the property that motivates
// the space: the same absolute luminance step is perceptually much larger in
// the shadows than in a bright highlight.
func TestDistanceJzazbzLuminanceDependence(t *testing.T) {
	white := [3]float64{0.9504559270516717, 1, 1.0890577507598784}
	gray := func(y float64) (float64, float64, float64) {
		return white[0] * y, white[1] * y, white[2] * y
	}

	steps := []struct {
		from, to float64
	}{
		{0, 1},
		{1, 2},
		{100, 101},
		{1000, 1001},
		{9000, 9001},
	}
	prev := math.Inf(1)
	for _, s := range steps {
		x1, y1, z1 := gray(s.from)
		x2, y2, z2 := gray(s.to)
		d, err := DistanceJzazbzAbs(x1, y1, z1, x2, y2, z2)
		if err != nil {
			t.Fatalf("DistanceJzazbzAbs(%g, %g): unexpected error: %v", s.from, s.to, err)
		}
		if !(d < prev) {
			t.Errorf("distance for %g->%g cd/m² step = %v, want less than the darker step's %v", s.from, s.to, d, prev)
		}
		prev = d
	}
}
