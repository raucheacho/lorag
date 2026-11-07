package rest

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListDirs lists the subfolders of a folder on the machine running Lorag, for
// the Studio's folder picker: a browser cannot hand a folder's path to a page.
// GET /fs/dirs?path=<absolute path, ~ or empty for the home folder>&hidden=1
func (h *Handler) ListDirs(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	p := strings.TrimSpace(r.URL.Query().Get("path"))
	switch {
	case p == "" || p == "~":
		p = home
	case strings.HasPrefix(p, "~/"):
		p = filepath.Join(home, p[2:])
	}
	if !filepath.IsAbs(p) {
		respondError(w, http.StatusBadRequest, "path must be absolute")
		return
	}
	p = filepath.Clean(p)

	entries, err := os.ReadDir(p)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	hidden := r.URL.Query().Get("hidden") == "1"
	dirs := []dirEntry{}
	for _, e := range entries {
		name := e.Name()
		if !hidden && strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(p, name)
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(full); err == nil && fi.IsDir() {
				isDir = true
			}
		}
		if isDir {
			dirs = append(dirs, dirEntry{Name: name, Path: full})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) })

	parent := filepath.Dir(p)
	if parent == p {
		parent = ""
	}
	shortcuts := []dirEntry{}
	if home != "" {
		shortcuts = append(shortcuts, dirEntry{Name: "Home", Path: home})
		for _, name := range []string{"Documents", "Desktop", "Downloads"} {
			full := filepath.Join(home, name)
			if fi, err := os.Stat(full); err == nil && fi.IsDir() {
				shortcuts = append(shortcuts, dirEntry{Name: name, Path: full})
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"path":      p,
		"parent":    parent,
		"dirs":      dirs,
		"shortcuts": shortcuts,
	})
}
