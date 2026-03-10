// Package main is the entry point for the Alibaba Cloud Observability MCP Server.
// It uses cobra for CLI subcommands (start, version, tools).
// Configuration is loaded from config.yaml and .env files.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"

	"github.com/alibabacloud-observability-mcp-server-go/internal/client"
	"github.com/alibabacloud-observability-mcp-server-go/internal/config"
	"github.com/alibabacloud-observability-mcp-server-go/internal/logger"
	"github.com/alibabacloud-observability-mcp-server-go/internal/server"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit/iaas"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit/paas"
	"github.com/alibabacloud-observability-mcp-server-go/internal/toolkit/shared"
	"github.com/spf13/cobra"
)

// Build-time variables injected via ldflags.
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// rootCmd creates the root cobra command.
func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alibabacloud-observability-mcp-server",
		Short: "Alibaba Cloud Observability MCP Server",
		Long:  "MCP server providing AI-driven access to Alibaba Cloud observability services (SLS, CMS).",
		// Show help when invoked without subcommand.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(startCmd())
	cmd.AddCommand(versionCmd())
	cmd.AddCommand(toolsCmd())

	return cmd
}

// configPath holds the path to the config file (set via --config flag).
var configPath string

// startCmd creates the "start" subcommand that boots the MCP server.
func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the MCP server",
		Long:  "Start the MCP server with configuration from config.yaml and .env files.",
		RunE:  runStart,
	}

	// Register --config flag for specifying config file path.
	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file (default: ./config.yaml)")

	return cmd
}

// runStart is the RunE handler for the start command.
func runStart(cmd *cobra.Command, _ []string) error {
	// Load configuration (config.yaml + .env + shell env fallback).
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize structured logger.
	logger.Init(cfg.Logging.Level, cfg.Logging.DebugMode)
	slog.Info("starting server",
		"version", version,
		"transport", cfg.Server.Transport,
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"toolkit_scope", cfg.Toolkit.Scope,
	)

	// Create credential provider.
	cred := client.NewCredentialProvider(cfg.Credentials.AccessKeyID, cfg.Credentials.AccessKeySecret)

	// Validate credentials early.
	if _, err := cred.GetAccessKeyID(); err != nil {
		return fmt.Errorf("credential check failed: %w", err)
	}

	// Create API clients.
	slsClient := client.NewSLSClient(cred, cfg)
	cmsClient := client.NewCMSClient(cred, cfg)

	// Wire CMS client to SLS client for TextToSQL delegation.
	slsClient.SetCMSClient(cmsClient)

	// Create toolkit registry and register toolkits based on scope.
	registry := toolkit.NewRegistry()
	paasToolkit := paas.NewPaaSToolkit(cmsClient)
	iaasToolkit := iaas.NewIaaSToolkit(slsClient, cmsClient)
	sharedToolkit := shared.New(cmsClient)
	toolkit.RegisterToolkits(registry, cfg.Toolkit.Scope, paasToolkit, iaasToolkit, sharedToolkit)

	slog.Info("toolkits registered",
		"scope", cfg.Toolkit.Scope,
		"tool_count", len(registry.List()),
	)

	// Create MCP server.
	srv, err := server.NewServer(cfg, registry, slsClient, cmsClient)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	// Run server (blocks until signal or error).
	ctx := context.Background()
	return server.Run(ctx, cfg, srv)
}

// versionCmd creates the "version" subcommand.
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "Version:    %s\n", version)
			fmt.Fprintf(cmd.OutOrStdout(), "Build Time: %s\n", buildTime)
			fmt.Fprintf(cmd.OutOrStdout(), "Git Commit: %s\n", gitCommit)
		},
	}
}

// toolsCmd creates the "tools" subcommand that lists registered tools.
func toolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List all registered MCP tools",
		Long:  "List all MCP tools available in the current toolkit scope (configured in config.yaml).",
		RunE:  runTools,
	}

	return cmd
}

// runTools lists all tools registered under the configured scope.
// It uses nil clients since only tool metadata (name/description) is needed.
func runTools(cmd *cobra.Command, _ []string) error {
	// Load configuration to get toolkit scope.
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Build registry with nil clients — we only need tool metadata.
	registry := toolkit.NewRegistry()
	paasToolkit := paas.NewPaaSToolkit(nil)
	iaasToolkit := iaas.NewIaaSToolkit(nil, nil)
	sharedToolkit := shared.New(nil)
	toolkit.RegisterToolkits(registry, cfg.Toolkit.Scope, paasToolkit, iaasToolkit, sharedToolkit)

	tools := registry.List()
	out := cmd.OutOrStdout()
	if len(tools) == 0 {
		fmt.Fprintln(out, "No tools registered for scope:", cfg.Toolkit.Scope)
		return nil
	}

	fmt.Fprintf(out, "Tools (scope=%s, count=%d):\n\n", cfg.Toolkit.Scope, len(tools))

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\n")
	fmt.Fprintf(w, "----\t-----------\n")
	for _, t := range tools {
		// Truncate description to first line for readability.
		desc := firstLine(t.Description)
		fmt.Fprintf(w, "%s\t%s\n", t.Name, desc)
	}
	return w.Flush()
}

// firstLine returns the first non-empty line of s, trimmed.
func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[:i]
			if line != "" {
				return line
			}
		}
	}
	return s
}
