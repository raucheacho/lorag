// Package query runs searches: vector, keyword (BM25) or hybrid, fused with
// Reciprocal Rank Fusion inside the engine.
package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/lorag/lorag/internal/activity"
	"github.com/lorag/lorag/internal/core/engine"
	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/embeddings"
	vectixdb "github.com/raucheacho/vectixdb/bindings/go"
)

// Mode selects the retrieval channels.
type Mode string

const (
	ModeHybrid  Mode = "hybrid"
	ModeVector  Mode = "vector"
	ModeKeyword Mode = "keyword"
)

// ParseMode defaults to hybrid.
func ParseMode(s string) Mode {
	switch Mode(strings.ToLower(s)) {
	case ModeVector, "semantic":
		return ModeVector
	case ModeKeyword, "bm25", "text":
		return ModeKeyword
	default:
		return ModeHybrid
	}
}

// rrfK is the usual Reciprocal Rank Fusion constant.
const rrfK = 60

// Engine handles search queries
type Engine struct {
	store    *store.BoltStore
	index    *engine.Engine
	embedder embeddings.Provider
	log      *activity.Log
}

// Signal is the contribution of one retrieval channel to a result.
type Signal struct {
	Rank  int     `json:"rank"`  // 1-based rank within the channel
	Score float32 `json:"score"` // similarity (vector) or BM25 score (keyword)
}

// Result represents a search result
type Result struct {
	ChunkID    string  `json:"chunk_id"`
	DocumentID string  `json:"document_id"`
	Content    string  `json:"content"`
	Score      float32 `json:"score"`
	Vector     *Signal `json:"vector,omitempty"`
	Keyword    *Signal `json:"keyword,omitempty"`
	Source     *Source `json:"source"`
}

// Source contains source document info
type Source struct {
	Path      string `json:"path"`
	Filename  string `json:"filename"`
	Position  int    `json:"position"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
}

// Query represents a search query
type Query struct {
	Text     string   `json:"text"`
	TopK     int      `json:"top_k"`
	Mode     Mode     `json:"mode,omitempty"`
	Filters  []Filter `json:"filters,omitempty"`
	MinScore float32  `json:"min_score,omitempty"`

	// Attribution for the activity log.
	Source activity.Source `json:"-"`
	Client string          `json:"-"`
	Tool   string          `json:"-"`
}

// Filter represents a search filter
type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// Timings breaks down where a search spent its time, in milliseconds.
type Timings struct {
	Embed  float64 `json:"embed_ms"`
	Search float64 `json:"search_ms"` // vector, text, filter and fusion in the engine
	Total  float64 `json:"total_ms"`
}

// Response is the full output of a search.
type Response struct {
	QueryID    string   `json:"query_id"`
	Mode       Mode     `json:"mode"`
	Results    []Result `json:"results"`
	Candidates int      `json:"candidates"`
	Timings    Timings  `json:"timings"`
	Warnings   []string `json:"warnings,omitempty"`
}

// NewEngine creates a new query engine. log may be nil.
func NewEngine(st *store.BoltStore, index *engine.Engine, embedder embeddings.Provider, log *activity.Log) *Engine {
	return &Engine{store: st, index: index, embedder: embedder, log: log}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// Run executes a search and records it in the activity log.
func (e *Engine) Run(ctx context.Context, q Query) (*Response, error) {
	start := time.Now()
	resp, err := e.run(ctx, q)
	if e.log != nil {
		entry := activity.Query{
			Source: q.Source, Client: q.Client, Tool: q.Tool,
			Text: q.Text, Mode: string(ParseMode(string(q.Mode))), TopK: q.TopK,
			DurationMs: ms(time.Since(start)),
		}
		if err != nil {
			entry.Error = err.Error()
		} else {
			for _, r := range resp.Results {
				entry.Hits = append(entry.Hits, activity.Hit{
					ChunkID: r.ChunkID, DocumentID: r.DocumentID, Score: r.Score,
					Filename: r.Source.Filename, Path: r.Source.Path, Position: r.Source.Position,
				})
			}
		}
		recorded := e.log.Record(entry)
		if resp != nil {
			resp.QueryID = recorded.ID
		}
	}
	return resp, err
}

func (e *Engine) run(ctx context.Context, q Query) (*Response, error) {
	start := time.Now()
	if strings.TrimSpace(q.Text) == "" {
		return nil, fmt.Errorf("query text is required")
	}
	if q.TopK <= 0 {
		q.TopK = 10
	}
	if q.TopK > 200 {
		q.TopK = 200
	}
	mode := ParseMode(string(q.Mode))
	resp := &Response{Mode: mode, Results: []Result{}}

	native, rest := splitFilters(q.Filters)
	req := engine.Request{Filter: native, K: q.TopK}
	// Filters the engine cannot evaluate are applied afterwards: fetch more
	// so they still leave TopK results.
	if len(rest) > 0 {
		req.K = min(q.TopK*4, 800)
	}

	if mode != ModeKeyword {
		t := time.Now()
		vec, err := e.embedder.Embed(q.Text)
		resp.Timings.Embed = ms(time.Since(t))
		switch {
		case err == nil:
			req.Vector = vec
		case mode == ModeVector:
			return nil, fmt.Errorf("embed query: %w", err)
		default:
			resp.Warnings = append(resp.Warnings, "embedding unavailable, fell back to keyword search: "+err.Error())
			resp.Mode = ModeKeyword
		}
	}
	if resp.Mode != ModeVector {
		req.Text = q.Text
	}

	t := time.Now()
	hits, err := e.index.Search(req)
	resp.Timings.Search = ms(time.Since(t))
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	resp.Candidates = len(hits)

	docs := map[string]*store.Document{}
	for _, h := range hits {
		if len(resp.Results) >= q.TopK {
			break
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		r := e.result(resp.Mode, h)
		if q.MinScore > 0 && r.Score < q.MinScore {
			if resp.Mode == ModeHybrid {
				continue // fused order is not monotonic in the normalized score
			}
			break
		}
		if len(rest) > 0 {
			doc, ok := docs[h.DocumentID]
			if !ok {
				doc, _ = e.store.GetDocument(h.DocumentID)
				docs[h.DocumentID] = doc
			}
			if doc == nil || !matchesFilters(doc, rest) {
				continue
			}
		}
		resp.Results = append(resp.Results, r)
	}
	resp.Timings.Total = ms(time.Since(start))
	return resp, nil
}

// result converts an engine hit. Hybrid scores are reciprocal rank fusion
// normalized to [0,1] (1 = ranked first by both routes); single-route modes
// keep the route's own score.
func (e *Engine) result(mode Mode, h engine.Hit) Result {
	r := Result{
		ChunkID:    h.ID,
		DocumentID: h.DocumentID,
		Content:    h.Content,
		Source: &Source{
			Path:      h.Path,
			Filename:  h.Filename,
			Position:  h.Position,
			StartByte: h.StartByte,
			EndByte:   h.EndByte,
		},
	}
	if h.VectorRank > 0 {
		r.Vector = &Signal{Rank: h.VectorRank, Score: e.index.Similarity(h.VectorDistance)}
	}
	if h.TextRank > 0 {
		r.Keyword = &Signal{Rank: h.TextRank, Score: h.TextScore}
	}
	switch mode {
	case ModeHybrid:
		var s float64
		if r.Vector != nil {
			s += 1 / float64(rrfK+r.Vector.Rank)
		}
		if r.Keyword != nil {
			s += 1 / float64(rrfK+r.Keyword.Rank)
		}
		r.Score = float32(s / (2 / float64(rrfK+1)))
	case ModeVector:
		r.Score = r.Vector.Score
	default:
		r.Score = r.Keyword.Score
	}
	return r
}

// nativeFields are stored on every chunk, so equality filters on them run
// inside the engine.
var nativeFields = map[string]string{
	"path":        engine.FieldPath,
	"filename":    engine.FieldFilename,
	"extension":   engine.FieldExtension,
	"document_id": engine.FieldDocumentID,
}

// splitFilters returns the engine filter for what it can evaluate and the
// filters left to check against the document.
func splitFilters(filters []Filter) (*vectixdb.Filter, []Filter) {
	var native []*vectixdb.Filter
	var rest []Filter
	for _, f := range filters {
		field, ok := nativeFields[f.Field]
		if !ok {
			rest = append(rest, f)
			continue
		}
		switch f.Operator {
		case "eq", "=":
			native = append(native, vectixdb.Eq(field, vectixdb.Keyword(f.Value)))
		case "neq", "!=":
			native = append(native, vectixdb.Not(vectixdb.Eq(field, vectixdb.Keyword(f.Value))))
		default:
			rest = append(rest, f)
		}
	}
	switch len(native) {
	case 0:
		return nil, rest
	case 1:
		return native[0], rest
	}
	return vectixdb.And(native...), rest
}

// Search keeps the historical signature used by the REST and MCP layers.
func (e *Engine) Search(ctx context.Context, q Query) ([]Result, error) {
	resp, err := e.Run(ctx, q)
	if err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// WithContext replaces each result's content with the chunk and its
// neighbors (contextChunks on each side).
func (e *Engine) WithContext(results []Result, contextChunks int) []Result {
	if contextChunks <= 0 {
		return results
	}
	out := make([]Result, 0, len(results))
	for _, r := range results {
		chunks, err := e.store.GetChunksByDocument(r.DocumentID)
		if err != nil {
			out = append(out, r)
			continue
		}
		for i, c := range chunks {
			if c.ID != r.ChunkID {
				continue
			}
			lo, hi := max(i-contextChunks, 0), min(i+contextChunks+1, len(chunks))
			parts := make([]string, 0, hi-lo)
			for _, s := range chunks[lo:hi] {
				parts = append(parts, s.Content)
			}
			r.Content = strings.Join(parts, "\n\n---\n\n")
			break
		}
		out = append(out, r)
	}
	return out
}

// matchesFilters checks if a document matches all filters
func matchesFilters(doc *store.Document, filters []Filter) bool {
	for _, f := range filters {
		value := ""
		switch f.Field {
		case "path":
			value = doc.Path
		case "filename":
			value = doc.Filename
		case "extension":
			value = doc.Extension
		default:
			value = doc.Metadata[f.Field]
		}

		match := false
		switch f.Operator {
		case "eq", "=":
			match = value == f.Value
		case "neq", "!=":
			match = value != f.Value
		case "contains":
			match = strings.Contains(value, f.Value)
		case "startswith":
			match = strings.HasPrefix(value, f.Value)
		case "endswith":
			match = strings.HasSuffix(value, f.Value)
		}
		if !match {
			return false
		}
	}
	return true
}

// FormatResults renders results as plain text for LLM consumption.
func FormatResults(results []Result) string {
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "[%d] %s — chunk %d (score %.3f)\n%s\n\n", i+1, r.Source.Path, r.Source.Position, r.Score, r.Content)
	}
	return b.String()
}
