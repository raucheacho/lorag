package rest

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/lorag/lorag/ui"
)

// NewRouter creates the HTTP router: REST API under /api/v1, the Studio
// (embedded single-page app) everywhere else.
func NewRouter(handler *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(120 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{"Mcp-Session-Id"},
		MaxAge:         300,
	}))

	r.Get("/health", handler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/stats", handler.Stats)
		r.Get("/overview", handler.Overview)
		r.Get("/embedder", handler.EmbedderStatus)

		r.Post("/search", handler.Search)

		r.Route("/index", func(r chi.Router) {
			r.Post("/file", handler.IndexFile)
			r.Post("/directory", handler.IndexDirectory)
			r.Post("/rebuild", handler.RebuildIndex)
			r.Post("/compact", handler.CompactIndex)
			r.Post("/save", handler.SaveIndex)
		})

		r.Get("/ingestion", handler.Ingestion)
		r.Post("/upload", handler.Upload)
		r.Route("/queue", func(r chi.Router) {
			r.Get("/stats", handler.GetQueueStats)
			r.Get("/tasks/{id}", handler.GetTaskStatus)
		})

		r.Route("/activity", func(r chi.Router) {
			r.Get("/", handler.ListActivity)
			r.Get("/{id}", handler.GetActivity)
		})

		r.Route("/documents", func(r chi.Router) {
			r.Get("/", handler.ListDocuments)
			r.Post("/reindex-media", handler.ReindexMedia)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", handler.GetDocument)
				r.Delete("/", handler.DeleteDocument)
				r.Post("/reindex", handler.ReindexDocument)
				r.Get("/chunks", handler.GetDocumentChunks)
				r.Get("/file", handler.DocumentFile)
			})
		})

		r.Route("/watches", func(r chi.Router) {
			r.Get("/", handler.ListWatches)
			r.Post("/", handler.AddWatch)
			r.Delete("/{id}", handler.RemoveWatch)
		})

		r.Route("/memory", func(r chi.Router) {
			r.Get("/", handler.ListMemory)
			r.Post("/", handler.SaveMemory)
			r.Post("/recall", handler.RecallMemory)
			r.Get("/stats", handler.MemoryStats)
			r.Get("/events", handler.MemoryEvents)
			r.Get("/health", handler.MemoryHealth)
			r.Get("/export", handler.ExportMemory)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", handler.GetMemory)
				r.Patch("/", handler.UpdateMemory)
				r.Delete("/", handler.DeleteMemory)
				r.Post("/feedback", handler.FeedbackMemory)
			})
		})

		r.Get("/settings", handler.GetSettings)
		r.Put("/settings", handler.UpdateSettings)
		r.Get("/models/status", handler.GetModelStatus)
	})

	r.Get("/*", studioHandler())
	return r
}

// studioHandler serves the embedded Studio, falling back to index.html for
// client-side routes.
func studioHandler() http.HandlerFunc {
	staticFS, err := fs.Sub(ui.FS(), "dist")
	if err != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Studio not built: run `make ui`", http.StatusNotFound)
		}
	}
	files := http.FileServer(http.FS(staticFS))
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(staticFS, path); err != nil {
			if _, err := fs.Stat(staticFS, "index.html"); err != nil {
				http.Error(w, "Studio not built: run `make ui`", http.StatusNotFound)
				return
			}
			r.URL.Path = "/"
		} else if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	}
}
