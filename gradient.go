package colorful

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// GamutPolicy controls how colors that leave the sRGB gamut while blending
// through a perceptual color space are handled when they are returned as
// sRGB colors.
type GamutPolicy int

const (
	// GamutClamp clamps every out-of-gamut channel back into [0..1].
	// This is the default and guarantees that all returned colors are
	// representable in sRGB.
	GamutClamp GamutPolicy = iota

	// GamutIgnore leaves the raw, possibly out-of-gamut values untouched.
	// Useful when the output is converted back into a wide-gamut space or
	// post-processed with a custom gamut-mapping step.
	GamutIgnore
)

// A GradientStop is an anchor of a multi-stop gradient. Positions must be
// strictly increasing and are only used to validate the stop ordering:
// arc-length resampling is driven by the cumulative perceptual distance
// between stops, not by their Pos values. Consequently, spacing the
// positions evenly or unevenly produces the same sampled colors.
type GradientStop struct {
	Pos float64
	Col Color
}

// A GradientSpace defines the perceptual color space a gradient is built in.
// From projects an sRGB color into the space's three coordinates and To
// performs the inverse projection. The arc length of the interpolation path
// is measured as the Euclidean length in these coordinates, which stays
// well-defined even when intermediate points fall outside the sRGB gamut.
type GradientSpace interface {
	From(c Color) [3]float64
	To(p [3]float64) Color
}

type gradientSpaceFunc struct {
	from func(c Color) [3]float64
	to   func(p [3]float64) Color
}

func (g gradientSpaceFunc) From(c Color) [3]float64 { return g.from(c) }
func (g gradientSpaceFunc) To(p [3]float64) Color   { return g.to(p) }

// LabSpace interpolates and measures arc length in CIE L*a*b* (D65).
// It is the default space and pairs with Color.DistanceLab.
func LabSpace() GradientSpace {
	return gradientSpaceFunc{
		from: func(c Color) [3]float64 { l, a, b := c.Lab(); return [3]float64{l, a, b} },
		to:   func(p [3]float64) Color { return Lab(p[0], p[1], p[2]) },
	}
}

// LuvSpace interpolates and measures arc length in CIE L*u*v* (D65).
// It pairs with Color.DistanceLuv.
func LuvSpace() GradientSpace {
	return gradientSpaceFunc{
		from: func(c Color) [3]float64 { l, u, v := c.Luv(); return [3]float64{l, u, v} },
		to:   func(p [3]float64) Color { return Luv(p[0], p[1], p[2]) },
	}
}

// OkLabSpace interpolates and measures arc length in OkLab.
func OkLabSpace() GradientSpace {
	return gradientSpaceFunc{
		from: func(c Color) [3]float64 { l, a, b := c.OkLab(); return [3]float64{l, a, b} },
		to:   func(p [3]float64) Color { return OkLab(p[0], p[1], p[2]) },
	}
}

// GradientSettings configures a perceptually uniform multi-stop gradient.
type GradientSettings struct {
	// Space selects the perceptual color space the interpolation path is
	// tessellated and measured in. The arc length is the Euclidean length of
	// the path in this space's coordinates, which remains valid for points
	// outside the sRGB gamut. When nil, LabSpace is used.
	Space GradientSpace

	// MaxStep bounds the Euclidean distance between two adjacent tessellation
	// points in the chosen Space and therefore the precision upper bound of
	// the arc-length approximation. Each stop gap is subdivided until every
	// piece is at most MaxStep long. Smaller values give a denser, more
	// accurate tessellation. When <= 0 (or non-finite handling aside), the
	// DefaultMaxStep is used.
	MaxStep float64

	// MaxSegments caps how many pieces a single stop gap may be split into,
	// protecting against pathological inputs. When <= 0, DefaultMaxSegments
	// is used.
	MaxSegments int

	// Gamut selects how out-of-gamut interpolation results are handled when
	// converted back to sRGB. The zero value, GamutClamp, clamps results into
	// the sRGB cube; GamutIgnore returns the raw, possibly invalid, values.
	Gamut GamutPolicy
}

// Default precision upper bound for the tessellation, in Lab distance units.
// It is well below one Lab just-noticeable-difference, so the polygonal
// approximation of the interpolation path is visually exact.
const DefaultMaxStep = 0.01

// Default hard upper bound on the number of tessellation pieces per stop gap.
const DefaultMaxSegments = 100000

// Sentinel errors returned by PerceptualGradientEx. They are returned wrapped
// in a more descriptive error; match them with the Is method (or errors.Is on
// newer Go versions) rather than a direct equality check.
var (
	// ErrEmptyGradient is returned when no stops are supplied.
	ErrEmptyGradient = errors.New("colorful: gradient has no stops")

	// ErrBadStopCount is returned when fewer than two stops are supplied.
	ErrBadStopCount = errors.New("colorful: gradient needs at least two stops")

	// ErrNonFiniteColor is returned when a stop or interpolation result contains NaN or +/-Inf.
	ErrNonFiniteColor = errors.New("colorful: gradient stop contains a non-finite value")

	// ErrOutOfGamut is returned when a stop is not a valid sRGB color.
	ErrOutOfGamut = errors.New("colorful: gradient stop is outside the sRGB gamut")

	// ErrBadPosition is returned when stop positions are not strictly increasing.
	ErrBadPosition = errors.New("colorful: gradient stop positions must be strictly increasing")

	// ErrBadCount is returned when fewer than two output colors are requested.
	ErrBadCount = errors.New("colorful: gradient must produce at least two colors")

	// ErrNonFiniteSetting is returned when a configured precision value is non-finite.
	ErrNonFiniteSetting = errors.New("colorful: gradient setting contains a non-finite value")
)

// PerceptualGradient builds the default perceptually uniform multi-stop
// gradient, interpolating in CIE L*a*b* and resampling by cumulative
// arc-length in Lab distance. It is equivalent to calling PerceptualGradientEx
// with zero-valued GradientSettings.
func PerceptualGradient(stops []GradientStop, count int) ([]Color, error) {
	return PerceptualGradientEx(stops, count, GradientSettings{})
}

// PerceptualGradientEx builds a multi-stop gradient whose output colors are
// spaced evenly by cumulative perceptual arc length instead of by parameter
// value.
//
// Every stop is projected into the configured perceptual Space (Lab by
// default), and the piecewise-linear path through those projected points is
// tessellated into segments no longer than MaxStep. The cumulative Euclidean
// length of that polyline is the arc length, and `count` colors are placed at
// equal intervals along it. Measuring in the perceptual coordinates rather
// than in sRGB keeps the result correct even when intermediate points leave
// the sRGB gamut; the Gamut policy only decides how such points are encoded
// back into sRGB on output.
//
// The first output color is exactly the first stop's sRGB color and the last
// output color is exactly the last stop's color; interior colors are produced
// by inverse-projecting the sampled perceptual coordinates.
//
// Adjacent duplicate stops (zero perceptual distance) collapse into a single
// point of the polyline. When every stop projects to the same point, the
// whole path has zero length and `count` copies of that color are returned.
//
// All failure modes are deterministic and return an error wrapping one of the
// Err* sentinels: empty stops, a single stop, non-finite or out-of-gamut
// colors, non-increasing positions, and count < 2.
func PerceptualGradientEx(stops []GradientStop, count int, settings GradientSettings) ([]Color, error) {
	if len(stops) == 0 {
		return nil, ErrEmptyGradient
	}
	if len(stops) < 2 {
		return nil, ErrBadStopCount
	}
	if count < 2 {
		return nil, ErrBadCount
	}

	space := settings.Space
	if space == nil {
		space = LabSpace()
	}

	if math.IsNaN(settings.MaxStep) || math.IsInf(settings.MaxStep, 0) {
		return nil, ErrNonFiniteSetting
	}
	maxStep := settings.MaxStep
	if maxStep <= 0.0 {
		maxStep = DefaultMaxStep
	}
	maxSegments := settings.MaxSegments
	if maxSegments <= 0 {
		maxSegments = DefaultMaxSegments
	}

	// Validate stops up front and project them once into the perceptual space.
	pts := make([][3]float64, len(stops))
	for i, s := range stops {
		if !isFiniteColor(s.Col) {
			return nil, wrapErr(ErrNonFiniteColor, "stop %d", i)
		}
		if !s.Col.IsValid() {
			return nil, wrapErr(ErrOutOfGamut, "stop %d", i)
		}
		if math.IsNaN(s.Pos) || math.IsInf(s.Pos, 0) {
			return nil, wrapErr(ErrBadPosition, "stop %d has a non-finite position", i)
		}
		if i > 0 && s.Pos <= stops[i-1].Pos {
			return nil, wrapErr(ErrBadPosition, "stop %d", i)
		}
		pts[i] = space.From(s.Col)
		if !isFinitePoint(pts[i]) {
			return nil, wrapErr(ErrNonFiniteColor, "stop %d projects to a non-finite point", i)
		}
	}

	// Tessellate each stop gap into pieces no longer than MaxStep and record
	// the cumulative arc length at every vertex.
	//
	// Because the path is linear in the perceptual coordinates, the arc length
	// up to a point is computed from the exact straight-line gap length rather
	// than accumulated from successive piece lengths. This keeps the cumulative
	// length at every internal stop vertex free of floating-point drift, so that
	// an equal-arc-length target landing exactly on a stop boundary is assigned
	// to the correct containing segment.
	type vertex struct {
		s  float64 // cumulative arc length at this vertex
		pt [3]float64
	}
	verts := make([]vertex, 0, len(stops)*4)
	verts = append(verts, vertex{0.0, pts[0]})
	total := 0.0

	for g := 0; g < len(pts)-1; g++ {
		p1, p2 := pts[g], pts[g+1]
		gapLen := euclid(p1, p2)
		if gapLen > 0.0 {
			n := int(math.Ceil(gapLen / maxStep))
			if n < 1 {
				n = 1
			}
			if n > maxSegments {
				n = maxSegments
			}
			gapStart := total
			// Vertex zero is the shared endpoint already stored, so only the
			// n new points are appended.
			for k := 1; k <= n; k++ {
				t := float64(k) / float64(n)
				pt := lerpPoint(p1, p2, t)
				if !isFinitePoint(pt) {
					return nil, wrapErr(ErrNonFiniteColor, "interpolation between stops %d and %d produced a non-finite point", g, g+1)
				}
				verts = append(verts, vertex{gapStart + gapLen*t, pt})
			}
			total += gapLen
		} else {
			// Adjacent duplicate points: a zero-length piece, appended once so
			// the stop still appears in order.
			verts = append(verts, vertex{total, p2})
		}
	}

	// Zero total length means every stop projects to the same point: return a
	// solid gradient in deterministic stop order.
	if total == 0.0 {
		out := make([]Color, count)
		for i := range out {
			out[i] = stops[0].Col
		}
		return out, nil
	}

	out := make([]Color, count)
	for i := 0; i < count; i++ {
		if i == 0 {
			// Preserve the first stop exactly, bypassing a color-space round trip.
			out[i] = stops[0].Col
			continue
		}
		if i == count-1 {
			// Preserve the last stop exactly.
			out[i] = stops[len(stops)-1].Col
			continue
		}

		target := total * float64(i) / float64(count-1)
		// Find the last vertex whose cumulative length is at or before the
		// target. Search(len, s > target) returns the first vertex strictly
		// past it; subtracting one gives the containing segment's lower bound.
		j := sort.Search(len(verts), func(k int) bool { return verts[k].s > target }) - 1
		if j < 0 {
			j = 0
		}
		if j >= len(verts)-1 {
			j = len(verts) - 2
		}
		lo, hi := verts[j], verts[j+1]
		var pt [3]float64
		if hi.s == lo.s {
			// Zero-length segment from a duplicate stop.
			pt = hi.pt
		} else {
			u := (target - lo.s) / (hi.s - lo.s)
			pt = lerpPoint(lo.pt, hi.pt, u)
		}
		col := space.To(pt)
		if !isFiniteColor(col) {
			return nil, wrapErr(ErrNonFiniteColor, "sampled point %d converted to a non-finite color", i)
		}
		if settings.Gamut == GamutClamp {
			col = col.Clamped()
		}
		out[i] = col
	}
	return out, nil
}

func euclid(p, q [3]float64) float64 {
	return math.Sqrt(sq(p[0]-q[0]) + sq(p[1]-q[1]) + sq(p[2]-q[2]))
}

func lerpPoint(p, q [3]float64, t float64) [3]float64 {
	return [3]float64{
		p[0] + t*(q[0]-p[0]),
		p[1] + t*(q[1]-p[1]),
		p[2] + t*(q[2]-p[2]),
	}
}

func isFinitePoint(p [3]float64) bool {
	for _, v := range p {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func isFiniteColor(c Color) bool {
	return !math.IsNaN(c.R) && !math.IsInf(c.R, 0) &&
		!math.IsNaN(c.G) && !math.IsInf(c.G, 0) &&
		!math.IsNaN(c.B) && !math.IsInf(c.B, 0)
}

func wrapErr(sentinel error, format string, args ...interface{}) error {
	return &gradientError{sentinel: sentinel, msg: fmt.Sprintf(format, args...)}
}

type gradientError struct {
	sentinel error
	msg      string
}

func (e *gradientError) Error() string { return e.msg + ": " + e.sentinel.Error() }

// Unwrap exposes the wrapped sentinel so errors.Is can identify the cause.
func (e *gradientError) Unwrap() error { return e.sentinel }

// Is matches the wrapped sentinel directly, supporting errors.Is(err, sentinel).
func (e *gradientError) Is(target error) bool { return target == e.sentinel }
