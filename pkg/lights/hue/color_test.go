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
