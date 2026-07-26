package lumenetescontroller

import (
	"encoding/base64"
	"fmt"

	admissionregistrationv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/admissionregistration/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	tls "github.com/pulumi/pulumi-tls/sdk/v4/go/tls"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	// webhookServiceName is the Service the API server dials to reach the
	// Light validating webhook - also the DNS name the server cert's SANs
	// are issued for (see newWebhookCert).
	webhookServiceName = "lumenetes-controller-webhook"
	// webhookPort is the port the webhook server (embedded in the
	// lumenetes-controller manager, see cmd/lumenetes-controller/main.go)
	// listens on - controller-runtime's own default.
	webhookPort = 9443
	// webhookCertVolumeName/webhookCertMountPath are where the generated
	// server cert Secret is mounted into the Deployment - see component.go.
	webhookCertVolumeName = "webhook-certs"
	webhookCertMountPath  = "/etc/lumenetes-controller/webhook-certs"
	// webhookPath is the validating webhook's serve path -
	// controller-runtime's ctrl.NewWebhookManagedBy derives this
	// automatically from the Light GVK (group "lumenetes.io", version
	// "v1alpha1", kind "Light"); must match exactly here.
	webhookPath = "/validate-lumenetes-io-v1alpha1-light"
)

// webhookCert is what newWebhookCert produces: the name of the Secret
// carrying the server cert/key (for the Deployment's volume) and the CA
// cert PEM (for the ValidatingWebhookConfiguration's CaBundle).
type webhookCert struct {
	SecretName pulumi.StringOutput
	CACertPEM  pulumi.StringOutput
}

// newWebhookCert generates a self-signed CA and a CA-signed server
// certificate for the Light validating webhook via the Pulumi tls provider
// - there is no cert-manager anywhere in this cluster, and bringing one in
// just for this one webhook was an explicitly rejected option in favor of
// this simpler, fully declarative alternative.
//
// Neither the CA nor the server cert auto-rotates: both are stable Pulumi
// state, regenerated only if their own inputs change or on an explicit
// `pulumi up --replace` - so a multi-year validity period is chosen to
// make that a non-issue for the practical lifetime of this homelab, rather
// than building real rotation machinery for a single low-stakes webhook.
func newWebhookCert(ctx *pulumi.Context, name string, namespace pulumi.StringInput, opts ...pulumi.ResourceOption) (*webhookCert, error) {
	caKey, err := tls.NewPrivateKey(ctx, fmt.Sprintf("%s-webhook-ca-key", name), &tls.PrivateKeyArgs{
		Algorithm:  pulumi.String("ECDSA"),
		EcdsaCurve: pulumi.String("P256"),
	}, opts...)
	if err != nil {
		return nil, err
	}

	caCert, err := tls.NewSelfSignedCert(ctx, fmt.Sprintf("%s-webhook-ca", name), &tls.SelfSignedCertArgs{
		PrivateKeyPem:       caKey.PrivateKeyPem,
		IsCaCertificate:     pulumi.Bool(true),
		ValidityPeriodHours: pulumi.Int(87600), // 10 years
		AllowedUses: pulumi.StringArray{
			pulumi.String("cert_signing"),
			pulumi.String("crl_signing"),
			pulumi.String("digital_signature"),
			pulumi.String("key_encipherment"),
		},
		Subject: &tls.SelfSignedCertSubjectArgs{
			CommonName: pulumi.String("lumenetes-controller-webhook-ca"),
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	serverKey, err := tls.NewPrivateKey(ctx, fmt.Sprintf("%s-webhook-server-key", name), &tls.PrivateKeyArgs{
		Algorithm:  pulumi.String("ECDSA"),
		EcdsaCurve: pulumi.String("P256"),
	}, opts...)
	if err != nil {
		return nil, err
	}

	// SANs must match exactly what the API server dials: the in-cluster
	// Service DNS name, both short and fully-qualified forms.
	serviceFQDN := pulumi.Sprintf("%s.%s.svc", webhookServiceName, namespace)
	dnsNames := pulumi.StringArray{
		serviceFQDN,
		pulumi.Sprintf("%s.%s.svc.cluster.local", webhookServiceName, namespace),
	}

	csr, err := tls.NewCertRequest(ctx, fmt.Sprintf("%s-webhook-csr", name), &tls.CertRequestArgs{
		PrivateKeyPem: serverKey.PrivateKeyPem,
		DnsNames:      dnsNames,
		Subject: &tls.CertRequestSubjectArgs{
			CommonName: serviceFQDN,
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	serverCert, err := tls.NewLocallySignedCert(ctx, fmt.Sprintf("%s-webhook-server-cert", name), &tls.LocallySignedCertArgs{
		CertRequestPem:      csr.CertRequestPem,
		CaPrivateKeyPem:     caKey.PrivateKeyPem,
		CaCertPem:           caCert.CertPem,
		ValidityPeriodHours: pulumi.Int(43800), // 5 years
		AllowedUses: pulumi.StringArray{
			pulumi.String("server_auth"),
			pulumi.String("digital_signature"),
			pulumi.String("key_encipherment"),
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	// kubernetes.io/tls requires exactly these two keys - matches
	// controller-runtime's own default CertName/KeyName ("tls.crt"/
	// "tls.key"), so no extra Items ordering is needed on the volume.
	secret, err := corev1.NewSecret(ctx, fmt.Sprintf("%s-webhook-cert-secret", name), &corev1.SecretArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String("lumenetes-controller-webhook-certs"),
			Namespace: namespace,
		},
		Type: pulumi.String("kubernetes.io/tls"),
		StringData: pulumi.StringMap{
			"tls.crt": serverCert.CertPem,
			"tls.key": serverKey.PrivateKeyPem,
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	return &webhookCert{
		SecretName: secret.Metadata.Name().Elem(),
		CACertPEM:  caCert.CertPem,
	}, nil
}

// newWebhookService creates the ClusterIP Service the API server dials to
// reach the Light validating webhook, and the ValidatingWebhookConfiguration
// itself. Selects the Deployment's pods by the same app=lumenetes-controller
// label component.go's Deployment already carries.
func newWebhookService(ctx *pulumi.Context, name string, namespace pulumi.StringInput, caCertPEM pulumi.StringOutput, opts ...pulumi.ResourceOption) error {
	_, err := corev1.NewService(ctx, fmt.Sprintf("%s-webhook-service", name), &corev1.ServiceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(webhookServiceName),
			Namespace: namespace,
		},
		Spec: &corev1.ServiceSpecArgs{
			Selector: pulumi.StringMap{
				"app": pulumi.String("lumenetes-controller"),
			},
			Ports: corev1.ServicePortArray{
				&corev1.ServicePortArgs{
					Port:       pulumi.Int(443),
					TargetPort: pulumi.Int(webhookPort),
					Protocol:   pulumi.String("TCP"),
				},
			},
		},
	}, opts...)
	if err != nil {
		return err
	}

	// FailurePolicy: Fail, not Ignore - the webhook runs in the same
	// single-replica pod as groupcontroller/lightscontroller, so whenever
	// it's unreachable those controllers aren't running either: there's no
	// window where an invalid write could slip in specifically because
	// enforcement was down. Rules only cover "lights" (not "lights/status"),
	// so Poller/EventConsumer's status-subresource writes are entirely
	// unaffected regardless of FailurePolicy.
	_, err = admissionregistrationv1.NewValidatingWebhookConfiguration(ctx, fmt.Sprintf("%s-webhook-config", name), &admissionregistrationv1.ValidatingWebhookConfigurationArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: pulumi.String("lumenetes-light-validator"),
		},
		Webhooks: admissionregistrationv1.ValidatingWebhookArray{
			&admissionregistrationv1.ValidatingWebhookArgs{
				Name:                    pulumi.String("light.lumenetes.io"),
				AdmissionReviewVersions: pulumi.StringArray{pulumi.String("v1")},
				SideEffects:             pulumi.String("None"),
				FailurePolicy:           pulumi.String("Fail"),
				ClientConfig: &admissionregistrationv1.WebhookClientConfigArgs{
					// Unlike corev1.Secret's Data/StringData, the
					// pulumi-kubernetes provider does not auto-base64 this
					// field - it's passed through to the API server
					// verbatim, which expects the wire-format (base64)
					// value directly. Confirmed live: passing the raw PEM
					// failed with "illegal base64 data at input byte 0".
					CaBundle: caCertPEM.ApplyT(func(pem string) string {
						return base64.StdEncoding.EncodeToString([]byte(pem))
					}).(pulumi.StringOutput),
					Service: &admissionregistrationv1.ServiceReferenceArgs{
						Name:      pulumi.String(webhookServiceName),
						Namespace: namespace,
						Path:      pulumi.String(webhookPath),
						Port:      pulumi.Int(443),
					},
				},
				Rules: admissionregistrationv1.RuleWithOperationsArray{
					&admissionregistrationv1.RuleWithOperationsArgs{
						ApiGroups:   pulumi.StringArray{pulumi.String("lumenetes.io")},
						ApiVersions: pulumi.StringArray{pulumi.String("v1alpha1")},
						Resources:   pulumi.StringArray{pulumi.String("lights")},
						Operations:  pulumi.StringArray{pulumi.String("CREATE"), pulumi.String("UPDATE")},
					},
				},
			},
		},
	}, opts...)
	return err
}
