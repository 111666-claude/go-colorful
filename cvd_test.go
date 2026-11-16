package colorful

import (
	"math"
	"testing"
)

// Reference matrices from Table 1 of Machado, Oliveira & Fernandes (2009),
// "A Physiologically-based Model for Simulation of Color Vision Deficiency",
// IEEE TVCG 15(6), pp. 1291-1298. Values cross-checked against the
// colour-science `CVD_MATRICES_MACHADO2010` dataset.
var cvdReferenceMatrices = map[CVDKind][11][3][3]float64{
	Protan: {
		{{1.000000, 0.000000, -0.000000}, {0.000000, 1.000000, 0.000000}, {-0.000000, -0.000000, 1.000000}},
		{{0.856167, 0.182038, -0.038205}, {0.029342, 0.955115, 0.015544}, {-0.002880, -0.001563, 1.004443}},
		{{0.734766, 0.334872, -0.069637}, {0.051840, 0.919198, 0.028963}, {-0.004928, -0.004209, 1.009137}},
		{{0.630323, 0.465641, -0.095964}, {0.069181, 0.890046, 0.040773}, {-0.006308, -0.007724, 1.014032}},
		{{0.539009, 0.579343, -0.118352}, {0.082546, 0.866121, 0.051332}, {-0.007136, -0.011959, 1.019095}},
		{{0.458064, 0.679578, -0.137642}, {0.092785, 0.846313, 0.060902}, {-0.007494, -0.016807, 1.024301}},
		{{0.385450, 0.769005, -0.154455}, {0.100526, 0.829802, 0.069673}, {-0.007442, -0.022190, 1.029632}},
		{{0.319627, 0.849633, -0.169261}, {0.106241, 0.815969, 0.077790}, {-0.007025, -0.028051, 1.035076}},
		{{0.259411, 0.923008, -0.182420}, {0.110296, 0.804340, 0.085364}, {-0.006276, -0.034346, 1.040622}},
		{{0.203876, 0.990338, -0.194214}, {0.112975, 0.794542, 0.092483}, {-0.005222, -0.041043, 1.046265}},
		{{0.152286, 1.052583, -0.204868}, {0.114503, 0.786281, 0.099216}, {-0.003882, -0.048116, 1.051998}},
	},
	Deutan: {
		{{1.000000, 0.000000, -0.000000}, {0.000000, 1.000000, 0.000000}, {-0.000000, -0.000000, 1.000000}},
		{{0.866435, 0.177704, -0.044139}, {0.049567, 0.939063, 0.011370}, {-0.003453, 0.007233, 0.996220}},
		{{0.760729, 0.319078, -0.079807}, {0.090568, 0.889315, 0.020117}, {-0.006027, 0.013325, 0.992702}},
		{{0.675425, 0.433850, -0.109275}, {0.125303, 0.847755, 0.026942}, {-0.007950, 0.018572, 0.989378}},
		{{0.605511, 0.528560, -0.134071}, {0.155318, 0.812366, 0.032316}, {-0.009376, 0.023176, 0.986200}},
		{{0.547494, 0.607765, -0.155259}, {0.181692, 0.781742, 0.036566}, {-0.010410, 0.027275, 0.983136}},
		{{0.498864, 0.674741, -0.173604}, {0.205199, 0.754872, 0.039929}, {-0.011131, 0.030969, 0.980162}},
		{{0.457771, 0.731899, -0.189670}, {0.226409, 0.731012, 0.042579}, {-0.011595, 0.034333, 0.977261}},
		{{0.422823, 0.781057, -0.203881}, {0.245752, 0.709602, 0.044646}, {-0.011843, 0.037423, 0.974421}},
		{{0.392952, 0.823610, -0.216562}, {0.263559, 0.690210, 0.046232}, {-0.011910, 0.040281, 0.971630}},
		{{0.367322, 0.860646, -0.227968}, {0.280085, 0.672501, 0.047413}, {-0.011820, 0.042940, 0.968881}},
	},
	Tritan: {
		{{1.000000, 0.000000, -0.000000}, {0.000000, 1.000000, 0.000000}, {-0.000000, -0.000000, 1.000000}},
		{{0.926670, 0.092514, -0.019184}, {0.021191, 0.964503, 0.014306}, {0.008437, 0.054813, 0.936750}},
		{{0.895720, 0.133330, -0.029050}, {0.029997, 0.945400, 0.024603}, {0.013027, 0.104707, 0.882266}},
		{{0.905871, 0.127791, -0.033662}, {0.026856, 0.941251, 0.031893}, {0.013410, 0.148296, 0.838294}},
		{{0.948035, 0.089490, -0.037526}, {0.014364, 0.946792, 0.038844}, {0.010853, 0.193991, 0.795156}},
		{{1.017277, 0.027029, -0.044306}, {-0.006113, 0.958479, 0.047634}, {0.006379, 0.248708, 0.744913}},
		{{1.104996, -0.046633, -0.058363}, {-0.032137, 0.971635, 0.060503}, {0.001336, 0.317922, 0.680742}},
		{{1.193214, -0.109812, -0.083402}, {-0.058496, 0.979410, 0.079086}, {-0.002346, 0.403492, 0.598854}},
		{{1.257728, -0.139648, -0.118081}, {-0.078003, 0.975409, 0.102594}, {-0.003316, 0.501214, 0.502102}},
		{{1.278864, -0.125333, -0.153531}, {-0.084748, 0.957674, 0.127074}, {-0.000989, 0.601151, 0.399838}},
		{{1.255528, -0.076749, -0.178779}, {-0.078411, 0.930809, 0.147602}, {0.004733, 0.691367, 0.303900}},
	},
}

// TestCVDMatrices verifies that the built-in table matches the published
// Table 1 values at every tabulated severity.
func TestCVDMatrices(t *testing.T) {
	for kind, refs := range cvdReferenceMatrices {
		for i := 0; i < 11; i++ {
			got := cvdMatrices[kind][i]
			for r := 0; r < 3; r++ {
				for c := 0; c < 3; c++ {
					if got[r][c] != refs[i][r][c] {
						t.Errorf("%v severity %.1f matrix[%d][%d] = %v, want %v",
							kind, float64(i)/10.0, r, c, got[r][c], refs[i][r][c])
					}
				}
			}
		}
	}
}

// TestCVDMatrixInterpolation checks that intermediate severities are obtained by
// linear interpolation between neighbouring tabulated matrices, and that the
// endpoints are exact.
func TestCVDMatrixInterpolation(t *testing.T) {
	for kind := range cvdReferenceMatrices {
		for _, tc := range []struct {
			severity float64
			lo, hi   int
			frac     float64
		}{
			{0.0, 0, 1, 0.0},
			{0.1, 1, 2, 0.0},
			{0.25, 2, 3, 0.5},
			{0.873, 8, 9, 0.73}, // The paper's worked interpolation example.
			{0.95, 9, 10, 0.5},
			{1.0, 9, 10, 1.0},
		} {
			m := cvdMatrix(kind, tc.severity)
			for r := 0; r < 3; r++ {
				for c := 0; c < 3; c++ {
					want := (1-tc.frac)*cvdMatrices[kind][tc.lo][r][c] + tc.frac*cvdMatrices[kind][tc.hi][r][c]
					if math.Abs(m[r][c]-want) > 1e-15 {
						t.Errorf("%v severity %v matrix[%d][%d] = %v, want %v",
							kind, tc.severity, r, c, m[r][c], want)
					}
				}
			}
		}
	}
}

// Reference results computed independently from the published matrices using
// the same pipeline: sRGB -> linear RGB, matrix multiply, clamp to [0, 1],
// linear RGB -> sRGB. The linear-space outputs equal the tabulated matrix
// entries exactly at severity 1.0.
var cvdReferenceValues = []struct {
	kind     CVDKind
	severity float64
	in, want Color
	label    string
}{
	{Protan, 0.5, Color{1, 0, 0}, Color{0.707029341, 0.336773327, 0.000000000}, "red"},
	{Protan, 0.5, Color{0, 1, 0}, Color{0.843156821, 0.929140372, 0.000000000}, "green"},
	{Protan, 0.5, Color{0, 0, 1}, Color{0.000000000, 0.273737267, 1.000000000}, "blue"},
	{Protan, 1.0, Color{1, 0, 0}, Color{0.426608472, 0.372654277, 0.000000000}, "red"},
	{Protan, 1.0, Color{0, 1, 0}, Color{1.000000000, 0.899428066, 0.000000000}, "green"},
	{Protan, 1.0, Color{0, 0, 1}, Color{0.000000000, 0.347866826, 1.000000000}, "blue"},
	{Protan, 1.0, Color{0.2, 0.4, 0.6}, Color{0.312605538, 0.409837291, 0.608525882}, "steel blue"},
	{Deutan, 0.5, Color{1, 0, 0}, Color{0.765812310, 0.463373007, 0.000000000}, "red"},
	{Deutan, 0.5, Color{0, 1, 0}, Color{0.802318764, 0.897128493, 0.180227328}, "green"},
	{Deutan, 0.5, Color{0, 0, 1}, Color{0.000000000, 0.210787610, 0.992550076}, "blue"},
	{Deutan, 1.0, Color{1, 0, 0}, Color{0.640059555, 0.565806941, 0.000000000}, "red"},
	{Deutan, 1.0, Color{0, 1, 0}, Color{0.936051046, 0.839247735, 0.229191866}, "green"},
	{Deutan, 1.0, Color{0, 0, 1}, Color{0.000000000, 0.241171359, 0.986194366}, "blue"},
	{Deutan, 1.0, Color{0.2, 0.4, 0.6}, Color{0.257412495, 0.371448157, 0.596043244}, "steel blue"},
	{Tritan, 0.5, Color{1, 0, 0}, Color{1.000000000, 0.000000000, 0.073400302}, "red"},
	{Tritan, 0.5, Color{0, 1, 0}, Color{0.179341003, 0.981522046, 0.535821817}, "green"},
	{Tritan, 0.5, Color{0, 0, 1}, Color{0.000000000, 0.241745789, 0.878175025}, "blue"},
	{Tritan, 1.0, Color{1, 0, 0}, Color{1.000000000, 0.000000000, 0.058385974}, "red"},
	{Tritan, 1.0, Color{0, 1, 0}, Color{0.000000000, 0.968947521, 0.849616272}, "green"},
	{Tritan, 1.0, Color{0, 0, 1}, Color{0.000000000, 0.420379986, 0.587278796}, "blue"},
	{Tritan, 1.0, Color{0.2, 0.4, 0.6}, Color{0.000000000, 0.446844922, 0.471755956}, "steel blue"},
}

func TestSimulateCVDReferenceValues(t *testing.T) {
	const eps = 1e-9
	for _, tc := range cvdReferenceValues {
		got, err := SimulateCVD(tc.in, tc.kind, tc.severity)
		if err != nil {
			t.Errorf("%v %v severity %v: unexpected error: %v", tc.kind, tc.label, tc.severity, err)
			continue
		}
		if math.Abs(got.R-tc.want.R) > eps || math.Abs(got.G-tc.want.G) > eps || math.Abs(got.B-tc.want.B) > eps {
			t.Errorf("%v %v severity %v: got %v, want %v", tc.kind, tc.label, tc.severity, got, tc.want)
		}
	}
}

// TestSimulateCVDLinearSpace checks directly that the matrix is applied in
// linear RGB and not to gamma-encoded values: at severity 1.0 the simulated
// linear red channel must equal the matrix's first row applied to the
// linearized input.
func TestSimulateCVDLinearSpace(t *testing.T) {
	in := Color{0.8, 0.3, 0.1}
	lr, lg, lb := in.LinearRgb()
	m := cvdMatrices[Tritan][10]
	wantLinear := [3]float64{
		m[0][0]*lr + m[0][1]*lg + m[0][2]*lb,
		m[1][0]*lr + m[1][1]*lg + m[1][2]*lb,
		m[2][0]*lr + m[2][1]*lg + m[2][2]*lb,
	}
	got, err := SimulateCVD(in, Tritan, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	gr, gg, gb := got.LinearRgb()
	for i, pair := range [][2]float64{{gr, wantLinear[0]}, {gg, wantLinear[1]}, {gb, wantLinear[2]}} {
		want := clamp01(pair[1])
		if math.Abs(pair[0]-want) > 1e-12 {
			t.Errorf("linear channel %d = %v, want %v", i, pair[0], want)
		}
	}

	// Sanity check: applying the matrix to the gamma-encoded values would
	// give a different answer, so this test would catch that mistake.
	wrongR := m[0][0]*in.R + m[0][1]*in.G + m[0][2]*in.B
	if math.Abs(gr-clamp01(wrongR)) < 1e-6 {
		t.Errorf("linear channel 0 unexpectedly matches a gamma-space application")
	}
}

func TestSimulateCVDZeroSeverityIsIdentity(t *testing.T) {
	colors := []Color{
		{0, 0, 0}, {1, 1, 1}, {0.5, 0.5, 0.5},
		{1, 0, 0}, {0, 1, 0}, {0, 0, 1},
		{0.2, 0.4, 0.6}, {0.9, 0.1, 0.35},
	}
	for kind := range cvdReferenceMatrices {
		for _, c := range colors {
			got, err := SimulateCVD(c, kind, 0.0)
			if err != nil {
				t.Fatalf("%v severity 0: unexpected error: %v", kind, err)
			}
			if got != c {
				t.Errorf("%v severity 0: got %v, want identical %v", kind, got, c)
			}
		}
	}
}

// TestSimulateCVDContinuity checks that the result changes continuously as the
// severity grows from 0 to 1. Continuity is asserted in linear light, where the
// simulation actually operates; sRGB re-encoding has an infinite slope at zero,
// so a pure primary can move noticeably in encoded units for a tiny severity
// change even though the linear result is continuous. The sRGB bound below is
// generous enough for that slope but still far below a real discontinuity at a
// tabulated knot.
func TestSimulateCVDContinuity(t *testing.T) {
	const severityStep = 1e-4
	colors := []Color{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {0.2, 0.4, 0.6}, {0.7, 0.7, 0.1}}
	for kind := range cvdReferenceMatrices {
		for _, c := range colors {
			prev, err := SimulateCVD(c, kind, 0.0)
			if err != nil {
				t.Fatal(err)
			}
			pr, pg, pb := prev.LinearRgb()
			maxLinearStep, maxSrgbStep := 0.0, 0.0
			for i := 1; i <= 1.0/severityStep; i++ {
				s := float64(i) * severityStep
				cur, err := SimulateCVD(c, kind, s)
				if err != nil {
					t.Fatalf("%v severity %v: %v", kind, s, err)
				}
				cr, cg, cb := cur.LinearRgb()
				linearStep := math.Abs(cr-pr) + math.Abs(cg-pg) + math.Abs(cb-pb)
				srgbStep := math.Abs(cur.R-prev.R) + math.Abs(cur.G-prev.G) + math.Abs(cur.B-prev.B)
				if linearStep > maxLinearStep {
					maxLinearStep = linearStep
				}
				if srgbStep > maxSrgbStep {
					maxSrgbStep = srgbStep
				}
				prev, pr, pg, pb = cur, cr, cg, cb
			}
			if maxLinearStep > 5e-3 {
				t.Errorf("%v %v: discontinuity in linear light, max step %v for severity step %v",
					kind, c, maxLinearStep, severityStep)
			}
			if maxSrgbStep > 0.05 {
				t.Errorf("%v %v: discontinuity in sRGB, max step %v for severity step %v",
					kind, c, maxSrgbStep, severityStep)
			}
		}
	}
}

// TestSimulateCVDGraysStayNeutral verifies that neutral grays remain neutral:
// all three output channels must be equal for gray input.
func TestSimulateCVDGraysStayNeutral(t *testing.T) {
	const eps = 1e-6
	grays := []float64{0.0, 0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.99, 1.0}
	for kind := range cvdReferenceMatrices {
		for _, g := range grays {
			for i := 0; i <= 20; i++ {
				s := float64(i) / 20.0
				got, err := SimulateCVD(Color{g, g, g}, kind, s)
				if err != nil {
					t.Fatalf("%v gray %v severity %v: %v", kind, g, s, err)
				}
				if math.Abs(got.R-got.G) > eps || math.Abs(got.R-got.B) > eps {
					t.Errorf("%v gray %v severity %v: got non-neutral %v", kind, g, s, got)
				}
				// Gray must also retain its lightness value.
				if math.Abs(got.R-g) > eps {
					t.Errorf("%v gray %v severity %v: lightness changed to %v", kind, g, s, got.R)
				}
			}
		}
	}
}

// TestSimulateCVDAlwaysValid verifies that every output is finite and inside
// the sRGB cube, including for out-of-gamut inputs and for primaries that the
// matrices push outside [0, 1].
func TestSimulateCVDAlwaysValid(t *testing.T) {
	inputs := []Color{
		{0, 0, 0}, {1, 1, 1},
		{1, 0, 0}, {0, 1, 0}, {0, 0, 1},
		{1, 0, 1}, {0, 1, 1}, {1, 1, 0},
		{-0.5, 0.5, 1.5}, {2, -1, 0.5}, {1e-9, 1 - 1e-9, 0.5},
	}
	for kind := range cvdReferenceMatrices {
		for _, in := range inputs {
			for i := 0; i <= 100; i++ {
				s := float64(i) / 100.0
				got, err := SimulateCVD(in, kind, s)
				if err != nil {
					t.Fatalf("%v %v severity %v: %v", kind, in, s, err)
				}
				if !isFinite(got.R) || !isFinite(got.G) || !isFinite(got.B) {
					t.Errorf("%v %v severity %v: non-finite output %v", kind, in, s, got)
				}
				if !got.IsValid() {
					t.Errorf("%v %v severity %v: invalid output %v", kind, in, s, got)
				}
			}
		}
	}
}

// TestSimulateCVDDistinctKinds verifies that the three deficiencies do not
// silently fall back to one another.
func TestSimulateCVDDistinctKinds(t *testing.T) {
	in := Color{1, 0, 0}
	protan, _ := SimulateCVD(in, Protan, 1.0)
	deutan, _ := SimulateCVD(in, Deutan, 1.0)
	tritan, _ := SimulateCVD(in, Tritan, 1.0)
	if protan == deutan || protan == tritan || deutan == tritan {
		t.Errorf("different deficiency kinds produced identical results: %v %v %v", protan, deutan, tritan)
	}
}

func TestSimulateCVDErrors(t *testing.T) {
	valid := Color{0.2, 0.4, 0.6}
	tests := []struct {
		name     string
		col      Color
		kind     CVDKind
		severity float64
	}{
		{"unknown kind negative", valid, CVDKind(-1), 0.5},
		{"unknown kind too large", valid, CVDKind(3), 0.5},
		{"unknown kind far", valid, CVDKind(42), 0.5},
		{"severity negative", valid, Protan, -0.001},
		{"severity above one", valid, Protan, 1.001},
		{"severity NaN", valid, Protan, math.NaN()},
		{"severity +Inf", valid, Protan, math.Inf(1)},
		{"severity -Inf", valid, Protan, math.Inf(-1)},
		{"color NaN R", Color{math.NaN(), 0, 0}, Protan, 0.5},
		{"color NaN G", Color{0, math.NaN(), 0}, Deutan, 0.5},
		{"color NaN B", Color{0, 0, math.NaN()}, Tritan, 0.5},
		{"color +Inf", Color{math.Inf(1), 0, 0}, Protan, 0.5},
		{"color -Inf", Color{0, math.Inf(-1), 0}, Protan, 0.5},
		{"color NaN at severity zero", Color{math.NaN(), 0, 0}, Protan, 0},
		{"unknown kind at severity zero", valid, CVDKind(99), 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SimulateCVD(tc.col, tc.kind, tc.severity)
			if err == nil {
				t.Fatalf("SimulateCVD(%v, %v, %v) = %v, want error", tc.col, tc.kind, tc.severity, got)
			}
			if got != (Color{}) {
				t.Errorf("error case returned non-zero color %v", got)
			}
		})
	}
}

func TestCVDKindString(t *testing.T) {
	for kind, want := range map[CVDKind]string{Protan: "protan", Deutan: "deutan", Tritan: "tritan"} {
		if got := kind.String(); got != want {
			t.Errorf("CVDKind(%d).String() = %q, want %q", int(kind), got, want)
		}
	}
	if got := CVDKind(7).String(); got != "CVDKind(7)" {
		t.Errorf("CVDKind(7).String() = %q, want %q", got, "CVDKind(7)")
	}
}

func BenchmarkSimulateCVD(b *testing.B) {
	c := Color{0.2, 0.4, 0.6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SimulateCVD(c, Protan, 0.5)
	}
}
