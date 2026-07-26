// Package sun computes sunrise, solar noon, sunset, and solar midnight for
// a location and date, using the standard NOAA solar position equations
// (equation of time + solar declination + hour angle - see
// https://gml.noaa.gov/grad/solcalc/solareqns.PDF). It exists to anchor
// internal/circadian's keyframe interpolation to the sun's actual position
// rather than fixed wall-clock times, so a circadian schedule keeps making
// sense across the seasons without hand-editing.
package sun

import (
	"fmt"
	"math"
	"time"
)

// Coordinates is a location in decimal degrees, positive north/east.
type Coordinates struct {
	Latitude  float64
	Longitude float64
}

// Times are the key solar instants for a Coordinates on one UTC calendar
// day. SolarNoon/SolarMidnight are the sun's actual transit times at
// Longitude, not clock noon/midnight - depending on where Longitude falls
// within its timezone and the day of year (the "equation of time"), solar
// noon can be tens of minutes away from 12:00 local clock time.
// SolarMidnight is defined as exactly 12h after SolarNoon on the same
// input day.
type Times struct {
	Sunrise       time.Time
	SolarNoon     time.Time
	Sunset        time.Time
	SolarMidnight time.Time
}

// solarNoonZenith is the sun's zenith angle (degrees) used for sunrise/
// sunset, not 90 - it accounts for the sun's apparent radius (~16') and
// standard atmospheric refraction at the horizon (~34'), the same
// convention every standard sunrise/sunset calculator uses.
const solarNoonZenith = 90.833

// Compute derives Times for coords on date's UTC calendar day. It operates
// entirely in UTC - no timezone/DST handling, Coordinates.Longitude alone
// determines how far solar noon drifts from 12:00 UTC. Returns an error
// rather than a garbage Times for the polar day/night case (the sun never
// rises or sets on date at this latitude), surfaced when the hour-angle
// formula's acos argument falls outside [-1, 1].
func Compute(coords Coordinates, date time.Time) (Times, error) {
	day := date.UTC().Truncate(24 * time.Hour)
	t := julianCentury(day)

	l0 := normalizeDegrees(280.46646 + t*(36000.76983+t*0.0003032))
	m := 357.52911 + t*(35999.05029-0.0001537*t)
	e := 0.016708634 - t*(0.000042037+0.0000001267*t)

	mRad := degToRad(m)
	center := math.Sin(mRad)*(1.914602-t*(0.004817+0.000014*t)) +
		math.Sin(2*mRad)*(0.019993-0.000101*t) +
		math.Sin(3*mRad)*0.000289

	trueLongitude := l0 + center
	apparentLongitude := trueLongitude - 0.00569 - 0.00478*math.Sin(degToRad(125.04-1934.136*t))

	meanObliquity := 23 + (26+(21.448-t*(46.815+t*(0.00059-t*0.001813)))/60)/60
	obliquity := meanObliquity + 0.00256*math.Cos(degToRad(125.04-1934.136*t))

	// declination is in radians already (math.Asin's output), unlike the
	// other intermediate angles above which stay in degrees until used.
	declination := math.Asin(math.Sin(degToRad(obliquity)) * math.Sin(degToRad(apparentLongitude)))

	y := math.Pow(math.Tan(degToRad(obliquity)/2), 2)
	l0Rad := degToRad(l0)
	eqTimeMinutes := 4 * radToDeg(
		y*math.Sin(2*l0Rad)-
			2*e*math.Sin(mRad)+
			4*e*y*math.Sin(mRad)*math.Cos(2*l0Rad)-
			0.5*y*y*math.Sin(4*l0Rad)-
			1.25*e*e*math.Sin(2*mRad),
	)

	// Minutes from UTC midnight at which local apparent solar time (which
	// runs Longitude/15h + eqTime ahead of UTC) reads 12:00.
	solarNoonMinutes := 720 - 4*coords.Longitude - eqTimeMinutes

	latRad := degToRad(coords.Latitude)
	cosHourAngle := math.Cos(degToRad(solarNoonZenith))/(math.Cos(latRad)*math.Cos(declination)) -
		math.Tan(latRad)*math.Tan(declination)
	if cosHourAngle < -1 || cosHourAngle > 1 {
		return Times{}, fmt.Errorf("sun: no sunrise/sunset at latitude %.4f on %s (polar day or night)", coords.Latitude, day.Format("2006-01-02"))
	}
	hourAngleMinutes := 4 * radToDeg(math.Acos(cosHourAngle))

	return Times{
		Sunrise:       addMinutes(day, solarNoonMinutes-hourAngleMinutes),
		SolarNoon:     addMinutes(day, solarNoonMinutes),
		Sunset:        addMinutes(day, solarNoonMinutes+hourAngleMinutes),
		SolarMidnight: addMinutes(day, solarNoonMinutes+720),
	}, nil
}

// julianCentury returns the number of Julian centuries since J2000.0
// (2000-01-01T12:00:00Z) for day at 12:00 UTC - the reference instant the
// NOAA solar position equations are defined against. Sub-day precision
// beyond this doesn't materially change declination/equation-of-time for a
// lighting schedule.
func julianCentury(day time.Time) float64 {
	noon := day.Add(12 * time.Hour)
	julianDay := float64(noon.Unix())/86400.0 + 2440587.5
	return (julianDay - 2451545.0) / 36525.0
}

func addMinutes(day time.Time, minutes float64) time.Time {
	return day.Add(time.Duration(minutes * float64(time.Minute)))
}

func degToRad(d float64) float64 { return d * math.Pi / 180 }
func radToDeg(r float64) float64 { return r * 180 / math.Pi }

func normalizeDegrees(d float64) float64 {
	d = math.Mod(d, 360)
	if d < 0 {
		d += 360
	}
	return d
}
