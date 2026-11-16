package colorful

import (
	"errors"
	"math"
	"testing"
)

// Reference values are taken verbatim from the public test suite of the
// APCA-W3 0.1.9 reference implementation (npm package apca-w3@0.1.9,
// test/index.js) and from running that reference implementation directly for
// the alpha and edge cases. See apca.go for the source link.

// apcaTol is far tighter than the ~1/255 tolerance used elsewhere: APCA must
// reproduce the published reference numbers, and Go's math.Pow matches the
// reference's Math.Pow to the last few digits.
const apcaTol = 1e-9

func mustHex(t testing.TB, s string) Color {
	t.Helper()
	c, err := Hex(s)
	if err != nil {
		t.Fatalf("Hex(%q): %v", s, err)
	}
	return c
}

func TestContrastAPCAOfficialPairs(t *testing.T) {
	// The nine published reference pairs, foreground (text) first,
	// background second. Polarity is part of the contract.
	cases := []struct {
		fg, bg string
		want   float64
	}{
		{"#888", "#fff", 63.056469930209424},
		{"#fff", "#888", -68.54146436644962},
		{"#000", "#aaa", 58.146262578561334},
		{"#aaa", "#000", -56.24113336839742},
		{"#123", "#def", 91.66830811481631},
		{"#def", "#123", -93.06770049484275},
		{"#123", "#444", 8.32326136957393},
		{"#444", "#123", -7.526878460278154},
	}
	for _, tc := range cases {
		fg, bg := mustHex(t, tc.fg), mustHex(t, tc.bg)
		got, err := fg.ContrastAPCA(bg)
		if err != nil {
			t.Fatalf("ContrastAPCA(%s,%s): unexpected error %v", tc.fg, tc.bg, err)
		}
		if math.Abs(got-tc.want) > apcaTol {
			t.Errorf("ContrastAPCA(%s on %s) = %v, want %v", tc.fg, tc.bg, got, tc.want)
		}
		if math.IsNaN(got) {
			t.Errorf("ContrastAPCA(%s on %s) returned NaN", tc.fg, tc.bg)
		}
	}
}

func TestContrastAPCAPolarity(t *testing.T) {
	fg, bg := mustHex(t, "#123"), mustHex(t, "#def")
	darkOnLight, _ := fg.ContrastAPCA(bg)
	lightOnDark, _ := bg.ContrastAPCA(fg)

	// Dark text on a light background is positive, the reverse is negative.
	if darkOnLight <= 0 {
		t.Errorf("dark text on light background = %v, want positive", darkOnLight)
	}
	if lightOnDark >= 0 {
		t.Errorf("light text on dark background = %v, want negative", lightOnDark)
	}
	// The magnitudes intentionally differ: APCA is not symmetric.
	if darkOnLight == -lightOnDark {
		t.Errorf("swapped contrasts must differ, got %v and %v", darkOnLight, lightOnDark)
	}
}

func TestContrastAPCAExtremes(t *testing.T) {
	black, white := mustHex(t, "#000"), mustHex(t, "#fff")

	cases := []struct {
		name        string
		fg, bg      Color
		want        float64
		exactZero   bool
		refPositive bool
		refNegative bool
	}{
		{"black on white", black, white, 106.04067321268862, false, true, false},
		{"white on black", white, black, -107.88473318309848, false, false, true},
		{"white on white", white, white, 0, true, false, false},
		{"black on black", black, black, 0, true, false, false},
		{"gray on itself", mustHex(t, "#808080"), mustHex(t, "#808080"), 0, true, false, false},
	}
	for _, tc := range cases {
		got, err := tc.fg.ContrastAPCA(tc.bg)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", tc.name, err)
		}
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Fatalf("%s: non-finite result %v", tc.name, got)
		}
		if tc.exactZero {
			if got != 0.0 {
				t.Errorf("%s = %v, want exactly 0", tc.name, got)
			}
			continue
		}
		if math.Abs(got-tc.want) > apcaTol {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
		if tc.refPositive && got < 0 {
			t.Errorf("%s = %v, want positive", tc.name, got)
		}
		if tc.refNegative && got > 0 {
			t.Errorf("%s = %v, want negative", tc.name, got)
		}
	}
}

func TestContrastAPCANearBlackStability(t *testing.T) {
	// Reference output for neutral grays at these sRGB channel levels.
	// The soft black clamp keeps the transition at and below blkThrs (0.022)
	// smooth and finite on both polarities.
	cases := []struct {
		v        float64
		bow, wob float64
	}{
		{0.0, 106.04067321268862, -107.88473318309848},
		{0.005, 106.0392651429039, -107.88369046792683},
		{0.01, 106.03324421385648, -107.8792313593355},
		{0.02, 106.00154129806873, -107.85574034883234},
		{0.022, 105.99151469285964, -107.84830679052074},
		{0.023, 105.98599929432429, -107.84421692535886},
		{0.05, 105.69446295684573, -107.62719695838285},
		{0.1, 104.34510811605888, -106.60302792536702},
	}
	white := mustHex(t, "#fff")
	for _, tc := range cases {
		gray := Color{tc.v, tc.v, tc.v}
		bow, err := gray.ContrastAPCA(white)
		if err != nil {
			t.Fatalf("v=%v: %v", tc.v, err)
		}
		wob, err := white.ContrastAPCA(gray)
		if err != nil {
			t.Fatalf("v=%v: %v", tc.v, err)
		}
		if math.Abs(bow-tc.bow) > apcaTol {
			t.Errorf("gray %v on white = %v, want %v", tc.v, bow, tc.bow)
		}
		if math.Abs(wob-tc.wob) > apcaTol {
			t.Errorf("white on gray %v = %v, want %v", tc.v, wob, tc.wob)
		}
	}
}

func TestContrastAPCALowClipAndDeltaY(t *testing.T) {
	// Two grays one 8-bit step apart fall under the loClip threshold and
	// report zero on both polarities; near-identical luminances hit the
	// deltaYmin early return. Neither path may flip polarity.
	c1 := Color{120.0 / 255.0, 120.0 / 255.0, 120.0 / 255.0}
	c2 := Color{121.0 / 255.0, 121.0 / 255.0, 121.0 / 255.0}
	if got, _ := c1.ContrastAPCA(c2); got != 0.0 {
		t.Errorf("close grays c1 on c2 = %v, want 0", got)
	}
	if got, _ := c2.ContrastAPCA(c1); got != 0.0 {
		t.Errorf("close grays c2 on c1 = %v, want 0", got)
	}
}

func TestContrastAPCAAlphaOfficial(t *testing.T) {
	// The reference suite's translucent case: #0006 is black with
	// alpha 0x66/255 = 0.4, composited over an opaque white background.
	black, white := mustHex(t, "#000"), mustHex(t, "#fff")
	got, err := AlphaColor(black, 0.4).ContrastAPCA(AlphaColor(white, 1.0), white)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(got-54.62184067441377) > apcaTol {
		t.Errorf("translucent black on white = %v, want 54.62184067441377", got)
	}

	// Opaque inputs through the alpha API must match the plain Color API.
	plain, _ := black.ContrastAPCA(white)
	viaAlpha, err := AlphaColor(black, 1.0).ContrastAPCA(AlphaColor(white, 1.0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain != viaAlpha {
		t.Errorf("opaque alpha API = %v, plain API = %v", viaAlpha, plain)
	}
}

func TestContrastAPCAAlphaSweep(t *testing.T) {
	// Black text of varying alpha over opaque white, reference values.
	black, white := mustHex(t, "#000"), mustHex(t, "#fff")
	cases := []struct {
		alpha, want float64
	}{
		{0.0, 0.0},
		{0.25, 34.38896630677129},
		{0.5, 67.13321580182021},
		{0.75, 94.18855656698307},
		{1.0, 106.04067321268862},
	}
	for _, tc := range cases {
		got, err := AlphaColor(black, tc.alpha).ContrastAPCA(AlphaColor(white, 1.0), white)
		if err != nil {
			t.Fatalf("alpha %v: %v", tc.alpha, err)
		}
		if math.Abs(got-tc.want) > apcaTol {
			t.Errorf("alpha %v = %v, want %v", tc.alpha, got, tc.want)
		}
	}
}

func TestContrastAPCATranslucentBackground(t *testing.T) {
	black, white := mustHex(t, "#000"), mustHex(t, "#fff")

	// Opaque black text, white background at alpha 0.5 over a black base.
	got, err := AlphaColor(black, 1.0).ContrastAPCA(AlphaColor(white, 0.5), black)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(got-36.948340848331) > apcaTol {
		t.Errorf("translucent white bg over black = %v, want 36.948340848331", got)
	}

	// Both layers translucent: black text 0.7 over white background 0.6,
	// black base.
	got, err = AlphaColor(black, 0.7).ContrastAPCA(AlphaColor(white, 0.6), black)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(got-43.5073136279602) > apcaTol {
		t.Errorf("both translucent = %v, want 43.5073136279602", got)
	}

	// White text, black background at alpha 0.4 over a white base.
	got, err = AlphaColor(white, 1.0).ContrastAPCA(AlphaColor(black, 0.4), white)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(got-(-59.916844601667485)) > apcaTol {
		t.Errorf("translucent black bg over white = %v, want -59.916844601667485", got)
	}
}

func TestContrastAPCATransparentWithoutBase(t *testing.T) {
	black, white := mustHex(t, "#000"), mustHex(t, "#fff")

	got, err := AlphaColor(black, 0.5).ContrastAPCA(AlphaColor(white, 1.0))
	if !errors.Is(err, ErrAPCATransparent) {
		t.Errorf("translucent foreground without base: err = %v, want ErrAPCATransparent", err)
	}
	if got != 0.0 || math.IsNaN(got) {
		t.Errorf("translucent foreground without base = %v, want finite 0", got)
	}

	got, err = AlphaColor(black, 1.0).ContrastAPCA(AlphaColor(white, 0.5))
	if !errors.Is(err, ErrAPCATransparent) {
		t.Errorf("translucent background without base: err = %v, want ErrAPCATransparent", err)
	}
	if got != 0.0 {
		t.Errorf("translucent background without base = %v, want 0", got)
	}

	// A fully transparent layer is likewise undefined without a base.
	got, err = AlphaColor(black, 0.0).ContrastAPCA(AlphaColor(white, 1.0))
	if !errors.Is(err, ErrAPCATransparent) {
		t.Errorf("fully transparent foreground without base: err = %v, want ErrAPCATransparent", err)
	}
	if got != 0.0 {
		t.Errorf("fully transparent foreground without base = %v, want 0", got)
	}
}

func TestContrastAPCANonFinite(t *testing.T) {
	black, white := mustHex(t, "#000"), mustHex(t, "#fff")
	nanC := Color{math.NaN(), 0.0, 0.0}
	infC := Color{0.0, math.Inf(1), 0.0}
	negInfC := Color{0.0, 0.0, math.Inf(-1)}

	plainCases := []struct {
		name   string
		fg, bg Color
	}{
		{"NaN foreground", nanC, white},
		{"+Inf background", black, infC},
		{"-Inf foreground", negInfC, white},
	}
	for _, tc := range plainCases {
		got, err := tc.fg.ContrastAPCA(tc.bg)
		if !errors.Is(err, ErrAPCANonFinite) {
			t.Errorf("%s: err = %v, want ErrAPCANonFinite", tc.name, err)
		}
		if got != 0.0 || math.IsNaN(got) {
			t.Errorf("%s: got %v, want finite 0", tc.name, got)
		}
	}

	// Non-finite alpha, including in the supplied base.
	got, err := AlphaColor(black, math.NaN()).ContrastAPCA(AlphaColor(white, 1.0), white)
	if !errors.Is(err, ErrAPCANonFinite) || got != 0.0 {
		t.Errorf("NaN alpha: got (%v, %v), want (0, ErrAPCANonFinite)", got, err)
	}
	got, err = AlphaColor(black, 0.5).ContrastAPCA(AlphaColor(white, 1.0), Color{math.Inf(1), 0, 0})
	if !errors.Is(err, ErrAPCANonFinite) || got != 0.0 {
		t.Errorf("non-finite base: got (%v, %v), want (0, ErrAPCANonFinite)", got, err)
	}
}

func TestContrastAPCAOutOfGamutClamps(t *testing.T) {
	white, black := mustHex(t, "#fff"), mustHex(t, "#000")

	// Channels outside [0..1] are treated like Clamped(), never producing
	// NaN; this matches the library's existing convention rather than the
	// reference's error-zero path.
	high := Color{1.5, 1.0, 1.0}
	low := Color{-0.2, 0.0, 0.0}

	got, err := high.ContrastAPCA(white)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ := high.Clamped().ContrastAPCA(white)
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("out-of-gamut high channel returned non-finite %v", got)
	}
	if got != want {
		t.Errorf("high channel = %v, want clamped result %v", got, want)
	}

	got, err = low.ContrastAPCA(white)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want, _ = low.Clamped().ContrastAPCA(white) // black on white
	if math.IsNaN(got) || got != want {
		t.Errorf("low channel = %v, want clamped result %v", got, want)
	}
	if blackOnWhite, _ := black.ContrastAPCA(white); got != blackOnWhite {
		t.Errorf("clamped-low = %v, want black-on-white %v", got, blackOnWhite)
	}

	// Alpha outside [0..1] clamps to the nearest endpoint.
	at, err := AlphaColor(black, 1.5).ContrastAPCA(AlphaColor(white, 1.0), white)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	opaque, _ := black.ContrastAPCA(white)
	if at != opaque {
		t.Errorf("alpha 1.5 = %v, want clamped-to-1 result %v", at, opaque)
	}
	at, err = AlphaColor(black, -0.5).ContrastAPCA(AlphaColor(white, 1.0), white)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if at != 0.0 {
		t.Errorf("alpha -0.5 = %v, want 0 (fully transparent over bg)", at)
	}
}

func TestColorAlphaIsValid(t *testing.T) {
	black := mustHex(t, "#000")
	if !AlphaColor(black, 0.5).IsValid() {
		t.Error("in-range ColorAlpha reported invalid")
	}
	if AlphaColor(black, 1.5).IsValid() {
		t.Error("alpha 1.5 reported valid")
	}
	if AlphaColor(Color{2.0, 0, 0}, 0.5).IsValid() {
		t.Error("out-of-gamut channel reported valid")
	}
}

func BenchmarkContrastAPCA(b *testing.B) {
	fg, bg := mustHex(b, "#123"), mustHex(b, "#def")
	var sink float64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, _ := fg.ContrastAPCA(bg)
		sink = v
	}
	bench_result = sink
}
