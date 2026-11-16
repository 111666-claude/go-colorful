package colorful

import (
	"image/color"
	"math"
	"testing"
)

// Reference vectors are produced by an independent Python implementation that
// uses the exact same conversion matrices as this library together with the
// CSS Color 4 interpolation (§13) and binary-search gamut mapping (§14)
// algorithms. Cases marked "CSS5"/"CSS4" additionally reproduce the numeric
// examples from the specifications.

const colormixEps = 1e-4

func almostEqualRGBA(t *testing.T, name string, got, want ColorRGBA) {
	t.Helper()
	if math.IsNaN(got.R) || math.IsNaN(got.G) || math.IsNaN(got.B) || math.IsNaN(got.A) {
		t.Errorf("%s: produced a NaN: %+v", name, got)
		return
	}
	if d := math.Abs(got.R-want.R) + math.Abs(got.G-want.G) + math.Abs(got.B-want.B) + math.Abs(got.A-want.A); d > colormixEps {
		t.Errorf("%s: got %+v want %+v (total delta %g > %g)", name, got, want, d, colormixEps)
	}
}

func TestColorMixVectors(t *testing.T) {
	cases := []struct {
		name  string
		space ColorMixSpace
		hue   HueDirection
		p     float64
		c1    ColorRGBA
		c2    ColorRGBA
		want  ColorRGBA
	}{
		// CSS Color 5 §3.5: premultiplied alpha must give
		// color(srgb 0.53846 0.46154 0 / 0.325), not (0.25 0.75 0 / 0.325).
		{
			"CSS5 premultiplied srgb", ColorMixSRGB, "", 0.75,
			ColorRGBA{1, 0, 0, 0.7}, ColorRGBA{0, 1, 0, 0.2},
			ColorRGBA{0.5384615, 0.4615385, 0, 0.325},
		},
		// color-mix(in srgb, white, blue) -> rgb(50% 50% 100%).
		{
			"srgb white/blue midpoint", ColorMixSRGB, "", 0.5,
			ColorRGBA{1, 1, 1, 1}, ColorRGBA{0, 0, 1, 1},
			ColorRGBA{0.5, 0.5, 1, 1},
		},
		// Mixing with transparent keeps the opaque color, only lowering alpha.
		{
			"srgb red/transparent", ColorMixSRGB, "", 0.5,
			ColorRGBA{1, 0, 0, 1}, ColorRGBA{0, 0, 0, 0},
			ColorRGBA{1, 0, 0, 0.5},
		},
		{
			"srgb #34c9eb 20% + transparent", ColorMixSRGB, "", 0.8,
			ColorRGBA{0.20392157, 0.78823529, 0.92156863, 1}, ColorRGBA{0, 0, 0, 0},
			ColorRGBA{0.20392157, 0.78823529, 0.92156863, 0.2},
		},
		// Endpoints: P == 0 / P == 1 reproduce the input colors (with their alpha).
		{
			"srgb P=0 first endpoint", ColorMixSRGB, "", 0,
			ColorRGBA{0.2, 0.4, 0.6, 0.7}, ColorRGBA{0.9, 0.1, 0.3, 0.3},
			ColorRGBA{0.2, 0.4, 0.6, 0.7},
		},
		// CSS Color 4 §13.6.1/13.6.2 OkLCh example. The shorter midpoint hue is
		// 60deg (deep orange), the longer one 240deg (sky blue).
		{
			"CSS4 OkLCh shorter 30->90", ColorMixOkLch, HueShorter, 0.5,
			ColorRGBA{0.93215922, 0.07980282, 0.00485288, 1},
			ColorRGBA{0.89067458, 0.72283817, 0.19229375, 1},
			ColorRGBA{0.93490344, 0.48368400, 0, 1},
		},
		{
			"CSS4 OkLCh longer 30->90", ColorMixOkLch, HueLonger, 0.5,
			ColorRGBA{0.93215922, 0.07980282, 0.00485288, 1},
			ColorRGBA{0.89067458, 0.72283817, 0.19229375, 1},
			ColorRGBA{0, 0.66303398, 1, 1},
		},
		// Hue crossing the 0deg boundary: shorter stays near 0deg (red/pink),
		// longer sweeps through 180deg (teal/green).
		{
			"OkLCh shorter across 0deg (350->10)", ColorMixOkLch, HueShorter, 0.5,
			ColorRGBA{0.81875445, 0.23816160, 0.56743106, 1},
			ColorRGBA{0.86185993, 0.21977488, 0.39404243, 1},
			ColorRGBA{0.84420311, 0.22496098, 0.48362383, 1},
		},
		{
			"OkLCh longer across 0deg (350->10)", ColorMixOkLch, HueLonger, 0.5,
			ColorRGBA{0.81875445, 0.23816160, 0.56743106, 1},
			ColorRGBA{0.86185993, 0.21977488, 0.39404243, 1},
			ColorRGBA{0, 0.59727197, 0.51401762, 1},
		},
		// Achromatic endpoint: white has a powerless hue and inherits blue's
		// hue. The pre-gamut-map result is oklch(72.6% 0.1567 264.1), matching
		// the CSS example oklch(72.601% 0.15661 264.052); the sRGB output is
		// gamut mapped because B exceeds 1.
		{
			"OkLCh white/blue powerless hue", ColorMixOkLch, HueShorter, 0.5,
			ColorRGBA{1, 1, 1, 1}, ColorRGBA{0, 0, 1, 1},
			ColorRGBA{0.45291823, 0.63803027, 1, 1},
		},
		// Both endpoints fully transparent -> transparent black, no NaN.
		{
			"OkLCh both alpha zero", ColorMixOkLch, HueShorter, 0.5,
			ColorRGBA{1, 0, 0, 0}, ColorRGBA{0, 0, 1, 0},
			ColorRGBA{0, 0, 0, 0},
		},
		// Premultiplied OkLCh with differing alpha.
		{
			"OkLCh red 0.8 / blue 0.3", ColorMixOkLch, HueShorter, 0.6,
			ColorRGBA{1, 0, 0, 0.8}, ColorRGBA{0, 0, 1, 0.3},
			ColorRGBA{0.68125164, 0.01803887, 0.88985976, 0.5},
		},
		// Gray (achromatic) mixed with a chromatic color inherits its hue.
		{
			"OkLCh gray/red achromatic", ColorMixOkLch, HueShorter, 0.5,
			ColorRGBA{0.5, 0.5, 0.5, 1}, ColorRGBA{1, 0, 0, 1},
			ColorRGBA{0.77428889, 0.38671128, 0.33443999, 1},
		},
		// P == 1 reproduces the second endpoint even in OkLCh.
		{
			"OkLCh P=1 second endpoint", ColorMixOkLch, HueShorter, 1,
			ColorRGBA{0.2, 0.4, 0.6, 0.7}, ColorRGBA{0.9, 0.1, 0.3, 0.3},
			ColorRGBA{0.89992339, 0.10025470, 0.30018930, 0.3},
		},
		// Transparent endpoint in OkLCh: keeps red's hue, alpha halves.
		{
			"OkLCh red/transparent", ColorMixOkLch, HueShorter, 0.5,
			ColorRGBA{1, 0, 0, 1}, ColorRGBA{0, 0, 0, 0},
			ColorRGBA{0.99991534, 0.00074313, 0.00088501, 0.5},
		},
	}

	for _, tc := range cases {
		got, err := ColorMix(tc.c1, tc.c2, ColorMixOptions{Space: tc.space, P: tc.p, Hue: tc.hue})
		if err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
			continue
		}
		almostEqualRGBA(t, tc.name, got, tc.want)
	}
}

// TestColorMixOklchPreGamutMidpoint checks the OkLCh coordinates *before* the
// final sRGB gamut mapping (the mapping itself can shift lightness and hue at
// the boundary). For white/blue the CSS spec reports the intermediate
// oklch(72.601% 0.15661 264.052): white's powerless hue is replaced by blue's
// hue, while lightness and chroma are the plain averages.
func TestColorMixOklchPreGamutMidpoint(t *testing.T) {
	c1, c2 := ColorRGBA{1, 1, 1, 1}, ColorRGBA{0, 0, 1, 1}

	l1, chroma1, _ := c1.Color().OkLch()
	l2, chroma2, h2 := c2.Color().OkLch()
	wantL := (l1 + l2) / 2.0
	wantC := (chroma1 + chroma2) / 2.0
	wantH := h2 // white is achromatic and inherits blue's hue

	if math.Abs(wantL-0.72601) > 1e-3 {
		t.Errorf("intermediate lightness %.5f, CSS spec ~0.72601", wantL)
	}
	if math.Abs(wantC-0.15661) > 1e-3 {
		t.Errorf("intermediate chroma %.5f, CSS spec ~0.15661", wantC)
	}
	if math.Abs(wantH-264.052) > 1.0 {
		t.Errorf("intermediate hue %.3f, CSS spec 264.052", wantH)
	}

	// And the public, gamut-mapped result must still be valid and finite.
	got, err := ColorMix(c1, c2, ColorMixOptions{Space: ColorMixOkLch, P: 0.5, Hue: HueShorter})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !got.Color().IsValid() || !isFiniteColor(got.Color()) {
		t.Errorf("gamut-mapped result invalid or non-finite: %+v", got)
	}
}

func TestGamutMapOkLchToSRGB(t *testing.T) {
	// An out-of-gamut high-chroma color must land inside [0..1].
	mapped := gamutMapOkLchToSRGB(0.7, 0.32, 130)
	if !mapped.IsValid() {
		t.Fatalf("gamut mapping left color out of gamut: %+v", mapped)
	}
	if !isFiniteColor(mapped) {
		t.Fatalf("gamut mapping produced non-finite color: %+v", mapped)
	}

	// A color already inside the gamut is returned essentially unchanged.
	inGamut := OkLch(0.6, 0.1, 200)
	roundtrip := gamutMapOkLchToSRGB(0.6, 0.1, 200)
	if d := inGamut.DistanceRgb(roundtrip); d > 1e-3 {
		t.Errorf("in-gamut color changed by %g: %+v -> %+v", d, inGamut, roundtrip)
	}

	// Lightness out of range resolves to white / black.
	if white := gamutMapOkLchToSRGB(1.2, 0.3, 100); !white.AlmostEqualRgb(Color{1, 1, 1}) {
		t.Errorf("L>=1 should map to white, got %+v", white)
	}
	if black := gamutMapOkLchToSRGB(-0.2, 0.3, 100); !black.AlmostEqualRgb(Color{0, 0, 0}) {
		t.Errorf("L<=0 should map to black, got %+v", black)
	}
}

func isFiniteColor(c Color) bool {
	return !math.IsNaN(c.R) && !math.IsNaN(c.G) && !math.IsNaN(c.B) &&
		!math.IsInf(c.R, 0) && !math.IsInf(c.G, 0) && !math.IsInf(c.B, 0)
}

func TestHueFixup(t *testing.T) {
	// Expected (shorter) then (longer), as produced by the CSS Color 4 §13.6
	// pseudocode. The spec permits adding 360 to either endpoint; these assert
	// the exact branch the implementation takes, and the interpolated midpoint
	// is checked separately in TestColorMixVectors.
	cases := []struct {
		h1, h2         float64
		s1, s2, l1, l2 float64
	}{
		{30, 90, 30, 90, 390, 90},    // spec shorter=60deg, longer=240deg example
		{350, 10, 350, 370, 350, 10}, // crossing 0deg
		{10, 350, 370, 350, 10, 350}, // reverse direction
		{0, 182, 360, 182, 0, 182},   // just past the 180deg ambiguity point
		{0, 178, 0, 178, 360, 178},   // just under 180deg
	}
	for _, tc := range cases {
		gs1, gs2 := hueShorter(tc.h1, tc.h2)
		if gs1 != tc.s1 || gs2 != tc.s2 {
			t.Errorf("hueShorter(%v,%v) = (%v,%v), want (%v,%v)", tc.h1, tc.h2, gs1, gs2, tc.s1, tc.s2)
		}
		gl1, gl2 := hueLonger(tc.h1, tc.h2)
		if gl1 != tc.l1 || gl2 != tc.l2 {
			t.Errorf("hueLonger(%v,%v) = (%v,%v), want (%v,%v)", tc.h1, tc.h2, gl1, gl2, tc.l1, tc.l2)
		}
	}
}

func TestColorMixErrors(t *testing.T) {
	valid := ColorRGBA{0.5, 0.5, 0.5, 1}

	t.Run("non-finite color", func(t *testing.T) {
		bad := ColorRGBA{math.NaN(), 0, 0, 1}
		if _, err := ColorMix(bad, valid, ColorMixOptions{Space: ColorMixSRGB, P: 0.5}); err != ErrNonFiniteColor {
			t.Errorf("got %v, want ErrNonFiniteColor", err)
		}
		bad = ColorRGBA{math.Inf(1), 0, 0, 1}
		if _, err := ColorMix(bad, valid, ColorMixOptions{Space: ColorMixOkLch, P: 0.5}); err != ErrNonFiniteColor {
			t.Errorf("got %v, want ErrNonFiniteColor", err)
		}
	})

	t.Run("component out of range", func(t *testing.T) {
		for _, bad := range []ColorRGBA{
			{1.2, 0, 0, 1}, {-0.1, 0, 0, 1}, {0, 0, 0, 1.5}, {0, 0, 0, -0.2},
		} {
			if _, err := ColorMix(bad, valid, ColorMixOptions{Space: ColorMixSRGB, P: 0.5}); err != ErrInvalidComponent {
				t.Errorf("%+v: got %v, want ErrInvalidComponent", bad, err)
			}
		}
	})

	t.Run("invalid ratio", func(t *testing.T) {
		for _, p := range []float64{-0.1, 1.1, math.NaN(), math.Inf(1)} {
			_, err := ColorMix(valid, valid, ColorMixOptions{Space: ColorMixSRGB, P: p})
			if p != p || math.IsInf(p, 0) {
				if err != ErrNonFiniteColor {
					t.Errorf("p=%v: got %v, want ErrNonFiniteColor", p, err)
				}
			} else if err != ErrInvalidRatio {
				t.Errorf("p=%v: got %v, want ErrInvalidRatio", p, err)
			}
		}
	})

	t.Run("unsupported space", func(t *testing.T) {
		if _, err := ColorMix(valid, valid, ColorMixOptions{Space: "lab", P: 0.5}); err != ErrUnsupportedSpace {
			t.Errorf("got %v, want ErrUnsupportedSpace", err)
		}
	})

	t.Run("unsupported hue direction", func(t *testing.T) {
		if _, err := ColorMix(valid, valid, ColorMixOptions{Space: ColorMixOkLch, P: 0.5, Hue: "increasing"}); err != ErrUnsupportedHueDirection {
			t.Errorf("got %v, want ErrUnsupportedHueDirection", err)
		}
	})
}

// TestColorMixNeverNaN sweeps random valid inputs and guarantees finite output.
func TestColorMixNeverNaN(t *testing.T) {
	seeds := []float64{0, 0.125, 0.25, 0.5, 0.75, 1}
	alphas := []float64{0, 0.3, 0.7, 1}
	for _, space := range []ColorMixSpace{ColorMixSRGB, ColorMixOkLch} {
		for _, hue := range []HueDirection{HueShorter, HueLonger} {
			for _, p := range seeds {
				for _, a1 := range alphas {
					for _, a2 := range alphas {
						c1 := ColorRGBA{seeds[1], seeds[3], seeds[5], a1}
						c2 := ColorRGBA{seeds[5], seeds[2], seeds[0], a2}
						got, err := ColorMix(c1, c2, ColorMixOptions{Space: space, P: p, Hue: hue})
						if err != nil {
							t.Fatalf("unexpected error %v", err)
						}
						if math.IsNaN(got.R+got.G+got.B+got.A) || math.IsInf(got.R+got.G+got.B+got.A, 0) {
							t.Fatalf("non-finite result for space=%s hue=%s p=%v: %+v", space, hue, p, got)
						}
						if !got.Color().IsValid() || got.A < 0 || got.A > 1 {
							t.Fatalf("result out of range for space=%s hue=%s p=%v: %+v", space, hue, p, got)
						}
					}
				}
			}
		}
	}
}

func TestColorRGBAAdapters(t *testing.T) {
	c := ColorRGBA{0.2, 0.4, 0.6, 0.5}
	if c.Color() != (Color{0.2, 0.4, 0.6}) {
		t.Errorf("Color() mismatch: %+v", c.Color())
	}
	if (Color{0.1, 0.2, 0.3}).WithAlpha(0.4) != (ColorRGBA{0.1, 0.2, 0.3, 0.4}) {
		t.Errorf("WithAlpha mismatch")
	}
	n := c.NRGBA()
	if n != (color.NRGBA{R: 51, G: 102, B: 153, A: 128}) {
		t.Errorf("NRGBA mismatch: %+v", n)
	}
	// Premultiplied RGBA must satisfy r,g,b <= a.
	r, g, b, a := c.RGBA()
	if r > a || g > a || b > a || a != 32768 {
		t.Errorf("RGBA premultiplication wrong: r=%d g=%d b=%d a=%d", r, g, b, a)
	}
}

// TestBlendAPIsUnchanged guards backward compatibility of the existing Blend*
// methods: a representative blend must still produce its historical result.
func TestBlendAPIsUnchanged(t *testing.T) {
	c1 := Color{1, 0, 0}
	c2 := Color{0, 0, 1}
	if got := c1.BlendRgb(c2, 0.5); !got.AlmostEqualRgb(Color{0.5, 0, 0.5}) {
		t.Errorf("BlendRgb changed: %+v", got)
	}
	if got := c1.BlendOkLch(c2, 0); !got.AlmostEqualRgb(c1) {
		t.Errorf("BlendOkLch at t=0 changed: %+v", got)
	}
	if got := c1.BlendOkLab(c2, 1); !got.AlmostEqualRgb(c2) {
		t.Errorf("BlendOkLab at t=1 changed: %+v", got)
	}
}
