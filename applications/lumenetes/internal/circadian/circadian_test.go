package circadian

import (
	"testing"
	"time"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/sun"
)

var equator = sun.Coordinates{Latitude: 0, Longitude: 0}

func fourKeyframes() []lumenetesv1alpha1.CircadianKeyframe {
	return []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 15, ColorTempK: 2200},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarNoon, Brightness: 100, ColorTempK: 6500},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunset, Brightness: 70, ColorTempK: 3500},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarMidnight, Brightness: 5, ColorTempK: 2000},
	}
}

func TestInterpolate_TooFewKeyframes(t *testing.T) {
	_, _, _, err := Interpolate(nil, equator, time.Now())
	if err == nil {
		t.Fatal("expected error for 0 keyframes")
	}
	one := []lumenetesv1alpha1.CircadianKeyframe{{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 10, ColorTempK: 2000}}
	if _, _, _, err := Interpolate(one, equator, time.Now()); err == nil {
		t.Fatal("expected error for 1 keyframe")
	}
}

func TestInterpolate_UnknownAnchor(t *testing.T) {
	kfs := []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: "bogus", Brightness: 10, ColorTempK: 2000},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunset, Brightness: 90, ColorTempK: 5000},
	}
	if _, _, _, err := Interpolate(kfs, equator, time.Now()); err == nil {
		t.Fatal("expected error for unknown anchor")
	}
}

func TestInterpolate_ExactlyAtKeyframeInstant(t *testing.T) {
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	kfs := fourKeyframes()

	b, c, _, err := Interpolate(kfs, equator, times.Sunrise)
	if err != nil {
		t.Fatalf("Interpolate at sunrise: %v", err)
	}
	if b != 15 || c != 2200 {
		t.Errorf("Interpolate at exact sunrise = (%d, %d), want (15, 2200)", b, c)
	}

	b, c, _, err = Interpolate(kfs, equator, times.SolarNoon)
	if err != nil {
		t.Fatalf("Interpolate at solar noon: %v", err)
	}
	if b != 100 || c != 6500 {
		t.Errorf("Interpolate at exact solar noon = (%d, %d), want (100, 6500)", b, c)
	}
}

func TestInterpolate_MidpointBetweenKeyframes(t *testing.T) {
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	kfs := fourKeyframes()

	midpoint := times.Sunrise.Add(times.SolarNoon.Sub(times.Sunrise) / 2)
	b, c, _, err := Interpolate(kfs, equator, midpoint)
	if err != nil {
		t.Fatalf("Interpolate at midpoint: %v", err)
	}
	// Sunrise (15, 2200) -> SolarNoon (100, 6500), so the midpoint should
	// be close to the arithmetic mean of each, within rounding.
	if wantB := int32(57); abs32(b-wantB) > 1 {
		t.Errorf("brightness at midpoint = %d, want ~%d", b, wantB)
	}
	if wantC := int32(4350); abs32(c-wantC) > 1 {
		t.Errorf("colorTempK at midpoint = %d, want ~%d", c, wantC)
	}
}

func TestInterpolate_MidnightWrapBeforeDawn(t *testing.T) {
	// A couple hours before today's sunrise should be close to today's
	// sunrise keyframe (15, 2200), not close to last night's solar
	// midnight value or - if the wraparound search window were missing
	// entirely - erroring out for lack of a same-day "before" keyframe.
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	kfs := fourKeyframes()

	before := times.Sunrise.Add(-2 * time.Hour)
	b, _, _, err := Interpolate(kfs, equator, before)
	if err != nil {
		t.Fatalf("Interpolate before dawn: %v", err)
	}
	if b < 5 || b > 15 {
		t.Errorf("brightness 2h before sunrise = %d, want between the solar-midnight (5) and sunrise (15) keyframes, close to sunrise", b)
	}
}

func TestInterpolate_MidnightWrapAfterDusk(t *testing.T) {
	// A couple hours after today's sunset should be close to today's
	// sunset keyframe (70, 3500), heading toward tonight's solar midnight
	// (5, 2000) - proving the "late at night" direction of the wrap.
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	kfs := fourKeyframes()

	after := times.Sunset.Add(2 * time.Hour)
	b, _, _, err := Interpolate(kfs, equator, after)
	if err != nil {
		t.Fatalf("Interpolate after dusk: %v", err)
	}
	if b < 5 || b > 70 {
		t.Errorf("brightness 2h after sunset = %d, want between sunset (70) and solar-midnight (5) keyframes", b)
	}
}

func TestInterpolate_CrossMidnightSpan(t *testing.T) {
	// A minimal 2-keyframe schedule (sunrise/sunset only) forces the
	// bracket found for the dead of night to span yesterday's sunset to
	// today's sunrise - a real cross-midnight interpolation, not just
	// "close to the nearer keyframe".
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	yesterday, err := sun.Compute(equator, date.AddDate(0, 0, -1))
	if err != nil {
		t.Fatalf("sun.Compute yesterday: %v", err)
	}
	today, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute today: %v", err)
	}
	kfs := []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 10, ColorTempK: 2000},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunset, Brightness: 100, ColorTempK: 6000},
	}

	midnight := yesterday.Sunset.Add(today.Sunrise.Sub(yesterday.Sunset) / 2)
	b, c, _, err := Interpolate(kfs, equator, midnight)
	if err != nil {
		t.Fatalf("Interpolate at cross-midnight midpoint: %v", err)
	}
	if wantB := int32(55); abs32(b-wantB) > 2 {
		t.Errorf("brightness at cross-midnight midpoint = %d, want ~%d", b, wantB)
	}
	if wantC := int32(4000); abs32(c-wantC) > 5 {
		t.Errorf("colorTempK at cross-midnight midpoint = %d, want ~%d", c, wantC)
	}
}

func TestInterpolate_OutOfOrderKeyframesStillWork(t *testing.T) {
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}

	inOrder := fourKeyframes()
	reversed := make([]lumenetesv1alpha1.CircadianKeyframe, len(inOrder))
	for i, kf := range inOrder {
		reversed[len(inOrder)-1-i] = kf
	}

	midpoint := times.Sunrise.Add(times.SolarNoon.Sub(times.Sunrise) / 2)
	bOrdered, cOrdered, _, err := Interpolate(inOrder, equator, midpoint)
	if err != nil {
		t.Fatalf("Interpolate (in-order): %v", err)
	}
	bReversed, cReversed, _, err := Interpolate(reversed, equator, midpoint)
	if err != nil {
		t.Fatalf("Interpolate (reversed): %v", err)
	}
	if bOrdered != bReversed || cOrdered != cReversed {
		t.Errorf("keyframe order changed the result: in-order=(%d,%d) reversed=(%d,%d)", bOrdered, cOrdered, bReversed, cReversed)
	}
}

func TestInterpolate_DuplicateKeyframesDontBreakInterpolation(t *testing.T) {
	// Two keyframes resolving to the exact same instant (same anchor,
	// same offset) shouldn't produce a degenerate/zero-length interval
	// error - they're redundant, not contradictory, since `now` is always
	// bracketed relative to itself, not to adjacent list entries.
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	kfs := []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 10, ColorTempK: 2000},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 10, ColorTempK: 2000},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunset, Brightness: 100, ColorTempK: 6000},
	}
	b, c, _, err := Interpolate(kfs, equator, times.Sunrise)
	if err != nil {
		t.Fatalf("Interpolate with duplicate keyframes: %v", err)
	}
	if b != 10 || c != 2000 {
		t.Errorf("Interpolate with duplicates at sunrise = (%d, %d), want (10, 2000)", b, c)
	}
}

func TestInterpolate_MaxOffsetKeyframesStillBracket(t *testing.T) {
	// Keyframes at the +/-12h OffsetMinutes bound should still bracket
	// correctly within the 3-day search window.
	kfs := []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarNoon, OffsetMinutes: -720, Brightness: 0, ColorTempK: 2000},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarNoon, OffsetMinutes: 720, Brightness: 100, ColorTempK: 6000},
	}
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	if _, _, _, err := Interpolate(kfs, equator, times.SolarNoon); err != nil {
		t.Fatalf("Interpolate with max-offset keyframes: %v", err)
	}
}

func TestInterpolate_OnDefaultsToUnchanged(t *testing.T) {
	// None of fourKeyframes() sets On, so the schedule doesn't manage
	// on/off at all - the resolved value should be Unchanged, not On/Off.
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	_, _, on, err := Interpolate(fourKeyframes(), equator, times.SolarNoon)
	if err != nil {
		t.Fatalf("Interpolate: %v", err)
	}
	if on != lumenetesv1alpha1.CircadianOnStateUnchanged {
		t.Errorf("On = %q, want %q", on, lumenetesv1alpha1.CircadianOnStateUnchanged)
	}
}

func TestInterpolate_OnStepFunction(t *testing.T) {
	// On is a step function, not a curve: it should hold Off from solar
	// midnight right up to (and including) sunrise, then flip to On -
	// never some blended in-between value.
	date := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times, err := sun.Compute(equator, date)
	if err != nil {
		t.Fatalf("sun.Compute: %v", err)
	}
	kfs := []lumenetesv1alpha1.CircadianKeyframe{
		{Anchor: lumenetesv1alpha1.CircadianAnchorSolarMidnight, Brightness: 0, ColorTempK: 2200, On: lumenetesv1alpha1.CircadianOnStateOff},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunrise, Brightness: 40, ColorTempK: 3000, On: lumenetesv1alpha1.CircadianOnStateOn},
		{Anchor: lumenetesv1alpha1.CircadianAnchorSunset, Brightness: 50, ColorTempK: 3000},
	}

	_, _, on, err := Interpolate(kfs, equator, times.Sunrise.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("Interpolate before sunrise: %v", err)
	}
	if on != lumenetesv1alpha1.CircadianOnStateOff {
		t.Errorf("On 2h before sunrise = %q, want %q", on, lumenetesv1alpha1.CircadianOnStateOff)
	}

	_, _, on, err = Interpolate(kfs, equator, times.Sunrise)
	if err != nil {
		t.Fatalf("Interpolate at sunrise: %v", err)
	}
	if on != lumenetesv1alpha1.CircadianOnStateOn {
		t.Errorf("On at sunrise = %q, want %q", on, lumenetesv1alpha1.CircadianOnStateOn)
	}

	_, _, on, err = Interpolate(kfs, equator, times.Sunset.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("Interpolate after sunset: %v", err)
	}
	if on != lumenetesv1alpha1.CircadianOnStateOn {
		t.Errorf("On 2h after sunset = %q, want %q (holds until next day's midnight Off)", on, lumenetesv1alpha1.CircadianOnStateOn)
	}
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
