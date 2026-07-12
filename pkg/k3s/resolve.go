package k3s

import (
	"context"
	"fmt"
	"strings"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/ssh"
)

// ResolveClusterEndpoint finds a reachable address to run Pulumi against:
// the cluster VIP if it's already up (e.g. kube-vip is already deployed),
// falling back to trying each node directly, in infra.yaml order - needed
// for the very first "up", before kube-vip exists to serve the VIP.
//
// Returns the working address along with SSH credentials for it. The VIP
// is tried using the first node's credentials, since kube-vip routes it to
// whichever node currently holds it (same convention as the "kubeconfig"
// command's default, VIP-based connection).
func ResolveClusterEndpoint(ctx context.Context, infraCfg *config.InfraConfig) (address, sshUser, sshPassword string, err error) {
	if len(infraCfg.Nodes) == 0 {
		return "", "", "", fmt.Errorf("no nodes defined in infra.yaml")
	}
	first := infraCfg.Nodes[0]

	type candidate struct {
		address, user, password string
	}

	var candidates []candidate
	if infraCfg.Cluster.VIP != "" {
		candidates = append(candidates, candidate{infraCfg.Cluster.VIP, first.SSH.User, first.SSH.Password})
	}
	for _, node := range infraCfg.Nodes {
		candidates = append(candidates, candidate{node.Address, node.SSH.User, node.SSH.Password})
	}

	var tried []string
	for _, c := range candidates {
		client := ssh.NewClientWithPassword(c.address, c.user, c.password)
		if connErr := client.Connect(ctx); connErr != nil {
			tried = append(tried, c.address)
			continue
		}
		client.Close()
		return c.address, c.user, c.password, nil
	}

	return "", "", "", fmt.Errorf("no reachable cluster endpoint found (tried: %s)", strings.Join(tried, ", "))
}
