// This file provides simulation of the three main types of color vision
// deficiency (CVD), for previewing colors as they are perceived by people with
// protanopia/protanomaly, deuteranopia/deuteranomaly, or
// tritanopia/tritanomaly.
//
// The simulation uses the pre-computed matrices from:
//
//	Gustavo M. Machado, Manuel M. Oliveira, and Leandro A. F. Fernandes,
//	"A Physiologically-based Model for Simulation of Color Vision Deficiency",
//	IEEE Transactions on Visualization and Computer Graphics, Volume 15 (2009),
//	Number 6, November/December 2009, pp. 1291-1298.
//	DOI: 10.1109/TVCG.2009.113
//
// Table 1 of that paper lists one matrix per deficiency type and severity level
// from 0.0 (no deficiency) to 1.0 (dichromacy), in steps of 0.1. The matrices
// operate on linear RGB; this implementation therefore linearizes the sRGB
// input, applies the matrix, and re-encodes the result to sRGB.

package colorful

import (
	"fmt"
	"math"
)

// CVDKind identifies a type of color vision deficiency.
type CVDKind int

const (
	// Protan simulates protanopia/protanomaly: reduced or absent L-cone
	// (long wavelength, red) sensitivity.
	Protan CVDKind = iota
	// Deutan simulates deuteranopia/deuteranomaly: reduced or absent M-cone
	// (medium wavelength, green) sensitivity.
	Deutan
	// Tritan simulates tritanopia/tritanomaly: reduced or absent S-cone
	// (short wavelength, blue) sensitivity.
	Tritan
)

// String returns the name of the deficiency, one of "protan", "deutan" or
// "tritan".
func (k CVDKind) String() string {
	switch k {
	case Protan:
		return "protan"
	case Deutan:
		return "deutan"
	case Tritan:
		return "tritan"
	}
	return fmt.Sprintf("CVDKind(%d)", int(k))
}

// cvdMatrices holds the Machado et al. (2009) Table 1 matrices, indexed by
// deficiency kind, then by severity in tenths: index 0 is severity 0.0 and
// index 10 is severity 1.0. Each matrix is row-major and maps linear RGB to
// simulated linear RGB.
var cvdMatrices = map[CVDKind][11][3][3]float64{
	Protan: {
		{{1.000000, 0.000000, -0.000000}, {0.000000, 1.000000, 0.000000}, {-0.000000, -0.000000, 1.000000}}, // 0.0
		{{0.856167, 0.182038, -0.038205}, {0.029342, 0.955115, 0.015544}, {-0.002880, -0.001563, 1.004443}}, // 0.1
		{{0.734766, 0.334872, -0.069637}, {0.051840, 0.919198, 0.028963}, {-0.004928, -0.004209, 1.009137}}, // 0.2
		{{0.630323, 0.465641, -0.095964}, {0.069181, 0.890046, 0.040773}, {-0.006308, -0.007724, 1.014032}}, // 0.3
		{{0.539009, 0.579343, -0.118352}, {0.082546, 0.866121, 0.051332}, {-0.007136, -0.011959, 1.019095}}, // 0.4
		{{0.458064, 0.679578, -0.137642}, {0.092785, 0.846313, 0.060902}, {-0.007494, -0.016807, 1.024301}}, // 0.5
		{{0.385450, 0.769005, -0.154455}, {0.100526, 0.829802, 0.069673}, {-0.007442, -0.022190, 1.029632}}, // 0.6
		{{0.319627, 0.849633, -0.169261}, {0.106241, 0.815969, 0.077790}, {-0.007025, -0.028051, 1.035076}}, // 0.7
		{{0.259411, 0.923008, -0.182420}, {0.110296, 0.804340, 0.085364}, {-0.006276, -0.034346, 1.040622}}, // 0.8
		{{0.203876, 0.990338, -0.194214}, {0.112975, 0.794542, 0.092483}, {-0.005222, -0.041043, 1.046265}}, // 0.9
		{{0.152286, 1.052583, -0.204868}, {0.114503, 0.786281, 0.099216}, {-0.003882, -0.048116, 1.051998}}, // 1.0
	},
	Deutan: {
		{{1.000000, 0.000000, -0.000000}, {0.000000, 1.000000, 0.000000}, {-0.000000, -0.000000, 1.000000}}, // 0.0
		{{0.866435, 0.177704, -0.044139}, {0.049567, 0.939063, 0.011370}, {-0.003453, 0.007233, 0.996220}},  // 0.1
		{{0.760729, 0.319078, -0.079807}, {0.090568, 0.889315, 0.020117}, {-0.006027, 0.013325, 0.992702}},  // 0.2
		{{0.675425, 0.433850, -0.109275}, {0.125303, 0.847755, 0.026942}, {-0.007950, 0.018572, 0.989378}},  // 0.3
		{{0.605511, 0.528560, -0.134071}, {0.155318, 0.812366, 0.032316}, {-0.009376, 0.023176, 0.986200}},  // 0.4
		{{0.547494, 0.607765, -0.155259}, {0.181692, 0.781742, 0.036566}, {-0.010410, 0.027275, 0.983136}},  // 0.5
		{{0.498864, 0.674741, -0.173604}, {0.205199, 0.754872, 0.039929}, {-0.011131, 0.030969, 0.980162}},  // 0.6
		{{0.457771, 0.731899, -0.189670}, {0.226409, 0.731012, 0.042579}, {-0.011595, 0.034333, 0.977261}},  // 0.7
		{{0.422823, 0.781057, -0.203881}, {0.245752, 0.709602, 0.044646}, {-0.011843, 0.037423, 0.974421}},  // 0.8
		{{0.392952, 0.823610, -0.216562}, {0.263559, 0.690210, 0.046232}, {-0.011910, 0.040281, 0.971630}},  // 0.9
		{{0.367322, 0.860646, -0.227968}, {0.280085, 0.672501, 0.047413}, {-0.011820, 0.042940, 0.968881}},  // 1.0
	},
	Tritan: {
		{{1.000000, 0.000000, -0.000000}, {0.000000, 1.000000, 0.000000}, {-0.000000, -0.000000, 1.000000}},  // 0.0
		{{0.926670, 0.092514, -0.019184}, {0.021191, 0.964503, 0.014306}, {0.008437, 0.054813, 0.936750}},    // 0.1
		{{0.895720, 0.133330, -0.029050}, {0.029997, 0.945400, 0.024603}, {0.013027, 0.104707, 0.882266}},    // 0.2
		{{0.905871, 0.127791, -0.033662}, {0.026856, 0.941251, 0.031893}, {0.013410, 0.148296, 0.838294}},    // 0.3
		{{0.948035, 0.089490, -0.037526}, {0.014364, 0.946792, 0.038844}, {0.010853, 0.193991, 0.795156}},    // 0.4
		{{1.017277, 0.027029, -0.044306}, {-0.006113, 0.958479, 0.047634}, {0.006379, 0.248708, 0.744913}},   // 0.5
		{{1.104996, -0.046633, -0.058363}, {-0.032137, 0.971635, 0.060503}, {0.001336, 0.317922, 0.680742}},  // 0.6
		{{1.193214, -0.109812, -0.083402}, {-0.058496, 0.979410, 0.079086}, {-0.002346, 0.403492, 0.598854}}, // 0.7
		{{1.257728, -0.139648, -0.118081}, {-0.078003, 0.975409, 0.102594}, {-0.003316, 0.501214, 0.502102}}, // 0.8
		{{1.278864, -0.125333, -0.153531}, {-0.084748, 0.957674, 0.127074}, {-0.000989, 0.601151, 0.399838}}, // 0.9
		{{1.255528, -0.076749, -0.178779}, {-0.078411, 0.930809, 0.147602}, {0.004733, 0.691367, 0.303900}},  // 1.0
	},
}

// cvdMatrix returns the simulation matrix for kind at the given severity,
// linearly interpolating between the tabulated matrices at severity steps of
// 0.1 as recommended by Machado et al. The severity must be in [0, 1] and kind
// must be a known CVDKind; both are validated by SimulateCVD.
func cvdMatrix(kind CVDKind, severity float64) [3][3]float64 {
	pos := severity * 10.0
	lo := int(math.Floor(pos))
	if lo >= 10 {
		return cvdMatrices[kind][10]
	}
	frac := pos - float64(lo)
	m := cvdMatrices[kind]
	l, h := m[lo], m[lo+1]
	var out [3][3]float64
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			out[r][c] = (1.0-frac)*l[r][c] + frac*h[r][c]
		}
	}
	return out
}

// SimulateCVD returns col as perceived by a person with the given color vision
// deficiency at the given severity.
//
// kind selects the deficiency (Protan, Deutan or Tritan) and severity ranges
// from 0 (normal color vision; the returned color equals col) to 1 (full
// dichromacy). Intermediate values model anomalous trichromacy. The severity is
// continuous: intermediate matrices are obtained by linear interpolation
// between the tabulated Machado et al. (2009) matrices.
//
// The model is applied in linear RGB: col is linearized, multiplied by the
// simulation matrix, and the result is re-encoded to sRGB. Because the matrix
// can produce linear components outside [0, 1], which cannot be displayed, each
// channel of the result is clamped to [0, 1] before re-encoding. This is a
// per-channel gamut clip; it affects only out-of-gamut results and always
// yields a valid, finite Color. The clamp also applies at severity 0, so an
// out-of-sRGB input (which is not displayable to begin with) is clamped rather
// than returned unchanged; for inputs already in [0, 1] severity 0 is the
// exact identity. Neutral grays stay neutral because the simulation matrices
// have rows summing to one.
//
// An error is returned if col has a non-finite component, if kind is unknown,
// or if severity is NaN or outside [0, 1]. No partially-simulated color is
// returned in that case.
func SimulateCVD(col Color, kind CVDKind, severity float64) (Color, error) {
	if !isFinite(col.R) || !isFinite(col.G) || !isFinite(col.B) {
		return Color{}, fmt.Errorf("cvd: color %v has a non-finite component", col)
	}
	if _, ok := cvdMatrices[kind]; !ok {
		return Color{}, fmt.Errorf("cvd: unknown color vision deficiency kind %v", kind)
	}
	if math.IsNaN(severity) || severity < 0.0 || severity > 1.0 {
		return Color{}, fmt.Errorf("cvd: severity %v is outside [0, 1]", severity)
	}

	// A severity of zero is the identity by definition; avoid the round-trip
	// through linear RGB so in-gamut inputs come back bit-for-bit unchanged.
	if severity == 0.0 {
		return col.Clamped(), nil
	}

	m := cvdMatrix(kind, severity)
	r, g, b := col.LinearRgb()
	nr := m[0][0]*r + m[0][1]*g + m[0][2]*b
	ng := m[1][0]*r + m[1][1]*g + m[1][2]*b
	nb := m[2][0]*r + m[2][1]*g + m[2][2]*b

	return Color{
		delinearize(clamp01(nr)),
		delinearize(clamp01(ng)),
		delinearize(clamp01(nb)),
	}, nil
}

// isFinite reports whether v is neither NaN nor an infinity.
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
