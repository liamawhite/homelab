// Package lightwebhook implements the validating admission webhook for
// Light: the Kubernetes API server rejects a create/update before it's ever
// persisted if it would leave Spec.Color and Spec.ColorTempK both set. A
// Hue light only has one active color mode (xy vs. mirek) at a time - see
// internal/lightscontroller.Reconciler.enact's doc comment for the real
// production bug (a stale Spec.Color fighting a CircadianSchedule-driven
// Spec.ColorTempK, confirmed live as a repeating ~1s flash) this invariant
// exists to prevent from ever being reintroduced by any future writer.
package lightwebhook

import (
	"context"
	"fmt"

	lumenetesv1alpha1 "github.com/liamawhite/lumenetes/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Validator implements webhook.CustomValidator for Light. Registered via
// ctrl.NewWebhookManagedBy in cmd/lumenetes-controller/main.go.
type Validator struct{}

var _ webhook.CustomValidator = (*Validator)(nil)

func (v *Validator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return validate(obj)
}

func (v *Validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return validate(newObj)
}

func (v *Validator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validate is a plain, dependency-free function - unit-testable directly,
// same philosophy as internal/lightscontroller.diffLight.
func validate(obj runtime.Object) (admission.Warnings, error) {
	light, ok := obj.(*lumenetesv1alpha1.Light)
	if !ok {
		return nil, fmt.Errorf("expected a Light but got %T", obj)
	}
	if light.Spec.Color != "" && light.Spec.ColorTempK != 0 {
		return nil, fmt.Errorf("spec.color and spec.colorTempK are mutually exclusive - a Hue light has one active color mode at a time; clear one before setting the other")
	}
	return nil, nil
}
