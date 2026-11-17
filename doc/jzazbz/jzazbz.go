package main

import (
	"fmt"
	"log"

	"github.com/lucasb-eyer/go-colorful"
)

// Jzazbz is defined on absolute luminance, so the same RGB difference does not
// map to the same perceived distance in the shadows and in a bright highlight.
// This example shows both input scales and the absolute-XYZ entry points.
func main() {
	// SDR: linearized RGB 1.0 means 203 cd/m² diffuse white.
	white := colorful.Color{R: 1, G: 1, B: 1}
	red := colorful.Color{R: 1, G: 0, B: 0}

	jz, az, bz, err := white.Jzazbz()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sRGB white (SDR, 203 cd/m²):      jzazbz(%.5f %.5f %.5f)\n", jz, az, bz)

	// HDR: linearized RGB 1.0 means the 10,000 cd/m² PQ peak.
	hdrJz, hdrAz, hdrBz, err := white.JzazbzHDR()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sRGB white (HDR, 10000 cd/m²):     jzazbz(%.5f %.5f %.5f)\n", hdrJz, hdrAz, hdrBz)

	sdrDist, err := white.DistanceJzazbz(red)
	if err != nil {
		log.Fatal(err)
	}
	hdrDist, err := white.DistanceJzazbzHDR(red)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("white/red distance, SDR scale:     %.5f\n", sdrDist)
	fmt.Printf("white/red distance, HDR scale:     %.5f (not interchangeable)\n", hdrDist)

	// Absolute XYZ in cd/m²: the same 1 cd/m² step is a big perceptual change
	// near black and a small one in a bright highlight.
	darkStep, err := colorful.DistanceJzazbzAbs(0, 0, 0, 0, 1, 0)
	if err != nil {
		log.Fatal(err)
	}
	brightStep, err := colorful.DistanceJzazbzAbs(0, 9000, 0, 0, 9001, 0)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("1 cd/m² step at 0 cd/m²:           %.5f\n", darkStep)
	fmt.Printf("1 cd/m² step at 9000 cd/m²:        %.5f\n", brightStep)

	// Invalid input yields an error and finite zeros, never NaN.
	_, _, _, err = colorful.XyzAbsToJzazbz(0, -1, 0)
	fmt.Printf("negative luminance:                %v\n", err)
}
