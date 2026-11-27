package embeddings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaShortensATextTooLongForTheModel(t *testing.T) {
	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		sizes = append(sizes, len([]rune(req.Prompt)))
		if len([]rune(req.Prompt)) > 1000 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"embedding":[0.1,0.2]}`))
	}))
	defer srv.Close()

	o := NewOllamaProvider(Config{URL: srv.URL, Model: "m"})
	vec, err := o.Embed(strings.Repeat("é", 4000))
	if err != nil || len(vec) != 2 {
		t.Fatalf("embed: %v %v", vec, err)
	}
	if want := []int{4000, 2000, 1000}; len(sizes) != 3 || sizes[0] != want[0] || sizes[2] != want[2] {
		t.Errorf("tries = %v, want %v", sizes, want)
	}

	// Another failure is not retried.
	fail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer fail.Close()
	if _, err := NewOllamaProvider(Config{URL: fail.URL, Model: "m"}).Embed(strings.Repeat("a", 4000)); err == nil {
		t.Error("expected an error")
	}
}
