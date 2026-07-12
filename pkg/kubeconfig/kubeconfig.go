// Package kubeconfig holds logic for locating the homelab kubeconfig that is
// shared between the CLI (which writes it) and Pulumi (which reads it).
package kubeconfig

import (
	"os"
	"path/filepath"
	"strings"
)

// ContextName is the context name the CLI merges the cluster's kubeconfig
// under when writing to the default kubeconfig path.
const ContextName = "homelab"

// DefaultPath resolves where kubectl would look for its kubeconfig:
// $KUBECONFIG (its first entry, if it lists several), else ~/.kube/config.
func DefaultPath() (string, error) {
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		return strings.Split(kc, string(os.PathListSeparator))[0], nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube", "config"), nil
}
