package cmd

import (
	"log/slog"
	"os"

	pulumicmd "github.com/liamawhite/homelab/cli/cmd/pulumi"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "homelab",
	Short: "Homelab infrastructure management CLI",
	Long:  `A CLI tool to provision Raspberry Pi nodes and manage K3s clusters for your homelab.`,
}

func Execute() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command failed", "error", err)
		os.Exit(1)
	}
}

func init() {
	// Global persistent flags
	rootCmd.PersistentFlags().String("config", "", "Path to infra.yaml config file (auto-detected if not specified)")
	rootCmd.PersistentFlags().String("timeout", "2m", "Total time budget for the up/preview/refresh/cancel operation itself (Go duration format, e.g. 90s, 2m, 5m) - a stuck resource (e.g. a Deployment rollout wait) aborts the whole operation once this elapses, rather than each resource getting its own allowance")

	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(k3sCmd)
	rootCmd.AddCommand(kubeconfigCmd)
	rootCmd.AddCommand(nodeCmd)
	rootCmd.AddCommand(pulumicmd.UpCmd)
	rootCmd.AddCommand(pulumicmd.PreviewCmd)
	rootCmd.AddCommand(pulumicmd.RefreshCmd)
	rootCmd.AddCommand(pulumicmd.CancelCmd)
}
