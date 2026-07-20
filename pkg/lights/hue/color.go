package hue

import (
	"fmt"
	"math"
)

// xyToHex approximates the sRGB hex color a light's CIE xy chromaticity
// coordinates would render as, using Philips' documented Wide RGB D65
// conversion matrix and sRGB gamma correction. This is a display swatch,
// not a control value - it assumes full brightness (Y=1) since the actual
// dimming level is already shown separately.
func xyToHex(x, y float64) string {
	if y == 0 {
		return ""
	}

	z := 1.0 - x - y
	Y := 1.0
	X := (Y / y) * x
	Z := (Y / y) * z

	r := X*1.656492 - Y*0.354851 - Z*0.255038
	g := -X*0.707196 + Y*1.655397 + Z*0.036152
	b := X*0.051713 - Y*0.121364 + Z*1.011530

	r, g, b = gammaCorrect(r), gammaCorrect(g), gammaCorrect(b)

	if m := math.Max(r, math.Max(g, b)); m > 1 {
		r, g, b = r/m, g/m, b/m
	}
	r, g, b = math.Max(r, 0), math.Max(g, 0), math.Max(b, 0)

	return fmt.Sprintf("#%02x%02x%02x", toByte(r), toByte(g), toByte(b))
}

func gammaCorrect(c float64) float64 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1.0/2.4) - 0.055
}

func toByte(c float64) int {
	return int(math.Round(c * 255))
}

// mirekToKelvin converts a Hue color-temperature "mirek" value (153-500)
// to the more familiar Kelvin scale (roughly 2000-6500K).
func mirekToKelvin(mirek int) int {
	if mirek == 0 {
		return 0
	}
	return int(math.Round(1_000_000.0 / float64(mirek)))
}
