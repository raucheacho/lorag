package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	// Version info
	Version   = "dev"
	BuildTime = "unknown"
	Commit    = "unknown"

	// API client
	apiBaseURL = "http://localhost:8080/api/v1"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "loragctl",
		Short: "Lorag CLI - Command line interface for Lorag",
		Long: `loragctl is the command line interface for the Lorag RAG engine.

It allows you to index files, search documents, and manage watches
from the command line.`,
		Version: fmt.Sprintf("%s (built %s)", Version, BuildTime),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Check for LORAG_HOST and LORAG_PORT env vars
			host := os.Getenv("LORAG_HOST")
			port := os.Getenv("LORAG_PORT")
			
			if host == "" {
				host = "localhost"
			}
			if port == "" {
				port = "8080"
			}
			
			apiBaseURL = fmt.Sprintf("http://%s:%s/api/v1", host, port)
		},
	}

	// Add commands
	rootCmd.AddCommand(indexCmd())
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(watchCmd())
	rootCmd.AddCommand(docCmd())
	rootCmd.AddCommand(memCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func indexCmd() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "index <path>",
		Short: "Index a file or directory",
		Long:  `Index a file or directory for semantic search.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			
			// Resolve absolute path
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			
			// Check if file or directory
			info, err := os.Stat(absPath)
			if err != nil {
				return fmt.Errorf("path not found: %w", err)
			}
			
			var endpoint string
			var reqBody map[string]interface{}
			
			if info.IsDir() {
				endpoint = "/index/directory"
				reqBody = map[string]interface{}{
					"path":      absPath,
					"recursive": recursive,
				}
				fmt.Printf("Indexing directory: %s\n", absPath)
			} else {
				endpoint = "/index/file"
				reqBody = map[string]interface{}{
					"path": absPath,
				}
				fmt.Printf("Indexing file: %s\n", absPath)
			}
			
			resp, err := apiPost(endpoint, reqBody)
			if err != nil {
				return err
			}
			
			fmt.Printf("✓ %s\n", resp["message"])
			fmt.Printf("  Documents: %v\n", resp["documents"])
			fmt.Printf("  Chunks: %v\n", resp["chunks"])
			
			return nil
		},
	}
	
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Index recursively for directories")
	
	return cmd
}

func searchCmd() *cobra.Command {
	var topK int
	var minScore float32

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search indexed documents",
		Long:  `Search through indexed documents using semantic search.`,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			
			reqBody := map[string]interface{}{
				"query":     query,
				"top_k":     topK,
				"min_score": minScore,
			}
			
			fmt.Printf("Searching: %s\n\n", query)
			
			resp, err := apiPost("/search", reqBody)
			if err != nil {
				return err
			}
			
			results, ok := resp["results"].([]interface{})
			if !ok || len(results) == 0 {
				fmt.Println("No results found.")
				return nil
			}
			
			for i, r := range results {
				result := r.(map[string]interface{})
				score := result["score"].(float64)
				content := result["content"].(string)
				
				source := result["source"].(map[string]interface{})
				filename := source["filename"].(string)
				
				// Truncate content
				lines := strings.Split(content, "\n")
				if len(lines) > 5 {
					content = strings.Join(lines[:5], "\n") + "\n..."
				}
				
				fmt.Printf("[%d] %s (score: %.3f)\n", i+1, filename, score)
				fmt.Printf("%s\n\n", content)
			}
			
			return nil
		},
	}
	
	cmd.Flags().IntVarP(&topK, "top-k", "k", 5, "Number of results to return")
	cmd.Flags().Float32VarP(&minScore, "min-score", "s", 0, "Minimum relevance score")
	
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show server status",
		Long:  `Display the current status and statistics of the Lorag server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/stats")
			if err != nil {
				return err
			}
			
			fmt.Println("Lorag Server Status")
			fmt.Println("===================")
			
			for k, v := range resp {
				fmt.Printf("%s: %v\n", k, v)
			}
			
			return nil
		},
	}
}

func watchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage watched directories",
		Long:  `Add, remove, or list watched directories for automatic indexing.`,
	}
	
	// List watches
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all watches",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/watches")
			if err != nil {
				return err
			}
			
			watchesRaw, ok := resp["watches"].([]interface{})
			if !ok || len(watchesRaw) == 0 {
				fmt.Println("No active watches.")
				return nil
			}
			watches := watchesRaw
			
			fmt.Printf("Active watches (%d):\n\n", len(watches))
			for _, w := range watches {
				watch := w.(map[string]interface{})
				fmt.Printf("ID: %s\n", watch["id"])
				fmt.Printf("  Path: %s\n", watch["path"])
				fmt.Printf("  Recursive: %v\n", watch["recursive"])
				fmt.Println()
			}
			
			return nil
		},
	}
	
	// Add watch
	addCmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a directory watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			
			recursive, _ := cmd.Flags().GetBool("recursive")
			
			reqBody := map[string]interface{}{
				"path":      absPath,
				"recursive": recursive,
			}
			
			resp, err := apiPost("/watches", reqBody)
			if err != nil {
				return err
			}
			
			fmt.Printf("Added watch: %s\n", resp["id"])
			fmt.Printf("  Path: %s\n", resp["path"])
			fmt.Printf("  Recursive: %v\n", resp["recursive"])
			
			return nil
		},
	}
	addCmd.Flags().BoolP("recursive", "r", false, "Watch recursively")
	
	// Remove watch
	removeCmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a directory watch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			
			err := apiDelete(fmt.Sprintf("/watches/%s", id))
			if err != nil {
				return err
			}
			
			fmt.Printf("Removed watch: %s\n", id)
			return nil
		},
	}
	
	cmd.AddCommand(listCmd, addCmd, removeCmd)
	
	return cmd
}

func docCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Manage documents",
		Long:  `List or delete indexed documents.`,
	}
	
	// List documents
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List indexed documents",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/documents")
			if err != nil {
				return err
			}
			
			docs, ok := resp["documents"].([]interface{})
			if !ok || len(docs) == 0 {
				fmt.Println("No documents indexed.")
				return nil
			}
			
			fmt.Printf("Indexed documents (%d):\n\n", len(docs))
			for _, d := range docs {
				doc := d.(map[string]interface{})
				fmt.Printf("ID: %s\n", doc["id"])
				fmt.Printf("  File: %s\n", doc["filename"])
				fmt.Printf("  Path: %s\n", doc["path"])
				fmt.Printf("  Chunks: %.0f\n", doc["chunk_count"])
				fmt.Println()
			}
			
			return nil
		},
	}
	
	// Delete document
	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a document from the index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			
			err := apiDelete(fmt.Sprintf("/documents/%s", id))
			if err != nil {
				return err
			}
			
			fmt.Printf("Deleted document: %s\n", id)
			return nil
		},
	}
	
	cmd.AddCommand(listCmd, deleteCmd)
	
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("loragctl %s (commit %s, built %s)\n", Version, Commit, BuildTime)
		},
	}
}

// API helper functions

func apiGet(endpoint string) (map[string]interface{}, error) {
	url := apiBaseURL + endpoint
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()
	
	return parseResponse(resp)
}

func apiPost(endpoint string, body interface{}) (map[string]interface{}, error) {
	url := apiBaseURL + endpoint
	
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()
	
	return parseResponse(resp)
}

func apiDelete(endpoint string) error {
	url := apiBaseURL + endpoint
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}
	
	return nil
}

func parseResponse(resp *http.Response) (map[string]interface{}, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	if resp.StatusCode >= 400 {
		var errResp map[string]interface{}
		if err := json.Unmarshal(body, &errResp); err == nil {
			if msg, ok := errResp["error"].(string); ok {
				return nil, fmt.Errorf("server error: %s", msg)
			}
		}
		return nil, fmt.Errorf("server error: %s", string(body))
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	
	return result, nil
}
