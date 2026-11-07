package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestListDirs(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"b", "A", ".hidden", "A/nested"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "b"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	list := func(query string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		(&Handler{}).ListDirs(rec, httptest.NewRequest(http.MethodGet, "/api/v1/fs/dirs?"+query, nil))
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	names := func(out map[string]any) []string {
		var ns []string
		for _, d := range out["dirs"].([]any) {
			ns = append(ns, d.(map[string]any)["name"].(string))
		}
		return ns
	}

	code, out := list("path=" + url.QueryEscape(root))
	if code != http.StatusOK {
		t.Fatalf("status %d: %v", code, out)
	}
	if got := names(out); len(got) != 3 || got[0] != "A" || got[1] != "b" || got[2] != "link" {
		t.Fatalf("dirs = %v, want [A b link] (sorted, no file, no dotfolder, symlink followed)", got)
	}
	if out["parent"] != filepath.Dir(root) {
		t.Fatalf("parent = %v", out["parent"])
	}

	if _, out = list("hidden=1&path=" + url.QueryEscape(root)); len(names(out)) != 4 {
		t.Fatalf("hidden=1: dirs = %v", names(out))
	}
	if code, _ = list("path=relative/dir"); code != http.StatusBadRequest {
		t.Fatalf("relative path: status %d", code)
	}
	if code, _ = list("path=" + url.QueryEscape(filepath.Join(root, "file.txt"))); code != http.StatusBadRequest {
		t.Fatalf("file: status %d", code)
	}
	if code, out = list(""); code != http.StatusOK || out["path"] == "" {
		t.Fatalf("home: status %d, %v", code, out)
	}
}
