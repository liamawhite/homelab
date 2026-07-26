// Package circadian interpolates a CircadianSchedule's keyframes to a
// single (brightness, colorTempK) pair for a given instant - the pure
// computation internal/groupcontroller.Reconciler and
// internal/circadianschedulecontroller both call, so neither duplicates
// the other's notion of "what does this schedule output right now."
package circadian

import (
	"fmt"
	"math"
	"sort"
	"time"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"github.com/liamawhite/lumenetes/internal/sun"
)

// resolvedInstant is one keyframe resolved to an absolute instant on one
// specific day.
type resolvedInstant struct {
	at         time.Time
	brightness int32
	colorTempK int32
}

// Interpolate returns the brightness/colorTempK keyframes interpolates to
// at now, for a location at coords. Every keyframe's Anchor+OffsetMinutes
// is resolved against the UTC calendar day before, of, and after now (three
// sun.Compute calls) so a keyframe near solar midnight always has the
// correct same-day neighbor to bracket now against, even right around
// midnight itself - a single day's sun.Times can't represent both "last
// night's last keyframe" and "this morning's first keyframe"
// simultaneously. now is then bracketed between the two resolved instants
// nearest it (one at-or-before, one strictly after) and linearly
// interpolated between them.
//
// Errors (never a silently wrong value) if: fewer than 2 keyframes are
// given, an anchor doesn't resolve (an unknown CircadianAnchor, or
// sun.Compute failing - e.g. polar day/night on one of the three days
// searched), or now can't be bracketed within the resolved window (should
// be unreachable given CircadianKeyframe.OffsetMinutes' +/-12h bound, but
// checked defensively rather than trusted).
func Interpolate(keyframes []lumenetesv1alpha1.CircadianKeyframe, coords sun.Coordinates, now time.Time) (brightness, colorTempK int32, err error) {
	if len(keyframes) < 2 {
		return 0, 0, fmt.Errorf("circadian: need at least 2 keyframes, got %d", len(keyframes))
	}

	day := now.UTC().Truncate(24 * time.Hour)
	resolved := make([]resolvedInstant, 0, len(keyframes)*3)
	for _, dayOffset := range []int{-1, 0, 1} {
		times, err := sun.Compute(coords, day.AddDate(0, 0, dayOffset))
		if err != nil {
			return 0, 0, fmt.Errorf("circadian: %w", err)
		}
		for _, kf := range keyframes {
			anchor, err := anchorTime(times, kf.Anchor)
			if err != nil {
				return 0, 0, err
			}
			resolved = append(resolved, resolvedInstant{
				at:         anchor.Add(time.Duration(kf.OffsetMinutes) * time.Minute),
				brightness: kf.Brightness,
				colorTempK: kf.ColorTempK,
			})
		}
	}

	sort.Slice(resolved, func(i, j int) bool { return resolved[i].at.Before(resolved[j].at) })

	var before, after *resolvedInstant
	for i := range resolved {
		if !resolved[i].at.After(now) {
			before = &resolved[i]
			continue
		}
		after = &resolved[i]
		break
	}
	if before == nil || after == nil {
		return 0, 0, fmt.Errorf("circadian: could not bracket %s within the resolved keyframe window - check OffsetMinutes stay within +/-12h", now)
	}

	span := after.at.Sub(before.at)
	if span <= 0 {
		// Unreachable given after is picked strictly After now and before
		// is at-or-before now, but a wrong value here would write straight
		// to a real Light's Spec - fail closed instead of dividing by zero
		// or a negative span.
		return 0, 0, fmt.Errorf("circadian: degenerate keyframe interval between %s and %s", before.at, after.at)
	}
	frac := float64(now.Sub(before.at)) / float64(span)

	return lerp(before.brightness, after.brightness, frac), lerp(before.colorTempK, after.colorTempK, frac), nil
}

func anchorTime(times sun.Times, anchor lumenetesv1alpha1.CircadianAnchor) (time.Time, error) {
	switch anchor {
	case lumenetesv1alpha1.CircadianAnchorSunrise:
		return times.Sunrise, nil
	case lumenetesv1alpha1.CircadianAnchorSolarNoon:
		return times.SolarNoon, nil
	case lumenetesv1alpha1.CircadianAnchorSunset:
		return times.Sunset, nil
	case lumenetesv1alpha1.CircadianAnchorSolarMidnight:
		return times.SolarMidnight, nil
	default:
		return time.Time{}, fmt.Errorf("circadian: unknown anchor %q", anchor)
	}
}

func lerp(a, b int32, frac float64) int32 {
	return a + int32(math.Round(float64(b-a)*frac))
}
