package k3s

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/liamawhite/homelab/cli/pkg/ssh"
	"gopkg.in/yaml.v3"
)

// ExtractKubeconfig extracts and modifies the kubeconfig from a K3s node
func ExtractKubeconfig(ctx context.Context, sshClient *ssh.Client, nodeAddr string) (string, error) {
	// Read kubeconfig from node
	kubeconfig, err := sshClient.ReadFile("/etc/rancher/k3s/k3s.yaml", true)
	if err != nil {
		return "", fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(kubeconfig), &config); err != nil {
		return "", fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Replace 127.0.0.1 with actual node address
	if clusters, ok := config["clusters"].([]interface{}); ok {
		for _, cluster := range clusters {
			if clusterMap, ok := cluster.(map[string]interface{}); ok {
				if clusterData, ok := clusterMap["cluster"].(map[string]interface{}); ok {
					if server, ok := clusterData["server"].(string); ok {
						clusterData["server"] = strings.Replace(server, "127.0.0.1", nodeAddr, 1)
					}
				}
			}
		}
	}

	// Marshal back to YAML
	modified, err := yaml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}

	return string(modified), nil
}

// WriteKubeconfig writes kubeconfig to a file with appropriate permissions
func WriteKubeconfig(kubeconfig, path string) error {
	if err := os.WriteFile(path, []byte(kubeconfig), 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	slog.Info("Kubeconfig written successfully", "path", path)
	return nil
}
