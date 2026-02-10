package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ProgressEvent represents a real-time progress event
type ProgressEvent struct {
	Type        string  `json:"type"`        // "started", "progress", "completed", "failed"
	TaskID      string  `json:"task_id"`
	Path        string  `json:"path"`
	Current     int     `json:"current"`
	Total       int     `json:"total"`
	Percentage  float64 `json:"percentage"`
	Speed       string  `json:"speed,omitempty"`
	ETA         string  `json:"eta,omitempty"`
	Chunks      int     `json:"chunks,omitempty"`
	Message     string  `json:"message,omitempty"`
	Timestamp   int64   `json:"timestamp"`
}

// ProgressStream handles SSE for indexing progress
func (h *Handler) ProgressStream(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a ticker for sending updates
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Create done channel
	done := r.Context().Done()

	// Initial stats
	lastStats := h.indexer.GetQueueStats()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			// Get current stats
			stats := h.indexer.GetQueueStats()

			// Only send if changed
			if hasStatsChanged(lastStats, stats) {
				lastStats = stats

				// Calculate progress
				event := buildProgressEvent(stats)

				// Send SSE
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "data: %s\n\n", data)

				// Flush
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}
	}
}

// hasStatsChanged checks if stats have meaningfully changed
func hasStatsChanged(old, new map[string]interface{}) bool {
	keys := []string{"running", "completed", "queue_depth"}
	for _, key := range keys {
		if old[key] != new[key] {
			return true
		}
	}
	return false
}

// buildProgressEvent builds a progress event from stats
func buildProgressEvent(stats map[string]interface{}) ProgressEvent {
	running, _ := stats["running"].(int)
	completed, _ := stats["completed"].(int)
	queueDepth, _ := stats["queue_depth"].(int)

	total := running + completed + queueDepth
	percentage := 0.0
	if total > 0 {
		percentage = float64(completed) / float64(total) * 100
	}

	return ProgressEvent{
		Type:       "progress",
		Current:    completed,
		Total:      total,
		Percentage: percentage,
		Timestamp:  time.Now().Unix(),
	}
}

// IndexWithProgress handles indexing with real-time progress updates
func (h *Handler) IndexWithProgress(w http.ResponseWriter, r *http.Request) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}

	// Start indexing in background
	go func() {
		_, _ = h.indexer.IndexDirectory(r.Context(), req.Path, req.Recursive)
	}()

	// Return immediately with accepted
	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"success": true,
		"message": "Indexing started with progress tracking",
		"status_endpoint": "/api/v1/progress",
	})
}

// SystemStats returns comprehensive system stats
func (h *Handler) SystemStats(w http.ResponseWriter, r *http.Request) {
	// Get indexer stats
	indexerStats := h.indexer.GetStats()

	// Get queue stats
	queueStats := h.indexer.GetQueueStats()

	// Combine
	stats := map[string]interface{}{
		"indexer": indexerStats,
		"queue":   queueStats,
		"timestamp": time.Now().Unix(),
	}

	respondJSON(w, http.StatusOK, stats)
}
