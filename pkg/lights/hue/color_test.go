package hue

import (
	"fmt"
	"math"
	"testing"
)

func TestKelvinMirekRoundTrip(t *testing.T) {
	for _, k := range []int{2000, 2700, 4000, 6500} {
		mirek := kelvinToMirek(k)
		gotK := mirekToKelvin(mirek)
		if diff := math.Abs(float64(gotK - k)); diff > 20 {
			t.Errorf("kelvinToMirek(%d)=%d -> mirekToKelvin=%d, diff %v too large", k, mirek, gotK, diff)
		}
	}
}

func TestHexXYRoundTrip(t *testing.T) {
	for _, hex := range []string{"#ffedbb", "#ffcf79", "#ff0000", "#00ff00", "#0000ff", "#ffffff"} {
		x, y, err := hexToXY(hex)
		if err != nil {
			t.Fatalf("hexToXY(%q) failed: %v", hex, err)
		}
		got := xyToHex(x, y)
		if got == "" {
			t.Fatalf("xyToHex(%v, %v) returned empty string for input %q", x, y, hex)
		}
		wantR, wantG, wantB := hexInts(t, hex)
		gotR, gotG, gotB := hexInts(t, got)
		const tolerance = 5
		if absDiff(wantR, gotR) > tolerance || absDiff(wantG, gotG) > tolerance || absDiff(wantB, gotB) > tolerance {
			t.Errorf("hexToXY/xyToHex round trip for %q produced %q, outside tolerance", hex, got)
		}
	}
}

func TestHexToXYInvalid(t *testing.T) {
	if _, _, err := hexToXY("#000000"); err == nil {
		t.Error("hexToXY(\"#000000\") should fail - no chromaticity point for black")
	}
	if _, _, err := hexToXY("not-a-color"); err == nil {
		t.Error("hexToXY(\"not-a-color\") should fail")
	}
}

func TestColorsMatch(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical strings match", "#00ff00", "#00ff00", true},
		{"both empty (unsupported) match", "", "", true},
		{"empty never matches a real color", "", "#00ff00", false},
		{"real color never matches empty", "#00ff00", "", false},
		// #ff6cb5 is the real observed round-trip result for a commanded
		// #ff69b4, seen live against an actual bridge this session (xy
		// distance ~0.003) - genuine numerical round-trip noise for an
		// in-gamut color, correctly absorbed by the tolerance.
		{"within round-trip tolerance", "#ff69b4", "#ff6cb5", true},
		{"clearly different colors don't match", "#00ff00", "#ff0000", false},
		// #00ff89 is the real observed result for a commanded #00ff00,
		// also seen live this session - but its xy distance from #00ff00
		// is ~0.18, far larger than round-trip noise (compare: red vs.
		// green is ~0.69). This is this bulb's gamut clamping a
		// request for a green outside what its LEDs can actually
		// produce to the closest one they can - a real, correctly
		// *not*-matched case, not a tolerance-calibration bug. A color
		// request that's genuinely outside a light's gamut will keep
		// showing as drift no matter how the tolerance is tuned, which
		// is arguably the right outcome: it's telling you the light
		// can't actually produce what was asked.
		{"out-of-gamut clamping is not round-trip noise", "#00ff00", "#00ff89", false},
		{"malformed hex never matches anything", "#00ff00", "not-a-color", false},
		{"two malformed hex strings don't match each other", "not-a-color", "also-not-a-color", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ColorsMatch(tc.a, tc.b); got != tc.want {
				t.Errorf("ColorsMatch(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			// Symmetric by construction (Euclidean distance) - assert it stays that way.
			if got := ColorsMatch(tc.b, tc.a); got != tc.want {
				t.Errorf("ColorsMatch(%q, %q) (swapped) = %v, want %v", tc.b, tc.a, got, tc.want)
			}
		})
	}
}

func hexInts(t *testing.T, hex string) (int, int, int) {
	t.Helper()
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("failed to parse hex %q: %v", hex, err)
	}
	return r, g, b
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
