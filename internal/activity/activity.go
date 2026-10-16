// Package activity keeps an in-memory log of the searches served by Lorag,
// so that what agents asked and what they got back can be inspected.
package activity

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Source identifies the interface a query came through.
type Source string

const (
	SourceMCP    Source = "mcp"
	SourceAPI    Source = "api"
	SourceStudio Source = "studio"
)

// Hit is a compact reference to one returned chunk.
type Hit struct {
	ChunkID    string  `json:"chunk_id"`
	DocumentID string  `json:"document_id"`
	Filename   string  `json:"filename"`
	Path       string  `json:"path"`
	Position   int     `json:"position"`
	Score      float32 `json:"score"`
}

// Query is one recorded search.
type Query struct {
	ID         string    `json:"id"`
	Time       time.Time `json:"time"`
	Source     Source    `json:"source"`
	Client     string    `json:"client,omitempty"`
	Tool       string    `json:"tool,omitempty"`
	Text       string    `json:"query"`
	Mode       string    `json:"mode"`
	TopK       int       `json:"top_k"`
	DurationMs float64   `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
	Hits       []Hit     `json:"hits"`
}

// Log is a fixed-size ring buffer of queries. Safe for concurrent use.
type Log struct {
	mu    sync.RWMutex
	items []Query
	next  int
	full  bool
	seq   atomic.Uint64
}

// NewLog keeps the last size queries.
func NewLog(size int) *Log {
	if size <= 0 {
		size = 1000
	}
	return &Log{items: make([]Query, size)}
}

// Record stores q, assigning its ID and timestamp when missing.
func (l *Log) Record(q Query) Query {
	if q.ID == "" {
		q.ID = "q_" + strconv.FormatUint(l.seq.Add(1), 10)
	}
	if q.Time.IsZero() {
		q.Time = time.Now().UTC()
	}
	l.mu.Lock()
	l.items[l.next] = q
	l.next = (l.next + 1) % len(l.items)
	if l.next == 0 {
		l.full = true
	}
	l.mu.Unlock()
	return q
}

// Filter selects queries from List.
type Filter struct {
	Source Source
	Limit  int
}

// List returns matching queries, newest first.
func (l *Log) List(f Filter) []Query {
	l.mu.RLock()
	defer l.mu.RUnlock()
	n := l.next
	if l.full {
		n = len(l.items)
	}
	out := make([]Query, 0, min(n, max(f.Limit, 0)))
	for i := 0; i < n; i++ {
		idx := (l.next - 1 - i + len(l.items)) % len(l.items)
		q := l.items[idx]
		if f.Source != "" && q.Source != f.Source {
			continue
		}
		out = append(out, q)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out
}

// Get returns the query with the given ID, if still in the buffer.
func (l *Log) Get(id string) (Query, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, q := range l.items {
		if q.ID == id {
			return q, true
		}
	}
	return Query{}, false
}

// Summary aggregates the buffer for dashboards.
type Summary struct {
	Total   int            `json:"total"`
	BySrc   map[Source]int `json:"by_source"`
	P50Ms   float64        `json:"p50_ms"`
	P95Ms   float64        `json:"p95_ms"`
	Errors  int            `json:"errors"`
	Empty   int            `json:"empty"`
	Clients map[string]int `json:"clients"`
}

// Summarize computes counts and latency percentiles over the buffer.
func (l *Log) Summarize() Summary {
	qs := l.List(Filter{})
	s := Summary{Total: len(qs), BySrc: map[Source]int{}, Clients: map[string]int{}}
	lat := make([]float64, 0, len(qs))
	for _, q := range qs {
		s.BySrc[q.Source]++
		if q.Client != "" {
			s.Clients[q.Client]++
		}
		if q.Error != "" {
			s.Errors++
		} else if len(q.Hits) == 0 {
			s.Empty++
		}
		lat = append(lat, q.DurationMs)
	}
	s.P50Ms = percentile(lat, 0.50)
	s.P95Ms = percentile(lat, 0.95)
	return s
}

func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	// insertion sort is fine for a 1000-entry buffer
	for i := 1; i < len(cp); i++ {
		for j := i; j > 0 && cp[j] < cp[j-1]; j-- {
			cp[j], cp[j-1] = cp[j-1], cp[j]
		}
	}
	return cp[int(p*float64(len(cp)-1))]
}
