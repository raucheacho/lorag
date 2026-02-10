package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/lorag/lorag/internal/config"
	"github.com/lorag/lorag/internal/server"
)

var (
	// Version info
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "lorag",
		Short: "Lorag - Local RAG + MCP Engine",
		Long: `Lorag is a local-first RAG (Retrieval-Augmented Generation) engine
with MCP (Model Context Protocol) support for AI agents.

It allows you to index your local documents and search them using
semantic search, all while keeping your data private and local.`,
		Version: fmt.Sprintf("%s (built %s)", Version, BuildTime),
	}

	// Add commands
	rootCmd.AddCommand(serverCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func serverCmd() *cobra.Command {
	var (
		host string
		port int
		dataDir string
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the Lorag server",
		Long:  `Start the Lorag HTTP server with REST API and MCP support.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Override with flags
			if host != "" {
				cfg.ServerHost = host
			}
			if port != 0 {
				cfg.ServerPort = port
			}
			if dataDir != "" {
				cfg.DataDir = dataDir
			}

			// Create and start server
			srv, err := server.NewServer(cfg)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			return srv.Start()
		},
	}

	cmd.Flags().StringVarP(&host, "host", "H", "", "Server host (env: LORAG_HOST)")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Server port (env: LORAG_PORT)")
	cmd.Flags().StringVarP(&dataDir, "data-dir", "d", "", "Data directory (env: LORAG_DATA_DIR)")

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Lorag %s (built %s)\n", Version, BuildTime)
		},
	}
}
