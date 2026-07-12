// Package pulumi implements the CLI's "up" and "preview" commands, which run
// the Pulumi program in pkg/deploy fully inline via the Automation API - no
// on-disk Pulumi project directory involved, project/backend settings are
// defined directly in Go below.
package pulumi

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/liamawhite/homelab/pkg/config"
	"github.com/liamawhite/homelab/pkg/deploy"
	"github.com/liamawhite/homelab/pkg/k3s"
	"github.com/liamawhite/homelab/pkg/ssh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

const (
	projectName = "homelab"
	stackName   = "homelab"

	// stateDir is where checkpoint state is stored (see .gitattributes -
	// it's git-crypt'd), resolved relative to the repo root. These commands
	// are expected to be run from there, same as the rest of the CLI.
	stateDir = ".pulumi-state"
)

// prepareStack resolves a reachable cluster endpoint, extracts a kubeconfig
// from it, and returns an Automation API stack wired up to deploy against
// that cluster.
func prepareStack(cmd *cobra.Command) (context.Context, auto.Stack, error) {
	ctx := context.Background()

	slog.Info("Loading configuration")
	infraCfg, err := config.LoadInfra(cmd)
	if err != nil {
		return nil, auto.Stack{}, err
	}

	slog.Info("Resolving a reachable cluster endpoint")
	address, sshUser, sshPassword, err := k3s.ResolveClusterEndpoint(ctx, infraCfg)
	if err != nil {
		return nil, auto.Stack{}, err
	}
	slog.Info("Found reachable cluster endpoint", "address", address)

	client := ssh.NewClientWithPassword(address, sshUser, sshPassword)
	if err := client.Connect(ctx); err != nil {
		return nil, auto.Stack{}, err
	}
	defer client.Close()

	slog.Info("Extracting kubeconfig")
	kubeconfig, err := k3s.ExtractKubeconfig(ctx, client, address)
	if err != nil {
		return nil, auto.Stack{}, err
	}

	backendDir, err := filepath.Abs(stateDir)
	if err != nil {
		return nil, auto.Stack{}, fmt.Errorf("failed to resolve state directory: %w", err)
	}

	project := workspace.Project{
		Name:    tokens.PackageName(projectName),
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		Backend: &workspace.ProjectBackend{URL: "file://" + backendDir},
	}

	// State lives in the git-crypt'd .pulumi-state/ dir, so secrets only
	// need to survive at rest behind git-crypt - the passphrase-based
	// secrets provider itself is left blank rather than adding a second
	// secret to manage.
	stack, err := auto.UpsertStackInlineSource(ctx, stackName, projectName,
		deploy.Program(kubeconfig, infraCfg), auto.Project(project),
		auto.EnvVars(map[string]string{
			"PULUMI_CONFIG_PASSPHRASE":      "",
			"PULUMI_K8S_ENABLE_PATCH_FORCE": "true",
		}))
	if err != nil {
		return nil, auto.Stack{}, fmt.Errorf("failed to create pulumi stack: %w", err)
	}

	return ctx, stack, nil
}
