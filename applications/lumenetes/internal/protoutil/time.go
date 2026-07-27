// Package protoutil holds tiny helpers shared by every internal/*service
// package that maps a lumenetes.io CR into its lumenetes.v1 proto
// equivalent.
package protoutil

import (
	"google.golang.org/protobuf/types/known/timestamppb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Time converts a Kubernetes metav1.Time into a *timestamppb.Timestamp,
// returning nil for the zero value (a field that was never synced) instead
// of the proto epoch.
func Time(t metav1.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t.Time)
}
