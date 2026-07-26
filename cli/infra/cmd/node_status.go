package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/k3s"
	"github.com/liamawhite/homelab/pkg/probe"
	"github.com/liamawhite/homelab/pkg/raspberry"
	"github.com/liamawhite/homelab/pkg/ssh"
	"github.com/spf13/cobra"
)

var nodeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show health and MAC address for each node",
	Long: `Checks each node defined in infra.yaml: whether it responds on the
network, whether SSH login succeeds (using the ssh.user/ssh.password from
that node's infra.yaml entry), whether the "pi bootstrap" step has been run,
whether k3s is installed, and whether that node's own Kubernetes API server
responds (dialed directly at its own address, not the VIP, and not another
node's). STATUS is "healthy" if all checks pass, otherwise it lists which
failed.

Also reports the node's MAC address and any other IPs seen advertising it.

Example:
  homelab node status`,
	RunE: runNodeStatus,
}

type nodeStatus struct {
	Name         string
	Address      string
	MAC          string
	Ping         bool
	SSH          bool
	Bootstrapped bool
	K3sInstalled bool
	APIHealthy   bool
	OtherIPs     string
}

// status renders a single health summary: "healthy" if every check passed,
// otherwise a list of what failed. Downstream checks that couldn't run
// because an earlier one failed aren't reported separately.
func (s nodeStatus) status() string {
	var problems []string
	if !s.Ping {
		problems = append(problems, "unreachable")
	} else if !s.SSH {
		problems = append(problems, "ssh failed")
	} else if !s.Bootstrapped {
		problems = append(problems, "not bootstrapped")
	} else if !s.K3sInstalled {
		problems = append(problems, "k3s not installed")
	} else if !s.APIHealthy {
		problems = append(problems, "kube api unreachable")
	}

	if len(problems) == 0 {
		return "healthy"
	}
	return strings.Join(problems, "; ")
}

// nodeChecks holds the results of checks that require an authenticated SSH
// session, so a node's goroutine only has to connect once for all of them.
type nodeChecks struct {
	Bootstrapped bool
	APIHealthy   bool
}

// checkNode opens an authenticated SSH connection and runs the bootstrap
// check (reusing the same file/package checks Provision uses, so
// "bootstrapped" here always means the same thing it does to
// `pi bootstrap`) and, if k3s is installed, a direct check that this
// specific node's own Kubernetes API is responding - not just the VIP or
// another node's.
func checkNode(node config.NodeConfig, k3sInstalled bool) nodeChecks {
	client := ssh.NewClientWithPassword(node.Address, node.SSH.User, node.SSH.Password)
	if err := client.Connect(context.Background()); err != nil {
		return nodeChecks{}
	}
	defer client.Close()

	var checks nodeChecks
	checks.Bootstrapped = raspberry.NewProvisioner(client).CheckBootstrapped().Bootstrapped()

	if k3sInstalled {
		if kubeconfig, err := k3s.ExtractKubeconfig(context.Background(), client, node.Address); err == nil {
			checks.APIHealthy = k3s.CheckAPIHealth(kubeconfig)
		}
	}

	return checks
}

func runNodeStatus(cmd *cobra.Command, args []string) error {
	// cli/pkg/ssh logs progress via slog straight to stdout, which would
	// otherwise interleave with this command's table output.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	infraCfg, err := config.LoadInfra(cmd)
	if err != nil {
		return err
	}

	statuses := make([]nodeStatus, len(infraCfg.Nodes))

	var wg sync.WaitGroup
	for i, node := range infraCfg.Nodes {
		wg.Add(1)
		go func(i int, node config.NodeConfig) {
			defer wg.Done()

			var result probe.Result
			var sshResult probe.SSHResult

			var inner sync.WaitGroup
			inner.Add(2)
			go func() {
				defer inner.Done()
				result = probe.Host(node.Address)
			}()
			go func() {
				defer inner.Done()
				sshResult = probe.SSH(node.Address, node.SSH.User, node.SSH.Password)
			}()
			inner.Wait()

			var checks nodeChecks
			if sshResult.Authenticated {
				checks = checkNode(node, sshResult.K3sInstalled)
			}

			statuses[i] = nodeStatus{
				Name:         node.Name,
				Address:      node.Address,
				MAC:          result.MAC,
				Ping:         result.Reachable,
				SSH:          sshResult.Authenticated,
				Bootstrapped: checks.Bootstrapped,
				APIHealthy:   checks.APIHealthy,
				K3sInstalled: sshResult.K3sInstalled,
			}
		}(i, node)
	}
	wg.Wait()

	macToIPs := make(map[string][]string)
	for ip, mac := range probe.Table() {
		macToIPs[mac] = append(macToIPs[mac], ip)
	}

	for i := range statuses {
		statuses[i].OtherIPs = otherIPs(macToIPs, statuses[i].MAC, statuses[i].Address)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NODE\tIP\tMAC\tOTHER IPS\tSTATUS")
	for _, s := range statuses {
		mac := s.MAC
		if mac == "" {
			mac = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Address, mac, s.OtherIPs, s.status())
	}

	return w.Flush()
}

// otherIPs returns the other IPs (besides self) currently seen advertising
// mac in the local neighbor table, joined for display, or "-" if none.
func otherIPs(macToIPs map[string][]string, mac, self string) string {
	if mac == "" {
		return "-"
	}

	var others []string
	for _, ip := range macToIPs[mac] {
		if ip != self {
			others = append(others, ip)
		}
	}
	if len(others) == 0 {
		return "-"
	}

	sort.Strings(others)
	return strings.Join(others, ",")
}
