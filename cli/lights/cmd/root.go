package cmd

import (
	"context"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "homelab-lights",
	Short: "Smart lights management CLI",
	Long:  `A CLI tool to discover and pair Hue bridges, and manage the lights and switches they control.`,
}

func Execute() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		slog.Error("Command failed", "error", err)
		os.Exit(1)
	}
}

func init() {
	// Global persistent flags
	rootCmd.PersistentFlags().String("config", "", "Path to infra.yaml config file (auto-detected if not specified)")

	rootCmd.AddCommand(hubCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(switchesCmd)
}
