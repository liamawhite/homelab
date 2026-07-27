package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CircadianAnchor is a named solar event a CircadianKeyframe resolves
// against (see internal/sun.Times) - "sunrise", "solarNoon", "sunset", or
// "solarMidnight".
type CircadianAnchor string

const (
	CircadianAnchorSunrise       CircadianAnchor = "sunrise"
	CircadianAnchorSolarNoon     CircadianAnchor = "solarNoon"
	CircadianAnchorSunset        CircadianAnchor = "sunset"
	CircadianAnchorSolarMidnight CircadianAnchor = "solarMidnight"
)

// CircadianOnState is a CircadianKeyframe's tri-state on/off directive -
// explicitly "on", explicitly "off", or "unchanged" (this keyframe doesn't
// affect on/off state at all). A plain *bool could only distinguish two of
// these three (nil for unchanged, true/false for on/off), which is enough
// for SceneLightState's one-shot apply, but internal/circadian.Interpolate
// needs "unchanged" to be a real, explicit value it can search past to find
// the nearest keyframe that actually does set on/off - an enum makes that a
// named state instead of relying on nil-pointer plumbing to mean the same
// thing.
type CircadianOnState string

const (
	CircadianOnStateUnchanged CircadianOnState = "unchanged"
	CircadianOnStateOn        CircadianOnState = "on"
	CircadianOnStateOff       CircadianOnState = "off"
)

// +kubebuilder:object:generate=true

// CircadianKeyframe pins an absolute (Brightness, ColorTempK) pair to a
// point in the solar day - Anchor plus OffsetMinutes, not a wall-clock
// time, so the schedule keeps making sense across the seasons as sunrise/
// sunset drift. Unlike SceneLightState's pointer fields (which mean "leave
// this untouched"), Brightness/ColorTempK here are required: a continuous
// curve needs a value defined at every keyframe, not a sparse override -
// there's no "untouched" between two points on a curve.
type CircadianKeyframe struct {
	// Anchor is the solar event this keyframe is offset from.
	// +kubebuilder:validation:Enum=sunrise;solarNoon;sunset;solarMidnight
	Anchor CircadianAnchor `json:"anchor"`
	// OffsetMinutes shifts this keyframe from Anchor, positive is later.
	// Bounded to +/-12h so internal/circadian.Interpolate's search window
	// (the day before/of/after "now") is always sufficient to resolve it.
	// +kubebuilder:validation:Minimum=-720
	// +kubebuilder:validation:Maximum=720
	OffsetMinutes int32 `json:"offsetMinutes,omitempty"`
	// Brightness at this keyframe, 0-100.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	Brightness int32 `json:"brightness"`
	// ColorTempK at this keyframe, in Kelvin.
	// +kubebuilder:validation:Minimum=1000
	// +kubebuilder:validation:Maximum=10000
	ColorTempK int32 `json:"colorTempK"`
	// On explicitly turns the group's lights on or off at this keyframe, or
	// leaves on/off state unchanged (the default) - unlike Brightness/
	// ColorTempK, on/off isn't a continuous curve, so this is a sparse
	// override, not a required per-keyframe value. internal/
	// circadian.Interpolate resolves the effective value for "now" as the
	// nearest keyframe at-or-before now (searching backward, wrapping
	// across days) whose On isn't "unchanged" - a step function, not
	// interpolated/blended between two keyframes the way Brightness/
	// ColorTempK are.
	// +kubebuilder:validation:Enum=unchanged;on;off
	// +kubebuilder:default=unchanged
	On CircadianOnState `json:"on,omitempty"`
}

// +kubebuilder:object:generate=true

// CircadianScheduleSpec declares a continuous brightness/colorTempK curve
// for all of a Group's lights, anchored to sun position rather than fixed
// times. Unlike Scene, there's no per-light Lights list - a circadian
// curve is one uniform "ambient tone" applied to every light in Group, not
// per-fixture state. A CircadianSchedule does no enactment itself -
// internal/groupcontroller.Reconciler applies the current interpolated
// value onto each target Light.Spec when the owning Group's
// Spec.ActiveScene references this schedule (Kind: CircadianSchedule),
// reusing the same per-light capability-sentinel skip Scene enactment
// already uses.
type CircadianScheduleSpec struct {
	// Group is the name of the Group this schedule applies to. A Group's
	// Spec.ActiveScene must reference this schedule's own metadata.name
	// for it to ever be enacted - same reciprocal-match convention as
	// SceneSpec.Group.
	Group string `json:"group"`
	// Latitude of the location Keyframes are anchored to, decimal degrees
	// positive north. Required, not defaulted: 0 is a real location (Null
	// Island, off the coast of Ghana) rather than a safe "unset" sentinel,
	// so a schedule created without one would silently compute sun times
	// for the wrong place instead of failing validation - confirmed live,
	// this is exactly what happened before Latitude/Longitude moved from a
	// global --latitude/--longitude flag onto this Spec.
	// +kubebuilder:validation:Minimum=-90
	// +kubebuilder:validation:Maximum=90
	Latitude float64 `json:"latitude"`
	// Longitude of the location Keyframes are anchored to, decimal degrees
	// positive east. Required - see Latitude's doc comment for why.
	// +kubebuilder:validation:Minimum=-180
	// +kubebuilder:validation:Maximum=180
	Longitude float64 `json:"longitude"`
	// Keyframes define the curve, at least 2, resolved and interpolated
	// against "now" by internal/circadian.Interpolate. Order in this list
	// doesn't matter - they're sorted by resolved instant before
	// interpolating.
	// +kubebuilder:validation:MinItems=2
	Keyframes []CircadianKeyframe `json:"keyframes"`
}

// +kubebuilder:object:generate=true

// CircadianScheduleStatus reports the schedule's live-computed output,
// independent of whether any Group currently selects it via
// Spec.ActiveScene - useful for tuning Keyframes via `kubectl get` before
// wiring it live.
type CircadianScheduleStatus struct {
	// CurrentBrightness is what Spec.Keyframes interpolates to right now,
	// nil if ValidationError is set.
	CurrentBrightness *int32 `json:"currentBrightness,omitempty"`
	// CurrentColorTempK is what Spec.Keyframes interpolates to right now,
	// nil if ValidationError is set.
	CurrentColorTempK *int32 `json:"currentColorTempK,omitempty"`
	// ValidationError reports why Spec couldn't be interpolated - e.g.
	// Group is unset, or internal/circadian.Interpolate rejected
	// Keyframes (degenerate/duplicate resolved instants, an unresolvable
	// anchor). Empty means Spec is well-formed.
	ValidationError string `json:"validationError,omitempty"`
	// LastSynced is when this status was last recomputed.
	LastSynced metav1.Time `json:"lastSynced,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Group",type="string",JSONPath=".spec.group"
// +kubebuilder:printcolumn:name="Brightness",type="integer",JSONPath=".status.currentBrightness"
// +kubebuilder:printcolumn:name="ColorTempK",type="integer",JSONPath=".status.currentColorTempK"
// +kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.validationError",priority=1
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CircadianSchedule is a user-named, continuous lighting curve for all of
// a Group's lights, anchored to sun position. Cluster scoped, user-chosen
// name (e.g. "living-space-circadian"), same reasoning as Scene/Group - a
// CircadianSchedule has no Hue-side identity of its own.
type CircadianSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CircadianScheduleSpec   `json:"spec,omitempty"`
	Status CircadianScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CircadianScheduleList is a list of CircadianSchedule resources.
type CircadianScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CircadianSchedule `json:"items"`
}
