package hue

import (
	"fmt"
	"math"
	"strconv"
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

// kelvinToMirek converts Kelvin back to a Hue "mirek" value - the same
// formula as mirekToKelvin, since mirek = 1,000,000/K is self-inverse.
func kelvinToMirek(k int) int {
	if k == 0 {
		return 0
	}
	return int(math.Round(1_000_000.0 / float64(k)))
}

func inverseGammaCorrect(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// hexToXY converts a "#rrggbb" color back to the CIE xy chromaticity
// coordinates xyToHex derives it from, using the exact mathematical
// inverse of that function's XYZ->RGB matrix (verified by directly
// inverting the 3x3 matrix - the result matches the widely-published
// Philips "Wide RGB D65" RGB->XYZ matrix to 5 decimal places) so the two
// functions round-trip consistently rather than drifting apart under a
// different, independently-sourced matrix.
func hexToXY(hex string) (x, y float64, err error) {
	if len(hex) != 7 || hex[0] != '#' {
		return 0, 0, fmt.Errorf("invalid hex color %q", hex)
	}
	rInt, err := strconv.ParseUint(hex[1:3], 16, 8)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hex color %q: %w", hex, err)
	}
	gInt, err := strconv.ParseUint(hex[3:5], 16, 8)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hex color %q: %w", hex, err)
	}
	bInt, err := strconv.ParseUint(hex[5:7], 16, 8)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hex color %q: %w", hex, err)
	}

	r := inverseGammaCorrect(float64(rInt) / 255.0)
	g := inverseGammaCorrect(float64(gInt) / 255.0)
	b := inverseGammaCorrect(float64(bInt) / 255.0)

	X := r*0.664511 + g*0.154324 + b*0.162028
	Y := r*0.283881 + g*0.668433 + b*0.047685
	Z := r*0.000088 + g*0.072310 + b*0.986039

	if X+Y+Z == 0 {
		return 0, 0, fmt.Errorf("cannot derive a chromaticity point from %q", hex)
	}
	return X / (X + Y + Z), Y / (X + Y + Z), nil
}
