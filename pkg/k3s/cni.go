package k3s

import (
	"fmt"

	"github.com/liamawhite/homelab/pkg/ssh"
)

// flannelDisabledConfig is written to /etc/rancher/k3s/config.yaml. K3s
// reads this file in addition to any install-time flags on every service
// start, so this takes effect without re-running the install script.
const flannelDisabledConfig = `flannel-backend: "none"
disable-network-policy: true
`

// DisableFlannel disables K3s's built-in Flannel CNI and its network policy
// controller on the node reachable via client, then restarts the node's k3s
// service so the change takes effect.
//
// This is a one-way, disruptive step: pod networking on this node is broken
// from this point until a replacement CNI (see pkg/components/cilium) is
// installed cluster-wide and every existing pod is recreated. It is only
// meaningful as part of the coordinated maintenance-window migration
// described in that component's package doc - do not run this against a
// single node in isolation on an otherwise-healthy cluster.
func DisableFlannel(client *ssh.Client) error {
	if err := client.WriteFile("/etc/rancher/k3s/config.yaml", flannelDisabledConfig, true); err != nil {
		return fmt.Errorf("failed to write /etc/rancher/k3s/config.yaml: %w", err)
	}

	service, err := detectServiceName(client)
	if err != nil {
		return err
	}

	if _, _, err := client.ExecuteSudo(fmt.Sprintf("systemctl restart %s", service)); err != nil {
		return fmt.Errorf("failed to restart %s: %w", service, err)
	}

	// Best-effort: a leftover Flannel VXLAN interface can conflict with the
	// replacement CNI's own interface setup if left in place. Ignore
	// errors - the interface may already be gone.
	_, _, _ = client.ExecuteSudo("ip link delete flannel.1")

	return nil
}

// EnableFlannel reverts DisableFlannel: removes the config.yaml override so
// K3s falls back to its default Flannel CNI on next start, then restarts
// the node's k3s service. No config.yaml existed before DisableFlannel
// first created one, so removing it entirely (rather than editing it)
// restores the exact pre-migration state.
func EnableFlannel(client *ssh.Client) error {
	if _, _, err := client.ExecuteSudo("rm -f /etc/rancher/k3s/config.yaml"); err != nil {
		return fmt.Errorf("failed to remove /etc/rancher/k3s/config.yaml: %w", err)
	}

	service, err := detectServiceName(client)
	if err != nil {
		return err
	}

	if _, _, err := client.ExecuteSudo(fmt.Sprintf("systemctl restart %s", service)); err != nil {
		return fmt.Errorf("failed to restart %s: %w", service, err)
	}

	return nil
}

// detectServiceName returns "k3s" for a server node or "k3s-agent" for an
// agent node, based on which systemd unit is actually present - every node
// in this repo's infra.yaml is a server node today, but this stays correct
// if an agent node is ever added.
func detectServiceName(client *ssh.Client) (string, error) {
	if _, _, err := client.Execute("systemctl list-unit-files k3s.service --no-legend | grep -q k3s.service"); err == nil {
		return "k3s", nil
	}
	if _, _, err := client.Execute("systemctl list-unit-files k3s-agent.service --no-legend | grep -q k3s-agent.service"); err == nil {
		return "k3s-agent", nil
	}
	return "", fmt.Errorf("could not detect k3s service name: neither k3s.service nor k3s-agent.service found")
}
