package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lorag/lorag/internal/api/mcp"
	"github.com/lorag/lorag/internal/api/rest"
	"github.com/lorag/lorag/internal/config"
	"github.com/lorag/lorag/internal/core/indexer"
	"github.com/lorag/lorag/internal/core/query"
	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/core/vector"
	"github.com/lorag/lorag/internal/embeddings"
	"github.com/lorag/lorag/internal/watcher"
)

// Server holds the application server
type Server struct {
	config         *config.Config
	store          *store.BoltStore
	settingsStore  *store.SettingsStore
	vectorIdx      vector.Index
	embedder       embeddings.Provider
	indexer        *indexer.Indexer
	query          *query.Engine
	watcher        *watcher.Watcher
	httpServer     *http.Server
	appSettings    *store.AppSettings
}

// NewServer creates a new server
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize store
	st, err := store.NewBoltStore(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	// Initialize settings store
	settingsStore := store.NewSettingsStore(st)
	
	// Load persisted settings or use defaults
	appSettings, err := settingsStore.GetAppSettings()
	if err != nil {
		fmt.Printf("Warning: failed to load settings, using defaults: %v\n", err)
		appSettings = store.DefaultSettings()
	}

	// Override with config if explicitly set via env vars
	if cfg.EmbeddingProvider != "" {
		appSettings.EmbeddingProvider = cfg.EmbeddingProvider
	}
	if cfg.EmbeddingModel != "" {
		appSettings.EmbeddingModel = cfg.EmbeddingModel
	}
	if cfg.EmbeddingURL != "" {
		appSettings.EmbeddingURL = cfg.EmbeddingURL
	}

	fmt.Printf("✓ Settings loaded (template: %s)\n", appSettings.DefaultTemplate)

	// Initialize embedding provider
	embedCfg := embeddings.Config{
		Type:      embeddings.ProviderType(appSettings.EmbeddingProvider),
		Model:     appSettings.EmbeddingModel,
		URL:       appSettings.EmbeddingURL,
		Dimension: appSettings.EmbeddingDim,
		Timeout:   30 * time.Second,
	}

	embedder, err := embeddings.NewProvider(embedCfg)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	// Initialize vector index (HNSW)
	vectorCfg := vector.Config{
		Type:        vector.IndexType(appSettings.VectorIndexType),
		Dimension:   appSettings.EmbeddingDim,
		M:           appSettings.VectorM,
		EfConstruct: appSettings.VectorEf,
		MaxElements: appSettings.MaxChunks,
		Distance:    "cosine",
	}

	vectorIdx, err := vector.NewIndex(vectorCfg)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("failed to create vector index: %w", err)
	}

	// Try to load existing vector index
	vectorPath := cfg.VectorIndexPath()
	if _, err := os.Stat(vectorPath); err == nil {
		fmt.Println("Loading existing vector index...")
		if err := vectorIdx.Load(vectorPath); err != nil {
			fmt.Printf("Warning: failed to load vector index: %v\n", err)
		} else {
			fmt.Println("✓ Vector index loaded")
		}
	}

	// Initialize indexer with async queue
	indexerCfg := indexer.Config{
		MaxChunks:    cfg.MaxChunksAllowed(),
		ChunkSize:    appSettings.ChunkSize,
		ChunkOverlap: appSettings.ChunkOverlap,
		Workers:      appSettings.IndexWorkers,
		MaxQueue:     cfg.MaxQueueSize,
	}

	idx := indexer.NewIndexer(st, settingsStore, vectorIdx, embedder, indexerCfg)

	// Initialize query engine
	qe := query.NewEngine(st, vectorIdx, embedder)

	s := &Server{
		config:        cfg,
		store:         st,
		settingsStore: settingsStore,
		appSettings:   appSettings,
		vectorIdx:     vectorIdx,
		embedder:      embedder,
		indexer:       idx,
		query:         qe,
	}

	// Initialize watcher and restore watched folders
	if err := s.initWatcherWithSettings(); err != nil {
		fmt.Printf("Warning: failed to initialize watcher: %v\n", err)
	}

	return s, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Setup HTTP handlers
	handler := rest.NewHandler(s.store, s.settingsStore, s.indexer, s.query, s.watcher)
	router := rest.NewRouter(handler)

	// Add MCP handler
	mcpServer := mcp.NewServer(s.store, s.settingsStore, s.indexer, s.query)
	router.(*chi.Mux).HandleFunc("/mcp", mcpServer.Handle)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    s.config.GetAddress(),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("🚀 Lorag server starting on http://%s\n", s.config.GetAddress())
		fmt.Printf("📚 API: http://%s/api/v1/\n", s.config.GetAddress())
		fmt.Printf("🔌 MCP: http://%s/mcp\n", s.config.GetAddress())
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.Shutdown(ctx)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Save vector index
	if s.vectorIdx != nil {
		fmt.Println("Saving vector index...")
		vectorPath := s.config.VectorIndexPath()
		if err := s.vectorIdx.Save(vectorPath); err != nil {
			fmt.Printf("Warning: failed to save vector index: %v\n", err)
		} else {
			fmt.Println("✓ Vector index saved")
		}
	}

	// Shutdown HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			fmt.Printf("HTTP server shutdown error: %v\n", err)
		}
	}

	// Stop watcher
	if s.watcher != nil {
		if err := s.watcher.Stop(); err != nil {
			fmt.Printf("Watcher stop error: %v\n", err)
		}
	}

	// Close store
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			fmt.Printf("Store close error: %v\n", err)
		}
	}

	fmt.Println("✓ Server stopped")
	return nil
}

// initWatcherWithSettings initializes the file watcher with persisted watches
func (s *Server) initWatcherWithSettings() error {
	w, err := watcher.NewWatcher(s.store, s.indexer)
	if err != nil {
		return err
	}

	if err := w.Start(context.Background()); err != nil {
		return err
	}

	s.watcher = w

	// Restore watched folders from settings
	watches, err := s.settingsStore.GetWatches()
	if err != nil {
		fmt.Printf("Warning: failed to load watches: %v\n", err)
		return nil
	}

	for _, watch := range watches {
		if _, err := w.AddWatch(watch.Path, watch.Recursive, watch.Patterns); err != nil {
			fmt.Printf("Warning: failed to restore watch for %s: %v\n", watch.Path, err)
		} else {
			fmt.Printf("✓ Restored watch: %s\n", watch.Path)
		}
	}

	return nil
}

// GetStats returns server statistics
func (s *Server) GetStats() map[string]interface{} {
	return s.indexer.GetStats()
}

// GetSettingsStore returns the settings store
func (s *Server) GetSettingsStore() *store.SettingsStore {
	return s.settingsStore
}

// GetAppSettings returns the current app settings
func (s *Server) GetAppSettings() *store.AppSettings {
	return s.appSettings
}

// SaveAppSettings saves and applies new settings
func (s *Server) SaveAppSettings(settings *store.AppSettings) error {
	if err := s.settingsStore.SaveAppSettings(settings); err != nil {
		return err
	}
	s.appSettings = settings
	return nil
}
