package colorful

import (
	"math"
	"testing"
)

// Display-P3 primaries and secondaries, all outside the sRGB gamut.
var displayP3OutOfGamut = []struct {
	name string
	c    Color
}{
	{"P3 red", DisplayP3(1, 0, 0)},
	{"P3 green", DisplayP3(0, 1, 0)},
	{"P3 blue", DisplayP3(0, 0, 1)},
	{"P3 cyan", DisplayP3(0, 1, 1)},
	{"P3 magenta", DisplayP3(1, 0, 1)},
	{"P3 yellow", DisplayP3(1, 1, 0)},
	{"P3 saturated orange", DisplayP3(1, 0.5, 0)},
	{"P3 deep pink", DisplayP3(1, 0.2, 0.6)},
}

func hueDelta(h1, h2 float64) float64 {
	d := math.Abs(h1 - h2)
	if d > 180 {
		d = 360 - d
	}
	return d
}

// The mapped color must be a legal sRGB color with (nearly) the original
// OkLch lightness and hue, and a reduced chroma.
func TestGamutMapOkLchDisplayP3Primaries(t *testing.T) {
	for _, tt := range displayP3OutOfGamut {
		if tt.c.IsValid() {
			t.Fatalf("%v: test color %v is unexpectedly already in sRGB gamut", tt.name, tt.c)
		}
		mapped, err := tt.c.GamutMapOkLch()
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", tt.name, err)
		}
		if !mapped.IsValid() {
			t.Errorf("%v: mapped color %v is not a legal sRGB color", tt.name, mapped)
		}

		l0, c0, h0 := tt.c.OkLch()
		l1, c1, h1 := mapped.OkLch()

		if math.Abs(l1-l0) > 1e-3 {
			t.Errorf("%v: lightness changed from %v to %v", tt.name, l0, l1)
		}
		if hueDelta(h0, h1) > 0.5 {
			t.Errorf("%v: hue changed from %v to %v degrees", tt.name, h0, h1)
		}
		if c1 >= c0 {
			t.Errorf("%v: chroma not reduced: %v -> %v", tt.name, c0, c1)
		}
	}
}

// Chroma reduction must preserve hue better than naive per-channel clamping.
func TestGamutMapOkLchBeatsClamping(t *testing.T) {
	for _, tt := range displayP3OutOfGamut {
		_, _, h0 := tt.c.OkLch()
		_, _, hClamped := tt.c.Clamped().OkLch()
		clampShift := hueDelta(h0, hClamped)
		// P3 and sRGB share the blue primary, so clamping is exact there;
		// only compare where clamping actually distorts the hue.
		if clampShift < 0.1 {
			continue
		}
		mapped, err := tt.c.GamutMapOkLch()
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", tt.name, err)
		}
		_, _, hMapped := mapped.OkLch()
		if mappedShift := hueDelta(h0, hMapped); mappedShift >= clampShift {
			t.Errorf("%v: hue shift %v deg not better than clamping's %v deg",
				tt.name, mappedShift, clampShift)
		}
	}
}

// Colors already inside the sRGB gamut are returned unchanged.
func TestGamutMapOkLchInGamutUnchanged(t *testing.T) {
	for _, tt := range vals {
		mapped, err := tt.c.GamutMapOkLch()
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", tt.c, err)
		}
		if mapped != tt.c {
			t.Errorf("in-gamut color %v changed to %v", tt.c, mapped)
		}
	}
}

// Mapping an out-of-gamut color twice must be stable (idempotent).
func TestGamutMapOkLchIdempotent(t *testing.T) {
	for _, tt := range displayP3OutOfGamut {
		once, err := tt.c.GamutMapOkLch()
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", tt.name, err)
		}
		twice, err := once.GamutMapOkLch()
		if err != nil {
			t.Fatalf("%v: unexpected error remapping %v: %v", tt.name, once, err)
		}
		if twice != once {
			t.Errorf("%v: not idempotent: %v -> %v -> %v", tt.name, tt.c, once, twice)
		}
	}
}

// A grayscale ramp is in gamut and must pass through untouched, including
// the black and white endpoints.
func TestGamutMapOkLchGrayscale(t *testing.T) {
	for i := 0; i <= 100; i++ {
		v := float64(i) / 100
		c := Color{v, v, v}
		mapped, err := c.GamutMapOkLch()
		if err != nil {
			t.Fatalf("gray %v: unexpected error: %v", v, err)
		}
		if mapped != c.Clamped() {
			t.Errorf("gray %v changed to %v", c, mapped)
		}
	}
}

// Near-black and near-white wide-gamut colors must map to valid colors
// without lightness jumps, and the endpoints snap exactly to black/white.
func TestGamutMapOkLchEndpoints(t *testing.T) {
	// Colors just inside the endpoints.
	nearBlack := DisplayP3(0.002, 0.0, 0.0)
	nearWhite := DisplayP3(1.0, 1.0, 0.998)
	black := Color{0, 0, 0}
	white := Color{1, 1, 1}

	for _, c := range []Color{nearBlack, nearWhite} {
		mapped, err := c.GamutMapOkLch()
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", c, err)
		}
		if !mapped.IsValid() {
			t.Errorf("%v mapped to invalid sRGB color %v", c, mapped)
		}
		l0, _, _ := c.OkLch()
		l1, _, _ := mapped.OkLch()
		if math.Abs(l1-l0) > 1e-3 {
			t.Errorf("%v: lightness jumped from %v to %v", c, l0, l1)
		}
	}

	// The endpoints themselves.
	if mapped, err := black.GamutMapOkLch(); err != nil || mapped != black {
		t.Errorf("black mapped to %v (err=%v)", mapped, err)
	}
	if mapped, err := white.GamutMapOkLch(); err != nil || mapped != white {
		t.Errorf("white mapped to %v (err=%v)", mapped, err)
	}

	// Endpoint neighborhoods are continuous: a tiny lightness step near
	// black/white must not cause a large output jump.
	for _, base := range []float64{0.0, 1.0} {
		prev, err := Color{base, base, base}.GamutMapOkLch()
		if err != nil {
			t.Fatalf("endpoint %v: unexpected error: %v", base, err)
		}
		for d := 1e-4; d < 1e-2; d *= 1.5 {
			v := base + d
			if base == 1.0 {
				v = base - d
			}
			cur, err := Color{v, v, v}.GamutMapOkLch()
			if err != nil {
				t.Fatalf("gray %v: unexpected error: %v", v, err)
			}
			if math.Abs(cur.R-prev.R) > 2*d {
				t.Errorf("discontinuity near %v: gray %v -> %v, previous %v", base, v, cur, prev)
			}
			prev = cur
		}
	}
}

// Non-finite channels and unrepresentable lightness must produce explicit
// errors, never a silently clamped color.
func TestGamutMapOkLchErrors(t *testing.T) {
	invalid := []struct {
		name string
		c    Color
	}{
		{"NaN red", Color{math.NaN(), 0, 0}},
		{"NaN green", Color{0, math.NaN(), 0}},
		{"NaN blue", Color{0, 0, math.NaN()}},
		{"+Inf", Color{math.Inf(1), 0.5, 0.5}},
		{"-Inf", Color{0.5, math.Inf(-1), 0.5}},
		{"lightness above 1", Color{2, 2, 2}},
		{"lightness below 0", Color{-1, -1, -1}},
		{"huge channel", Color{1e300, 0, 0}},
	}
	for _, tt := range invalid {
		mapped, err := tt.c.GamutMapOkLch()
		if err == nil {
			t.Errorf("%v: expected error, got color %v", tt.name, mapped)
		}
	}
}

// Boundary colors: sRGB primaries expressed in Display-P3 sit exactly on
// the gamut boundary and must survive the round trip essentially unchanged.
func TestGamutMapOkLchBoundaryColors(t *testing.T) {
	primaries := []Color{
		{1, 0, 0}, {0, 1, 0}, {0, 0, 1},
		{1, 1, 0}, {0, 1, 1}, {1, 0, 1},
	}
	for _, c := range primaries {
		r, g, b := c.DisplayP3()
		wide := DisplayP3(r, g, b)
		mapped, err := wide.GamutMapOkLch()
		if err != nil {
			t.Fatalf("%v: unexpected error: %v", c, err)
		}
		if !mapped.AlmostEqualRgb(c) {
			t.Errorf("boundary color %v via DisplayP3 mapped to %v", c, mapped)
		}
	}
}
