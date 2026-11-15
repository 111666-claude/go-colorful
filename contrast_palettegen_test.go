package colorful

import (
	"errors"
	"math"
	"testing"
)

// verifyContrastPalette checks every constraint the generator promises:
// displayable sRGB, WCAG contrast against the background, and pairwise
// CIEDE2000 distance.
func verifyContrastPalette(t *testing.T, pal []Color, bg Color, wantCount int, minContrast, minDistance float64) {
	t.Helper()

	if len(pal) != wantCount {
		t.Fatalf("got %d colors, want %d", len(pal), wantCount)
	}

	for i, col := range pal {
		if !col.IsValid() {
			t.Errorf("color %d is outside the sRGB gamut: %v", i, col)
		}
		if ratio := ContrastRatio(col, bg); ratio < minContrast-1e-9 {
			t.Errorf("color %d %v has contrast %.6f against background %v, want >= %v",
				i, col, ratio, bg, minContrast)
		}
	}

	for i := 0; i < len(pal); i++ {
		for j := i + 1; j < len(pal); j++ {
			if d := pal[i].DistanceCIEDE2000(pal[j]); d < minDistance-1e-9 {
				t.Errorf("colors %d (%v) and %d (%v) are too close: CIEDE2000 %.6f < %v",
					i, pal[i], j, pal[j], d, minDistance)
			}
		}
	}
}

func TestContrastPaletteDarkBackground(t *testing.T) {
	bg := Color{0.05, 0.05, 0.07}
	const count = 8
	const minContrast = 4.5
	const minDistance = 0.1

	pal, err := ContrastPalette(count, bg, minContrast, minDistance, 12345)
	if err != nil {
		t.Fatalf("ContrastPalette failed: %v", err)
	}
	verifyContrastPalette(t, pal, bg, count, minContrast, minDistance)
}

func TestContrastPaletteLightBackground(t *testing.T) {
	bg := Color{0.96, 0.96, 0.94}
	const count = 8
	const minContrast = 4.5
	const minDistance = 0.1

	pal, err := ContrastPalette(count, bg, minContrast, minDistance, 12345)
	if err != nil {
		t.Fatalf("ContrastPalette failed: %v", err)
	}
	verifyContrastPalette(t, pal, bg, count, minContrast, minDistance)
}

func TestContrastPaletteStrictConstraints(t *testing.T) {
	// WCAG AAA body text against a near-black HUD background. This is a
	// deliberately narrow slice of the gamut, so it also stresses the search.
	bg := Color{0.02, 0.02, 0.03}
	const count = 6
	const minContrast = 7.0
	const minDistance = 0.12

	pal, err := ContrastPalette(count, bg, minContrast, minDistance, 99)
	if err != nil {
		t.Fatalf("ContrastPalette failed: %v", err)
	}
	verifyContrastPalette(t, pal, bg, count, minContrast, minDistance)
}

func TestContrastPaletteSingleColor(t *testing.T) {
	bg := Color{0.9, 0.9, 0.9}
	pal, err := ContrastPalette(1, bg, 4.5, 0.5, 7)
	if err != nil {
		t.Fatalf("ContrastPalette failed: %v", err)
	}
	verifyContrastPalette(t, pal, bg, 1, 4.5, 0.5)
}

func TestContrastPaletteDeterministic(t *testing.T) {
	bg := Color{0.1, 0.1, 0.12}
	first, err := ContrastPalette(6, bg, 4.5, 0.1, 2024)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	second, err := ContrastPalette(6, bg, 4.5, 0.1, 2024)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if len(first) != len(second) {
		t.Fatalf("determinism broken: lengths %d and %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("determinism broken at color %d: %v != %v", i, first[i], second[i])
		}
	}
}

func TestContrastPaletteDifferentSeedsDiffer(t *testing.T) {
	bg := Color{0.1, 0.1, 0.12}
	a, err := ContrastPalette(5, bg, 4.5, 0.1, 1)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	b, err := ContrastPalette(5, bg, 4.5, 0.1, 2)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	same := len(a) == len(b)
	for i := 0; same && i < len(a); i++ {
		same = a[i] == b[i]
	}
	if same {
		t.Error("different seeds produced identical palettes; seed appears to be ignored")
	}
}

func TestContrastPaletteInvalidArguments(t *testing.T) {
	bg := Color{0.1, 0.1, 0.1}
	bad := Color{1.2, 0.1, 0.1}

	tests := []struct {
		name     string
		count    int
		settings ContrastPaletteSettings
		wantErr  error
	}{
		{"zero count", 0, ContrastPaletteSettings{Background: bg, MinContrast: 4.5}, ErrInvalidColorCount},
		{"negative count", -3, ContrastPaletteSettings{Background: bg, MinContrast: 4.5}, ErrInvalidColorCount},
		{"invalid background", 3, ContrastPaletteSettings{Background: bad, MinContrast: 4.5}, ErrInvalidBackground},
		{"NaN contrast", 3, ContrastPaletteSettings{Background: bg, MinContrast: math.NaN()}, ErrInvalidContrast},
		{"+Inf contrast", 3, ContrastPaletteSettings{Background: bg, MinContrast: math.Inf(1)}, ErrInvalidContrast},
		{"-Inf contrast", 3, ContrastPaletteSettings{Background: bg, MinContrast: math.Inf(-1)}, ErrInvalidContrast},
		{"contrast below 1", 3, ContrastPaletteSettings{Background: bg, MinContrast: 0.5}, ErrInvalidContrast},
		{"contrast above 21", 3, ContrastPaletteSettings{Background: bg, MinContrast: 21.5}, ErrInvalidContrast},
		{"NaN distance", 3, ContrastPaletteSettings{Background: bg, MinContrast: 4.5, MinDistance: math.NaN()}, ErrInvalidDistance},
		{"+Inf distance", 3, ContrastPaletteSettings{Background: bg, MinContrast: 4.5, MinDistance: math.Inf(1)}, ErrInvalidDistance},
		{"negative distance", 3, ContrastPaletteSettings{Background: bg, MinContrast: 4.5, MinDistance: -0.1}, ErrInvalidDistance},
		{"negative budget", 3, ContrastPaletteSettings{Background: bg, MinContrast: 4.5, SearchBudget: -1}, ErrInvalidSearchBudget},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pal, err := ContrastPaletteEx(tt.count, tt.settings)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got error %v, want %v", err, tt.wantErr)
			}
			if pal != nil {
				t.Fatalf("got a palette alongside the error: %v", pal)
			}
		})
	}
}

func TestContrastPaletteUnreachableContrast(t *testing.T) {
	// A mid-gray background caps out around 5:1 against black or white, so
	// no sRGB color can reach 7:1.
	bg := Color{0.5, 0.5, 0.5}
	if max := maxContrastForBackground(bg.RelativeLuminance()); max >= 7.0 {
		t.Fatalf("test assumption broken: mid-gray allows %.4f:1", max)
	}

	_, err := ContrastPaletteEx(3, ContrastPaletteSettings{
		Background:  bg,
		MinContrast: 7.0,
		// A generous budget proves the error comes from reachability, not
		// from running out of samples.
		SearchBudget: 200000,
	})
	if !errors.Is(err, ErrUnreachableContrast) {
		t.Fatalf("got error %v, want %v", err, ErrUnreachableContrast)
	}
}

func TestContrastPaletteExhaustedBudget(t *testing.T) {
	bg := Color{0.05, 0.05, 0.07}
	_, err := ContrastPaletteEx(5, ContrastPaletteSettings{
		Background:   bg,
		MinContrast:  4.5,
		MinDistance:  0.1,
		SearchBudget: 3,
	})
	if !errors.Is(err, ErrNoPaletteSolution) {
		t.Fatalf("got error %v, want %v", err, ErrNoPaletteSolution)
	}
}

func TestContrastPaletteImpossibleDistance(t *testing.T) {
	// A CIEDE2000 distance of 2.0 exceeds the maximum possible distance
	// between any two sRGB colors, so no palette can satisfy it.
	bg := Color{0.05, 0.05, 0.07}
	_, err := ContrastPaletteEx(4, ContrastPaletteSettings{
		Background:  bg,
		MinContrast: 4.5,
		MinDistance: 2.0,
	})
	if !errors.Is(err, ErrNoPaletteSolution) {
		t.Fatalf("got error %v, want %v", err, ErrNoPaletteSolution)
	}
}

func TestContrastRatio(t *testing.T) {
	black := Color{0, 0, 0}
	white := Color{1, 1, 1}

	if got := ContrastRatio(black, white); math.Abs(got-21.0) > 1e-9 {
		t.Errorf("ContrastRatio(black, white) = %v, want 21", got)
	}
	if got := ContrastRatio(white, black); math.Abs(got-21.0) > 1e-9 {
		t.Errorf("ContrastRatio(white, black) = %v, want 21", got)
	}
	if got := ContrastRatio(black, black); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("ContrastRatio(black, black) = %v, want 1", got)
	}
	if got := ContrastRatio(white, white); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("ContrastRatio(white, white) = %v, want 1", got)
	}
	if a, b := ContrastRatio(Color{0.2, 0.4, 0.6}, Color{0.8, 0.7, 0.1}),
		ContrastRatio(Color{0.8, 0.7, 0.1}, Color{0.2, 0.4, 0.6}); a != b {
		t.Errorf("ContrastRatio is not symmetric: %v != %v", a, b)
	}
}

func TestRelativeLuminance(t *testing.T) {
	if got := (Color{0, 0, 0}).RelativeLuminance(); got != 0 {
		t.Errorf("black luminance = %v, want 0", got)
	}
	if got := (Color{1, 1, 1}).RelativeLuminance(); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("white luminance = %v, want 1", got)
	}
	if got := (Color{0, 1, 0}).RelativeLuminance(); math.Abs(got-0.7152) > 1e-9 {
		t.Errorf("green luminance = %v, want 0.7152", got)
	}
}
