package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lorag/lorag/internal/activity"
	"github.com/lorag/lorag/internal/core/engine"
	"github.com/lorag/lorag/internal/core/indexer"
	"github.com/lorag/lorag/internal/core/query"
	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/embeddings"
	"github.com/lorag/lorag/internal/memory"
	"github.com/lorag/lorag/internal/parsers"
	"github.com/lorag/lorag/internal/watcher"
)

// Deps are the services the REST API exposes.
type Deps struct {
	Store     *store.BoltStore
	Settings  *store.SettingsStore
	Indexer   *indexer.Indexer
	Query     *query.Engine
	Engine    *engine.Engine
	Memory    *memory.Service
	Embedder  embeddings.Provider
	Activity  *activity.Log
	Watcher   *watcher.Watcher
	Version   string
	DataDir   string
	StartedAt time.Time
	Addr      string
}

// Handler holds HTTP handlers
type Handler struct {
	Deps
}

// NewHandler creates a new handler
func NewHandler(d Deps) *Handler {
	return &Handler{Deps: d}
}

// Health returns health status
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"version":   h.Version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Stats returns document and index counts.
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.Indexer.GetStats()
	if h.Watcher != nil {
		stats["watches"] = len(h.Watcher.ListWatches())
	}
	respondJSON(w, http.StatusOK, stats)
}

// Overview aggregates everything the Studio home page shows.
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	docs, _ := h.Store.CountDocuments()
	chunks, _ := h.Store.CountChunks()
	settings, _ := h.Settings.GetAppSettings()
	idx := h.Engine.Stats()
	watches := 0
	if h.Watcher != nil {
		watches = len(h.Watcher.ListWatches())
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"version":      h.Version,
		"core_version": idx.CoreVersion,
		"uptime_s":     int(time.Since(h.StartedAt).Seconds()),
		"data_dir":     absPath(h.DataDir),
		"addr":         h.Addr,
		"documents":    docs,
		"chunks":       chunks,
		"max_chunks":   h.Indexer.GetStats()["maxChunks"],
		"db_bytes":     h.Store.Size(),
		"index":        idx,
		"in_sync":      idx.Chunks == chunks,
		"embedder": map[string]any{
			"provider": settings.EmbeddingProvider,
			"model":    settings.EmbeddingModel,
			"url":      settings.EmbeddingURL,
			"dim":      settings.EmbeddingDim,
		},
		"queue":    h.Indexer.GetQueueStats(),
		"watches":  watches,
		"activity": h.Activity.Summarize(),
	})
}

func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// EmbedderStatus checks that the embedding service answers.
func (h *Handler) EmbedderStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	type result struct {
		vec []float32
		err error
	}
	ch := make(chan result, 1)
	go func() {
		v, err := h.Embedder.Embed("lorag health check")
		ch <- result{v, err}
	}()
	select {
	case res := <-ch:
		out := map[string]any{
			"ok":         res.err == nil,
			"latency_ms": float64(time.Since(start).Microseconds()) / 1000,
			"expected":   h.Engine.Dim(),
		}
		if res.err != nil {
			out["error"] = res.err.Error()
		} else {
			out["dim"] = len(res.vec)
			if len(res.vec) != h.Engine.Dim() {
				out["ok"] = false
				out["error"] = "model returns " + strconv.Itoa(len(res.vec)) + " dimensions, index expects " + strconv.Itoa(h.Engine.Dim())
			}
		}
		respondJSON(w, http.StatusOK, out)
	case <-time.After(10 * time.Second):
		respondJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "timed out after 10s"})
	}
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query    string         `json:"query"`
	TopK     int            `json:"top_k"`
	Mode     string         `json:"mode,omitempty"` // hybrid (default), vector, keyword
	MinScore float32        `json:"min_score"`
	Context  int            `json:"context_chunks"`
	Filters  []query.Filter `json:"filters,omitempty"`
	Template string         `json:"template,omitempty"`
	Format   string         `json:"format,omitempty"` // "json" or "text"
}

// SearchResponse represents search results
type SearchResponse struct {
	*query.Response
	Total int    `json:"total"`
	Time  string `json:"time"`
}

// Search handles search requests
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		respondError(w, http.StatusBadRequest, "query is required")
		return
	}

	src := activity.SourceAPI
	if r.Header.Get("X-Lorag-Client") == "studio" {
		src = activity.SourceStudio
	}
	resp, err := h.Query.Run(r.Context(), query.Query{
		Text:     req.Query,
		TopK:     req.TopK,
		Mode:     query.ParseMode(req.Mode),
		MinScore: req.MinScore,
		Filters:  req.Filters,
		Source:   src,
		Client:   r.UserAgent(),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.Results = h.Query.WithContext(resp.Results, req.Context)

	if req.Format == "text" {
		tpl, ok := searchTemplates[req.Template]
		if !ok {
			tpl = searchTemplates["default"]
		}
		out := strings.ReplaceAll(tpl, "{{query}}", req.Query)
		out = strings.ReplaceAll(out, "{{count}}", strconv.Itoa(len(resp.Results)))
		out = strings.ReplaceAll(out, "{{results}}", query.FormatResults(resp.Results))
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(out))
		return
	}

	respondJSON(w, http.StatusOK, SearchResponse{
		Response: resp,
		Total:    len(resp.Results),
		Time:     time.Now().UTC().Format(time.RFC3339),
	})
}

var searchTemplates = map[string]string{
	"default": "Found {{count}} results for: {{query}}\n\n{{results}}\nBased on these documents, please answer: {{query}}",
	"concise": "Results: {{query}}\n\n{{results}}\nBrief answer:",
	"detailed": "Sources for: {{query}}\n\n{{results}}\nDetailed answer with citations:",
}

// IndexRequest represents an index request
type IndexRequest struct {
	Path      string   `json:"path"`
	Recursive bool     `json:"recursive"`
	Patterns  []string `json:"patterns,omitempty"`
	Async     bool     `json:"async,omitempty"`
}

// IndexResponse represents index result
type IndexResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Documents int    `json:"documents"`
	Chunks    int    `json:"chunks"`
	TaskID    string `json:"task_id,omitempty"`
}

// IndexFile handles file indexing (sync or async)
func (h *Handler) IndexFile(w http.ResponseWriter, r *http.Request) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}
	if req.Async {
		task, err := h.Indexer.SubmitTask(context.Background(), req.Path)
		if err != nil {
			respondError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		respondJSON(w, http.StatusAccepted, IndexResponse{Success: true, Message: "queued", TaskID: task.ID})
		return
	}
	doc, err := h.Indexer.IndexFile(r.Context(), req.Path)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, IndexResponse{Success: true, Message: "indexed", Documents: 1, Chunks: doc.ChunkCount})
}

// IndexDirectory handles directory indexing. With async, it returns
// immediately and progress shows up in /ingestion.
func (h *Handler) IndexDirectory(w http.ResponseWriter, r *http.Request) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}
	if info, err := os.Stat(req.Path); err != nil || !info.IsDir() {
		respondError(w, http.StatusBadRequest, "not a directory: "+req.Path)
		return
	}
	if req.Async {
		go h.Indexer.IndexDirectory(context.Background(), req.Path, req.Recursive)
		respondJSON(w, http.StatusAccepted, IndexResponse{Success: true, Message: "indexing started"})
		return
	}
	result, err := h.Indexer.IndexDirectory(r.Context(), req.Path, req.Recursive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, IndexResponse{
		Success:   result.Failed == 0,
		Message:   result.String(),
		Documents: result.Indexed,
		Chunks:    result.TotalChunks,
	})
}

// RebuildIndex repopulates the search index from the document store.
func (h *Handler) RebuildIndex(w http.ResponseWriter, r *http.Request) {
	go func() {
		h.Indexer.Rebuild(context.Background())
	}()
	respondJSON(w, http.StatusAccepted, map[string]string{"status": "rebuilding"})
}

// CompactIndex rewrites the index without deleted chunks.
func (h *Handler) CompactIndex(w http.ResponseWriter, r *http.Request) {
	if err := h.Engine.Compact(); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, h.Engine.Stats())
}

// SaveIndex checkpoints the index. Writes are durable as soon as they
// return; a checkpoint only trims the log so the next start is faster.
func (h *Handler) SaveIndex(w http.ResponseWriter, r *http.Request) {
	if err := h.Engine.Checkpoint(); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, h.Engine.Stats())
}

// Ingestion returns the queue state and recent indexing events.
func (h *Handler) Ingestion(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 100, 200)
	var running []map[string]any
	if q := h.Indexer.GetQueue(); q != nil {
		for _, t := range q.GetRunning() {
			running = append(running, map[string]any{"id": t.ID, "path": t.Path, "started_at": t.StartTime})
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"queue":   h.Indexer.GetQueueStats(),
		"running": running,
		"events":  h.Indexer.RecentEvents(limit),
	})
}

// GetTaskStatus returns the status of an async task
func (h *Handler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	q := h.Indexer.GetQueue()
	if q == nil {
		respondError(w, http.StatusNotFound, "async indexing disabled")
		return
	}
	if res, ok := q.GetCompleted(id); ok {
		out := map[string]any{"task_id": id, "status": "completed", "chunks": res.ChunksCount, "duration_ms": res.Duration.Milliseconds()}
		if res.Error != nil {
			out["status"], out["error"] = "failed", res.Error.Error()
		}
		if res.Document != nil {
			out["document_id"] = res.Document.ID
		}
		respondJSON(w, http.StatusOK, out)
		return
	}
	for _, t := range q.GetRunning() {
		if t.ID == id {
			respondJSON(w, http.StatusOK, map[string]any{"task_id": id, "status": "running", "path": t.Path})
			return
		}
	}
	respondJSON(w, http.StatusNotFound, map[string]any{"task_id": id, "status": "not_found"})
}

// GetQueueStats returns queue statistics
func (h *Handler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.Indexer.GetQueueStats())
}

// ListActivity returns recent searches, newest first.
func (h *Handler) ListActivity(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{
		"queries": h.Activity.List(activity.Filter{
			Source: activity.Source(r.URL.Query().Get("source")),
			Limit:  intParam(r, "limit", 200, 1000),
		}),
		"summary": h.Activity.Summarize(),
	})
}

// GetActivity returns one recorded search.
func (h *Handler) GetActivity(w http.ResponseWriter, r *http.Request) {
	q, ok := h.Activity.Get(chi.URLParam(r, "id"))
	if !ok {
		respondError(w, http.StatusNotFound, "query not found")
		return
	}
	respondJSON(w, http.StatusOK, q)
}

func intParam(r *http.Request, name string, def, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// ListDocuments handles document listing with search, filter, sort and paging.
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit := intParam(r, "limit", 50, 500)
	docs, total, err := h.Store.QueryDocuments(store.DocumentQuery{
		Offset:    max(offset, 0),
		Limit:     limit,
		Search:    q.Get("q"),
		Extension: q.Get("ext"),
		Sort:      q.Get("sort"),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"documents": docs,
		"total":     total,
		"offset":    offset,
		"limit":     limit,
	})
}

// GetDocument handles single document retrieval
func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	doc, err := h.Store.GetDocument(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusNotFound, "document not found")
		return
	}
	respondJSON(w, http.StatusOK, doc)
}

// DeleteDocument handles document deletion
func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Indexer.DeleteDocument(r.Context(), id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// ReindexDocument handles document reindexing
func (h *Handler) ReindexDocument(w http.ResponseWriter, r *http.Request) {
	doc, err := h.Indexer.ReindexDocument(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, doc)
}

// GetDocumentChunks returns a document's chunks in order, without embeddings.
func (h *Handler) GetDocumentChunks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	chunks, err := h.Store.GetChunksByDocument(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type chunkView struct {
		*store.Chunk
		Dim     int  `json:"dim"`
		Indexed bool `json:"indexed"`
	}
	out := make([]chunkView, len(chunks))
	for i, c := range chunks {
		indexed, _ := h.Engine.Get(c.ID)
		out[i] = chunkView{Chunk: c, Dim: len(c.Embedding), Indexed: indexed != nil}
		c.Embedding = nil
	}
	respondJSON(w, http.StatusOK, map[string]any{"chunks": out, "total": len(out)})
}

// WatchRequest represents a watch request
type WatchRequest struct {
	Path      string   `json:"path"`
	Recursive bool     `json:"recursive"`
	Patterns  []string `json:"patterns,omitempty"`
}

// AddWatch handles adding a new watch
func (h *Handler) AddWatch(w http.ResponseWriter, r *http.Request) {
	if h.Watcher == nil {
		respondError(w, http.StatusServiceUnavailable, "watcher not available")
		return
	}
	var req WatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}
	watch, err := h.Watcher.AddWatch(req.Path, req.Recursive, req.Patterns)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	watches, _ := h.Settings.GetWatches()
	watches = append(watches, store.WatchEntry{Path: watch.Path, Recursive: req.Recursive, Patterns: req.Patterns})
	h.Settings.SaveWatches(watches)
	respondJSON(w, http.StatusCreated, watch)
}

// RemoveWatch handles removing a watch
func (h *Handler) RemoveWatch(w http.ResponseWriter, r *http.Request) {
	if h.Watcher == nil {
		respondError(w, http.StatusServiceUnavailable, "watcher not available")
		return
	}
	id := chi.URLParam(r, "id")
	var path string
	for _, wt := range h.Watcher.ListWatches() {
		if wt.ID == id {
			path = wt.Path
		}
	}
	if err := h.Watcher.RemoveWatch(id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	watches, _ := h.Settings.GetWatches()
	kept := watches[:0]
	for _, wt := range watches {
		if wt.Path != path {
			kept = append(kept, wt)
		}
	}
	h.Settings.SaveWatches(kept)
	respondJSON(w, http.StatusOK, map[string]string{"status": "removed", "id": id})
}

// ListWatches handles listing all watches
func (h *Handler) ListWatches(w http.ResponseWriter, r *http.Request) {
	if h.Watcher == nil {
		respondJSON(w, http.StatusOK, map[string]any{"watches": []any{}})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"watches": h.Watcher.ListWatches()})
}

// GetSettings returns current settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.Settings.GetAppSettings()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	respondJSON(w, http.StatusOK, settings)
}

// UpdateSettings merges the request body into the stored settings. Every
// field except default_template takes effect on restart.
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	current, err := h.Settings.GetAppSettings()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	before := *current
	if err := json.NewDecoder(r.Body).Decode(current); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := validateSettings(current); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.Settings.SaveAppSettings(current); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}
	after := *current
	before.DefaultTemplate, after.DefaultTemplate = "", ""
	respondJSON(w, http.StatusOK, map[string]any{
		"status":           "saved",
		"restart_required": !reflect.DeepEqual(before, after),
		"settings":         current,
	})
}

func validateSettings(s *store.AppSettings) error {
	switch {
	case s.EmbeddingDim <= 0 || s.EmbeddingDim > 65536:
		return errors.New("embedding_dim must be between 1 and 65536")
	case s.VectorMetric != "cosine" && s.VectorMetric != "l2" && s.VectorMetric != "dot":
		return errors.New("vector_metric must be cosine, l2 or dot")
	case s.VectorQuantized && s.VectorMetric != "cosine":
		return errors.New("int8 quantization requires the cosine metric")
	case s.VectorM < 2 || s.VectorM > 256:
		return errors.New("vector_m must be between 2 and 256")
	case s.VectorEf < s.VectorM:
		return errors.New("vector_ef must be at least vector_m")
	case s.VectorEfSearch < 1:
		return errors.New("vector_ef_search must be positive")
	case s.BM25K1 < 0 || s.BM25B < 0 || s.BM25B > 1:
		return errors.New("bm25_k1 must be >= 0 and bm25_b in [0,1]")
	case s.ChunkSize < 32 || s.ChunkOverlap < 0 || s.ChunkOverlap >= s.ChunkSize:
		return errors.New("chunk_size must be >= 32 and chunk_overlap smaller than chunk_size")
	case s.IndexWorkers < 0:
		return errors.New("index_workers must be >= 0")
	case s.MemoryMinSimilarity < 0 || s.MemoryMinSimilarity >= 1 || s.MemoryMinSimilarityText < 0 || s.MemoryMinSimilarityText >= 1:
		return errors.New("memory thresholds must be between 0 (automatic) and 1")
	}
	return nil
}

// ReindexMediaRequest represents a request to reindex media files
type ReindexMediaRequest struct {
	Types []string `json:"types"` // "audio", "video", "image"
}

var mediaExtensions = map[string][]string{
	"image": {".jpg", ".jpeg", ".png", ".gif", ".webp"},
	"audio": {".wav", ".mp3", ".m4a", ".ogg", ".flac"},
	"video": {".mp4", ".avi", ".mkv", ".mov", ".webm"},
}

// ReindexMedia reindexes all media files of the given types in the background.
func (h *Handler) ReindexMedia(w http.ResponseWriter, r *http.Request) {
	var req ReindexMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Types) == 0 {
		req.Types = []string{"audio", "video", "image"}
	}
	wanted := map[string]bool{}
	for _, t := range req.Types {
		for _, ext := range mediaExtensions[t] {
			wanted[ext] = true
		}
	}
	docs, _, err := h.Store.QueryDocuments(store.DocumentQuery{})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	var ids []string
	for _, d := range docs {
		if wanted[strings.ToLower(filepath.Ext(d.Path))] {
			ids = append(ids, d.ID)
		}
	}
	go func() {
		for _, id := range ids {
			h.Indexer.ReindexDocument(context.Background(), id)
		}
	}()
	respondJSON(w, http.StatusOK, map[string]any{"status": "reindexing_started", "count": len(ids), "types": req.Types})
}

// GetModelStatus returns the status of the optional media models.
func (h *Handler) GetModelStatus(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.Settings.GetAppSettings()
	homeDir, _ := os.UserHomeDir()
	manager := parsers.NewWhisperModelManager(filepath.Join(homeDir, ".lorag"))
	respondJSON(w, http.StatusOK, map[string]any{
		"ocr": map[string]any{"enabled": settings.OcrEnabled, "model": settings.OcrModel},
		"transcription": map[string]any{
			"enabled": settings.TranscriptionEnabled,
			"model":   settings.WhisperModel,
			"model_status": map[string]any{
				"tiny":  manager.GetStatusText(parsers.WhisperTiny),
				"base":  manager.GetStatusText(parsers.WhisperBase),
				"small": manager.GetStatusText(parsers.WhisperSmall),
			},
		},
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
