package rest

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/lorag/lorag/internal/parsers"
)

// DocumentFile serves the original file of an indexed document, with range
// requests so audio and video can be opened at a given minute. Only paths
// recorded in the store are served.
func (h *Handler) DocumentFile(w http.ResponseWriter, r *http.Request) {
	doc, err := h.Store.GetDocument(chi.URLParam(r, "id"))
	if err != nil || doc == nil {
		respondError(w, http.StatusNotFound, "document not found")
		return
	}
	f, err := os.Open(doc.Path)
	if err != nil {
		respondError(w, http.StatusNotFound, "file no longer on disk: "+doc.Path)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		respondError(w, http.StatusNotFound, "not a file")
		return
	}
	ctype := mime.TypeByExtension(strings.ToLower(filepath.Ext(doc.Path)))
	if ctype == "" || parsers.KindOf(doc.Extension) == parsers.KindCode || strings.HasPrefix(ctype, "text/html") {
		// Shown as text, never rendered: an indexed HTML file must not run
		// scripts in the Studio's origin.
		ctype = "text/plain; charset=utf-8"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", doc.Filename))
	http.ServeContent(w, r, doc.Filename, info.ModTime(), f)
}

var unsafeName = regexp.MustCompile(`[^\w.\- ]+`)

// maxUpload bounds one uploaded file.
const maxUpload = 4 << 30

// Upload stores dropped files under <data>/imports/<date>/ and indexes them
// in the background; progress shows up in /ingestion.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		respondError(w, http.StatusBadRequest, "expected multipart/form-data")
		return
	}
	dir := filepath.Join(h.DataDir, "imports", time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type saved struct {
		Name   string `json:"name"`
		Path   string `json:"path"`
		Size   int64  `json:"size"`
		TaskID string `json:"task_id,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	var out []saved
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if part.FileName() == "" {
			continue
		}
		name := strings.TrimSpace(unsafeName.ReplaceAllString(filepath.Base(part.FileName()), "_"))
		if name == "" || name == "." || name == ".." {
			name = "upload"
		}
		s := saved{Name: name}
		if parsers.KindOf(filepath.Ext(name)) == "" {
			s.Error = "unsupported file type"
			out = append(out, s)
			continue
		}
		path := uniquePath(filepath.Join(dir, name))
		n, err := writeFile(path, io.LimitReader(part, maxUpload+1))
		switch {
		case err != nil:
			s.Error = err.Error()
		case n > maxUpload:
			os.Remove(path)
			s.Error = "file larger than 4 GiB"
		default:
			s.Path, s.Size = path, n
			if task, err := h.Indexer.SubmitTask(context.Background(), path); err == nil {
				s.TaskID = task.ID
			} else {
				go h.Indexer.IndexFile(context.Background(), path)
			}
		}
		out = append(out, s)
	}
	respondJSON(w, http.StatusOK, map[string]any{"files": out})
}

func writeFile(path string, r io.Reader) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
	}
	return n, err
}

// uniquePath appends " (2)", " (3)"… when the name is taken.
func uniquePath(p string) string {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(c); os.IsNotExist(err) {
			return c
		}
	}
}
