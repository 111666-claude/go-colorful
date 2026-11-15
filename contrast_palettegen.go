package colorful

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
)

// WCAG contrast ratio bounds. A contrast ratio is always within [1, 21].
const (
	WCAGMinContrastRatio = 1.0
	WCAGMaxContrastRatio = 21.0

	// DefaultContrastPaletteSearchBudget is the number of random candidate
	// colors sampled when ContrastPaletteSettings.SearchBudget is zero.
	DefaultContrastPaletteSearchBudget = 100000

	// maxContrastPalettePool bounds how many feasible candidates are kept for
	// the pairwise selection. Sampling more than this would only slow the
	// O(pool * colorsCount) distance computations down without improving the
	// result noticeably.
	maxContrastPalettePool = 4096
)

// Errors returned by ContrastPaletteEx and ContrastPalette. They are wrapped
// with details, so use errors.Is to test for them.
var (
	// ErrInvalidColorCount is returned when the requested number of colors is
	// not positive.
	ErrInvalidColorCount = errors.New("colorful: palette color count must be greater than zero")

	// ErrInvalidBackground is returned when the background is not a finite,
	// displayable sRGB color.
	ErrInvalidBackground = errors.New("colorful: background color must be a displayable sRGB color")

	// ErrInvalidContrast is returned when the minimum WCAG contrast is not a
	// finite value in [1, 21].
	ErrInvalidContrast = errors.New("colorful: minimum WCAG contrast must be finite and in [1, 21]")

	// ErrInvalidDistance is returned when the minimum CIEDE2000 distance is not
	// a finite, non-negative value.
	ErrInvalidDistance = errors.New("colorful: minimum CIEDE2000 distance must be finite and non-negative")

	// ErrInvalidSearchBudget is returned when a negative search budget is given.
	ErrInvalidSearchBudget = errors.New("colorful: search budget must not be negative")

	// ErrUnreachableContrast is returned when no displayable sRGB color can
	// reach the requested contrast ratio against the background.
	ErrUnreachableContrast = errors.New("colorful: minimum WCAG contrast ratio is unreachable for this background")

	// ErrNoPaletteSolution is returned when the finite search budget was
	// exhausted before enough colors satisfying all constraints could be found.
	ErrNoPaletteSolution = errors.New("colorful: no palette satisfies the constraints within the search budget")
)

// PaletteError wraps one of the sentinel errors above with details about the
// rejected values. Use errors.Is(err, ErrInvalidContrast) or inspect Kind
// directly to determine the cause.
type PaletteError struct {
	// Kind is the sentinel error describing the category of failure.
	Kind error

	// Message contains human-readable details, or "" if there are none.
	Message string
}

func (e *PaletteError) Error() string {
	if e.Message == "" {
		return e.Kind.Error()
	}
	return e.Kind.Error() + ": " + e.Message
}

// Unwrap allows errors.Is and errors.As to see through to the sentinel error.
func (e *PaletteError) Unwrap() error {
	return e.Kind
}

func paletteErrorf(kind error, format string, args ...interface{}) error {
	return &PaletteError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// ContrastPaletteSettings configures ContrastPaletteEx.
type ContrastPaletteSettings struct {
	// Background is the color the palette has to be legible against. It must
	// be a valid (displayable) sRGB color.
	Background Color

	// MinContrast is the minimum WCAG contrast ratio between the background
	// and every generated color. It must be in [1, 21]. Use 4.5 for WCAG AA
	// body text and 3.0 for large text or graphical UI elements.
	MinContrast float64

	// MinDistance is the minimum pairwise CIEDE2000 distance between generated
	// colors, in the same scale as Color.DistanceCIEDE2000 (0 is identical,
	// 0.01 is roughly one just-noticeable difference, 1.0 is very different).
	MinDistance float64

	// Seed initializes the pseudo-random candidate search. Generating twice
	// with the same settings and seed yields exactly the same palette.
	Seed int64

	// SearchBudget is the maximum number of candidate colors sampled. Zero
	// selects DefaultContrastPaletteSearchBudget. The search always terminates
	// after this many samples; it never lowers the requested constraints.
	SearchBudget int
}

// RelativeLuminance returns the WCAG relative luminance of the color, a value
// in [0, 1] where 0 is black and 1 is white. The color is expected to be a
// displayable sRGB color.
func (col Color) RelativeLuminance() float64 {
	r, g, b := col.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// ContrastRatio returns the WCAG contrast ratio between two colors. The result
// is in [1, 21]; 1 means no contrast and 21 is black against white.
func ContrastRatio(c1, c2 Color) float64 {
	l1 := c1.RelativeLuminance()
	l2 := c2.RelativeLuminance()
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// maxContrastForBackground is the best contrast any sRGB color can reach
// against a background with the given relative luminance. It is achieved by
// either black or white.
func maxContrastForBackground(luminance float64) float64 {
	againstBlack := (luminance + 0.05) / 0.05
	againstWhite := 1.05 / (luminance + 0.05)
	if againstBlack > againstWhite {
		return againstBlack
	}
	return againstWhite
}

// ContrastPalette generates colorsCount colors that are all legible against
// background and visually distinct from each other.
//
// Every returned color has a WCAG contrast ratio of at least minContrast with
// background, a pairwise CIEDE2000 distance of at least minDistance to every
// other returned color, and lies inside the displayable sRGB gamut.
//
// The seed makes the result reproducible. The search is bounded by
// DefaultContrastPaletteSearchBudget; if that is not enough, the function
// returns an error wrapping ErrNoPaletteSolution rather than silently
// returning colors that violate the constraints or lowering the thresholds.
//
// For more control (including the search budget), use ContrastPaletteEx.
func ContrastPalette(colorsCount int, background Color, minContrast, minDistance float64, seed int64) ([]Color, error) {
	return ContrastPaletteEx(colorsCount, ContrastPaletteSettings{
		Background:  background,
		MinContrast: minContrast,
		MinDistance: minDistance,
		Seed:        seed,
	})
}

// ContrastPaletteEx is ContrastPalette with a settings struct, allowing the
// search budget to be configured.
//
// The algorithm samples HSLuv colors (which keeps the result colorful and
// perceptually spread out), rejects those that miss the contrast requirement,
// and then greedily picks the candidate farthest away from the already chosen
// colors as long as it still clears MinDistance. When no candidate clears the
// distance, it restarts from another candidate a few times; if all attempts
// fail it returns ErrNoPaletteSolution.
func ContrastPaletteEx(colorsCount int, settings ContrastPaletteSettings) ([]Color, error) {
	return ContrastPaletteWithRand(colorsCount, settings, rand.New(rand.NewSource(settings.Seed)))
}

// ContrastPaletteWithRand behaves like ContrastPaletteEx but uses the given
// random source instead of one seeded with settings.Seed, which is ignored.
// This is useful when the caller wants to control the random stream; supplying
// an identically initialized source produces the same palette.
func ContrastPaletteWithRand(colorsCount int, settings ContrastPaletteSettings, random RandInterface) ([]Color, error) {
	if colorsCount <= 0 {
		return nil, paletteErrorf(ErrInvalidColorCount, "got %d", colorsCount)
	}
	if !settings.Background.IsValid() {
		return nil, paletteErrorf(ErrInvalidBackground, "%v", settings.Background)
	}
	if math.IsNaN(settings.MinContrast) || math.IsInf(settings.MinContrast, 0) ||
		settings.MinContrast < WCAGMinContrastRatio || settings.MinContrast > WCAGMaxContrastRatio {
		return nil, paletteErrorf(ErrInvalidContrast, "got %v", settings.MinContrast)
	}
	if math.IsNaN(settings.MinDistance) || math.IsInf(settings.MinDistance, 0) || settings.MinDistance < 0 {
		return nil, paletteErrorf(ErrInvalidDistance, "got %v", settings.MinDistance)
	}
	if settings.SearchBudget < 0 {
		return nil, paletteErrorf(ErrInvalidSearchBudget, "got %d", settings.SearchBudget)
	}

	// Reject impossible requests up front instead of burning the whole search
	// budget and reporting a generic "no solution" error.
	maxContrast := maxContrastForBackground(settings.Background.RelativeLuminance())
	if settings.MinContrast > maxContrast+1e-12 {
		return nil, paletteErrorf(ErrUnreachableContrast,
			"background allows at most %.4f:1, requested %.4f:1", maxContrast, settings.MinContrast)
	}

	budget := settings.SearchBudget
	if budget == 0 {
		budget = DefaultContrastPaletteSearchBudget
	}

	// Sample the feasible part of the sRGB gamut. HSLuv(h, s, l) always
	// returns a displayable color (it gamut-maps and clamps internally), so
	// the only filter needed here is the contrast requirement.
	maxPool := maxContrastPalettePool
	if colorsCount > maxPool {
		maxPool = colorsCount
	}
	pool := make([]Color, 0, maxPool)
	sampled := 0
	for ; sampled < budget && len(pool) < maxPool; sampled++ {
		candidate := HSLuv(random.Float64()*360.0, random.Float64(), random.Float64())
		if ContrastRatio(candidate, settings.Background) >= settings.MinContrast {
			pool = append(pool, candidate)
		}
	}

	if len(pool) < colorsCount {
		return nil, paletteErrorf(ErrNoPaletteSolution,
			"only %d of %d requested colors met the contrast requirement after %d samples",
			len(pool), colorsCount, sampled)
	}

	// A greedy farthest-point selection can get unlucky with its first pick,
	// so try a few random starting points before giving up.
	const restarts = 4
	for attempt := 0; attempt < restarts; attempt++ {
		palette, ok := farthestPointPalette(pool, colorsCount, settings.MinDistance, random)
		if ok {
			return palette, nil
		}
	}

	return nil, paletteErrorf(ErrNoPaletteSolution,
		"could not fit %d colors with contrast >= %v and CIEDE2000 distance >= %v after %d samples",
		colorsCount, settings.MinContrast, settings.MinDistance, sampled)
}

// farthestPointPalette greedily selects colorsCount colors from pool so that
// every pair is at least minDistance apart. It reports false if it cannot do
// so, without ever returning a palette that violates minDistance.
func farthestPointPalette(pool []Color, colorsCount int, minDistance float64, random RandInterface) ([]Color, bool) {
	selected := make([]Color, 0, colorsCount)
	used := make([]bool, len(pool))
	minDists := make([]float64, len(pool))

	start := random.Intn(len(pool))
	selected = append(selected, pool[start])
	used[start] = true
	for i := range pool {
		minDists[i] = pool[i].DistanceCIEDE2000(pool[start])
	}

	for len(selected) < colorsCount {
		best := -1
		bestDist := -1.0
		for i := range pool {
			if !used[i] && minDists[i] > bestDist {
				best = i
				bestDist = minDists[i]
			}
		}
		if best < 0 || bestDist < minDistance {
			return nil, false
		}

		picked := pool[best]
		used[best] = true
		selected = append(selected, picked)
		for i := range pool {
			if used[i] {
				continue
			}
			if d := pool[i].DistanceCIEDE2000(picked); d < minDists[i] {
				minDists[i] = d
			}
		}
	}

	return selected, true
}
