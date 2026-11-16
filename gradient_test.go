package colorful

import (
	"math"
	"sort"
	"testing"
)

// mustHex parses a hex color or fails the test. Mirrors the fixture style
// used elsewhere in the package but aborts instead of silently ignoring errors.
func mustHex(t *testing.T, h string) Color {
	t.Helper()
	c, err := Hex(h)
	if err != nil {
		t.Fatalf("Hex(%q): %v", h, err)
	}
	return c
}

// errorIs keeps the tests working on the go 1.12 language level the module
// targets, whose standard library has no errors.Is.
func errorIs(err, target error) bool {
	if err == nil {
		return target == nil
	}
	type iser interface{ Is(error) bool }
	if ie, ok := err.(iser); ok && ie.Is(target) {
		return true
	}
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return errorIs(u.Unwrap(), target)
	}
	return err == target
}

// saturatedStops is the multi-saturated-color fixture: it walks several vivid
// hues whose parameter-space spacing crowds perceptually, which is exactly the
// case arc-length resampling is meant to fix. It spans black and white at the
// ends and saturated primaries in between.
func saturatedStops(t *testing.T) []GradientStop {
	hexes := []string{"#000000", "#ff0000", "#ffff00", "#00ff00", "#00ffff", "#0000ff", "#ff00ff", "#ffffff"}
	stops := make([]GradientStop, len(hexes))
	for i, h := range hexes {
		stops[i] = GradientStop{Pos: float64(i), Col: mustHex(t, h)}
	}
	return stops
}

// referenceArcSamples is an independent, straightforward implementation of the
// same arc-length resampling: project stops, collapse adjacent duplicate
// points (they contribute zero length), tessellate each straight gap into
// equal pieces no longer than maxStep, record cumulative straight-line length,
// and place count points at equal cumulative length. The production code must
// agree with this to floating-point precision. It returns the perceptual-space
// coordinates that the output colors are expected to map back to.
func referenceArcSamples(space GradientSpace, stops []GradientStop, count int, maxStep float64) [][3]float64 {
	pp := make([][3]float64, 0, len(stops))
	for _, s := range stops {
		p := space.From(s.Col)
		if len(pp) == 0 || euclid(pp[len(pp)-1], p) != 0.0 {
			pp = append(pp, p)
		}
	}
	type vv struct {
		s  float64
		pt [3]float64
	}
	verts := []vv{{0.0, pp[0]}}
	total := 0.0
	for g := 0; g < len(pp)-1; g++ {
		p1, p2 := pp[g], pp[g+1]
		gl := euclid(p1, p2)
		n := int(math.Ceil(gl / maxStep))
		if n < 1 {
			n = 1
		}
		gapStart := total
		for k := 1; k <= n; k++ {
			tt := float64(k) / float64(n)
			verts = append(verts, vv{gapStart + gl*tt, lerpPoint(p1, p2, tt)})
		}
		total += gl
	}
	want := make([][3]float64, count)
	want[0] = space.From(stops[0].Col)
	want[count-1] = space.From(stops[len(stops)-1].Col)
	if total == 0.0 {
		for i := range want {
			want[i] = pp[0]
		}
		return want
	}
	for i := 1; i < count-1; i++ {
		target := total * float64(i) / float64(count-1)
		j := sort.Search(len(verts), func(k int) bool { return verts[k].s > target }) - 1
		if j < 0 {
			j = 0
		}
		if j >= len(verts)-1 {
			j = len(verts) - 2
		}
		lo, hi := verts[j], verts[j+1]
		u := (target - lo.s) / (hi.s - lo.s)
		want[i] = lerpPoint(lo.pt, hi.pt, u)
	}
	return want
}

// The first and last output colors must be exactly the endpoint stops, even
// though every interior color is produced through a color-space round trip.
func TestPerceptualGradientEndpointsExact(t *testing.T) {
	stops := saturatedStops(t)
	const n = 32
	out, err := PerceptualGradient(stops, n)
	if err != nil {
		t.Fatalf("PerceptualGradient: %v", err)
	}
	if len(out) != n {
		t.Fatalf("got %d colors, want %d", len(out), n)
	}
	if out[0] != stops[0].Col {
		t.Errorf("first color %v, want exact %v", out[0], stops[0].Col)
	}
	if out[n-1] != stops[len(stops)-1].Col {
		t.Errorf("last color %v, want exact %v", out[n-1], stops[len(stops)-1].Col)
	}
}

// Arc-length resampling must place each interior point at exactly equal
// cumulative arc length. This is checked against an independent reference
// implementation in the same perceptual space. Equal adjacent *chord* lengths
// are not required: an internal stop is a kink between two straight segments,
// so an output interval straddling a kink legitimately splits into two shorter
// chords even though both endpoints sit on their target arc lengths.
func TestPerceptualGradientArcLengthLab(t *testing.T) {
	stops := saturatedStops(t)
	space := LabSpace()
	const n = 48
	out, err := PerceptualGradientEx(stops, n, GradientSettings{Gamut: GamutIgnore})
	if err != nil {
		t.Fatalf("PerceptualGradientEx: %v", err)
	}
	want := referenceArcSamples(space, stops, n, DefaultMaxStep)
	for i := 1; i < n-1; i++ {
		got := space.From(out[i])
		if d := euclid(got, want[i]); d > 1e-10 {
			t.Errorf("point %d deviates from arc-length reference by %v", i, d)
		}
	}

	// Sanity check: the resampled colors are not the same as naive equal
	// parameter sampling, proving arc-length resampling actually changes spacing.
	naive := make([]Color, n)
	for i := 0; i < n; i++ {
		u := float64(i) / float64(n-1) * float64(len(stops)-1)
		g := int(math.Floor(u))
		if g >= len(stops)-1 {
			g = len(stops) - 2
		}
		naive[i] = stops[g].Col.BlendLab(stops[g+1].Col, u-float64(g))
	}
	differ := 0
	for i := range out {
		if out[i].DistanceLab(naive[i]) > 1e-6 {
			differ++
		}
	}
	if differ == 0 {
		t.Error("arc-length resampling produced the identical colors as naive parameter sampling")
	}
}

// The black->white fixture is a single straight segment with no kinks, so its
// adjacent chords must be exactly equal and it must stay neutral throughout.
func TestPerceptualGradientBlackWhite(t *testing.T) {
	stops := []GradientStop{
		{Pos: 0, Col: mustHex(t, "#000000")},
		{Pos: 1, Col: mustHex(t, "#ffffff")},
	}
	const n = 16
	out, err := PerceptualGradient(stops, n)
	if err != nil {
		t.Fatalf("PerceptualGradient: %v", err)
	}
	if out[0] != stops[0].Col || out[n-1] != stops[1].Col {
		t.Errorf("endpoints not exact: %v ... %v", out[0], out[n-1])
	}
	space := LabSpace()
	prev := space.From(out[0])
	first := euclid(prev, space.From(out[1]))
	for i := 1; i < n; i++ {
		cur := space.From(out[i])
		if d := euclid(prev, cur); math.Abs(d-first) > 1e-12 {
			t.Errorf("step %d length %v differs from first step %v on a straight segment", i, d, first)
		}
		prev = cur
	}
	prevL := -1.0
	for i, c := range out {
		if !c.IsValid() {
			t.Errorf("color %d out of gamut: %v", i, c)
		}
		l, a, b := c.Lab()
		// sRGB round-trip of a neutral axis introduces tiny chroma residuals;
		// they must stay far below the 8-bit quantization threshold.
		if math.Abs(a) > 1e-3 || math.Abs(b) > 1e-3 {
			t.Errorf("color %d not neutral (a=%v b=%v)", i, a, b)
		}
		if l <= prevL {
			t.Errorf("lightness not strictly increasing at %d", i)
		}
		prevL = l
	}
}

// Adjacent duplicate stops collapse into a zero-length polyline piece. They
// still require strictly increasing positions, so duplicates get distinct ones.
func TestPerceptualGradientDuplicateStops(t *testing.T) {
	base := saturatedStops(t)
	stops := []GradientStop{
		{Pos: 0, Col: base[0].Col},
		{Pos: 1, Col: base[0].Col}, // leading duplicate color, distinct position
	}
	for i := 1; i < len(base); i++ {
		stops = append(stops, GradientStop{Pos: float64(i + 1), Col: base[i].Col})
	}
	last := base[len(base)-1].Col
	stops = append(stops, GradientStop{Pos: float64(len(base) + 1), Col: last}) // trailing duplicate color

	out, err := PerceptualGradient(stops, 40)
	if err != nil {
		t.Fatalf("PerceptualGradient with duplicates: %v", err)
	}
	if out[0] != base[0].Col || out[len(out)-1] != last {
		t.Errorf("endpoints wrong with duplicate stops")
	}
	// A zero-length duplicate stop must not change the geometric path, so the
	// result has to be identical to the same gradient with duplicates removed.
	dedup := make([]GradientStop, 0, len(base))
	dedup = append(dedup, base[0])
	for i := 1; i < len(base); i++ {
		if base[i].Col != dedup[len(dedup)-1].Col {
			dedup = append(dedup, GradientStop{Pos: float64(len(dedup)), Col: base[i].Col})
		}
	}
	want, err := PerceptualGradient(dedup, 40)
	if err != nil {
		t.Fatalf("deduplicated gradient: %v", err)
	}
	for i := range out {
		if out[i] != want[i] {
			t.Errorf("point %d changed by a duplicate stop: %v vs %v", i, out[i], want[i])
		}
	}
}

// When every stop is the same color, output is a deterministic solid gradient.
func TestPerceptualGradientAllIdentical(t *testing.T) {
	c := mustHex(t, "#3366cc")
	stops := []GradientStop{
		{Pos: 0, Col: c},
		{Pos: 1, Col: c},
		{Pos: 2, Col: c},
	}
	const n = 10
	out, err := PerceptualGradient(stops, n)
	if err != nil {
		t.Fatalf("all-identical gradient: %v", err)
	}
	if len(out) != n {
		t.Fatalf("got %d colors, want %d", len(out), n)
	}
	for i, got := range out {
		if got != c {
			t.Errorf("color %d = %v, want solid %v", i, got, c)
		}
	}
}

// All specified failure modes must produce a deterministic, identifiable error.
func TestPerceptualGradientErrors(t *testing.T) {
	ok := []GradientStop{
		{Pos: 0, Col: mustHex(t, "#ff0000")},
		{Pos: 1, Col: mustHex(t, "#0000ff")},
	}

	cases := []struct {
		name   string
		stops  []GradientStop
		count  int
		target error
	}{
		{"empty", nil, 4, ErrEmptyGradient},
		{"single", ok[:1], 4, ErrBadStopCount},
		{"count one", ok, 1, ErrBadCount},
		{"count zero", ok, 0, ErrBadCount},
		{"nan color", []GradientStop{{Pos: 0, Col: Color{math.NaN(), 0, 0}}, {Pos: 1, Col: ok[1].Col}}, 4, ErrNonFiniteColor},
		{"inf color", []GradientStop{{Pos: 0, Col: Color{0, 0, 0}}, {Pos: 1, Col: Color{1, math.Inf(1), 0}}}, 4, ErrNonFiniteColor},
		{"out of gamut", []GradientStop{{Pos: 0, Col: Color{1.1, 0, 0}}, {Pos: 1, Col: ok[1].Col}}, 4, ErrOutOfGamut},
		{"equal positions", []GradientStop{{Pos: 1, Col: ok[0].Col}, {Pos: 1, Col: ok[1].Col}}, 4, ErrBadPosition},
		{"decreasing positions", []GradientStop{{Pos: 2, Col: ok[0].Col}, {Pos: 1, Col: ok[1].Col}}, 4, ErrBadPosition},
		{"nan position", []GradientStop{{Pos: math.NaN(), Col: ok[0].Col}, {Pos: 1, Col: ok[1].Col}}, 4, ErrBadPosition},
		{"inf position", []GradientStop{{Pos: 0, Col: ok[0].Col}, {Pos: math.Inf(1), Col: ok[1].Col}}, 4, ErrBadPosition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PerceptualGradient(tc.stops, tc.count); !errorIs(err, tc.target) {
				t.Errorf("got err %v, want wrapping %v", err, tc.target)
			}
		})
	}

	if _, err := PerceptualGradientEx(ok, 4, GradientSettings{MaxStep: math.NaN()}); !errorIs(err, ErrNonFiniteSetting) {
		t.Errorf("non-finite MaxStep: got %v, want ErrNonFiniteSetting", err)
	}
}

// GamutClamp must return only valid sRGB colors while GamutIgnore preserves the
// raw out-of-gamut interpolation values. Red->blue in Lab genuinely leaves the
// gamut, so both branches are exercised on a real case.
func TestPerceptualGradientGamutPolicy(t *testing.T) {
	stops := []GradientStop{
		{Pos: 0, Col: mustHex(t, "#ff0000")},
		{Pos: 1, Col: mustHex(t, "#0000ff")},
	}
	const n = 24

	clamped, err := PerceptualGradientEx(stops, n, GradientSettings{Gamut: GamutClamp})
	if err != nil {
		t.Fatalf("clamped gradient: %v", err)
	}
	for i, c := range clamped {
		if !c.IsValid() {
			t.Errorf("clamped color %d out of gamut: %v", i, c)
		}
	}

	raw, err := PerceptualGradientEx(stops, n, GradientSettings{Gamut: GamutIgnore})
	if err != nil {
		t.Fatalf("raw gradient: %v", err)
	}
	sawOutOfGamut := false
	for _, c := range raw {
		if !c.IsValid() {
			sawOutOfGamut = true
		}
	}
	if !sawOutOfGamut {
		t.Error("expected at least one out-of-gamut color with GamutIgnore, found none")
	}
	// Endpoints stay exact under either policy because they are in-gamut.
	if raw[0] != stops[0].Col || raw[n-1] != stops[1].Col {
		t.Error("GamutIgnore must not alter the exact endpoints")
	}
	// Clamping must only move out-of-gamut colors, and never past the cube.
	for i := range raw {
		if raw[i].IsValid() && raw[i] != clamped[i] {
			t.Errorf("in-gamut color %d was altered by clamping", i)
		}
	}
}

// A smaller MaxStep tessellates more finely and must not move the result beyond
// the coarse step size; the MaxSegments cap must keep endpoints exact.
func TestPerceptualGradientPrecision(t *testing.T) {
	stops := saturatedStops(t)
	coarse, err := PerceptualGradientEx(stops, 32, GradientSettings{MaxStep: 0.1, Gamut: GamutIgnore})
	if err != nil {
		t.Fatalf("coarse: %v", err)
	}
	fine, err := PerceptualGradientEx(stops, 32, GradientSettings{MaxStep: 0.001, Gamut: GamutIgnore})
	if err != nil {
		t.Fatalf("fine: %v", err)
	}
	for i := range coarse {
		if d := coarse[i].DistanceLab(fine[i]); d > 0.1 {
			t.Errorf("color %d moved by %v when refining, want <= coarse MaxStep 0.1", i, d)
		}
	}
	capped, err := PerceptualGradientEx(stops, 32, GradientSettings{MaxStep: 1e-9, MaxSegments: 8, Gamut: GamutIgnore})
	if err != nil {
		t.Fatalf("capped: %v", err)
	}
	if capped[0] != stops[0].Col || capped[len(capped)-1] != stops[len(stops)-1].Col {
		t.Error("capped gradient lost exact endpoints")
	}
}

// The Luv space must produce points matching the arc-length reference measured
// directly in Luv coordinates, even though its path through saturated colors
// leaves the sRGB gamut and sRGB-chord distances would be misleading.
func TestPerceptualGradientLuvSpace(t *testing.T) {
	stops := saturatedStops(t)
	const n = 40
	out, err := PerceptualGradientEx(stops, n, GradientSettings{Space: LuvSpace(), Gamut: GamutIgnore})
	if err != nil {
		t.Fatalf("Luv gradient: %v", err)
	}
	want := referenceArcSamples(LuvSpace(), stops, n, DefaultMaxStep)
	for i := 1; i < n-1; i++ {
		got := LuvSpace().From(out[i])
		if d := euclid(got, want[i]); d > 1e-10 {
			t.Errorf("Luv point %d deviates from reference by %v", i, d)
		}
	}
	if out[0] != stops[0].Col || out[n-1] != stops[len(stops)-1].Col {
		t.Error("Luv gradient endpoints not exact")
	}
}

// OkLab must also match the reference, within tolerance for its larger sRGB
// round-trip error (OkLab inversion is less exact than Lab/Luv for these hues).
func TestPerceptualGradientOkLabSpace(t *testing.T) {
	stops := saturatedStops(t)
	const n = 40
	out, err := PerceptualGradientEx(stops, n, GradientSettings{Space: OkLabSpace(), Gamut: GamutIgnore})
	if err != nil {
		t.Fatalf("OkLab gradient: %v", err)
	}
	want := referenceArcSamples(OkLabSpace(), stops, n, DefaultMaxStep)
	for i := 1; i < n-1; i++ {
		got := OkLabSpace().From(out[i])
		if d := euclid(got, want[i]); d > 5e-4 {
			t.Errorf("OkLab point %d deviates from reference by %v", i, d)
		}
	}
}

// Positions only order the stops; their absolute spacing must not influence the
// arc-length result. Repeated runs must also be byte-for-byte deterministic.
func TestPerceptualGradientPositionIndependentAndStable(t *testing.T) {
	a := saturatedStops(t)
	b := make([]GradientStop, len(a))
	for i := range a {
		b[i] = GradientStop{Pos: float64(i) * float64(i), Col: a[i].Col} // nonlinear but increasing
	}
	outA, err := PerceptualGradientEx(a, 37, GradientSettings{Gamut: GamutIgnore})
	if err != nil {
		t.Fatal(err)
	}
	outB, err := PerceptualGradientEx(b, 37, GradientSettings{Gamut: GamutIgnore})
	if err != nil {
		t.Fatal(err)
	}
	for i := range outA {
		if outA[i] != outB[i] {
			t.Errorf("color %d depends on position values: %v vs %v", i, outA[i], outB[i])
		}
	}
	outA2, _ := PerceptualGradientEx(a, 37, GradientSettings{Gamut: GamutIgnore})
	for i := range outA {
		if outA[i] != outA2[i] {
			t.Errorf("non-deterministic output at %d: %v vs %v", i, outA[i], outA2[i])
		}
	}
}

// Two stops with count == 2 returns exactly the two endpoints.
func TestPerceptualGradientMinimal(t *testing.T) {
	stops := []GradientStop{
		{Pos: 0, Col: mustHex(t, "#112233")},
		{Pos: 5, Col: mustHex(t, "#445566")},
	}
	out, err := PerceptualGradient(stops, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != stops[0].Col || out[1] != stops[1].Col {
		t.Errorf("minimal gradient = %v, want the two exact endpoints", out)
	}
}

// The wrapped failure sentinels remain distinct and identifiable.
func TestGradientErrorSentinelsDistinct(t *testing.T) {
	sentinels := []error{ErrEmptyGradient, ErrBadStopCount, ErrNonFiniteColor, ErrOutOfGamut, ErrBadPosition, ErrBadCount, ErrNonFiniteSetting}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && a == b {
				t.Errorf("sentinels %d and %d are identical", i, j)
			}
		}
		if !errorIs(wrapErr(a, "ctx"), a) {
			t.Errorf("wrapErr did not preserve %v", a)
		}
	}
	if errorIs(nil, ErrEmptyGradient) {
		t.Error("nil error must not match a sentinel")
	}
}

var benchGradientSink []Color

func BenchmarkPerceptualGradient(b *testing.B) {
	hexes := []string{"#000000", "#ff0000", "#ffff00", "#00ff00", "#0000ff", "#ff00ff", "#ffffff"}
	stops := make([]GradientStop, len(hexes))
	for i, h := range hexes {
		c, _ := Hex(h)
		stops[i] = GradientStop{Pos: float64(i), Col: c}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchGradientSink, _ = PerceptualGradient(stops, 64)
	}
}
