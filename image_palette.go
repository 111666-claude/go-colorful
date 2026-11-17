// This file implements palette extraction from images. Given an image and a
// target number of colors, it finds a small set of representative colors
// together with the fraction of the image each of them covers.
//
// The clustering runs in CIE-L*a*b* space, which is perceptually uniform, and
// weights every visible pixel by its alpha value. Fully transparent pixels are
// ignored so that a transparent background never dominates the result. The
// whole algorithm is deterministic: the same image always yields the same
// palette, no random source is involved.

package colorful

import (
	"fmt"
	"image"
	"math"
)

// imagePaletteIterations is the maximum number of Lloyd iterations performed
// by the weighted k-means clustering. It usually converges long before that.
const imagePaletteIterations = 20

// PaletteColor represents a single color in a palette extracted from an image.
type PaletteColor struct {
	// Color is the representative color of this palette entry.
	Color Color

	// Weight is the fraction of the image's visible (non-transparent) weight
	// covered by this entry. All weights in an extracted palette sum to 1.
	Weight float64
}

// weightedPoint is a distinct visible color, expressed in CIE-L*a*b*
// coordinates, together with the alpha weight it accumulated.
type weightedPoint struct {
	lab    lab_t
	rgb    Color
	weight float64
	order  int // first-seen index, used to make tie-breaking deterministic
}

// pixelKey identifies an already-seen pixel color. The values returned by
// color.Color.RGBA are alpha-premultiplied, so this tuple uniquely identifies
// a source color regardless of the concrete image type.
type pixelKey struct {
	r, g, b, a uint32
}

// ExtractPalette extracts count representative colors from the given image.
// Pixels are weighted by their alpha value, which means partially transparent
// pixels contribute partially and fully transparent pixels are ignored
// entirely. The clustering is performed in the perceptually uniform
// CIE-L*a*b* color space, so the resulting colors represent what is actually
// visible rather than what dominates the raw pixel buffer.
//
// The returned palette contains exactly count entries, each holding a
// representative color and the fraction of visible weight it covers. The
// weights sum to 1. Results are deterministic: the same input image always
// produces the same palette.
//
// An error is returned when count is not positive, when the image is nil or
// empty, when the image has no visible pixels, or when count exceeds the
// number of distinct visible colors in the image.
func ExtractPalette(img image.Image, count int) ([]PaletteColor, error) {
	palette, _, err := extractPalette(img, count, false)
	return palette, err
}

// ExtractPaletteWithMapping behaves like ExtractPalette but additionally
// returns a pixel-to-palette-index mapping. The mapping is laid out
// row-major over the image's bounds, i.e. pixel (x, y) is found at index
// (y-Min.Y)*Dx()+(x-Min.X), and contains the index of the nearest palette
// entry for every visible pixel. Fully transparent pixels map to -1.
func ExtractPaletteWithMapping(img image.Image, count int) ([]PaletteColor, []int, error) {
	return extractPalette(img, count, true)
}

func extractPalette(img image.Image, count int, wantMapping bool) ([]PaletteColor, []int, error) {
	if img == nil {
		return nil, nil, fmt.Errorf("colorful: cannot extract a palette from a nil image")
	}
	if count <= 0 {
		return nil, nil, fmt.Errorf("colorful: palette color count must be positive, got %d", count)
	}

	points, totalWeight := collectVisiblePoints(img)
	if len(points) == 0 {
		return nil, nil, fmt.Errorf("colorful: cannot extract a palette from an image with no visible (non-transparent) pixels")
	}
	if count > len(points) {
		return nil, nil, fmt.Errorf("colorful: requested %d palette colors but the image contains only %d distinct visible colors", count, len(points))
	}

	// When every distinct color gets its own palette entry, clustering would
	// just reproduce the input. Skip it so the palette colors stay bit-exact.
	exact := count == len(points)

	var centroids []lab_t
	if exact {
		centroids = make([]lab_t, len(points))
		for i, p := range points {
			centroids[i] = p.lab
		}
	} else {
		centroids = weightedKMeans(points, count)
	}

	// Final assignment of every distinct color to its nearest centroid. This
	// also defines the coverage weights: identical RGB pixels always map to
	// the same centroid, so aggregating distinct-color weights is equivalent
	// to aggregating the weights of individual pixels.
	clusterWeight := make([]float64, count)
	for _, p := range points {
		clusterWeight[nearestCentroid(p.lab, centroids)] += p.weight
	}

	palette := make([]PaletteColor, count)
	for i := range palette {
		var col Color
		if exact {
			col = points[i].rgb
		} else {
			// Averaging in Lab space can leave the RGB gamut, so clamp.
			col = Lab(centroids[i].L, centroids[i].A, centroids[i].B).Clamped()
		}
		palette[i] = PaletteColor{
			Color:  col,
			Weight: clusterWeight[i] / totalWeight,
		}
	}

	var mapping []int
	if wantMapping {
		mapping = buildMapping(img, centroids)
	}
	return palette, mapping, nil
}

// collectVisiblePoints gathers the distinct visible colors of an image in
// CIE-L*a*b* space, each carrying the accumulated alpha weight of the pixels
// showing it. It also returns the total visible weight.
func collectVisiblePoints(img image.Image) ([]weightedPoint, float64) {
	bounds := img.Bounds()
	indices := make(map[pixelKey]int)
	points := make([]weightedPoint, 0)
	order := 0
	totalWeight := 0.0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			col := img.At(x, y)
			r, g, b, a := col.RGBA()
			if a == 0 {
				continue // Fully transparent: no visible contribution.
			}

			weight := float64(a) / 65535.0
			key := pixelKey{r, g, b, a}
			if idx, ok := indices[key]; ok {
				points[idx].weight += weight
			} else {
				rgb, _ := MakeColor(col)
				l, la, lb := rgb.Lab()
				indices[key] = len(points)
				points = append(points, weightedPoint{
					lab:    lab_t{l, la, lb},
					rgb:    rgb,
					weight: weight,
					order:  order,
				})
				order++
			}
			totalWeight += weight
		}
	}
	return points, totalWeight
}

// weightedKMeans clusters the given weighted Lab points into k groups using
// Lloyd's algorithm with weighted centroids. Initialization and every tie are
// resolved deterministically, so the output does not depend on any RNG.
func weightedKMeans(points []weightedPoint, k int) []lab_t {
	centroids := initCentroids(points, k)
	assignment := make([]int, len(points))

	sumL := make([]float64, k)
	sumA := make([]float64, k)
	sumB := make([]float64, k)
	sumW := make([]float64, k)

	for iter := 0; iter < imagePaletteIterations; iter++ {
		changed := false
		for i, p := range points {
			id := nearestCentroid(p.lab, centroids)
			if id != assignment[i] {
				assignment[i] = id
				changed = true
			}
		}
		if !changed {
			break
		}

		for i := range sumL {
			sumL[i], sumA[i], sumB[i], sumW[i] = 0.0, 0.0, 0.0, 0.0
		}
		for i, p := range points {
			id := assignment[i]
			w := p.weight
			sumL[id] += w * p.lab.L
			sumA[id] += w * p.lab.A
			sumB[id] += w * p.lab.B
			sumW[id] += w
		}
		for id := 0; id < k; id++ {
			if sumW[id] > 0.0 {
				centroids[id] = lab_t{
					L: sumL[id] / sumW[id],
					A: sumA[id] / sumW[id],
					B: sumB[id] / sumW[id],
				}
			} else {
				// An empty cluster can only appear through pathological
				// ties; re-seed it from a real, unused point.
				centroids[id] = reseedPoint(points, centroids)
			}
		}
	}
	return centroids
}

// kMeansParallelRounds is the number of over-sampling rounds performed by the
// deterministic k-means|| initialization.
const kMeansParallelRounds = 5

// kMeansParallelOverSample is the expected number of candidates added per
// over-sampling round in the standard k-means|| algorithm.
const kMeansParallelOverSample = 2.0

// initCentroids picks the initial k-means centroids deterministically using
// the k-means|| (parallel k-means++) strategy. In the randomized algorithm
// every point is accepted with probability ℓ times k times its weighted
// squared distance divided by the total cost; here each point is accepted
// when that probability reaches the same fixed threshold, which keeps the
// high-distance preference without any random source. After a few rounds the
// candidate set is reduced to k weighted farthest-first picks. First-seen
// order resolves all ties.
func initCentroids(points []weightedPoint, k int) []lab_t {
	n := len(points)
	chosen := make([]int, 0, 2*k)

	first := 0
	for i := 1; i < n; i++ {
		if points[i].weight > points[first].weight ||
			(points[i].weight == points[first].weight && points[i].order < points[first].order) {
			first = i
		}
	}
	chosen = append(chosen, first)

	isChosen := func(idx int) bool {
		for _, c := range chosen {
			if c == idx {
				return true
			}
		}
		return false
	}

	// Over-sampling phase: accept every point whose k-means|| acceptance
	// probability reaches the fixed threshold.
	threshold := kMeansParallelOverSample * float64(k) / float64(n)
	for round := 0; round < kMeansParallelRounds && len(chosen) < n; round++ {
		centroids := make([]lab_t, 0, len(chosen))
		for _, idx := range chosen {
			centroids = append(centroids, points[idx].lab)
		}

		total := 0.0
		scores := make([]float64, n)
		for i, p := range points {
			scores[i] = p.weight * nearestDistSq(p.lab, centroids)
			total += scores[i]
		}

		for i := range points {
			if isChosen(i) || total == 0.0 {
				continue
			}
			if scores[i]/total >= threshold {
				chosen = append(chosen, i)
			}
		}
	}

	// Reduction phase: take k weighted farthest-first picks from the
	// candidates, whose weights count how much mass they now represent.
	candidateWeight := make([]float64, len(chosen))
	centroids := make([]lab_t, len(chosen))
	for i, idx := range chosen {
		centroids[i] = points[idx].lab
	}
	for _, p := range points {
		nearest := nearestCentroid(p.lab, centroids)
		candidateWeight[nearest] += p.weight
	}

	result := make([]lab_t, 0, k)
	result = append(result, centroids[0])
	used := make([]bool, len(centroids))
	used[0] = true
	for len(result) < k {
		best := -1
		bestScore := 0.0
		for i, c := range centroids {
			if used[i] {
				continue
			}
			score := candidateWeight[i] * nearestDistSq(c, result)
			if best == -1 || score > bestScore ||
				(score == bestScore && points[chosen[i]].order < points[chosen[best]].order) {
				best, bestScore = i, score
			}
		}
		// All candidates are used; fill any remaining slots from the full
		// point set (only reachable when the data has very few distinct
		// colors, in which case the caller usually asks for few centers).
		if best == -1 {
			for i, p := range points {
				if isChosen(i) {
					continue
				}
				score := p.weight * nearestDistSq(p.lab, result)
				if best == -1 || score > bestScore ||
					(score == bestScore && p.order < points[best].order) {
					best, bestScore = i, score
				}
			}
			result = append(result, points[best].lab)
			continue
		}
		used[best] = true
		result = append(result, centroids[best])
	}
	return result
}

// reseedPoint returns the highest-weight point that is not (nearly) equal to
// an existing centroid, resolving ties by first-seen order.
func reseedPoint(points []weightedPoint, centroids []lab_t) lab_t {
	best := -1
	for i, p := range points {
		used := false
		for _, c := range centroids {
			if lab_eq(p.lab, c) {
				used = true
				break
			}
		}
		if used {
			continue
		}
		if best == -1 || p.weight > points[best].weight ||
			(p.weight == points[best].weight && p.order < points[best].order) {
			best = i
		}
	}
	return points[best].lab
}

// nearestCentroid returns the index of the centroid closest to p. Ties go to
// the lowest index, which keeps assignments deterministic.
func nearestCentroid(p lab_t, centroids []lab_t) int {
	best := 0
	bestDist := math.Inf(1)
	for i, c := range centroids {
		if dist := lab_dist(p, c); dist < bestDist {
			bestDist, best = dist, i
		}
	}
	return best
}

// nearestDistSq returns the squared Euclidean distance from p to the closest
// of the given centroids.
func nearestDistSq(p lab_t, centroids []lab_t) float64 {
	best := math.Inf(1)
	for _, c := range centroids {
		if d2 := sq(p.L-c.L) + sq(p.A-c.A) + sq(p.B-c.B); d2 < best {
			best = d2
		}
	}
	return best
}

// buildMapping assigns every pixel of the image to the index of its nearest
// centroid. The mapping is row-major over the image's bounds and uses -1 for
// fully transparent pixels.
func buildMapping(img image.Image, centroids []lab_t) []int {
	bounds := img.Bounds()
	width := bounds.Dx()
	mapping := make([]int, width*bounds.Dy())

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			idx := (y-bounds.Min.Y)*width + (x - bounds.Min.X)
			rgb, ok := MakeColor(img.At(x, y))
			if !ok {
				mapping[idx] = -1
				continue
			}
			l, a, b := rgb.Lab()
			mapping[idx] = nearestCentroid(lab_t{l, a, b}, centroids)
		}
	}
	return mapping
}
