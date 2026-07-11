package k3s

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liamawhite/homelab/cli/pkg/ssh"
	"gopkg.in/yaml.v3"
)

// ExtractKubeconfig extracts and modifies the kubeconfig from a K3s node,
// pointing its server address at vip instead of the node's own loopback
// address, so the result works from any machine and follows the current
// control-plane leader rather than one specific node.
func ExtractKubeconfig(ctx context.Context, sshClient *ssh.Client, vip string) (string, error) {
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

	// Replace 127.0.0.1 with the cluster VIP; the VIP is already a TLS SAN
	// on the K3s server certificate (see cluster.sans in infra.yaml).
	if clusters, ok := config["clusters"].([]interface{}); ok {
		for _, cluster := range clusters {
			if clusterMap, ok := cluster.(map[string]interface{}); ok {
				if clusterData, ok := clusterMap["cluster"].(map[string]interface{}); ok {
					if server, ok := clusterData["server"].(string); ok {
						clusterData["server"] = strings.Replace(server, "127.0.0.1", vip, 1)
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

// CheckAPIHealth parses a kubeconfig (as produced by ExtractKubeconfig) and
// makes a single authenticated request to its server's /livez endpoint,
// reporting whether the API server responded healthy. This is used to
// verify a specific node's own API server, as opposed to SSH or k3s-binary
// presence which don't confirm the API is actually serving requests.
func CheckAPIHealth(kubeconfig string) bool {
	var cfg struct {
		Clusters []struct {
			Cluster struct {
				Server                   string `yaml:"server"`
				CertificateAuthorityData string `yaml:"certificate-authority-data"`
			} `yaml:"cluster"`
		} `yaml:"clusters"`
		Users []struct {
			User struct {
				ClientCertificateData string `yaml:"client-certificate-data"`
				ClientKeyData         string `yaml:"client-key-data"`
			} `yaml:"user"`
		} `yaml:"users"`
	}

	if err := yaml.Unmarshal([]byte(kubeconfig), &cfg); err != nil {
		return false
	}
	if len(cfg.Clusters) == 0 || len(cfg.Users) == 0 {
		return false
	}

	caData, err := base64.StdEncoding.DecodeString(cfg.Clusters[0].Cluster.CertificateAuthorityData)
	if err != nil {
		return false
	}
	certData, err := base64.StdEncoding.DecodeString(cfg.Users[0].User.ClientCertificateData)
	if err != nil {
		return false
	}
	keyData, err := base64.StdEncoding.DecodeString(cfg.Users[0].User.ClientKeyData)
	if err != nil {
		return false
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return false
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caData) {
		return false
	}

	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      pool,
				Certificates: []tls.Certificate{cert},
			},
		},
	}

	resp, err := client.Get(cfg.Clusters[0].Cluster.Server + "/livez")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// WriteKubeconfig writes kubeconfig to a file with appropriate permissions
func WriteKubeconfig(kubeconfig, path string) error {
	if err := os.WriteFile(path, []byte(kubeconfig), 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}
	slog.Info("Kubeconfig written successfully", "path", path)
	return nil
}

// MergeKubeconfig merges a single-cluster kubeconfig (as produced by
// ExtractKubeconfig) into the kubeconfig file at path under contextName,
// replacing any existing cluster/context/user with that name, and makes it
// the active context. The target file (and its directory) is created if it
// doesn't exist.
func MergeKubeconfig(kubeconfig, path, contextName string) error {
	var extracted map[string]interface{}
	if err := yaml.Unmarshal([]byte(kubeconfig), &extracted); err != nil {
		return fmt.Errorf("failed to parse extracted kubeconfig: %w", err)
	}

	cluster := firstNamed(extracted, "clusters")
	kctx := firstNamed(extracted, "contexts")
	user := firstNamed(extracted, "users")
	if cluster == nil || kctx == nil || user == nil {
		return fmt.Errorf("extracted kubeconfig is missing clusters/contexts/users")
	}

	cluster["name"] = contextName
	user["name"] = contextName
	kctx["name"] = contextName
	if contextData, ok := kctx["context"].(map[string]interface{}); ok {
		contextData["cluster"] = contextName
		contextData["user"] = contextName
	}

	target, err := loadOrInitKubeconfig(path)
	if err != nil {
		return err
	}

	target["clusters"] = mergeNamed(asList(target["clusters"]), cluster, contextName)
	target["contexts"] = mergeNamed(asList(target["contexts"]), kctx, contextName)
	target["users"] = mergeNamed(asList(target["users"]), user, contextName)
	target["current-context"] = contextName

	data, err := yaml.Marshal(target)
	if err != nil {
		return fmt.Errorf("failed to marshal merged kubeconfig: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	slog.Info("Merged kubeconfig", "path", path, "context", contextName)
	return nil
}

// firstNamed returns the first entry of cfg[key] as a map, or nil if absent.
func firstNamed(cfg map[string]interface{}, key string) map[string]interface{} {
	list, _ := cfg[key].([]interface{})
	if len(list) == 0 {
		return nil
	}
	entry, _ := list[0].(map[string]interface{})
	return entry
}

// asList returns v as a []interface{}, or an empty slice if it isn't one.
func asList(v interface{}) []interface{} {
	list, _ := v.([]interface{})
	return list
}

// mergeNamed appends entry to list, replacing any existing item whose
// "name" field matches name.
func mergeNamed(list []interface{}, entry map[string]interface{}, name string) []interface{} {
	filtered := make([]interface{}, 0, len(list)+1)
	for _, item := range list {
		if m, ok := item.(map[string]interface{}); ok {
			if n, _ := m["name"].(string); n == name {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return append(filtered, entry)
}

// loadOrInitKubeconfig reads the kubeconfig at path, or returns a fresh
// empty structure if it doesn't exist yet.
func loadOrInitKubeconfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]interface{}{
			"apiVersion":  "v1",
			"kind":        "Config",
			"preferences": map[string]interface{}{},
			"clusters":    []interface{}{},
			"contexts":    []interface{}{},
			"users":       []interface{}{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse existing kubeconfig %s: %w", path, err)
	}
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	return cfg, nil
}
