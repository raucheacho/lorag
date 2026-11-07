package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/lorag/lorag/internal/config"
	"github.com/lorag/lorag/internal/server"
	vectixdb "github.com/raucheacho/vectixdb/bindings/go"
	"github.com/spf13/cobra"
)

var (
	// Version info
	Version   = "dev"
	BuildTime = "unknown"
	Commit    = "unknown"
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
	rootCmd.AddCommand(openCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func serverCmd() *cobra.Command {
	var (
		host    string
		port    int
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
			notice, err := cfg.ResolveDataDir()
			if err != nil {
				return err
			}
			if notice != "" {
				fmt.Fprintln(os.Stderr, "Note:", notice)
			}

			srv, err := server.NewServer(cfg, Version)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			return srv.Start()
		},
	}

	cmd.Flags().StringVarP(&host, "host", "H", "", "Server host (env: LORAG_HOST)")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "Server port (env: LORAG_PORT)")
	cmd.Flags().StringVarP(&dataDir, "data-dir", "d", "", "Data directory (env: LORAG_DATA_DIR, default ~/.lorag/data)")

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Lorag %s (commit %s, built %s, VectixDB %s)\n", Version, Commit, BuildTime, vectixdb.Version())
		},
	}
}

// openCmd opens the Studio in the browser, for when its address is forgotten.
func openCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "open",
		Short:         "Open the Studio in the browser",
		SilenceUsage:  true,
		SilenceErrors: true, // main prints it
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			host := cfg.ServerHost
			if host == "" || host == "0.0.0.0" || host == "::" {
				host = "localhost"
			}
			url := "http://" + net.JoinHostPort(host, strconv.Itoa(cfg.ServerPort))

			client := http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(url + "/health")
			if err != nil {
				return fmt.Errorf("lorag is not running on %s. Start it with `brew services start lorag`, `systemctl --user start lorag` or `lorag server`", url)
			}
			resp.Body.Close()

			fmt.Println("Studio:", url)
			var opener *exec.Cmd
			switch runtime.GOOS {
			case "darwin":
				opener = exec.Command("open", url)
			default:
				opener = exec.Command("xdg-open", url)
			}
			if err := opener.Start(); err != nil {
				fmt.Fprintln(os.Stderr, "Open this address in your browser.")
			}
			return nil
		},
	}
}
