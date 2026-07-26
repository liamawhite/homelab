package sun

import (
	"testing"
	"time"
)

// These tests check physical invariants of sunrise/sunset/solar-noon
// (day length at the equinox/equator, longitude shifting solar noon,
// seasonal day-length ordering, polar day/night) rather than a single
// hand-computed "golden" instant, since a golden value derived from this
// same formula wouldn't catch a shared derivation bug, and no external
// reference data is available to check against here.

func mustCompute(t *testing.T, coords Coordinates, date time.Time) Times {
	t.Helper()
	times, err := Compute(coords, date)
	if err != nil {
		t.Fatalf("Compute(%+v, %s): unexpected error: %v", coords, date, err)
	}
	return times
}

func TestCompute_EquinoxEquatorDayLength(t *testing.T) {
	// On the equinox, at the equator, day and night are each ~12h - a
	// touch over 12h once the standard atmospheric refraction/solar-disk
	// correction (solarNoonZenith) is accounted for.
	equinox := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	times := mustCompute(t, Coordinates{Latitude: 0, Longitude: 0}, equinox)

	dayLength := times.Sunset.Sub(times.Sunrise)
	if dayLength < 12*time.Hour || dayLength > 12*time.Hour+20*time.Minute {
		t.Errorf("equinox equator day length = %s, want ~12h (up to +20m for refraction)", dayLength)
	}
}

func TestCompute_EquatorDayLengthRoughlyConstant(t *testing.T) {
	// Near the equator, day length stays close to 12h year-round, unlike
	// higher latitudes.
	coords := Coordinates{Latitude: 0, Longitude: 0}
	for _, date := range []time.Time{
		time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
	} {
		times := mustCompute(t, coords, date)
		dayLength := times.Sunset.Sub(times.Sunrise)
		if dayLength < 12*time.Hour || dayLength > 12*time.Hour+20*time.Minute {
			t.Errorf("equator day length on %s = %s, want ~12h", date.Format("2006-01-02"), dayLength)
		}
	}
}

func TestCompute_SolarNoonMidpointOfSunriseSunset(t *testing.T) {
	// SolarNoon must sit exactly halfway between Sunrise and Sunset by
	// construction (the hour angle is applied symmetrically).
	times := mustCompute(t, Coordinates{Latitude: 51.5, Longitude: -0.1}, time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC))

	toNoon := times.SolarNoon.Sub(times.Sunrise)
	fromNoon := times.Sunset.Sub(times.SolarNoon)
	if diff := toNoon - fromNoon; diff < -time.Second || diff > time.Second {
		t.Errorf("sunrise->noon = %s, noon->sunset = %s, want equal", toNoon, fromNoon)
	}
}

func TestCompute_SolarMidnightTwelveHoursAfterNoon(t *testing.T) {
	times := mustCompute(t, Coordinates{Latitude: 40, Longitude: 10}, time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC))

	if diff := times.SolarMidnight.Sub(times.SolarNoon); diff != 12*time.Hour {
		t.Errorf("SolarMidnight - SolarNoon = %s, want exactly 12h", diff)
	}
}

func TestCompute_LongitudeShiftsSolarNoon(t *testing.T) {
	// Moving 15deg further east should shift solar noon ~1h earlier in
	// UTC (the sun transits sooner, local clock reads noon sooner
	// relative to UTC).
	date := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	west := mustCompute(t, Coordinates{Latitude: 20, Longitude: 0}, date)
	east := mustCompute(t, Coordinates{Latitude: 20, Longitude: 15}, date)

	shift := west.SolarNoon.Sub(east.SolarNoon)
	if shift < 59*time.Minute || shift > 61*time.Minute {
		t.Errorf("solar noon shift for +15deg longitude = %s, want ~1h", shift)
	}
}

func TestCompute_SeasonalDayLengthOrdering(t *testing.T) {
	// At a mid-northern latitude, the June solstice has a longer day than
	// the December solstice.
	coords := Coordinates{Latitude: 51.5, Longitude: 0}
	june := mustCompute(t, coords, time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC))
	december := mustCompute(t, coords, time.Date(2026, time.December, 21, 0, 0, 0, 0, time.UTC))

	juneLength := june.Sunset.Sub(june.Sunrise)
	decemberLength := december.Sunset.Sub(december.Sunrise)
	if juneLength <= decemberLength {
		t.Errorf("June day length %s not longer than December day length %s at 51.5N", juneLength, decemberLength)
	}
}

func TestCompute_PolarNightErrors(t *testing.T) {
	// Above the Arctic Circle in midwinter, the sun never rises.
	_, err := Compute(Coordinates{Latitude: 70, Longitude: 0}, time.Date(2026, time.December, 21, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("Compute at 70N on the December solstice: expected polar-night error, got nil")
	}
}

func TestCompute_PolarDayErrors(t *testing.T) {
	// Above the Arctic Circle in midsummer, the sun never sets.
	_, err := Compute(Coordinates{Latitude: 70, Longitude: 0}, time.Date(2026, time.June, 21, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("Compute at 70N on the June solstice: expected polar-day error, got nil")
	}
}

func TestCompute_TemperateLatitudeNoError(t *testing.T) {
	// Sanity check: ordinary latitudes never hit the polar day/night
	// branch, any time of year.
	coords := Coordinates{Latitude: -33.87, Longitude: 151.21} // Sydney
	for month := time.January; month <= time.December; month++ {
		if _, err := Compute(coords, time.Date(2026, month, 15, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Errorf("Compute(Sydney, %s): unexpected error: %v", month, err)
		}
	}
}
