package rest

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/lorag/lorag/ui"
)

// NewRouter creates a new HTTP router
func NewRouter(handler *Handler) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", handler.Health)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Stats
		r.Get("/stats", handler.Stats)

		// Search
		r.Post("/search", handler.Search)

		// Indexing
		r.Route("/index", func(r chi.Router) {
			r.Post("/file", handler.IndexFile)
			r.Post("/directory", handler.IndexDirectory)
		})

		// Queue (async indexing)
		r.Route("/queue", func(r chi.Router) {
			r.Get("/stats", handler.GetQueueStats)
			r.Get("/tasks/{id}", handler.GetTaskStatus)
		})

		// Progress (SSE for real-time updates)
		r.Get("/progress", handler.ProgressStream)
		r.Get("/system", handler.SystemStats)

		// Documents
		r.Route("/documents", func(r chi.Router) {
			r.Get("/", handler.ListDocuments)
			r.Post("/reindex-media", handler.ReindexMedia)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", handler.GetDocument)
				r.Delete("/", handler.DeleteDocument)
				r.Post("/reindex", handler.ReindexDocument)
				r.Get("/chunks", handler.GetDocumentChunks)
			})
		})

		// Watches
		r.Route("/watches", func(r chi.Router) {
			r.Get("/", handler.ListWatches)
			r.Post("/", handler.AddWatch)
			r.Route("/{id}", func(r chi.Router) {
				r.Delete("/", handler.RemoveWatch)
			})
		})

		// Settings
		r.Get("/settings", handler.GetSettings)
		r.Put("/settings", handler.UpdateSettings)
		r.Get("/models/status", handler.GetModelStatus)
		r.Get("/watches/persisted", handler.GetWatches)
		r.Put("/watches/persisted", handler.SaveWatches)
	})

	// Static files (UI) - serve embedded files
	staticFS, err := fs.Sub(ui.FS(), "dist")
	if err != nil {
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("UI Error"))
		})
	} else {
		fileServer := http.FileServer(http.FS(staticFS))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path == "/" {
				path = "/index.html"
			}
			
			_, err := fs.Stat(staticFS, path[1:])
			if err != nil {
				r.URL.Path = "/"
			}
			
			fileServer.ServeHTTP(w, r)
		})
	}

	return r
}
