package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lorag/lorag/internal/activity"
	"github.com/lorag/lorag/internal/api/mcp"
	"github.com/lorag/lorag/internal/api/rest"
	"github.com/lorag/lorag/internal/config"
	"github.com/lorag/lorag/internal/core/engine"
	"github.com/lorag/lorag/internal/core/indexer"
	"github.com/lorag/lorag/internal/core/query"
	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/embeddings"
	"github.com/lorag/lorag/internal/memory"
	"github.com/lorag/lorag/internal/watcher"
)

// Server holds the application server
type Server struct {
	config        *config.Config
	version       string
	store         *store.BoltStore
	settingsStore *store.SettingsStore
	engine        *engine.Engine
	memory        *memory.Service
	embedder      embeddings.Provider
	indexer       *indexer.Indexer
	query         *query.Engine
	activity      *activity.Log
	watcher       *watcher.Watcher
	httpServer    *http.Server
	appSettings   *store.AppSettings
	startedAt     time.Time
}

// NewServer creates a new server
func NewServer(cfg *config.Config, version string) (*Server, error) {
	st, err := store.NewBoltStore(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}
	settingsStore := store.NewSettingsStore(st)

	appSettings, err := settingsStore.GetAppSettings()
	if err != nil {
		log.Printf("failed to load settings, using defaults: %v", err)
		appSettings = store.DefaultSettings()
	}
	if cfg.EmbeddingProvider != "" {
		appSettings.EmbeddingProvider = cfg.EmbeddingProvider
	}
	if cfg.EmbeddingModel != "" {
		appSettings.EmbeddingModel = cfg.EmbeddingModel
	}
	if cfg.EmbeddingURL != "" {
		appSettings.EmbeddingURL = cfg.EmbeddingURL
	}
	if cfg.EmbeddingDim > 0 {
		appSettings.EmbeddingDim = cfg.EmbeddingDim
	}
	if cfg.MaxChunks > 0 {
		appSettings.MaxChunks = cfg.MaxChunks
	}

	embedder, err := embeddings.NewProvider(embeddings.Config{
		Type:      embeddings.ProviderType(appSettings.EmbeddingProvider),
		Model:     appSettings.EmbeddingModel,
		URL:       appSettings.EmbeddingURL,
		APIKey:    cfg.EmbeddingAPIKey,
		Dimension: appSettings.EmbeddingDim,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	eng, err := engine.Open(engine.Config{
		Dir:            cfg.IndexDir(),
		Model:          appSettings.EmbeddingModel,
		Dim:            appSettings.EmbeddingDim,
		Metric:         appSettings.VectorMetric,
		Quantized:      appSettings.VectorQuantized,
		M:              appSettings.VectorM,
		EfConstruction: appSettings.VectorEf,
		EfSearch:       appSettings.VectorEfSearch,
		BM25K1:         appSettings.BM25K1,
		BM25B:          appSettings.BM25B,
	})
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("failed to open search engine: %w", err)
	}

	mem, err := memory.Open(eng.DB(), embedder, appSettings.EmbeddingDim, memory.Options{
		Model:             appSettings.EmbeddingModel,
		MinSimilarity:     appSettings.MemoryMinSimilarity,
		MinSimilarityText: appSettings.MemoryMinSimilarityText,
	})
	if err != nil {
		eng.Close()
		st.Close()
		return nil, fmt.Errorf("failed to open memory: %w", err)
	}

	idx := indexer.NewIndexer(st, settingsStore, eng, embedder, indexer.Config{
		MaxChunks:    appSettings.MaxChunks,
		ChunkSize:    appSettings.ChunkSize,
		ChunkOverlap: appSettings.ChunkOverlap,
		Workers:      appSettings.IndexWorkers,
		MaxQueue:     cfg.MaxQueueSize,
	})
	act := activity.NewLog(1000)

	s := &Server{
		config:        cfg,
		version:       version,
		store:         st,
		settingsStore: settingsStore,
		appSettings:   appSettings,
		engine:        eng,
		memory:        mem,
		embedder:      embedder,
		indexer:       idx,
		query:         query.NewEngine(st, eng, embedder, act),
		activity:      act,
		startedAt:     time.Now(),
	}

	s.rebuildIfOutOfSync()

	if err := s.initWatcher(); err != nil {
		log.Printf("failed to initialize watcher: %v", err)
	}
	return s, nil
}

// rebuildIfOutOfSync repopulates the indexes from the store when they hold
// fewer chunks than the database, e.g. on first start after an upgrade or
// after the embedding dimension changed.
func (s *Server) rebuildIfOutOfSync() {
	chunks, _ := s.store.CountChunks()
	if chunks == 0 || s.engine.Len() >= chunks {
		return
	}
	log.Printf("search index has %d of %d chunks, rebuilding in background", s.engine.Len(), chunks)
	go func() {
		start := time.Now()
		n, err := s.indexer.Rebuild(context.Background())
		if err != nil {
			log.Printf("index rebuild failed: %v", err)
			return
		}
		log.Printf("index rebuilt: %d chunks in %s", n, time.Since(start).Round(time.Millisecond))
	}()
}

// Start starts the server and blocks until SIGINT/SIGTERM.
func (s *Server) Start() error {
	handler := rest.NewHandler(rest.Deps{
		Store:     s.store,
		Settings:  s.settingsStore,
		Indexer:   s.indexer,
		Query:     s.query,
		Engine:    s.engine,
		Memory:    s.memory,
		Embedder:  s.embedder,
		Activity:  s.activity,
		Watcher:   s.watcher,
		Version:   s.version,
		DataDir:   s.config.DataDir,
		StartedAt: s.startedAt,
		Addr:      s.config.GetAddress(),
	})
	router := rest.NewRouter(handler)

	mcpServer := mcp.NewServer(mcp.Deps{
		Store: s.store, Settings: s.settingsStore, Indexer: s.indexer, Query: s.query,
		Memory: s.memory, Activity: s.activity, Version: s.version,
	})
	router.(*chi.Mux).HandleFunc("/mcp", mcpServer.Handle)

	s.httpServer = &http.Server{
		Addr:              s.config.GetAddress(),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		addr := s.config.GetAddress()
		fmt.Printf("Lorag %s (core %s)\n", s.version, s.engine.Stats().CoreVersion)
		fmt.Printf("  Studio  http://%s\n", addr)
		fmt.Printf("  REST    http://%s/api/v1\n", addr)
		fmt.Printf("  MCP     http://%s/mcp\n", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errc <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
	case err := <-errc:
		s.Shutdown(context.Background())
		return err
	}

	fmt.Println("\nShutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.Shutdown(ctx)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}
	if s.watcher != nil {
		if err := s.watcher.Stop(); err != nil {
			log.Printf("watcher stop error: %v", err)
		}
	}
	if s.indexer != nil {
		s.indexer.Stop()
	}
	if s.engine != nil {
		if err := s.engine.Close(); err != nil {
			log.Printf("search index close error: %v", err)
		}
	}
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			log.Printf("store close error: %v", err)
		}
	}
	return nil
}

// initWatcher starts the file watcher and restores persisted watches.
func (s *Server) initWatcher() error {
	w, err := watcher.NewWatcher(s.store, s.indexer)
	if err != nil {
		return err
	}
	if err := w.Start(context.Background()); err != nil {
		return err
	}
	s.watcher = w

	watches, err := s.settingsStore.GetWatches()
	if err != nil {
		log.Printf("failed to load watches: %v", err)
		return nil
	}
	for _, watch := range watches {
		if _, err := w.AddWatch(watch.Path, watch.Recursive, watch.Patterns); err != nil {
			log.Printf("failed to restore watch for %s: %v", watch.Path, err)
		}
	}
	return nil
}
