package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lorag/lorag/internal/activity"
	"github.com/lorag/lorag/internal/memory"
)

func (h *Handler) memoryError(w http.ResponseWriter, err error) {
	if errors.Is(err, memory.ErrNotFound) {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondError(w, http.StatusBadRequest, err.Error())
}

// ListMemory returns entries filtered by status, space or a query.
func (h *Handler) ListMemory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	entries, err := h.Memory.List(memory.ListRequest{
		Kind:   q.Get("kind"),
		Status: memory.Status(q.Get("status")),
		Space:  q.Get("space"),
		Query:  q.Get("q"),
		Limit:  intParam(r, "limit", 500, 10_000),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": len(entries)})
}

// SaveMemory records a solved problem.
func (h *Handler) SaveMemory(w http.ResponseWriter, r *http.Request) {
	var req memory.SaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if r.Header.Get("X-Lorag-Client") == "studio" {
		req.Author, req.Origin = "person", "studio"
	} else if req.Origin == "" {
		req.Origin = r.UserAgent()
	}
	res, err := h.Memory.Save(req)
	if err != nil {
		h.memoryError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, res)
}

// RecallMemory looks a problem up and logs the recall like a search.
func (h *Handler) RecallMemory(w http.ResponseWriter, r *http.Request) {
	var req memory.RecallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	src := activity.SourceAPI
	if r.Header.Get("X-Lorag-Client") == "studio" {
		src = activity.SourceStudio
	}
	req.Origin = string(src)
	res, err := h.Memory.RecallLogged(h.Activity, req, src, r.UserAgent())
	if err != nil {
		h.memoryError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, res)
}

// GetMemory returns an entry and its history.
func (h *Handler) GetMemory(w http.ResponseWriter, r *http.Request) {
	e, hist, err := h.Memory.Get(chi.URLParam(r, "id"))
	if err != nil {
		h.memoryError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"entry": e, "history": hist})
}

// FeedbackMemory records whether a recalled fix worked.
func (h *Handler) FeedbackMemory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Worked bool   `json:"worked"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	origin := r.UserAgent()
	if r.Header.Get("X-Lorag-Client") == "studio" {
		origin = "person"
	}
	e, err := h.Memory.Feedback(chi.URLParam(r, "id"), req.Worked, req.Note, origin)
	if err != nil {
		h.memoryError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, e)
}

// UpdateMemory applies a correction by the person.
func (h *Handler) UpdateMemory(w http.ResponseWriter, r *http.Request) {
	var ed memory.Edit
	if err := json.NewDecoder(r.Body).Decode(&ed); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	e, err := h.Memory.Update(chi.URLParam(r, "id"), ed)
	if err != nil {
		h.memoryError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, e)
}

// DeleteMemory removes an entry from recall; the journal keeps its history.
func (h *Handler) DeleteMemory(w http.ResponseWriter, r *http.Request) {
	if err := h.Memory.Delete(chi.URLParam(r, "id")); err != nil {
		h.memoryError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// MemoryStats returns counts, hit rate, tokens saved and trends.
func (h *Handler) MemoryStats(w http.ResponseWriter, r *http.Request) {
	st, err := h.Memory.Stats()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, struct {
		*memory.Stats
		Thresholds memory.Thresholds `json:"thresholds"`
	}{st, h.Memory.Thresholds()})
}

// MemoryEvents returns the journal, newest first.
func (h *Handler) MemoryEvents(w http.ResponseWriter, r *http.Request) {
	evs, err := h.Memory.Events(intParam(r, "limit", 200, 10_000))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"events": evs})
}

// MemoryHealth lists entries to recheck, obsolete, stale and duplicates.
func (h *Handler) MemoryHealth(w http.ResponseWriter, r *http.Request) {
	hl, err := h.Memory.Health()
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, hl)
}

// ExportMemory streams every entry as JSON Lines.
func (h *Handler) ExportMemory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", `attachment; filename="lorag-memory.jsonl"`)
	if err := h.Memory.Export(w); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
	}
}
