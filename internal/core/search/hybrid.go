package search

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/lorag/lorag/internal/core/query"
	"github.com/lorag/lorag/internal/core/store"
	"github.com/lorag/lorag/internal/core/vector"
	"github.com/lorag/lorag/internal/embeddings"
)

// HybridEngine combines BM25 and vector search
type HybridEngine struct {
	store     *store.BoltStore
	vectorIdx vector.Index
	embedder  embeddings.Provider
	bm25      *BM25
	alpha     float64 // Weight for vector search (1-alpha for BM25)
}

// Config holds hybrid search configuration
type Config struct {
	Alpha         float64 // Weight for vector (0-1), default 0.7
	BM25K1        float64 // BM25 parameter k1, default 1.5
	BM25B         float64 // BM25 parameter b, default 0.75
	VectorWeight  float64
	KeywordWeight float64
}

// DefaultConfig returns default hybrid search config
func DefaultConfig() Config {
	return Config{
		Alpha:         0.7,
		BM25K1:        1.5,
		BM25B:         0.75,
		VectorWeight:  0.7,
		KeywordWeight: 0.3,
	}
}

// NewHybridEngine creates a new hybrid search engine
func NewHybridEngine(st *store.BoltStore, vectorIdx vector.Index, embedder embeddings.Provider, cfg Config) *HybridEngine {
	return &HybridEngine{
		store:     st,
		vectorIdx: vectorIdx,
		embedder:  embedder,
		bm25:      NewBM25(cfg.BM25K1, cfg.BM25B),
		alpha:     cfg.Alpha,
	}
}

// Result represents a hybrid search result
type Result struct {
	ChunkID      string  `json:"chunk_id"`
	DocumentID   string  `json:"document_id"`
	Content      string  `json:"content"`
	Score        float64 `json:"score"`
	VectorScore  float64 `json:"vector_score"`
	BM25Score    float64 `json:"bm25_score"`
	Source       *query.Source `json:"source"`
}

// Search performs hybrid search combining BM25 and vector similarity
func (h *HybridEngine) Search(ctx context.Context, q query.Query) ([]Result, error) {
	if q.TopK <= 0 {
		q.TopK = 10
	}

	// Get vector results
	queryEmbedding, err := h.embedder.Embed(q.Text)
	if err != nil {
		return nil, err
	}

	vectorResults, err := h.vectorIdx.Search(queryEmbedding, q.TopK*3)
	if err != nil {
		return nil, err
	}

	// Get BM25 results
	bm25Results := h.bm25Search(q.Text, q.TopK*3)

	// Merge and rank results
	merged := h.mergeResults(vectorResults, bm25Results)

	// Apply filters and build final results
	var results []Result
	for _, r := range merged {
		if len(results) >= q.TopK {
			break
		}

		chunk, err := h.store.GetChunk(r.ID)
		if err != nil {
			continue
		}

		doc, err := h.store.GetDocument(chunk.DocumentID)
		if err != nil {
			continue
		}

		// Apply filters
		if !h.matchesFilters(doc, q.Filters) {
			continue
		}

		result := Result{
			ChunkID:     chunk.ID,
			DocumentID:  chunk.DocumentID,
			Content:     chunk.Content,
			Score:       r.CombinedScore,
			VectorScore: r.VectorScore,
			BM25Score:   r.BM25Score,
			Source: &query.Source{
				Path:      doc.Path,
				Filename:  doc.Filename,
				Position:  chunk.Position,
				StartByte: chunk.StartByte,
				EndByte:   chunk.EndByte,
			},
		}
		results = append(results, result)
	}

	return results, nil
}

// ScoredResult holds intermediate scoring
type ScoredResult struct {
	ID            string
	VectorScore   float64
	BM25Score     float64
	CombinedScore float64
}

func (h *HybridEngine) mergeResults(vectorResults []vector.SearchResult, bm25Results []bm25Result) []ScoredResult {
	// Normalize scores to 0-1 range
	maxVectorDist := float64(0)
	for _, r := range vectorResults {
		if float64(r.Distance) > maxVectorDist {
			maxVectorDist = float64(r.Distance)
		}
	}
	if maxVectorDist == 0 {
		maxVectorDist = 1
	}

	maxBM25 := float64(0)
	for _, r := range bm25Results {
		if r.Score > maxBM25 {
			maxBM25 = r.Score
		}
	}
	if maxBM25 == 0 {
		maxBM25 = 1
	}

	// Combine scores
	scoreMap := make(map[string]*ScoredResult)

	// Add vector scores
	for _, r := range vectorResults {
		// Convert distance to similarity (closer = higher score)
		similarity := 1.0 - (float64(r.Distance) / maxVectorDist)
		scoreMap[r.ID] = &ScoredResult{
			ID:          r.ID,
			VectorScore: similarity,
		}
	}

	// Add BM25 scores
	for _, r := range bm25Results {
		normalized := r.Score / maxBM25
		if existing, ok := scoreMap[r.ID]; ok {
			existing.BM25Score = normalized
		} else {
			scoreMap[r.ID] = &ScoredResult{
				ID:        r.ID,
				BM25Score: normalized,
			}
		}
	}

	// Calculate combined scores
	var results []ScoredResult
	for _, r := range scoreMap {
		// Hybrid score: weighted combination
		// If only one score present, use it with reduced weight
		if r.VectorScore == 0 {
			r.CombinedScore = r.BM25Score * (1 - h.alpha) * 0.5
		} else if r.BM25Score == 0 {
			r.CombinedScore = r.VectorScore * h.alpha * 0.5
		} else {
			r.CombinedScore = r.VectorScore*h.alpha + r.BM25Score*(1-h.alpha)
		}
		results = append(results, *r)
	}

	// Sort by combined score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})

	return results
}

type bm25Result struct {
	ID    string
	Score float64
}

func (h *HybridEngine) bm25Search(query string, k int) []bm25Result {
	// Get all chunks (this is a simplified version - in production would use inverted index)
	// For now, search through recent documents only
	docs, _ := h.store.ListDocuments(0, 1000)
	
	var allChunks []*store.Chunk
	for _, doc := range docs {
		if doc.IsDeleted {
			continue
		}
		chunks, _ := h.store.GetChunksByDocument(doc.ID)
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	// Calculate average document length
	avgLen := 0
	for _, c := range allChunks {
		avgLen += len(c.Content)
	}
	avgLen = avgLen / len(allChunks)

	// Tokenize query
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}

	// Score each chunk
	var results []bm25Result
	for _, chunk := range allChunks {
		score := h.bm25.Score(queryTerms, chunk.Content, avgLen)
		if score > 0 {
			results = append(results, bm25Result{
				ID:    chunk.ID,
				Score: score,
			})
		}
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > k {
		results = results[:k]
	}

	return results
}

func (h *HybridEngine) matchesFilters(doc *store.Document, filters []query.Filter) bool {
	if len(filters) == 0 {
		return true
	}

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
			if v, ok := doc.Metadata[f.Field]; ok {
				value = v
			}
		}

		match := false
		switch f.Operator {
		case "eq", "=":
			match = value == f.Value
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

// BM25 implements the BM25 ranking function
type BM25 struct {
	k1 float64
	b  float64
}

// NewBM25 creates a new BM25 scorer
func NewBM25(k1, b float64) *BM25 {
	return &BM25{
		k1: k1,
		b:  b,
	}
}

// Score calculates BM25 score for a document
func (b *BM25) Score(queryTerms []string, document string, avgDocLen int) float64 {
	if avgDocLen == 0 {
		avgDocLen = 1
	}

	score := 0.0
	docLen := len(document)
	docTerms := tokenize(document)
	termFreqs := make(map[string]int)
	for _, t := range docTerms {
		termFreqs[t]++
	}

	for _, term := range queryTerms {
		freq := termFreqs[term]
		if freq == 0 {
			continue
		}

		// Simplified IDF (in a real system, use precomputed corpus statistics)
		idf := math.Log(1 + (float64(avgDocLen*10)-float64(freq)+0.5)/(float64(freq)+0.5))

		// BM25 term scoring
		numerator := float64(freq) * (b.k1 + 1)
		denominator := float64(freq) + b.k1*(1-b.b+b.b*float64(docLen)/float64(avgDocLen))
		score += idf * numerator / denominator
	}

	return score
}

// Simple tokenizer
func tokenize(text string) []string {
	// Convert to lowercase and split on non-alphanumeric
	text = strings.ToLower(text)
	var terms []string
	var current strings.Builder

	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			terms = append(terms, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		terms = append(terms, current.String())
	}

	return terms
}