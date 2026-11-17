package colorful

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"
)

// findPaletteEntry returns the index of the palette entry closest to want in
// CIE-L*a*b* space and fails if none of them is reasonably close.
func findPaletteEntry(t *testing.T, pal []PaletteColor, want Color) int {
	t.Helper()
	best, bestDist := -1, math.Inf(1)
	for i, entry := range pal {
		if d := entry.Color.DistanceLab(want); d < bestDist {
			best, bestDist = i, d
		}
	}
	if bestDist > 0.05 {
		t.Fatalf("no palette entry close to %v, closest distance %v", want.Hex(), bestDist)
	}
	return best
}

func setRGBA(img *image.RGBA, x, y int, col Color) {
	r, g, b := col.RGB255()
	// RGBA colors are alpha-premultiplied; with full alpha the premultiplied
	// values are identical to the straight values.
	img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
}

func setNRGBA(img *image.NRGBA, x, y int, col Color, alpha uint8) {
	r, g, b := col.RGB255()
	img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: alpha})
}

// TestSolidBlocks checks colors, coverage weights and the pixel mapping on an
// image made of fixed solid color blocks.
func TestSolidBlocks(t *testing.T) {
	red, _ := Hex("#ff0000")
	green, _ := Hex("#00ff00")
	blue, _ := Hex("#0000ff")

	// 4x4 image: 8 red, 4 green, 4 blue pixels, filled in scan order so the
	// first-seen order is red, green, blue.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			switch {
			case y < 2:
				setRGBA(img, x, y, red)
			case x < 2:
				setRGBA(img, x, y, green)
			default:
				setRGBA(img, x, y, blue)
			}
		}
	}

	pal, mapping, err := ExtractPaletteWithMapping(img, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pal) != 3 {
		t.Fatalf("expected 3 palette entries, got %d", len(pal))
	}

	// Every distinct color gets its own entry, so colors must be bit-exact
	// and weights must equal the area fractions.
	want := []struct {
		col    Color
		weight float64
	}{
		{red, 0.5},
		{green, 0.25},
		{blue, 0.25},
	}
	weightSum := 0.0
	for _, w := range want {
		idx := findPaletteEntry(t, pal, w.col)
		if !pal[idx].Color.AlmostEqualRgb(w.col) {
			t.Errorf("palette color %v does not match expected %v", pal[idx].Color.Hex(), w.col.Hex())
		}
		if math.Abs(pal[idx].Weight-w.weight) > 1e-12 {
			t.Errorf("weight for %v: got %v, want %v", w.col.Hex(), pal[idx].Weight, w.weight)
		}
		if !pal[idx].Color.IsValid() {
			t.Errorf("palette color %v is not a valid RGB color", pal[idx].Color)
		}
		weightSum += pal[idx].Weight
	}
	if math.Abs(weightSum-1.0) > 1e-12 {
		t.Errorf("palette weights sum to %v, want 1.0", weightSum)
	}

	// The mapping must be row-major and point each pixel at the entry for its
	// own color.
	if len(mapping) != 16 {
		t.Fatalf("mapping length: got %d, want 16", len(mapping))
	}
	redIdx := findPaletteEntry(t, pal, red)
	greenIdx := findPaletteEntry(t, pal, green)
	blueIdx := findPaletteEntry(t, pal, blue)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			wantIdx := redIdx
			if y >= 2 && x < 2 {
				wantIdx = greenIdx
			} else if y >= 2 {
				wantIdx = blueIdx
			}
			if got := mapping[y*4+x]; got != wantIdx {
				t.Errorf("mapping at (%d,%d): got %d, want %d", x, y, got, wantIdx)
			}
		}
	}

	// The plain entry point returns the same palette but no mapping.
	pal2, err := ExtractPalette(img, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range pal {
		if !pal2[i].Color.AlmostEqualRgb(pal[i].Color) || pal2[i].Weight != pal[i].Weight {
			t.Errorf("ExtractPalette and ExtractPaletteWithMapping disagree at index %d", i)
		}
	}
}

// TestTransparentBackgroundIgnored makes sure a large fully transparent area
// neither contributes weight nor shows up as a palette color.
func TestTransparentBackgroundIgnored(t *testing.T) {
	red, _ := Hex("#ff0000")
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	// Only a 2x2 opaque red block, the other 60 pixels stay transparent.
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			setNRGBA(img, x, y, red, 255)
		}
	}

	pal, mapping, err := ExtractPaletteWithMapping(img, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pal) != 1 {
		t.Fatalf("expected 1 palette entry, got %d", len(pal))
	}
	if !pal[0].Color.AlmostEqualRgb(red) {
		t.Errorf("expected pure red, got %v", pal[0].Color.Hex())
	}
	if math.Abs(pal[0].Weight-1.0) > 1e-12 {
		t.Errorf("expected weight 1.0, got %v", pal[0].Weight)
	}

	transparent, opaque := 0, 0
	for i, idx := range mapping {
		if idx == -1 {
			transparent++
		} else {
			opaque++
			if idx != 0 {
				t.Errorf("mapping[%d] = %d, want 0", i, idx)
			}
		}
	}
	if transparent != 60 || opaque != 4 {
		t.Errorf("mapping transparency counts: got %d transparent, %d opaque, want 60/4", transparent, opaque)
	}
}

// TestAlphaWeights verifies that partially transparent pixels contribute
// proportionally to their alpha value.
func TestAlphaWeights(t *testing.T) {
	red, _ := Hex("#ff0000")
	blue, _ := Hex("#0000ff")

	// One fully opaque red pixel and one half-transparent blue pixel.
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	setNRGBA(img, 0, 0, red, 255)
	setNRGBA(img, 1, 0, blue, 128)

	pal, err := ExtractPalette(img, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	redIdx := findPaletteEntry(t, pal, red)
	blueIdx := findPaletteEntry(t, pal, blue)

	// The blue pixel carries weight 128/255, red carries 1.
	wantRed := 1.0 / (1.0 + 128.0/255.0)
	wantBlue := (128.0 / 255.0) / (1.0 + 128.0/255.0)
	if math.Abs(pal[redIdx].Weight-wantRed) > 1e-6 {
		t.Errorf("red weight: got %v, want %v", pal[redIdx].Weight, wantRed)
	}
	if math.Abs(pal[blueIdx].Weight-wantBlue) > 1e-6 {
		t.Errorf("blue weight: got %v, want %v", pal[blueIdx].Weight, wantBlue)
	}
	if math.Abs(pal[redIdx].Weight+pal[blueIdx].Weight-1.0) > 1e-12 {
		t.Errorf("weights do not sum to 1: %v + %v", pal[redIdx].Weight, pal[blueIdx].Weight)
	}
}

// TestPaletteErrorCases covers the explicitly unsupported inputs.
func TestPaletteErrorCases(t *testing.T) {
	red, _ := Hex("#ff0000")

	// Nil image.
	if _, _, err := ExtractPaletteWithMapping(nil, 2); err == nil {
		t.Error("expected an error for a nil image")
	}

	// Non-positive color counts.
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	setNRGBA(img, 0, 0, red, 255)
	setNRGBA(img, 1, 0, red, 255)
	for _, n := range []int{0, -1, -10} {
		if _, err := ExtractPalette(img, n); err == nil {
			t.Errorf("expected an error for count %d", n)
		}
	}

	// Empty image (zero-area bounds).
	empty := image.NewRGBA(image.Rect(0, 0, 0, 0))
	if _, err := ExtractPalette(empty, 1); err == nil {
		t.Error("expected an error for an empty image")
	}

	// Fully transparent image.
	transparent := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	if _, err := ExtractPalette(transparent, 1); err == nil {
		t.Error("expected an error for a fully transparent image")
	}

	// Requesting more colors than there are distinct visible colors.
	twoColors := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	setNRGBA(twoColors, 0, 0, red, 255)
	blue, _ := Hex("#0000ff")
	setNRGBA(twoColors, 1, 0, blue, 255)
	if _, err := ExtractPalette(twoColors, 3); err == nil {
		t.Error("expected an error when asking for more colors than distinct visible colors")
	}
	// Asking for exactly the number of distinct colors must still work.
	pal, err := ExtractPalette(twoColors, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pal) != 2 {
		t.Errorf("expected 2 entries, got %d", len(pal))
	}
}

// TestSingleColorImage checks the monochromatic case and that the returned
// color is the exact input color with the full weight.
func TestSingleColorImage(t *testing.T) {
	red, _ := Hex("#d03020")
	img := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			setNRGBA(img, x, y, red, 255)
		}
	}

	pal, _, err := ExtractPaletteWithMapping(img, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pal) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(pal))
	}
	if !pal[0].Color.AlmostEqualRgb(red) {
		t.Errorf("expected %v, got %v", red.Hex(), pal[0].Color.Hex())
	}
	if math.Abs(pal[0].Weight-1.0) > 1e-12 {
		t.Errorf("expected weight 1.0, got %v", pal[0].Weight)
	}
	// All nine pixels must map to index 0.
	_, mapping, err := ExtractPaletteWithMapping(img, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, idx := range mapping {
		if idx != 0 {
			t.Errorf("mapping[%d] = %d, want 0", i, idx)
		}
	}
}

// TestReproducible ensures identical inputs produce identical palettes and
// that the result does not depend on the concrete image representation.
func TestReproducible(t *testing.T) {
	red, _ := Hex("#ff0000")
	blue, _ := Hex("#0000ff")

	makeNRGBA := func() *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, 16, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 16; x++ {
				col := red.BlendRgb(blue, float64(x)/15.0)
				setNRGBA(img, x, y, col, 255)
			}
		}
		return img
	}

	first, mapping1, err := ExtractPaletteWithMapping(makeNRGBA(), 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	second, mapping2, err := ExtractPaletteWithMapping(makeNRGBA(), 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Bit-exact reproducibility.
	for i := range first {
		if first[i].Color != second[i].Color || first[i].Weight != second[i].Weight {
			t.Fatalf("palette not reproducible at index %d", i)
		}
	}
	for i := range mapping1 {
		if mapping1[i] != mapping2[i] {
			t.Fatalf("mapping not reproducible at index %d", i)
		}
	}

	// The same logical image stored as premultiplied RGBA must yield the same
	// palette.
	src := makeNRGBA()
	rgba := image.NewRGBA(src.Rect)
	for y := 0; y < 4; y++ {
		for x := 0; x < 16; x++ {
			rgba.Set(x, y, src.At(x, y))
		}
	}
	other, _, err := ExtractPaletteWithMapping(rgba, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, entry := range first {
		idx := findPaletteEntry(t, other, entry.Color)
		if math.Abs(other[idx].Weight-entry.Weight) > 1e-9 {
			t.Errorf("weight mismatch between RGBA and NRGBA images: %v vs %v", other[idx].Weight, entry.Weight)
		}
		if other[idx].Color.DistanceLab(entry.Color) > 1e-9 {
			t.Errorf("color mismatch between RGBA and NRGBA images: %v vs %v", other[idx].Color.Hex(), entry.Color.Hex())
		}
	}
}

// TestGradientReconstruction runs k-means on an image made of four separated
// color regions, each with its own internal shading (as found in shaded pixel
// art), and checks that every region gets its own entry, coverage matches the
// region sizes and reconstruction error stays small.
func TestGradientReconstruction(t *testing.T) {
	baseColors := []Color{}
	for _, hex := range []string{"#ff0000", "#ffee00", "#00cc44", "#2233ff"} {
		c, err := Hex(hex)
		if err != nil {
			t.Fatalf("invalid hex %q: %v", hex, err)
		}
		baseColors = append(baseColors, c)
	}

	// Each region only varies the lightness slightly around its base color
	// in sRGB space (clamped so every shade stays a valid displayable
	// color), producing four well-separated, equally-sized clusters.
	const regionWidth, height = 16, 4
	width := regionWidth * len(baseColors)
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for region, base := range baseColors {
		for y := 0; y < height; y++ {
			for x := 0; x < regionWidth; x++ {
				tt := float64(x) / (regionWidth - 1)
				factor := 0.8 + 0.3*tt
				shade := Color{
					R: clamp01(base.R * factor),
					G: clamp01(base.G * factor),
					B: clamp01(base.B * factor),
				}
				setNRGBA(img, region*regionWidth+x, y, shade, 255)
			}
		}
	}

	pal, mapping, err := ExtractPaletteWithMapping(img, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pal) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(pal))
	}

	// Every region has the same size, so coverage must be balanced.
	for i, entry := range pal {
		if math.Abs(entry.Weight-0.25) > 0.02 {
			t.Errorf("entry %d weight %v, want ~0.25", i, entry.Weight)
		}
		if !entry.Color.IsValid() {
			t.Errorf("entry %d color %v is invalid", i, entry.Color)
		}
	}

	// Every visible pixel is mapped to a valid palette index and every entry
	// is used.
	used := map[int]bool{}
	for i, idx := range mapping {
		if idx < 0 || idx >= len(pal) {
			t.Fatalf("mapping[%d] out of range: %d", i, idx)
		}
		used[idx] = true
	}
	if len(used) != 4 {
		t.Errorf("only %d of 4 palette entries are used by the mapping", len(used))
	}

	// Reconstruction error in Lab: mean must be small, maximum bounded.
	var sumErr, maxErr float64
	n := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			rgb, ok := MakeColor(img.At(x, y))
			if !ok {
				continue
			}
			d := rgb.DistanceLab(pal[mapping[y*width+x]].Color)
			sumErr += d
			if d > maxErr {
				maxErr = d
			}
			n++
		}
	}
	meanErr := sumErr / float64(n)
	if meanErr > 0.08 {
		t.Errorf("mean reconstruction error too large: %v", meanErr)
	}
	if maxErr > 0.2 {
		t.Errorf("maximum reconstruction error too large: %v", maxErr)
	}

	// Each region's pixels must map to a palette entry close to its base
	// color, and distinct regions must map to distinct entries.
	regionEntries := map[int]int{}
	for region, base := range baseColors {
		idx := mapping[region*regionWidth]
		if d := pal[idx].Color.DistanceLab(base); d > 0.15 {
			t.Errorf("region %d entry is distance %v from base color %v", region, d, base.Hex())
		}
		if other, ok := regionEntries[idx]; ok {
			t.Errorf("regions %d and %d share palette entry %d", other, region, idx)
		}
		regionEntries[idx] = region
	}
}

// TestFewerColorsThanDistinct exercises real clustering: an image with many
// distinct colors is reduced to a smaller palette.
func TestFewerColorsThanDistinct(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 1))
	for x := 0; x < 32; x++ {
		col := Hsv(float64(x)*(360.0/32.0), 0.9, 0.9)
		setNRGBA(img, x, 0, col, 255)
	}

	pal, mapping, err := ExtractPaletteWithMapping(img, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pal) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(pal))
	}
	weightSum := 0.0
	for i, entry := range pal {
		if !entry.Color.IsValid() {
			t.Errorf("entry %d color %v is invalid", i, entry.Color)
		}
		if entry.Weight <= 0.0 || entry.Weight >= 1.0 {
			t.Errorf("entry %d has degenerate weight %v", i, entry.Weight)
		}
		weightSum += entry.Weight
	}
	if math.Abs(weightSum-1.0) > 1e-12 {
		t.Errorf("weights sum to %v, want 1.0", weightSum)
	}
	used := map[int]bool{}
	for _, idx := range mapping {
		if idx < 0 || idx >= 4 {
			t.Fatalf("mapping index out of range: %d", idx)
		}
		used[idx] = true
	}
	if len(used) != 4 {
		t.Errorf("only %d of 4 palette entries are used by the mapping", len(used))
	}
}

// TestRandomImages is a seeded, reproducible stress test over many random
// images. It checks the general invariants: correct entry count, valid
// colors, weights summing to one, consistent mapping/weights and that the
// total mapping weight equals the image's visible alpha weight.
func TestRandomImages(t *testing.T) {
	rng := rand.New(rand.NewSource(424242))

	for trial := 0; trial < 25; trial++ {
		w := 8 + rng.Intn(16)
		h := 8 + rng.Intn(16)
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		visibleWeight := 0.0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				// Roughly 30% of the pixels are fully transparent.
				alpha := uint8(0)
				if rng.Float64() > 0.3 {
					alpha = uint8(1 + rng.Intn(255))
				}
				col := Color{rng.Float64(), rng.Float64(), rng.Float64()}
				r, g, b := col.RGB255()
				img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: alpha})
				visibleWeight += float64(alpha) / 255.0
			}
		}

		points, _ := collectVisiblePoints(img)
		if len(points) == 0 {
			if _, err := ExtractPalette(img, 1); err == nil {
				t.Fatalf("trial %d: expected error for an image with no visible pixels", trial)
			}
			continue
		}

		k := 1 + rng.Intn(len(points))
		pal, mapping, err := ExtractPaletteWithMapping(img, k)
		if err != nil {
			t.Fatalf("trial %d: unexpected error: %v", trial, err)
		}
		if len(pal) != k {
			t.Fatalf("trial %d: expected %d entries, got %d", trial, k, len(pal))
		}
		if len(mapping) != w*h {
			t.Fatalf("trial %d: mapping length %d, want %d", trial, len(mapping), w*h)
		}

		weightSum := 0.0
		for i, entry := range pal {
			if !entry.Color.IsValid() {
				t.Errorf("trial %d: entry %d has invalid color %v", trial, i, entry.Color)
			}
			if entry.Weight < 0.0 || entry.Weight > 1.0 {
				t.Errorf("trial %d: entry %d has out-of-range weight %v", trial, i, entry.Weight)
			}
			weightSum += entry.Weight
		}
		if math.Abs(weightSum-1.0) > 1e-9 {
			t.Errorf("trial %d: weights sum to %v, want 1.0", trial, weightSum)
		}

		// Aggregate mapping weights independently and compare with the
		// reported palette weights.
		mappedWeight := make([]float64, k)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				idx := mapping[y*w+x]
				_, _, _, a := img.At(x, y).RGBA()
				if a == 0 {
					if idx != -1 {
						t.Fatalf("trial %d: transparent pixel mapped to %d", trial, idx)
					}
					continue
				}
				if idx < 0 || idx >= k {
					t.Fatalf("trial %d: visible pixel mapped to invalid index %d", trial, idx)
				}
				mappedWeight[idx] += float64(a) / 65535.0
			}
		}
		for i := range pal {
			want := mappedWeight[i] / visibleWeight
			if math.Abs(pal[i].Weight-want) > 1e-9 {
				t.Errorf("trial %d: entry %d weight %v disagrees with mapping %v", trial, i, pal[i].Weight, want)
			}
		}

		// Re-running must give a bit-identical result.
		pal2, mapping2, err := ExtractPaletteWithMapping(img, k)
		if err != nil {
			t.Fatalf("trial %d: unexpected error on re-run: %v", trial, err)
		}
		for i := range pal {
			if pal[i] != pal2[i] {
				t.Fatalf("trial %d: palette not reproducible at entry %d", trial, i)
			}
		}
		for i := range mapping {
			if mapping[i] != mapping2[i] {
				t.Fatalf("trial %d: mapping not reproducible at index %d", trial, i)
			}
		}
	}
}
