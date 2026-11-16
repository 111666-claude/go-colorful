package main

import (
	"fmt"

	"github.com/lucasb-eyer/go-colorful"
)

// APCA (Accessible Perceptual Contrast Algorithm) measures text readability,
// not color similarity. It is polarity sensitive: light text on a dark button
// returns a negative value, dark text on a light button a positive one, and
// swapping the two does not simply flip the sign.
func main() {
	darkText, _ := colorful.Hex("#123456")
	lightText, _ := colorful.Hex("#ddeeff")
	lightBg, _ := colorful.Hex("#ddeeff")
	darkBg, _ := colorful.Hex("#123456")

	lcDarkOnLight, _ := darkText.ContrastAPCA(lightBg)
	lcLightOnDark, _ := lightText.ContrastAPCA(darkBg)
	fmt.Printf("dark text on light button: %+.2f\n", lcDarkOnLight)
	fmt.Printf("light text on dark button: %+.2f\n", lcLightOnDark)

	// Alpha-aware call: translucent black text over an opaque white
	// background, with white itself as the opaque base seen behind it.
	black, _ := colorful.Hex("#000000")
	white, _ := colorful.Hex("#ffffff")
	translucentBlack := colorful.AlphaColor(black, 0.4)
	lcAlpha, err := translucentBlack.ContrastAPCA(
		colorful.AlphaColor(white, 1.0), white)
	if err != nil {
		panic(err)
	}
	fmt.Printf("black at 40%% alpha on white: %.2f\n", lcAlpha)

	// A translucent layer without an opaque base is undefined and errors
	// instead of silently producing a meaningless number.
	if _, err := translucentBlack.ContrastAPCA(
		colorful.AlphaColor(white, 1.0)); err != nil {
		fmt.Printf("missing opaque base: %v\n", err)
	}
}
