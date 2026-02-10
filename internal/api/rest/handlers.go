package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lorag/lorag/internal/core/indexer"
	"github.com/lorag/lorag/internal/core/query"
	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/parsers"
	"github.com/lorag/lorag/internal/watcher"
)

// Handler holds HTTP handlers
type Handler struct {
	store         *store.BoltStore
	settingsStore *store.SettingsStore
	indexer       *indexer.Indexer
	query         *query.Engine
	watcher       *watcher.Watcher
}

// NewHandler creates a new handler
func NewHandler(
	st *store.BoltStore,
	settings *store.SettingsStore,
	idx *indexer.Indexer,
	qe *query.Engine,
	w *watcher.Watcher,
) *Handler {
	return &Handler{
		store:         st,
		settingsStore: settings,
		indexer:       idx,
		query:         qe,
		watcher:       w,
	}
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// Health returns health status
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Version:   "0.1.0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Stats returns system statistics
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.indexer.GetStats()

	if h.watcher != nil {
		stats["watches"] = len(h.watcher.ListWatches())
	}

	respondJSON(w, http.StatusOK, stats)
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query     string  `json:"query"`
	TopK      int     `json:"top_k"`
	MinScore  float32 `json:"min_score"`
	Context   int     `json:"context_chunks"`
	Template  string  `json:"template,omitempty"`
	Format    string  `json:"format,omitempty"` // "json" or "text"
}

// SearchResponse represents search results
type SearchResponse struct {
	Results []query.Result `json:"results"`
	Total   int            `json:"total"`
	Time    string         `json:"time"`
}

// defaultTemplates for text formatting
var searchTemplates = map[string]string{
	"default": `Found {{count}} results for: {{query}}

{{results}}

Based on these documents, please answer: {{query}}`,
	"concise": `Results: {{query}}

{{results}}

Brief answer:`,
	"detailed": `Sources for: {{query}}

{{results}}

Detailed answer with citations:`,
}

// Search handles search requests
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		respondError(w, http.StatusBadRequest, "query is required")
		return
	}

	if req.TopK <= 0 {
		req.TopK = 10
	}

	q := query.Query{
		Text:     req.Query,
		TopK:     req.TopK,
		MinScore: req.MinScore,
	}

	var results []query.Result
	var err error

	if req.Context > 0 {
		results, err = h.query.SearchWithContext(r.Context(), q, req.Context)
	} else {
		results, err = h.query.Search(r.Context(), q)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// If text format requested with template, return formatted text
	if req.Format == "text" && req.Template != "" {
		template, ok := searchTemplates[req.Template]
		if !ok {
			template = searchTemplates["default"]
		}

		var resultsText strings.Builder
		for i, r := range results {
			resultsText.WriteString(fmt.Sprintf("[%d] %s (score: %.3f)\n%s\n\n", 
				i+1, r.Source.Filename, r.Score, r.Content))
		}

		output := strings.ReplaceAll(template, "{{query}}", req.Query)
		output = strings.ReplaceAll(output, "{{count}}", fmt.Sprintf("%d", len(results)))
		output = strings.ReplaceAll(output, "{{results}}", resultsText.String())

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(output))
		return
	}

	respondJSON(w, http.StatusOK, SearchResponse{
		Results: results,
		Total:   len(results),
		Time:    time.Now().UTC().Format(time.RFC3339),
	})
}

// IndexRequest represents an index request
type IndexRequest struct {
	Path      string   `json:"path"`
	Recursive bool     `json:"recursive"`
	Patterns  []string `json:"patterns,omitempty"`
	Async     bool     `json:"async,omitempty"` // Submit to queue, don't wait
}

// IndexResponse represents index result
type IndexResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Documents int    `json:"documents"`
	Chunks    int    `json:"chunks"`
	TaskID    string `json:"task_id,omitempty"` // Set if async
}

// TaskStatusResponse represents task status
type TaskStatusResponse struct {
	TaskID    string                 `json:"task_id"`
	Status    string                 `json:"status"` // pending, running, completed, failed
	Path      string                 `json:"path"`
	Progress  map[string]interface{} `json:"progress,omitempty"`
	Result    *TaskResult            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	QueueStats map[string]interface{} `json:"queue_stats"`
}

// TaskResult represents async task result
type TaskResult struct {
	DocumentID  string        `json:"document_id,omitempty"`
	Chunks      int           `json:"chunks"`
	Duration    time.Duration `json:"duration"`
	CompletedAt time.Time     `json:"completed_at"`
}

// IndexFile handles file indexing (sync or async)
func (h *Handler) IndexFile(w http.ResponseWriter, r *http.Request) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	// If async mode, submit to queue
	if req.Async {
		task, err := h.indexer.SubmitTask(r.Context(), req.Path)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusAccepted, IndexResponse{
			Success: true,
			Message: "task submitted to queue",
			TaskID:  task.ID,
		})
		return
	}

	// Synchronous mode
	doc, err := h.indexer.IndexFile(r.Context(), req.Path)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, IndexResponse{
		Success:   true,
		Message:   "indexed successfully",
		Documents: 1,
		Chunks:    doc.ChunkCount,
	})
}

// IndexDirectory handles directory indexing
func (h *Handler) IndexDirectory(w http.ResponseWriter, r *http.Request) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	result, err := h.indexer.IndexDirectory(r.Context(), req.Path, req.Recursive)
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

// GetTaskStatus returns the status of an async task
func (h *Handler) GetTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if taskID == "" {
		respondError(w, http.StatusBadRequest, "task ID required")
		return
	}

	queueStats := h.indexer.GetQueueStats()

	// Check if task is completed
	if result, ok := h.indexer.GetQueue().GetCompleted(taskID); ok {
		status := "completed"
		if result.Error != nil {
			status = "failed"
		}

		var taskResult *TaskResult
		if result.Document != nil {
			taskResult = &TaskResult{
				DocumentID:  result.Document.ID,
				Chunks:      result.ChunksCount,
				Duration:    result.Duration,
				CompletedAt: time.Now(),
			}
		}

		resp := TaskStatusResponse{
			TaskID:     taskID,
			Status:     status,
			Path:       result.TaskID, // We don't store path in result, using taskID as placeholder
			Result:     taskResult,
			QueueStats: queueStats,
		}
		if result.Error != nil {
			resp.Error = result.Error.Error()
		}

		respondJSON(w, http.StatusOK, resp)
		return
	}

	// Check if task is running
	running := h.indexer.GetQueue().GetRunning()
	for _, task := range running {
		if task.ID == taskID {
			respondJSON(w, http.StatusOK, TaskStatusResponse{
				TaskID:     taskID,
				Status:     "running",
				Path:       task.Path,
				Progress:   map[string]interface{}{"start_time": task.StartTime},
				QueueStats: queueStats,
			})
			return
		}
	}

	// Task not found
	respondJSON(w, http.StatusNotFound, TaskStatusResponse{
		TaskID:     taskID,
		Status:     "not_found",
		QueueStats: queueStats,
	})
}

// GetQueueStats returns queue statistics
func (h *Handler) GetQueueStats(w http.ResponseWriter, r *http.Request) {
	stats := h.indexer.GetQueueStats()
	respondJSON(w, http.StatusOK, stats)
}

// ListDocuments handles document listing
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	docs, err := h.store.ListDocuments(offset, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range docs {
		docs[i].Content = ""
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"documents": docs,
		"offset":    offset,
		"limit":     limit,
	})
}

// GetDocument handles single document retrieval
func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "document id is required")
		return
	}

	doc, err := h.store.GetDocument(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "document not found")
		return
	}

	respondJSON(w, http.StatusOK, doc)
}

// DeleteDocument handles document deletion
func (h *Handler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "document id is required")
		return
	}

	if err := h.indexer.DeleteDocument(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"id":     id,
	})
}

// ReindexDocument handles document reindexing
func (h *Handler) ReindexDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "document id is required")
		return
	}

	// Get document to find its path
	doc, err := h.store.GetDocument(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "document not found")
		return
	}

	// Delete old index
	if err := h.indexer.DeleteDocument(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete old index: "+err.Error())
		return
	}

	// Reindex the file
	_, err = h.indexer.IndexFile(r.Context(), doc.Path)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to reindex: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "reindexed",
		"id":     id,
	})
}

// GetDocumentChunks handles chunk retrieval for a document
func (h *Handler) GetDocumentChunks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "document id is required")
		return
	}

	chunks, err := h.store.GetChunksByDocument(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range chunks {
		chunks[i].Embedding = nil
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"chunks": chunks,
		"total":  len(chunks),
	})
}

// WatchRequest represents a watch request
type WatchRequest struct {
	Path      string   `json:"path"`
	Recursive bool     `json:"recursive"`
	Patterns  []string `json:"patterns,omitempty"`
}

// AddWatch handles adding a new watch
func (h *Handler) AddWatch(w http.ResponseWriter, r *http.Request) {
	if h.watcher == nil {
		respondError(w, http.StatusServiceUnavailable, "watcher not available")
		return
	}

	var req WatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	watch, err := h.watcher.AddWatch(req.Path, req.Recursive, req.Patterns)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Save to persistent settings
	watches, _ := h.settingsStore.GetWatches()
	watches = append(watches, store.WatchEntry{
		Path:      req.Path,
		Recursive: req.Recursive,
		Patterns:  req.Patterns,
	})
	h.settingsStore.SaveWatches(watches)

	respondJSON(w, http.StatusCreated, watch)
}

// RemoveWatch handles removing a watch
func (h *Handler) RemoveWatch(w http.ResponseWriter, r *http.Request) {
	if h.watcher == nil {
		respondError(w, http.StatusServiceUnavailable, "watcher not available")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "watch id is required")
		return
	}

	// Get the watch path before removing
	var watchPath string
	for _, w := range h.watcher.ListWatches() {
		if w.ID == id {
			watchPath = w.Path
			break
		}
	}

	if err := h.watcher.RemoveWatch(id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update persistent settings
	watches, _ := h.settingsStore.GetWatches()
	var newWatches []store.WatchEntry
	for _, w := range watches {
		if w.Path != watchPath {
			newWatches = append(newWatches, w)
		}
	}
	h.settingsStore.SaveWatches(newWatches)

	respondJSON(w, http.StatusOK, map[string]string{
		"status": "removed",
		"id":     id,
	})
}

// ListWatches handles listing all watches
func (h *Handler) ListWatches(w http.ResponseWriter, r *http.Request) {
	if h.watcher == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{"watches": []interface{}{}})
		return
	}

	watches := h.watcher.ListWatches()
	respondJSON(w, http.StatusOK, map[string]interface{}{"watches": watches})
}

// GetSettings returns current settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsStore.GetAppSettings()
	if err != nil {
		fmt.Printf("Error loading settings: %v\n", err)
		respondError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	fmt.Printf("Settings loaded: embedding_provider=%s, port=%d\n", settings.EmbeddingProvider, settings.Port)
	respondJSON(w, http.StatusOK, settings)
}

// UpdateSettings updates settings
func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var newSettings store.AppSettings
	if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
		fmt.Printf("Error decoding settings: %v\n", err)
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	fmt.Printf("Saving settings: provider=%s, port=%d\n", newSettings.EmbeddingProvider, newSettings.Port)
	
	// Save to persistent store
	if err := h.settingsStore.SaveAppSettings(&newSettings); err != nil {
		fmt.Printf("Error saving settings: %v\n", err)
		respondError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}
	
	fmt.Println("Settings saved successfully")
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "settings saved",
	})
}

// GetWatches returns saved watches
func (h *Handler) GetWatches(w http.ResponseWriter, r *http.Request) {
	watches, err := h.settingsStore.GetWatches()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load watches")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"watches": watches})
}

// SaveWatches saves watched folders
func (h *Handler) SaveWatches(w http.ResponseWriter, r *http.Request) {
	var watches []store.WatchEntry
	if err := json.NewDecoder(r.Body).Decode(&watches); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if err := h.settingsStore.SaveWatches(watches); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save watches")
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{
		"status": "watches saved",
	})
}

// ReindexMediaRequest represents a request to reindex media files
type ReindexMediaRequest struct {
	Types []string `json:"types"` // "audio", "video", "image"
}

// ReindexMedia reindexes all media files of specified types
func (h *Handler) ReindexMedia(w http.ResponseWriter, r *http.Request) {
	var req ReindexMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default to all media types if not specified
		req.Types = []string{"audio", "video", "image"}
	}
	
	// Get all documents
	docs, err := h.store.ListDocuments(0, 10000)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	
	// Filter media files
	var mediaPaths []string
	for _, doc := range docs {
		ext := strings.ToLower(filepath.Ext(doc.Path))
		
		isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp"
		isAudio := ext == ".wav" || ext == ".mp3" || ext == ".m4a" || ext == ".ogg" || ext == ".flac"
		isVideo := ext == ".mp4" || ext == ".avi" || ext == ".mkv" || ext == ".mov" || ext == ".webm"
		
		shouldReindex := false
		for _, t := range req.Types {
			switch t {
			case "image":
				if isImage {
					shouldReindex = true
				}
			case "audio":
				if isAudio {
					shouldReindex = true
				}
			case "video":
				if isVideo {
					shouldReindex = true
				}
			}
		}
		
		if shouldReindex {
			mediaPaths = append(mediaPaths, doc.Path)
		}
	}
	
	// Start reindexing in background
	go func() {
		for _, path := range mediaPaths {
			// Delete old index
			if doc, err := h.store.GetDocumentByPath(path); err == nil {
				h.indexer.DeleteDocument(r.Context(), doc.ID)
			}
			// Reindex
			h.indexer.IndexFile(r.Context(), path)
		}
	}()
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "reindexing_started",
		"count":     len(mediaPaths),
		"types":     req.Types,
	})
}

// GetModelStatus returns the status of AI models
func (h *Handler) GetModelStatus(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.settingsStore.GetAppSettings()
	
	// Get whisper model status
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".lorag")
	manager := parsers.NewWhisperModelManager(dataDir)
	
	whisperStatus := map[string]interface{}{
		"tiny":  manager.GetStatusText(parsers.WhisperTiny),
		"base":  manager.GetStatusText(parsers.WhisperBase),
		"small": manager.GetStatusText(parsers.WhisperSmall),
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ocr": map[string]interface{}{
			"enabled": settings.OcrEnabled,
			"model":   settings.OcrModel,
		},
		"transcription": map[string]interface{}{
			"enabled":      settings.TranscriptionEnabled,
			"model":        settings.WhisperModel,
			"model_status": whisperStatus,
		},
	})
}

// Helper functions

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{
		"error": message,
	})
}
