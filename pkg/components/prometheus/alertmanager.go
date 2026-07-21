package prometheus

import (
	"fmt"

	monitoringv1 "github.com/liamawhite/homelab/pkg/crds/prometheus/crds/kubernetes/monitoring/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	alertmanagerName  = "main"
	alertmanagerImage = "quay.io/prometheus/alertmanager"

	// AlertmanagerPodLabel is the "app.kubernetes.io/name" value this file's
	// PodMetadata.Labels stamps onto every pod the operator generates from
	// the Alertmanager CR - exported for the same reason
	// PrometheusPodLabel is (network.go's own HBONE CCNP pair, and
	// component.go's waypoint TargetLabels).
	AlertmanagerPodLabel = "alertmanager"

	// AlertmanagerServiceName is the fixed name of the Service newAlertmanager
	// creates - exported so pkg/deploy/deploy.go and this package's own
	// instance.go (Prometheus's Alerting.Alertmanagers config) can
	// reference it.
	AlertmanagerServiceName = "alertmanager-" + alertmanagerName
)

// newAlertmanager deploys a single-replica Alertmanager. Deliberately bare:
// no ConfigSecret is created, so - per the Alertmanager CRD's own documented
// behavior ("If either the secret or the alertmanager.yaml key is missing,
// the operator provisions a minimal Alertmanager configuration with one
// empty receiver") - the operator auto-provisions a no-op "null" receiver.
// Alerts Prometheus fires are visible in Alertmanager's own UI/API, but no
// external notification (Slack/email/webhook/...) is ever sent. This is a
// deliberate, user-confirmed choice: wire up a real ConfigSecret with actual
// receivers later if/when notification routing is wanted - nothing else in
// this package needs to change for that.
//
// No PVC either (Storage left unset) - matches Grafana's own "ephemeral is
// fine, nothing here is provisioned any other way" reasoning, and Alertmanager
// silence/notification-log state resetting on restart is an acceptable
// homelab tradeoff.
func newAlertmanager(ctx *pulumi.Context, name string, namespace pulumi.StringInput, version string, waypointName pulumi.StringOutput, opts ...pulumi.ResourceOption) (*monitoringv1.Alertmanager, *corev1.Service, error) {
	labels := pulumi.StringMap{
		"app.kubernetes.io/name":      pulumi.String(AlertmanagerPodLabel),
		"app.kubernetes.io/instance":  pulumi.String(alertmanagerName),
		"app.kubernetes.io/component": pulumi.String("alertmanager"),
		"app.kubernetes.io/version":   pulumi.String(version),
	}

	serviceAccount, err := corev1.NewServiceAccount(ctx, fmt.Sprintf("%s-sa", name), &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(AlertmanagerServiceName),
			Namespace: namespace,
		},
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	am, err := monitoringv1.NewAlertmanager(ctx, name, &monitoringv1.AlertmanagerArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(alertmanagerName),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: &monitoringv1.AlertmanagerSpecArgs{
			Image:              pulumi.Sprintf("%s:v%s", alertmanagerImage, version),
			Version:            pulumi.String(version),
			Replicas:           pulumi.Int(1),
			ServiceAccountName: serviceAccount.Metadata.Name().Elem(),
			SecurityContext: &monitoringv1.AlertmanagerSpecSecurityContextArgs{
				FsGroup:      pulumi.Int(2000),
				RunAsNonRoot: pulumi.Bool(true),
				RunAsUser:    pulumi.Int(1000),
			},
			Resources: &monitoringv1.AlertmanagerSpecResourcesArgs{
				Requests: pulumi.Map{"cpu": pulumi.String("25m"), "memory": pulumi.String("50Mi")},
				Limits:   pulumi.Map{"cpu": pulumi.String("100m"), "memory": pulumi.String("128Mi")},
			},
			NodeSelector: pulumi.StringMap{"kubernetes.io/os": pulumi.String("linux")},
			PodMetadata: &monitoringv1.AlertmanagerSpecPodMetadataArgs{
				Labels: labels,
			},
		},
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	service, err := corev1.NewService(ctx, fmt.Sprintf("%s-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(AlertmanagerServiceName),
			Namespace: namespace,
			Labels: pulumi.StringMap{
				"istio.io/use-waypoint": waypointName,
			},
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: pulumi.StringMap{"alertmanager": pulumi.String(alertmanagerName)},
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{Name: pulumi.String("web"), Port: pulumi.Int(9093), TargetPort: pulumi.String("web")},
			},
		},
	}, opts...)
	if err != nil {
		return nil, nil, err
	}

	return am, service, nil
}
